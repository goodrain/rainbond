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

	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/pkg/api/util"
	node_model "github.com/goodrain/rainbond/pkg/node/api/model"
	"github.com/goodrain/rainbond/pkg/node/core/k8s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/client-go/pkg/api/v1"
)

//DiscoverAction DiscoverAction
type DiscoverAction struct {
	conf *option.Conf
}

//CreateDiscoverActionManager CreateDiscoverActionManager
func CreateDiscoverActionManager(conf *option.Conf) *DiscoverAction {
	return &DiscoverAction{
		conf: conf,
	}
}

//DiscoverService DiscoverService
func (d *DiscoverAction) DiscoverService(serviceInfo string) (*node_model.SDS, *util.APIHandleError) {
	mm := strings.Split(serviceInfo, "_")
	if len(mm) < 3 {
		return nil, util.CreateAPIHandleError(400, fmt.Errorf("service_name is not in good format"))
	}
	namespace := mm[0]
	serviceAlias := mm[1]
	dPort := mm[2]
	//deployVersion := mm[3]

	labelname := fmt.Sprintf("name=%sService", serviceAlias)
	endpoint, err := k8s.K8S.Core().Endpoints(namespace).List(metav1.ListOptions{LabelSelector: labelname})
	if err != nil {
		return nil, util.CreateAPIHandleError(500, err)
	}
	services, err := k8s.K8S.Core().Services(namespace).List(metav1.ListOptions{LabelSelector: labelname})
	if err != nil {
		return nil, util.CreateAPIHandleError(500, err)
	}
	if len(endpoint.Items) == 0 {
		return nil, util.CreateAPIHandleError(400, fmt.Errorf("have no endpoints"))
	}
	var sdsL []*node_model.PieceSDS
	for key, item := range endpoint.Items {
		addressList := item.Subsets[0].Addresses
		if len(addressList) == 0 {
			addressList = item.Subsets[0].NotReadyAddresses
		}
		port := item.Subsets[0].Ports[0].Port
		if dPort != fmt.Sprintf("%d", port) {
			continue
		}
		toport := services.Items[key].Spec.Ports[0].Port
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

//DiscoverListeners DiscoverListeners
func (d *DiscoverAction) DiscoverListeners(tenantService, serviceCluster string) (*node_model.LDS, *util.APIHandleError) {
	nn := strings.Split(tenantService, "_")
	if len(nn) != 2 {
		return nil, util.CreateAPIHandleError(400,
			fmt.Errorf("namesapces and service_alias not in good format"))
	}
	namespace := nn[0]
	serviceAlias := nn[1]
	envs, err := d.ToolsGetMainPodEnvs(namespace, serviceAlias)
	if err != nil {
		return nil, err
	}
	mm := strings.Split(serviceCluster, "_")
	if len(mm) == 0 {
		return nil, util.CreateAPIHandleError(400, fmt.Errorf("service_name is not in good format"))
	}
	var ldsL []*node_model.PieceLDS
	for _, serviceAlias := range mm {
		labelname := fmt.Sprintf("name=%sService", serviceAlias)
		endpoint, err := k8s.K8S.Core().Endpoints(namespace).List(metav1.ListOptions{LabelSelector: labelname})
		if err != nil {
			return nil, util.CreateAPIHandleError(500, err)
		}
		services, err := k8s.K8S.Core().Services(namespace).List(metav1.ListOptions{LabelSelector: labelname})
		if err != nil {
			return nil, util.CreateAPIHandleError(500, err)
		}
		if len(endpoint.Items) == 0 {
			continue
		}
		for _, service := range services.Items {
			//TODO: HTTP inner的protocol添加资源时需要label
			inner, ok := service.Labels["service_type"]
			if !ok || inner != "inner" {
				continue
			}
			port := service.Spec.Ports[0]
			portProtocol, ok := service.Labels["port_protocol"]
			if ok {
				switch portProtocol {
				case "TCP":
					ptr := &node_model.PieceTCPRoute{
						Cluster: fmt.Sprintf("%s_%s_%v", namespace, serviceAlias, port.Port),
					}
					lrs := &node_model.LDSTCPRoutes{
						Routes: []*node_model.PieceTCPRoute{ptr},
					}
					lcg := &node_model.LDSTCPConfig{
						StatPrefix:  fmt.Sprintf("%s_%s_%v", namespace, serviceAlias, port.Port),
						RouteConfig: lrs,
					}
					lfs := &node_model.LDSFilters{
						Name:   "tcp_proxy",
						Config: lcg,
					}
					plds := &node_model.PieceLDS{
						Name:    fmt.Sprintf("%s_%s_%v", namespace, serviceAlias, port.Port),
						Address: fmt.Sprintf("tcp://0.0.0.0:%v", port.Port),
						Filters: []*node_model.LDSFilters{lfs},
					}
					ldsL = append(ldsL, plds)
					continue
				case "HTTP":
					hsf := &node_model.HTTPSingleFileter{
						Type: "decoder",
						Name: "router",
					}
					prs := &node_model.PieceHTTPRoutes{
						TimeoutMS: 0,
						Prefix:    d.ToolsGetRouterItem(serviceAlias, node_model.PREFIX, envs),
						Cluster:   fmt.Sprintf("%s_%s_%v", namespace, serviceAlias, port.Port),
					}
					envHeaders := d.ToolsGetRouterItem(serviceAlias, node_model.HEADERS, envs)
					var headers []*node_model.PieceHeader
					if envHeaders != "" {
						mm := strings.Split(envHeaders, ",")
						for _, h := range mm {
							nn := strings.Split(h, ":")
							header := &node_model.PieceHeader{
								Name:  nn[0],
								Value: nn[1],
							}
							headers = append(headers, header)
						}
					}
					if len(headers) != 0 {
						prs.Headers = headers
					}
					pvh := &node_model.PieceHTTPVirtualHost{
						//TODO: 目前支持自定义一个domain
						Name:    fmt.Sprintf("%s_%s_%v", namespace, serviceAlias, port.Port),
						Domains: []string{d.ToolsGetRouterItem(serviceAlias, node_model.DOMAINS, envs)},
						Routes:  []*node_model.PieceHTTPRoutes{prs},
					}
					rcg := &node_model.RouteConfig{
						VirtualHosts: []*node_model.PieceHTTPVirtualHost{pvh},
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
						Name:    fmt.Sprintf("%s_%s_%v", namespace, serviceAlias, port.Port),
						Address: fmt.Sprintf("tcp://0.0.0.0:%v", port.TargetPort),
						Filters: []*node_model.LDSFilters{lfs},
					}
					ldsL = append(ldsL, plds)
					continue
				}
			}
		}
	}
	lds := &node_model.LDS{
		Listeners: ldsL,
	}
	return lds, nil
}

//DiscoverClusters DiscoverClusters
func (d *DiscoverAction) DiscoverClusters(tenantService, serviceCluster string) (*node_model.CDS, *util.APIHandleError) {
	nn := strings.Split(tenantService, "_")
	if len(nn) != 2 {
		return nil, util.CreateAPIHandleError(400, fmt.Errorf("namesapces and service_alias not in good format"))
	}
	namespace := nn[0]
	serviceAlias := nn[1]
	envs, err := d.ToolsGetMainPodEnvs(namespace, serviceAlias)
	if err != nil {
		return nil, err
	}
	mm := strings.Split(serviceCluster, "_")
	if len(mm) == 0 {
		return nil, util.CreateAPIHandleError(400, fmt.Errorf("service_name is not in good format"))
	}
	var cdsL []*node_model.PieceCDS
	for _, serviceAlias := range mm {
		labelname := fmt.Sprintf("name=%sService", serviceAlias)
		services, err := k8s.K8S.Core().Services(namespace).List(metav1.ListOptions{LabelSelector: labelname})
		if err != nil {
			return nil, util.CreateAPIHandleError(500, err)
		}
		for _, service := range services.Items {
			inner, ok := service.Labels["service_type"]
			if !ok || inner != "inner" {
				continue
			}
			circuits, errC := strconv.Atoi(d.ToolsGetRouterItem(serviceAlias, node_model.LIMITS, envs))
			if errC != nil {
				circuits = 1024
				logrus.Warnf("strconv circuit error, ignore this error and set circuits to 1024")
			}
			cb := &node_model.CircuitBreakers{
				Default: &node_model.MaxConnections{
					MaxConnections: circuits,
				},
			}
			port := service.Spec.Ports[0]
			pcds := &node_model.PieceCDS{
				Name:             fmt.Sprintf("%s_%s_%v", namespace, serviceAlias, port.Port),
				Type:             "sds",
				ConnectTimeoutMS: 250,
				LBType:           "round_robin",
				ServiceName:      fmt.Sprintf("%s_%s_%v", namespace, serviceAlias, port.Port),
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

//ToolsGetK8SServiceList GetK8SServiceList
func (d *DiscoverAction) ToolsGetK8SServiceList(uuid string) (*v1.ServiceList, error) {
	serviceList, err := k8s.K8S.Core().Services(uuid).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return serviceList, nil
}

//ToolsGetMainPodEnvs ToolsGetMainPodEnvs
func (d *DiscoverAction) ToolsGetMainPodEnvs(namespace, serviceAlias string) (*[]v1.EnvVar, *util.APIHandleError) {
	pods, err := k8s.K8S.Core().Pods(namespace).List(metav1.ListOptions{LabelSelector: serviceAlias})
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
func (d *DiscoverAction) ToolsGetRouterItem(destAlias, kind string, envs *[]v1.EnvVar) string {
	switch kind {
	case node_model.PREFIX:
		ename := fmt.Sprintf("PREFIX_%s", destAlias)
		for _, e := range *envs {
			if e.Name == ename {
				return e.Value
			}
		}
		return "/"
	case node_model.LIMITS:
		ename := fmt.Sprintf("LIMIT_%s", destAlias)
		for _, e := range *envs {
			if e.Name == ename {
				return e.Value
			}
		}
		return "1024"
	case node_model.HEADERS:
		ename := fmt.Sprintf("HEADER_%s", destAlias)
		for _, e := range *envs {
			if e.Name == ename {
				return e.Value
			}
		}
		return ""
	case node_model.DOMAINS:
		ename := fmt.Sprintf("DOMAIN_%s", destAlias)
		for _, e := range *envs {
			if e.Name == ename {
				return e.Value
			}
		}
		return "*"
	}
	return ""
}

func getEnvValue(ename string, envs *[]v1.EnvVar) string {
	for _, e := range *envs {
		if e.Name == ename {
			return e.Value
		}
	}
	return ""
}
