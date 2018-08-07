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
	"time"

	"github.com/Sirupsen/logrus"
	api_model "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/api/util"
	"github.com/goodrain/rainbond/cmd/node/option"
	envoyv1 "github.com/goodrain/rainbond/node/core/envoy/v1"
	"github.com/goodrain/rainbond/node/core/store"
	"github.com/goodrain/rainbond/node/kubecache"
	"github.com/pquerna/ffjson/ffjson"
	"k8s.io/apimachinery/pkg/labels"
)

//DiscoverAction DiscoverAction
type DiscoverAction struct {
	conf    *option.Conf
	etcdCli *store.Client
	kubecli kubecache.KubeClient
}

//CreateDiscoverActionManager CreateDiscoverActionManager
func CreateDiscoverActionManager(conf *option.Conf, kubecli kubecache.KubeClient) *DiscoverAction {
	return &DiscoverAction{
		conf:    conf,
		etcdCli: store.DefalutClient,
		kubecli: kubecli,
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
	//dPort := mm[3]

	labelname := fmt.Sprintf("name=%sService", destServiceAlias)
	selector, err := labels.Parse(labelname)
	if err != nil {
		return nil, util.CreateAPIHandleError(500, err)
	}
	endpoints, err := d.kubecli.GetEndpoints(namespace, selector)
	if err != nil {
		return nil, util.CreateAPIHandleError(500, err)
	}
	services, err := d.kubecli.GetServices(namespace, selector)
	if err != nil {
		return nil, util.CreateAPIHandleError(500, err)
	}
	if len(endpoints) == 0 {
		if destServiceAlias == serviceAlias {
			labelname := fmt.Sprintf("name=%sServiceOUT", destServiceAlias)
			selector, err := labels.Parse(labelname)
			if err != nil {
				return nil, util.CreateAPIHandleError(500, err)
			}
			endpoints, err = d.kubecli.GetEndpoints(namespace, selector)
			if err != nil {
				return nil, util.CreateAPIHandleError(500, err)
			}
			if len(endpoints) == 0 {
				logrus.Debugf("outer endpoints items length is 0, continue")
				return nil, util.CreateAPIHandleError(400, fmt.Errorf("outer have no endpoints"))
			}
			services, err = d.kubecli.GetServices(namespace, selector)
			if err != nil {
				return nil, util.CreateAPIHandleError(500, err)
			}
		} else {
			return nil, util.CreateAPIHandleError(400, fmt.Errorf("inner have no endpoints"))
		}
	}
	var sdsL []*envoyv1.DiscoverHost
	for key, item := range endpoints {
		if len(item.Subsets) < 1 {
			continue
		}
		addressList := item.Subsets[0].Addresses
		if len(addressList) == 0 {
			addressList = item.Subsets[0].NotReadyAddresses
		}
		// rainbond create service only one port,so do not verify the port
		// port := item.Subsets[0].Ports[0].Port
		// if dPort != fmt.Sprintf("%d", port) {
		// 	continue
		// }
		toport := int(services[key].Spec.Ports[0].Port)
		if serviceAlias == destServiceAlias {
			if originPort, ok := services[key].Labels["origin_port"]; ok {
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
	if resources.BasePorts != nil && len(resources.BasePorts) > 0 {
		clusters, err := d.downstreamClusters(serviceAlias, namespace, resources.BasePorts)
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
	var portMap = make(map[int32]int)
	for i := range dependsServices {
		destService := dependsServices[i]
		destServiceAlias := destService.DependServiceAlias
		labelname := fmt.Sprintf("name=%sService", destServiceAlias)
		selector, err := labels.Parse(labelname)
		if err != nil {
			return nil, util.CreateAPIHandleError(500, err)
		}
		services, err := d.kubecli.GetServices(namespace, selector)
		if err != nil {
			return nil, util.CreateAPIHandleError(500, err)
		}
		if len(services) == 0 {
			continue
		}
		for _, service := range services {
			inner, ok := service.Labels["service_type"]
			port := service.Spec.Ports[0]
			if !ok || inner != "inner" {
				continue
			}
			pcds := &envoyv1.Cluster{
				Name:             fmt.Sprintf("%s_%s_%s_%v", namespace, serviceAlias, destServiceAlias, port.Port),
				Type:             "sds",
				ConnectTimeoutMs: 250,
				LbType:           "round_robin",
				ServiceName:      fmt.Sprintf("%s_%s_%s_%v", namespace, serviceAlias, destServiceAlias, port.Port),
				OutlierDetection: envoyv1.CreatOutlierDetection(destService.Options),
				CircuitBreaker:   envoyv1.CreateCircuitBreaker(destService.Options),
			}
			cdsClusters = append(cdsClusters, pcds)
			//create cluster base unique port
			if count, ok := portMap[port.Port]; ok && count == 1 {
				pcds := &envoyv1.Cluster{
					Name:             fmt.Sprintf("%s_%s_%v", namespace, serviceAlias, port.Port),
					Type:             "sds",
					ConnectTimeoutMs: 250,
					LbType:           "round_robin",
					ServiceName:      fmt.Sprintf("%s_%s_%s_%v", namespace, serviceAlias, destServiceAlias, port.Port),
					OutlierDetection: envoyv1.CreatOutlierDetection(destService.Options),
					CircuitBreaker:   envoyv1.CreateCircuitBreaker(destService.Options),
				}
				cdsClusters = append(cdsClusters, pcds)
				portMap[port.Port] = 2
			} else {
				portMap[port.Port] = 1
			}
			continue
		}
	}
	return
}

//downstreamClusters handle app self cluster
//only local port
func (d *DiscoverAction) downstreamClusters(serviceAlias, namespace string, ports []*api_model.BasePort) (cdsClusters envoyv1.Clusters, err *util.APIHandleError) {
	for i := range ports {
		port := ports[i]
		localhost := fmt.Sprintf("tcp://127.0.0.1:%d", port.Port)
		pcds := &envoyv1.Cluster{
			Name:             fmt.Sprintf("%s_%s_%v", namespace, serviceAlias, port.Port),
			Type:             "static",
			ConnectTimeoutMs: 250,
			LbType:           "round_robin",
			Hosts:            []envoyv1.Host{envoyv1.Host{URL: localhost}},
			CircuitBreaker:   envoyv1.CreateCircuitBreaker(port.Options),
		}
		cdsClusters = append(cdsClusters, pcds)
		continue
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
	if resources.BasePorts != nil && len(resources.BasePorts) > 0 {
		listeners, err := d.downstreamListener(serviceAlias, namespace, resources.BasePorts)
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
	for i := range dependsServices {
		destService := dependsServices[i]
		destServiceAlias := destService.DependServiceAlias
		start := time.Now()
		labelname := fmt.Sprintf("name=%sService", destServiceAlias)
		selector, err := labels.Parse(labelname)
		if err != nil {
			return nil, util.CreateAPIHandleError(500, err)
		}
		services, err := d.kubecli.GetServices(namespace, selector)
		if err != nil {
			return nil, util.CreateAPIHandleError(500, err)
		}
		fmt.Printf("get %s service cost time %s \n", destService.DependServiceAlias, time.Now().Sub(start).String())
		if len(services) == 0 {
			logrus.Debugf("inner endpoints items length is 0, continue")
			continue
		}
		for _, service := range services {
			inner, ok := service.Labels["service_type"]
			if !ok || inner != "inner" {
				continue
			}
			port := service.Spec.Ports[0].Port
			clusterName := fmt.Sprintf("%s_%s_%s_%d", namespace, serviceAlias, destServiceAlias, port)
			// Unique by listen port
			if _, ok := portMap[port]; !ok {
				listenerName := fmt.Sprintf("%s_%s_%d", namespace, serviceAlias, port)
				plds := envoyv1.CreateTCPCommonListener(listenerName, clusterName, fmt.Sprintf("tcp://127.0.0.1:%d", port))
				ldsL = append(ldsL, plds)
				portMap[port] = len(ldsL) - 1
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
					prs.Prefix = envoyv1.GetOptionValues(envoyv1.KeyPrefix, options).(string)
					wcn := &envoyv1.WeightedClusterEntry{
						Name:   clusterName,
						Weight: envoyv1.GetOptionValues(envoyv1.KeyWeight, options).(int),
					}
					prs.WeightedClusters = &envoyv1.WeightedCluster{
						Clusters: []*envoyv1.WeightedClusterEntry{wcn},
					}
					prs.Headers = envoyv1.GetOptionValues(envoyv1.KeyHeaders, options).([]envoyv1.Header)
					pvh := &envoyv1.VirtualHost{
						Name:    fmt.Sprintf("%s_%s_%s_%d", namespace, serviceAlias, destServiceAlias, port),
						Domains: envoyv1.GetOptionValues(envoyv1.KeyDomains, options).([]string),
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
		for i, lds := range ldsL {
			if lds.Address == "tcp://127.0.0.1:80" {
				ldsL = append(ldsL[:i], ldsL[i+1:]...)
				break
			}
		}
		plds := envoyv1.CreateHTTPCommonListener(fmt.Sprintf("%s_%s_http_80", namespace, serviceAlias), newVHL...)
		ldsL = append(ldsL, plds)
	}
	return ldsL, nil
}

//downstreamListener handle app self port listener
func (d *DiscoverAction) downstreamListener(serviceAlias, namespace string, ports []*api_model.BasePort) (envoyv1.Listeners, *util.APIHandleError) {
	var ldsL envoyv1.Listeners
	var portMap = make(map[int32]int, 0)
	for i := range ports {
		p := ports[i]
		port := int32(p.Port)
		clusterName := fmt.Sprintf("%s_%s_%d", namespace, serviceAlias, port)
		if _, ok := portMap[port]; !ok {
			plds := envoyv1.CreateTCPCommonListener(clusterName, clusterName, fmt.Sprintf("tcp://0.0.0.0:%d", p.ListenPort))
			ldsL = append(ldsL, plds)
			portMap[port] = 1
		}
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

//ToolsGetSourcesEnv rds
//envName maybe is plugin id
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
