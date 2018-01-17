// RAINBOND, Application Management Platform
// Copyright (C) 2014-2017 Goodrain Co., Ltd.

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
	"strconv"
	"strings"
	"reflect"

	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/cmd/node/option"
	api_model "github.com/goodrain/rainbond/pkg/api/model"
	"github.com/goodrain/rainbond/pkg/api/util"
	node_model "github.com/goodrain/rainbond/pkg/node/api/model"
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
func (d *DiscoverAction) DiscoverService(serviceInfo string) (*node_model.SDS, *util.APIHandleError) {
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
	var sdsL []*node_model.PieceSDS
	for key, item := range endpoints.Items {
		addressList := item.Subsets[0].Addresses
		if len(addressList) == 0 {
			addressList = item.Subsets[0].NotReadyAddresses
		}
		port := item.Subsets[0].Ports[0].Port
		if dPort != fmt.Sprintf("%d", port) {
			continue
		}
		toport := services.Items[key].Spec.Ports[0].Port
		if serviceAlias == destServiceAlias {
			if originPort, ok := services.Items[key].Labels["origin_port"]; ok {
				origin, err := strconv.Atoi(originPort)
				if err != nil {
					return nil, util.CreateAPIHandleError(500, fmt.Errorf("have no origin_port"))
				}
				toport = int32(origin)
			}
		}
		for _, ip := range addressList {
			sdsP := &node_model.PieceSDS{
				IPAddress: ip.IP,
				Port:      toport,
			}
			sdsL = append(sdsL, sdsP)
		}
	}
	sds := &node_model.SDS{
		Hosts: sdsL,
	}
	return sds, nil
}

//DiscoverListeners lds
func (d *DiscoverAction) DiscoverListeners(
	tenantService, serviceCluster string) (*node_model.LDS, *util.APIHandleError) {
	nn := strings.Split(tenantService, "_")
	if len(nn) != 3 {
		return nil, util.CreateAPIHandleError(400,
			fmt.Errorf("namesapces and service_alias not in good format"))
	}
	namespace := nn[0]
	pluginID := nn[1]
	serviceAlias := nn[2]
	mm := strings.Split(serviceCluster, "_")
	if len(mm) == 0 {
		return nil, util.CreateAPIHandleError(400, fmt.Errorf("service_name is not in good format"))
	}
	resources, err := d.ToolsGetRainbondResources(namespace, serviceAlias, pluginID)
	if err != nil && !strings.Contains(err.Error(), "is not exist") {
		logrus.Warnf("in lds get env %s error: %v", namespace+serviceAlias+pluginID, err)
		return nil, util.CreateAPIHandleError(500, fmt.Errorf(
			"get env %s error: %v", namespace+serviceAlias+pluginID, err))
	}
	
	logrus.Debugf("process go on")
	//TODO: console控制尽量不把小于1000的端口给用户使用
	var vhL []*node_model.PieceHTTPVirtualHost
	var ldsL []*node_model.PieceLDS
	for _, destServiceAlias := range mm {
		labelname := fmt.Sprintf("name=%sService", destServiceAlias)
		endpoint, err := k8s.K8S.Core().Endpoints(namespace).List(metav1.ListOptions{LabelSelector: labelname})
		if err != nil {
			return nil, util.CreateAPIHandleError(500, err)
		}
		services, err := k8s.K8S.Core().Services(namespace).List(metav1.ListOptions{LabelSelector: labelname})
		if err != nil {
			return nil, util.CreateAPIHandleError(500, err)
		}
		if len(endpoint.Items) == 0 {
			if destServiceAlias == serviceAlias {
				labelname := fmt.Sprintf("name=%sServiceOUT", destServiceAlias)
				var err error
				endpoint, err = k8s.K8S.Core().Endpoints(namespace).List(metav1.ListOptions{LabelSelector: labelname})
				if err != nil {
					return nil, util.CreateAPIHandleError(500, err)
				}
				if len(endpoint.Items) == 0 {
					logrus.Debugf("outer endpoints items length is 0, continue")
					continue
				}
				services, err = k8s.K8S.Core().Services(namespace).List(metav1.ListOptions{LabelSelector: labelname})
				if err != nil {
					return nil, util.CreateAPIHandleError(500, err)
				}
			} else {
				logrus.Debugf("inner endpoints items length is 0, continue")
				continue
			}
		}
		for _, service := range services.Items {
			//TODO: HTTP inner的protocol添加资源时需要label
			inner, ok := service.Labels["service_type"]
			if !ok || inner != "inner" {
				if destServiceAlias != serviceAlias {
					continue
				}
			}
			port := service.Spec.Ports[0].Port
			portProtocol, ok := service.Labels["port_protocol"]
			if !ok {
				logrus.Debugf("have no port Protocol")
			}
			if ok {
				logrus.Debugf("port protocol is %s", portProtocol)
				switch portProtocol {
				case "stream":
					ptr := &node_model.PieceTCPRoute{
						Cluster: fmt.Sprintf("%s_%s_%s_%d", namespace, serviceAlias, destServiceAlias, port),
					}
					lrs := &node_model.LDSTCPRoutes{
						Routes: []*node_model.PieceTCPRoute{ptr},
					}
					lcg := &node_model.LDSTCPConfig{
						StatPrefix:  fmt.Sprintf("%s_%s_%s_%d", namespace, serviceAlias, destServiceAlias, port),
						RouteConfig: lrs,
					}
					lfs := &node_model.LDSFilters{
						Name:   "tcp_proxy",
						Config: lcg,
					}
					plds := &node_model.PieceLDS{
						//TODO: Name length must within 60
						Name:    fmt.Sprintf("%s_%s_%s_%d", namespace, serviceAlias, destServiceAlias, port),
						Address: fmt.Sprintf("tcp://0.0.0.0:%d", port),
						Filters: []*node_model.LDSFilters{lfs},
					}
					ldsL = append(ldsL, plds)
					continue
				case "http":
					if destServiceAlias == serviceAlias {
						//主容器应用
						var vhLThin []*node_model.PieceHTTPVirtualHost
						options := make(map[string]interface{})
						if resources != nil {
							for _, bp := range resources.BasePorts {
								if bp.ServiceAlias == serviceAlias && int32(bp.Port) == port {
									options = bp.Options
								}
							}
						}
						prs := &node_model.PieceHTTPRoutes{
							TimeoutMS: 0,
							Prefix:    d.ToolsGetRouterItem(destServiceAlias, node_model.PREFIX, options).(string),
							Cluster:   fmt.Sprintf("%s_%s_%s_%d", namespace, serviceAlias, destServiceAlias, port),
						}
						pvh := &node_model.PieceHTTPVirtualHost{
							Name: fmt.Sprintf("%s_%s_%s_%d", namespace, serviceAlias, destServiceAlias, port),
							//Domains: d.ToolsGetRouterItem(destServiceAlias, node_model.DOMAINS, &sr).([]string),
							//TODO: 主容器应用domain默认为*
							Domains: []string{"*"},
							Routes:  []*node_model.PieceHTTPRoutes{prs},
						}
						vhLThin = append(vhLThin, pvh)
						hsf := &node_model.HTTPSingleFileter{
							Type:   "decoder",
							Name:   "router",
							Config: make(map[string]string),
						}
						rcg := &node_model.RouteConfig{
							VirtualHosts: vhLThin,
						}
						lhc := &node_model.LDSHTTPConfig{
							CodecType:   "auto",
							StatPrefix:  "ingress_http",
							RouteConfig: rcg,
							Filters:     []*node_model.HTTPSingleFileter{hsf},
						}
						lfs := &node_model.LDSFilters{
							Name:   "http_connection_manager",
							Config: lhc,
						}
						plds := &node_model.PieceLDS{
							Name:    fmt.Sprintf("%s_%s_http_%d", namespace, serviceAlias, port),
							Address: fmt.Sprintf("tcp://0.0.0.0:%d", port),
							Filters: []*node_model.LDSFilters{lfs},
						}
						//修改http-port console 完成
						ldsL = append(ldsL, plds)
					} else {
						//非主容器应用
						options := make(map[string]interface{})
						if resources != nil {
							for _, bp := range resources.BaseServices {
								if bp.DependServiceAlias == destServiceAlias && int32(bp.Port) == port {
									options = bp.Options
								}
							}
						}
						headers := d.ToolsGetRouterItem(
							destServiceAlias,
							node_model.HEADERS, options)
						prs := make(map[string]interface{})
						prs["timeout_ms"] = 0
						prs["prefix"] = d.ToolsGetRouterItem(destServiceAlias, node_model.PREFIX, options).(string)
						c := make(map[string]interface{})
						c["name"] = fmt.Sprintf("%s_%s_%s_%d", namespace, serviceAlias, destServiceAlias, port)
						c["weight"] = d.ToolsGetRouterItem(destServiceAlias, node_model.WEIGHT, options).(int)
						var wc node_model.WeightedClusters
						wc.Clusters = []map[string]interface{}{c}
						prs["weighted_clusters"] = wc
						if headers != nil {
							prs["headers"] = headers
						}
						pvh := &node_model.PieceHTTPVirtualHost{
							Name:    fmt.Sprintf("%s_%s_%s_%d", namespace, serviceAlias, destServiceAlias, port),
							Domains: d.ToolsGetRouterItem(destServiceAlias, node_model.DOMAINS, options).([]string),
							Routes:  []map[string]interface{}{prs},
						}
						vhL = append(vhL, pvh)
					}
					continue
				default:
					continue
				}
			}
		}
	}
	if len(vhL) != 0 {
		logrus.Debugf("vhl len is not 0")
		httpPort := 80
		hsf := &node_model.HTTPSingleFileter{
			Type:   "decoder",
			Name:   "router",
			Config: make(map[string]string),
		}
		var newVHL []*node_model.PieceHTTPVirtualHost	
		if len(vhL) > 1 {
			domainL := d.CheckSameDomainAndPrefix(resources)
			logrus.Debugf("domainL is %v", domainL)
			if len(domainL) > 0 {
				//存在相同的domain设置
				for d := range domainL {
					var c []map[string]interface{}
					var r []interface{}
					var pvh node_model.PieceHTTPVirtualHost	
					prs := make(map[string]interface{})
					prs["timeout_ms"] = 0
					for _, v := range vhL {
						if pvh.Name == "" {
							pvh.Name = v.Name
							pvh.Domains = v.Domains
							pvh.Routes = []map[string]interface{}{prs}
						}
						if v.Domains[0] == d {
							switch domainL[d]{
							case node_model.MODELWEIGHT:
								prs["prefix"] = v.Routes.([]map[string]interface{})[0]["prefix"].(string)
								c = append(c, v.Routes.([]map[string]interface{})[0]["weighted_clusters"].(map[string]interface{}))
							case node_model.MODELPREFIX:
								r = append(r, v.Routes.([]map[string]interface{})[0])
							}
						}else {
							newVHL = append(newVHL, v)
						}
					}
					if len(r) != 0 {
						pvh.Routes = r
						newVHL = append(newVHL, &pvh)
					}
					if len(c) != 0 {
						prs["weighted_clusters"] = c
						logrus.Debugf("prs is %v", prs)
						pvh.Routes = prs
						newVHL = append(newVHL, &pvh)
					}
				}
			}else {
				newVHL = vhL
			}
		}
		logrus.Debugf("newVHL is %v", newVHL)
		rcg := &node_model.RouteConfig{
			VirtualHosts: newVHL,
		}
		lhc := &node_model.LDSHTTPConfig{
			CodecType:   "auto",
			StatPrefix:  "ingress_http",
			RouteConfig: rcg,
			Filters:     []*node_model.HTTPSingleFileter{hsf},
		}
		lfs := &node_model.LDSFilters{
			Name:   "http_connection_manager",
			Config: lhc,
		}
		plds := &node_model.PieceLDS{
			Name:    fmt.Sprintf("%s_%s_http_80", namespace, serviceAlias),
			Address: fmt.Sprintf("tcp://0.0.0.0:%d", httpPort),
			Filters: []*node_model.LDSFilters{lfs},
		}
		//修改http-port console 完成
		ldsL = append(ldsL, plds)
	}
	lds := &node_model.LDS{
		Listeners: ldsL,
	}
	return lds, nil
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

//CheckSameDomainAndPrefix 检查是否存在相同domain以及prefix
func (d *DiscoverAction)CheckSameDomainAndPrefix(resources *api_model.ResourceSpec) (map[string]string){
	baseServices := resources.BaseServices
	domainL := make(map[string]string)
	if len(baseServices) == 0 {
		logrus.Debugf("has no base services resources")
		return domainL
	}
	filterL := make(map[string]int)
	for _, bs := range baseServices {
		l := len(filterL)
		domainName, _:= bs.Options[node_model.DOMAINS].(string)
		filterL[domainName] = 0
		if len(filterL) == l {
			domainL[domainName] = "use"
		}
	}
	for d := range domainL {
		prefixM := make(map[string]int)
		for _, bs := range baseServices {
			domainName, _ := bs.Options[node_model.DOMAINS].(string)
			if strings.Contains(domainName, ","){
				mm := strings.Split(domainName, ",")
				for _, n := range mm {
					if n == d {
						prefix, _ := bs.Options[node_model.PREFIX].(string)
						prefixM[prefix] = 0
						break
					}
				}
			}
		}
		if len(prefixM) == 1 {
			domainL[d] = node_model.MODELWEIGHT
		}else{
			domainL[d] = node_model.MODELPREFIX
		}
	}
	return domainL
}

//DiscoverClusters cds
func (d *DiscoverAction) DiscoverClusters(
	tenantService,
	serviceCluster string) (*node_model.CDS, *util.APIHandleError) {
	nn := strings.Split(tenantService, "_")
	if len(nn) != 3 {
		return nil, util.CreateAPIHandleError(400, fmt.Errorf("namesapces and service_alias not in good format"))
	}
	namespace := nn[0]
	pluginID := nn[1]
	serviceAlias := nn[2]
	resources, err := d.ToolsGetRainbondResources(namespace, serviceAlias, pluginID)
	logrus.Debugf("resources is %v", resources)
	if err != nil && !strings.Contains(err.Error(), "is not exist") {
		logrus.Warnf("in lds get env %s error: %v", namespace+serviceAlias+pluginID, err)
		return nil, util.CreateAPIHandleError(500, fmt.Errorf(
			"get env %s error: %v", namespace+serviceAlias+pluginID, err))
	}
	mm := strings.Split(serviceCluster, "_")
	if len(mm) == 0 {
		return nil, util.CreateAPIHandleError(400, fmt.Errorf("service_name is not in good format"))
	}
	var cdsL []*node_model.PieceCDS
	for _, destServiceAlias := range mm {
		labelname := fmt.Sprintf("name=%sService", destServiceAlias)
		services, err := k8s.K8S.Core().Services(namespace).List(metav1.ListOptions{LabelSelector: labelname})
		if err != nil {
			return nil, util.CreateAPIHandleError(500, err)
		}
		if len(services.Items) == 0 {
			if destServiceAlias == serviceAlias {
				labelname := fmt.Sprintf("name=%sServiceOUT", destServiceAlias)
				var err error
				services, err = k8s.K8S.Core().Services(namespace).List(metav1.ListOptions{LabelSelector: labelname})
				if err != nil {
					return nil, util.CreateAPIHandleError(500, err)
				}
			}
		}
		selfCount := 0
		for _, service := range services.Items {
			inner, ok := service.Labels["service_type"]
			port := service.Spec.Ports[0]
			originPort := service.Labels["origin_port"]
			options := make(map[string]interface{})
			if (!ok || inner != "inner") && serviceAlias != destServiceAlias {
				continue
			}
			if (serviceAlias == destServiceAlias) && selfCount == 1 {
				continue
			}
			if serviceAlias == destServiceAlias {
				if resources != nil {
					for _, bp := range resources.BasePorts {
						logrus.Debugf("bp.servicealias: %s, serviceAlias: %s, bp.Port:%s, originPort: %s",
							bp.ServiceAlias, serviceAlias, fmt.Sprintf("%d", bp.Port), originPort)
						if bp.ServiceAlias == serviceAlias && fmt.Sprintf("%d", bp.Port) == originPort {
							options = bp.Options
						}
					}
				}
			} else {
				if resources != nil {
					for _, bp := range resources.BaseServices {
						if bp.DependServiceAlias == destServiceAlias && int32(bp.Port) == port.Port {
							options = bp.Options
						}
					}
				}
			}
			logrus.Debugf("options is %s", options)
			selfCount++
			circuits := d.ToolsGetRouterItem(destServiceAlias, node_model.LIMITS, options).(int)
			maxRequests := d.ToolsGetRouterItem(destServiceAlias, node_model.MaxRequests, options).(int)
			maxRetries := d.ToolsGetRouterItem(destServiceAlias, node_model.MaxRetries, options).(int)
			maxPendingRequests := d.ToolsGetRouterItem(destServiceAlias, node_model.MaxPendingRequests, options).(int)
			cb := &node_model.CircuitBreakers{
				Default: &node_model.MaxConnections{
					MaxConnections:     circuits,
					MaxPendingRequests: maxPendingRequests,
					MaxRequests:        maxRequests,
					MaxRetries:         maxRetries,
				},
			}
			pcds := &node_model.PieceCDS{
				Name:             fmt.Sprintf("%s_%s_%s_%v", namespace, serviceAlias, destServiceAlias, port.Port),
				Type:             "sds",
				ConnectTimeoutMS: 250,
				LBType:           "round_robin",
				ServiceName:      fmt.Sprintf("%s_%s_%s_%v", namespace, serviceAlias, destServiceAlias, port.Port),
				CircuitBreakers:  cb,
			}
			cdsL = append(cdsL, pcds)
			continue
		}
	}
	cds := &node_model.CDS{
		Clusters: cdsL,
	}
	return cds, nil
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
			var np []node_model.PieceHeader 
			parents := strings.Split(headers.(string), ";")
			for _, h := range parents {
				headers := strings.Split(h, ":")
				//has_header:no 默认
				if len(headers) == 2 {
					if headers[0] == "has_header" && headers[1] == "no" {
						continue
					}
					ph := node_model.PieceHeader{
						Name: headers[0],
						Value: headers[1],
					}
					np = append(np, ph)
				}
			}
			return np
		}
		return nil
	case node_model.DOMAINS:
		if domain, ok := sr[node_model.DOMAINS]; ok {
			if strings.Contains(domain.(string), ","){
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

//ToolsGetRainbondResources 获取rainbond自定义resources
func (d *DiscoverAction) ToolsGetRainbondResources(
	namespace, sourceAlias, pluginID string) (*api_model.ResourceSpec, error) {
	k := fmt.Sprintf("/resources/define/%s/%s/%s", namespace, sourceAlias, pluginID)
	logrus.Debugf("etcd resources k is %s", k)
	resp, err := d.etcdCli.Get(k)
	if err != nil {
		logrus.Errorf("get etcd value error, %v", err)
		return nil, util.CreateAPIHandleError(500, err)
	}
	var rs api_model.ResourceSpec
	if resp.Count != 0 {
		v := resp.Kvs[0].Value
		if err := ffjson.Unmarshal(v, &rs); err != nil {
			logrus.Errorf("unmashal etcd v error, %v", err)
			return nil, util.CreateAPIHandleError(500, err)
		}
	} else {
		logrus.Debugf("key %s is not exist,", k)
		return nil, nil
	}
	return &rs, nil
}
