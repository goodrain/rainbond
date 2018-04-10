// Copyright (C) 2014-2018 Goodrain Co., Ltd.
// RAINBOND, Application Management Platform

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version. For any non-GPL usage of Rainbond,
// one or multiple Commercial Licenses authorized by Goodrain Co., Ltd.
// must be obtained first.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package service

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/cmd/node/option"
	api_model "github.com/goodrain/rainbond/pkg/api/model"
	"github.com/goodrain/rainbond/pkg/api/util"
	node_model "github.com/goodrain/rainbond/pkg/node/api/model"
	envoyv1 "github.com/goodrain/rainbond/pkg/node/core/envoy/v1"
	"github.com/goodrain/rainbond/pkg/node/core/k8s"
	"github.com/goodrain/rainbond/pkg/node/core/store"
	"github.com/pquerna/ffjson/ffjson"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/client-go/pkg/api/v1"
)

//DiscoverAction DiscoverAction
type DiscoverAction struct {
	conf    *option.Conf
	etcdCli *store.Client
}

//CreateDiscoverActionManager CreateDiscoverActionManager
func CreateDiscoverActionManager(conf *option.Conf) *DiscoverAction {
	return &DiscoverAction{
		conf:    conf,
		etcdCli: store.DefalutClient,
	}
}

//DiscoverService sds
func (d *DiscoverAction) DiscoverService(serviceInfo string) (*envoyv1.SDSHost, *util.APIHandleError) {
	mm := strings.Split(serviceInfo, "_")
	if len(mm) < 4 {
		return nil, util.CreateAPIHandleError(400, fmt.Errorf("service_name is not in good format"))
	}
	namespace := mm[0]
	serviceAlias := mm[1]
	destServiceAlias := mm[2]
	dPort := mm[3]

	labelname := fmt.Sprintf("name=%sService", destServiceAlias)
	endpoints, err := k8s.K8S.Core().Endpoints(namespace).List(metav1.ListOptions{LabelSelector: labelname})
	//logrus.Debugf("labelname is %s, endpoints is %v, items is %v", labelname, endpoints, endpoints.Items)
	if err != nil {
		return nil, util.CreateAPIHandleError(500, err)
	}
	services, err := k8s.K8S.Core().Services(namespace).List(metav1.ListOptions{LabelSelector: labelname})
	if err != nil {
		return nil, util.CreateAPIHandleError(500, err)
	}
	if len(endpoints.Items) == 0 {
		if destServiceAlias == serviceAlias {
			labelname := fmt.Sprintf("name=%sServiceOUT", destServiceAlias)
			var err error
			endpoints, err = k8s.K8S.Core().Endpoints(namespace).List(metav1.ListOptions{LabelSelector: labelname})
			if err != nil {
				return nil, util.CreateAPIHandleError(500, err)
			}
			if len(endpoints.Items) == 0 {
				logrus.Debugf("outer endpoints items length is 0, continue")
				return nil, util.CreateAPIHandleError(400, fmt.Errorf("outer have no endpoints"))
			}
			services, err = k8s.K8S.Core().Services(namespace).List(metav1.ListOptions{LabelSelector: labelname})
			if err != nil {
				return nil, util.CreateAPIHandleError(500, err)
			}
		} else {
			return nil, util.CreateAPIHandleError(400, fmt.Errorf("inner have no endpoints"))
		}
	}
	var sdsL []*envoyv1.DiscoverHost
	for key, item := range endpoints.Items {
		addressList := item.Subsets[0].Addresses
		if len(addressList) == 0 {
			addressList = item.Subsets[0].NotReadyAddresses
		}
		port := item.Subsets[0].Ports[0].Port
		if dPort != fmt.Sprintf("%d", port) {
			continue
		}
		toport := int(services.Items[key].Spec.Ports[0].Port)
		if serviceAlias == destServiceAlias {
			if originPort, ok := services.Items[key].Labels["origin_port"]; ok {
				origin, err := strconv.Atoi(originPort)
				if err != nil {
					return nil, util.CreateAPIHandleError(500, fmt.Errorf("have no origin_port"))
				}
				toport = origin
			}
		}
		for _, ip := range addressList {
			sdsP := &envoyv1.DiscoverHost{
				Address: ip.IP,
				Port:    toport,
			}
			sdsL = append(sdsL, sdsP)
		}
	}
	sds := &envoyv1.SDSHost{
		Hosts: sdsL,
	}
	return sds, nil
}

//DiscoverClusters cds discover
//create cluster by get depend app endpoints from plugin config
func (d *DiscoverAction) DiscoverClusters(
	tenantService,
	serviceCluster string) (*envoyv1.CDSCluter, *util.APIHandleError) {
	nn := strings.Split(tenantService, "_")
	if len(nn) != 3 {
		return nil, util.CreateAPIHandleError(400, fmt.Errorf("namesapces and service_alias not in good format"))
	}
	namespace := nn[0]
	pluginID := nn[1]
	serviceAlias := nn[2]
	var cds = &envoyv1.CDSCluter{}
	resources, err := d.ToolsGetRainbondResources(namespace, serviceAlias, pluginID)
	if err != nil {
		if strings.Contains(err.Error(), "is not exist") {
			return cds, nil
		}
		logrus.Warnf("in lds get env %s error: %v", namespace+serviceAlias+pluginID, err)
		return nil, util.CreateAPIHandleError(500, fmt.Errorf(
			"get env %s error: %v", namespace+serviceAlias+pluginID, err))
	}
	if resources.BaseServices != nil && len(resources.BaseServices) > 0 {
		clusters, err := d.upstreamClusters(serviceAlias, namespace, resources.BaseServices)
		if err != nil {
			return nil, err
		}
		cds.Clusters.Append(clusters)
	}
	return cds, nil
}

//upstreamClusters handle upstream app cluster
// handle kubernetes inner service
func (d *DiscoverAction) upstreamClusters(serviceAlias, namespace string, dependsServices []*api_model.BaseService) (cdsClusters envoyv1.Clusters, err *util.APIHandleError) {
	for _, destService := range dependsServices {
		destServiceAlias := destService.DependServiceAlias
		labelname := fmt.Sprintf("name=%sService", destServiceAlias)
		services, err := k8s.K8S.Core().Services(namespace).List(metav1.ListOptions{LabelSelector: labelname})
		if err != nil {
			return nil, util.CreateAPIHandleError(500, err)
		}
		if len(services.Items) == 0 {
			continue
		}
		for _, service := range services.Items {
			inner, ok := service.Labels["service_type"]
			port := service.Spec.Ports[0]
			//originPort := service.Labels["origin_port"]
			options := destService.Options
			if !ok || inner != "inner" {
				continue
			}
			circuits := d.ToolsGetRouterItem(destServiceAlias, node_model.LIMITS, options).(int)
			maxRequests := d.ToolsGetRouterItem(destServiceAlias, node_model.MaxRequests, options).(int)
			maxRetries := d.ToolsGetRouterItem(destServiceAlias, node_model.MaxRetries, options).(int)
			maxPendingRequests := d.ToolsGetRouterItem(destServiceAlias, node_model.MaxPendingRequests, options).(int)
			cb := &envoyv1.CircuitBreaker{
				Default: envoyv1.DefaultCBPriority{
					MaxConnections:     circuits,
					MaxPendingRequests: maxPendingRequests,
					MaxRequests:        maxRequests,
					MaxRetries:         maxRetries,
				},
			}
			pcds := &envoyv1.Cluster{
				Name:             fmt.Sprintf("%s_%s_%s_%v", namespace, serviceAlias, destServiceAlias, port.Port),
				Type:             "sds",
				ConnectTimeoutMs: 250,
				LbType:           "round_robin",
				ServiceName:      fmt.Sprintf("%s_%s_%s_%v", namespace, serviceAlias, destServiceAlias, port.Port),
				CircuitBreaker:   cb,
			}
			cdsClusters = append(cdsClusters, pcds)
			continue
		}
	}
	return
}

// DiscoverListeners lds
// create listens by get depend app endpoints from plugin config
func (d *DiscoverAction) DiscoverListeners(
	tenantService, serviceCluster string) (*envoyv1.LDSListener, *util.APIHandleError) {
	nn := strings.Split(tenantService, "_")
	if len(nn) != 3 {
		return nil, util.CreateAPIHandleError(400,
			fmt.Errorf("namesapces and service_alias not in good format"))
	}
	namespace := nn[0]
	pluginID := nn[1]
	serviceAlias := nn[2]
	lds := &envoyv1.LDSListener{}
	resources, err := d.ToolsGetRainbondResources(namespace, serviceAlias, pluginID)
	if err != nil {
		if strings.Contains(err.Error(), "is not exist") {
			return lds, nil
		}
		logrus.Warnf("in lds get env %s error: %v", namespace+serviceAlias+pluginID, err)
		return nil, util.CreateAPIHandleError(500, fmt.Errorf(
			"get env %s error: %v", namespace+serviceAlias+pluginID, err))
	}
	if resources.BaseServices != nil && len(resources.BaseServices) > 0 {
		listeners, err := d.upstreamListener(serviceAlias, namespace, resources.BaseServices)
		if err != nil {
			return nil, err
		}
		lds.Listeners.Append(listeners)
	}

	return lds, nil
}

//upstreamListener handle upstream app listener
// handle kubernetes inner service
func (d *DiscoverAction) upstreamListener(serviceAlias, namespace string, dependsServices []*api_model.BaseService) (envoyv1.Listeners, *util.APIHandleError) {
	var vhL []*envoyv1.VirtualHost
	var ldsL envoyv1.Listeners
	var portMap = make(map[int32]int, 0)
	for _, destService := range dependsServices {
		destServiceAlias := destService.DependServiceAlias
		labelname := fmt.Sprintf("name=%sService", destService.DependServiceAlias)
		services, err := k8s.K8S.Core().Services(namespace).List(metav1.ListOptions{LabelSelector: labelname})
		if err != nil {
			return nil, util.CreateAPIHandleError(500, err)
		}
		if len(services.Items) == 0 {
			logrus.Debugf("inner endpoints items length is 0, continue")
			continue
		}
		for _, service := range services.Items {
			serviceType, ok := service.Labels["service_type"]
			if !ok || serviceType != "inner" {
				if destServiceAlias != serviceAlias {
					continue
				}
			}
			port := service.Spec.Ports[0].Port
			clusterName := fmt.Sprintf("%s_%s_%s_%d", namespace, serviceAlias, destServiceAlias, port)
			if _, ok := portMap[port]; !ok {
				if v, ok := destService.Options["LISTEN"]; !ok || v == "true" {
					plds := envoyv1.CreateTCPCommonListener(clusterName, port)
					ldsL = append(ldsL, plds)
					portMap[port] = 1
				}
			}
			portProtocol, ok := service.Labels["port_protocol"]
			if !ok {
				portProtocol = destService.Protocol
			}
			if portProtocol != "" {
				//TODO: support more protocol
				switch portProtocol {
				case "http", "https":
					options := destService.Options
					var prs envoyv1.HTTPRoute
					prs.TimeoutMS = 0
					prs.Prefix = d.ToolsGetRouterItem(destServiceAlias, node_model.PREFIX, options).(string)
					wcn := &envoyv1.WeightedClusterEntry{
						Name:   clusterName,
						Weight: d.ToolsGetRouterItem(destServiceAlias, node_model.WEIGHT, options).(int),
					}
					prs.WeightedClusters = &envoyv1.WeightedCluster{
						Clusters: []*envoyv1.WeightedClusterEntry{wcn},
					}
					prs.Headers = d.ToolsGetRouterItem(destServiceAlias, node_model.HEADERS, options).([]envoyv1.Header)
					pvh := &envoyv1.VirtualHost{
						Name:    fmt.Sprintf("%s_%s_%s_%d", namespace, serviceAlias, destServiceAlias, port),
						Domains: d.ToolsGetRouterItem(destServiceAlias, node_model.DOMAINS, options).([]string),
						Routes:  []*envoyv1.HTTPRoute{&prs},
					}
					vhL = append(vhL, pvh)
					continue
				default:
					continue
				}
			}
		}
	}
	// create common http listener
	if len(vhL) != 0 {
		newVHL := envoyv1.UniqVirtualHost(vhL)
		plds := envoyv1.CreateHTTPCommonListener(fmt.Sprintf("%s_%s_http_80", namespace, serviceAlias), newVHL...)
		ldsL = append(ldsL, plds)
	}
	return ldsL, nil
}

//Duplicate Duplicate
func Duplicate(a interface{}) (ret []interface{}) {
	va := reflect.ValueOf(a)
	for i := 0; i < va.Len(); i++ {
		if i > 0 && reflect.DeepEqual(va.Index(i-1).Interface(), va.Index(i).Interface()) {
			continue
		}
		ret = append(ret, va.Index(i).Interface())
	}
	return ret
}

//CheckSameDomainAndPrefix check there is same domain or prefix
func (d *DiscoverAction) CheckSameDomainAndPrefix(resources *api_model.ResourceSpec) map[string]string {
	baseServices := resources.BaseServices
	domainL := make(map[string]string)
	if len(baseServices) == 0 {
		logrus.Debugf("has no base services resources")
		return domainL
	}
	filterL := make(map[string]int)
	for _, bs := range baseServices {
		l := len(filterL)
		domainName, _ := bs.Options[node_model.DOMAINS].(string)
		filterL[domainName] = 0
		if len(filterL) == l {
			domainL[domainName] = "use"
		}
	}
	for d := range domainL {
		prefixM := make(map[string]int)
		for _, bs := range baseServices {
			domainName, _ := bs.Options[node_model.DOMAINS].(string)
			if domainName == d {
				prefix, _ := bs.Options[node_model.PREFIX].(string)
				prefixM[prefix] = 0
			}
			// if strings.Contains(domainName, ","){
			// 	mm := strings.Split(domainName, ",")
			// 	for _, n := range mm {
			// 		if n == d {
			// 			prefix, _ := bs.Options[node_model.PREFIX].(string)
			// 			prefixM[prefix] = 0
			// 			continue
			// 		}
			// 	}
			// }
		}
		if len(prefixM) == 1 {
			domainL[d] = node_model.MODELWEIGHT
		} else {
			domainL[d] = node_model.MODELPREFIX
		}
	}
	return domainL
}

//ToolsGetSourcesEnv rds
func (d *DiscoverAction) ToolsGetSourcesEnv(
	namespace, sourceAlias, envName string) ([]byte, *util.APIHandleError) {
	k := fmt.Sprintf("/resources/define/%s/%s/%s", namespace, sourceAlias, envName)
	resp, err := d.etcdCli.Get(k)
	if err != nil {
		logrus.Errorf("get etcd value error, %v", err)
		return nil, util.CreateAPIHandleError(500, err)
	}
	if resp.Count != 0 {
		v := resp.Kvs[0].Value
		return v, nil
	}
	return []byte{}, nil
}

//ToolsGetK8SServiceList GetK8SServiceList
func (d *DiscoverAction) ToolsGetK8SServiceList(uuid string) (*v1.ServiceList, error) {
	serviceList, err := k8s.K8S.Core().Services(uuid).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return serviceList, nil
}

//ToolsGetMainPodEnvs ToolsGetMainPodEnvs
func (d *DiscoverAction) ToolsGetMainPodEnvs(namespace, serviceAlias string) (
	*[]v1.EnvVar,
	*util.APIHandleError) {
	labelname := fmt.Sprintf("name=%s", serviceAlias)
	pods, err := k8s.K8S.Core().Pods(namespace).List(metav1.ListOptions{LabelSelector: labelname})
	logrus.Debugf("service_alias %s pod is %v", serviceAlias, pods)
	if err != nil {
		return nil, util.CreateAPIHandleError(500, err)
	}
	if len(pods.Items) == 0 {
		return nil,
			util.CreateAPIHandleError(404, fmt.Errorf("have no pod for discover"))
	}
	if len(pods.Items[0].Spec.Containers) < 2 {
		return nil,
			util.CreateAPIHandleError(404, fmt.Errorf("have no net plugins for discover"))
	}
	for _, c := range pods.Items[0].Spec.Containers {
		for _, e := range c.Env {
			if e.Name == "PLUGIN_MOEL" && strings.Contains(e.Value, "net-plugin") {
				return &c.Env, nil
			}
		}
	}
	return nil, util.CreateAPIHandleError(404, fmt.Errorf("have no envs for plugin"))
}

//ToolsBuildPieceLDS ToolsBuildPieceLDS
func (d *DiscoverAction) ToolsBuildPieceLDS() {}

//ToolsGetRouterItem ToolsGetRouterItem
func (d *DiscoverAction) ToolsGetRouterItem(
	destAlias, kind string, sr map[string]interface{}) interface{} {
	switch kind {
	case node_model.PREFIX:
		if prefix, ok := sr[node_model.PREFIX]; ok {
			return prefix
		}
		return "/"
	case node_model.LIMITS:
		if circuit, ok := sr[node_model.LIMITS]; ok {
			cc, err := strconv.Atoi(circuit.(string))
			if err != nil {
				logrus.Errorf("strcon circuit error")
				return 1024
			}
			if cc == 10250 {
				return 0
			}
			return cc
		}
		return 1024
	case node_model.MaxRequests:
		if maxRequest, ok := sr[node_model.MaxRequests]; ok {
			mrt, err := strconv.Atoi(maxRequest.(string))
			if err != nil {
				logrus.Errorf("strcon max request error")
				return 1024
			}
			if mrt == 10250 {
				return 0
			}
			return mrt
		}
		return 1024
	case node_model.MaxPendingRequests:
		if maxPendingRequests, ok := sr[node_model.MaxPendingRequests]; ok {
			mpr, err := strconv.Atoi(maxPendingRequests.(string))
			if err != nil {
				logrus.Errorf("strcon max pending request error")
				return 1024
			}
			if mpr == 10250 {
				return 0
			}
			return mpr
		}
		return 1024
	case node_model.MaxRetries:
		if maxRetries, ok := sr[node_model.MaxRetries]; ok {
			mxr, err := strconv.Atoi(maxRetries.(string))
			if err != nil {
				logrus.Errorf("strcon max retry error")
				return 3
			}
			return mxr
		}
		return 3
	case node_model.HEADERS:
		if headers, ok := sr[node_model.HEADERS]; ok {
			var np []envoyv1.Header
			parents := strings.Split(headers.(string), ";")
			for _, h := range parents {
				headers := strings.Split(h, ":")
				//has_header:no 默认
				if len(headers) == 2 {
					if headers[0] == "has_header" && headers[1] == "no" {
						continue
					}
					ph := envoyv1.Header{
						Name:  headers[0],
						Value: headers[1],
					}
					np = append(np, ph)
				}
			}
			return np
		}
		var rc []envoyv1.Header
		return rc
	case node_model.DOMAINS:
		if domain, ok := sr[node_model.DOMAINS]; ok {
			if strings.Contains(domain.(string), ",") {
				mm := strings.Split(domain.(string), ",")
				return mm
			}
			return []string{domain.(string)}
		}
		return []string{destAlias}
	case node_model.WEIGHT:
		if weight, ok := sr[node_model.WEIGHT]; ok {
			w, err := strconv.Atoi(weight.(string))
			if err != nil {
				return 100
			}
			return w
		}
		return 100
	default:
		return nil
	}
}

//ToolsGetRainbondResources get plugin configs from etcd
//if not exist return error
func (d *DiscoverAction) ToolsGetRainbondResources(namespace, sourceAlias, pluginID string) (*api_model.ResourceSpec, error) {
	k := fmt.Sprintf("/resources/define/%s/%s/%s", namespace, sourceAlias, pluginID)
	resp, err := d.etcdCli.Get(k)
	if err != nil {
		logrus.Errorf("get etcd value error, %v", err)
		return nil, err
	}
	var rs api_model.ResourceSpec
	if resp.Count != 0 {
		v := resp.Kvs[0].Value
		if err := ffjson.Unmarshal(v, &rs); err != nil {
			logrus.Errorf("unmashal etcd v error, %v", err)
			return nil, err
		}
	} else {
		logrus.Warningf("resources is not exist, key is %s", k)
		return nil, fmt.Errorf("resources is not exist")
	}
	return &rs, nil
}
