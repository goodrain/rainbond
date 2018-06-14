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

package appm

import (
	"fmt"
	"os"

	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/event"

	"github.com/Sirupsen/logrus"

	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/pkg/api/v1"
)

//K8sServiceBuild K8sServiceBuild
type K8sServiceBuild struct {
	serviceID, eventID string
	service            *model.TenantServices
	tenant             *model.Tenants
	dbmanager          db.Manager
	logger             event.Logger
	replicationType    string
}

//K8sServiceBuilder 构建应用对应的k8s service
func K8sServiceBuilder(serviceID, replicationType string, logger event.Logger) (*K8sServiceBuild, error) {
	dbmanager := db.GetManager()
	service, err := dbmanager.TenantServiceDao().GetServiceByID(serviceID)
	if err != nil {
		return nil, fmt.Errorf("find service error. %v", err.Error())
	}
	tenant, err := dbmanager.TenantDao().GetTenantByUUID(service.TenantID)
	if err != nil {
		return nil, fmt.Errorf("find tenant error. %v", err.Error())
	}
	return &K8sServiceBuild{
		serviceID:       serviceID,
		eventID:         logger.Event(),
		dbmanager:       dbmanager,
		service:         service,
		tenant:          tenant,
		logger:          logger,
		replicationType: replicationType,
	}, nil
}

//GetTenantID 获取租户ID
func (k *K8sServiceBuild) GetTenantID() string {
	return k.tenant.UUID
}

//Build 构建应用的所有端口服务
func (k *K8sServiceBuild) Build() ([]*v1.Service, error) {
	ports, err := k.dbmanager.TenantServicesPortDao().GetPortsByServiceID(k.serviceID)
	if err != nil {
		return nil, fmt.Errorf("find service port from db error %s", err.Error())
	}
	crt, err := k.checkUpstreamPluginRelation()
	if err != nil {
		return nil, fmt.Errorf("get service upstream plugin relation error, %s", err.Error())
	}
	pp := make(map[int32]int)
	if crt {
		pluginPorts, err := k.dbmanager.TenantServicesStreamPluginPortDao().GetPluginMappingPorts(
			k.serviceID,
			model.UpNetPlugin,
		)
		if err != nil {
			return nil, fmt.Errorf("find upstream plugin mapping port error, %s", err.Error())
		}
		ports, pp, err = k.CreateUpstreamPluginMappingPort(ports, pluginPorts)
	}
	if err != nil {
		return nil, fmt.Errorf("create upstream port error, %s", err.Error())
	}
	var services []*v1.Service
	//创建分端口的负载均衡Service
	if ports != nil && len(ports) > 0 {
		for i := range ports {
			port := ports[i]
			if port.IsInnerService {
				services = append(services, k.createInnerService(port))
			}
			if port.IsOuterService {
				services = append(services, k.createOuterService(port))
			}
		}
	}
	//创建有状态服务DNS服务Service
	if k.replicationType == model.TypeStatefulSet {
		services = append(services, k.createStatefulService(ports))
	}
	if crt {
		services, _ = k.CreateUpstreamPluginMappingService(services, pp)
	}
	return services, nil
}

func (k *K8sServiceBuild) checkUpstreamPluginRelation() (bool, error) {
	return k.dbmanager.TenantServicePluginRelationDao().CheckSomeModelPluginByServiceID(
		k.serviceID,
		model.UpNetPlugin)
}

//CreateUpstreamPluginMappingPort 检查是否存在upstream插件，接管入口网络
func (k *K8sServiceBuild) CreateUpstreamPluginMappingPort(
	ports []*model.TenantServicesPort,
	pluginPorts []*model.TenantServicesStreamPluginPort,
) (
	[]*model.TenantServicesPort,
	map[int32]int,
	error) {
	//start from 65301
	pp := make(map[int32]int)
	for i := range ports {
		port := ports[i]
		for _, pport := range pluginPorts {
			if pport.ContainerPort == port.ContainerPort {
				pp[int32(pport.PluginPort)] = port.ContainerPort
				port.ContainerPort = pport.PluginPort
				port.MappingPort = pport.PluginPort
			}
		}
	}
	return ports, pp, nil
}

//CreateUpstreamPluginMappingService 增加service plugin mapport 标签
func (k *K8sServiceBuild) CreateUpstreamPluginMappingService(services []*v1.Service, pp map[int32]int) (
	[]*v1.Service,
	error) {
	for _, service := range services {
		logrus.Debugf("map is %v, port is %v, origin_port is %d",
			pp,
			service.Spec.Ports[0].Port,
			pp[service.Spec.Ports[0].Port])
		service.Labels["origin_port"] = fmt.Sprintf("%d", pp[service.Spec.Ports[0].Port])
	}
	return services, nil
}

//BuildOnPort 指定端口创建Service
func (k *K8sServiceBuild) BuildOnPort(p int, isOut bool) (*v1.Service, error) {
	port, err := k.dbmanager.TenantServicesPortDao().GetPort(k.serviceID, p)
	if err != nil {
		return nil, fmt.Errorf("find service port from db error %s", err.Error())
	}
	if port != nil {
		if !isOut && port.IsInnerService {
			return k.createInnerService(port), nil
		}
		if isOut && port.IsOuterService {
			return k.createOuterService(port), nil
		}
	}
	return nil, fmt.Errorf("tenant service port %d is not exist", p)
}

//createServiceAnnotations create service annotation
func (k *K8sServiceBuild) createServiceAnnotations() map[string]string {
	var annotations = make(map[string]string)
	if k.service.Replicas <= 1 {
		annotations["rainbond.com/tolerate-unready-endpoints"] = "true"
	}
	return annotations
}
func (k *K8sServiceBuild) createInnerService(port *model.TenantServicesPort) *v1.Service {
	var service v1.Service
	service.Name = fmt.Sprintf("service-%d-%d", port.ID, port.ContainerPort)
	service.Labels = map[string]string{
		"service_type":  "inner",
		"name":          k.service.ServiceAlias + "Service",
		"port_protocol": port.Protocol,
	}
	if k.service.Replicas <= 1 {
		service.Labels["rainbond.com/tolerate-unready-endpoints"] = "true"
	}
	service.Annotations = k.createServiceAnnotations()
	var servicePort v1.ServicePort
	if port.Protocol == "udp" {
		servicePort.Protocol = "UDP"
	} else {
		servicePort.Protocol = "TCP"
	}
	servicePort.TargetPort = intstr.FromInt(port.ContainerPort)
	servicePort.Port = int32(port.MappingPort)
	if servicePort.Port == 0 {
		servicePort.Port = int32(port.ContainerPort)
	}
	spec := v1.ServiceSpec{
		Ports:    []v1.ServicePort{servicePort},
		Selector: map[string]string{"name": k.service.ServiceAlias},
	}
	service.Spec = spec
	return &service
}

func (k *K8sServiceBuild) createOuterService(port *model.TenantServicesPort) *v1.Service {
	var service v1.Service
	service.Name = fmt.Sprintf("service-%d-%dout", port.ID, port.ContainerPort)
	service.Labels = map[string]string{
		"service_type":     "outer",
		"name":             k.service.ServiceAlias + "ServiceOUT",
		"tenant_name":      k.tenant.Name,
		"services_version": k.service.ServiceVersion,
		"domain":           k.service.Autodomain(k.tenant.Name, port.ContainerPort),
		"protocol":         port.Protocol,
		"port_protocol":    port.Protocol,
		"ca":               "",
		"key":              "",
		"event_id":         k.eventID,
	}
	if k.service.Replicas <= 1 {
		service.Labels["rainbond.com/tolerate-unready-endpoints"] = "true"
	}
	service.Annotations = k.createServiceAnnotations()
	//if port.Protocol == "stream" { //stream 协议获取映射端口
	if port.Protocol != "http" { //stream 协议获取映射端口
		mapPort, err := k.dbmanager.TenantServiceLBMappingPortDao().GetTenantServiceLBMappingPort(k.serviceID, port.ContainerPort)
		if err != nil {
			logrus.Error("get tenant service lb map port error", err.Error())
			service.Labels["lbmap_port"] = "0"
		} else {
			service.Labels["lbmap_port"] = fmt.Sprintf("%d", mapPort.Port)
		}
	}
	var servicePort v1.ServicePort
	//TODO: udp, tcp
	if port.Protocol == "udp" {
		servicePort.Protocol = "UDP"
	} else {
		servicePort.Protocol = "TCP"
	}
	servicePort.TargetPort = intstr.FromInt(port.ContainerPort)
	servicePort.Port = int32(port.ContainerPort)
	var portType v1.ServiceType
	if os.Getenv("CUR_NET") == "midonet" {
		portType = v1.ServiceTypeNodePort
	} else {
		portType = v1.ServiceTypeClusterIP
	}
	spec := v1.ServiceSpec{
		Ports:    []v1.ServicePort{servicePort},
		Selector: map[string]string{"name": k.service.ServiceAlias},
		Type:     portType,
	}
	service.Spec = spec
	return &service
}

func (k *K8sServiceBuild) createStatefulService(ports []*model.TenantServicesPort) *v1.Service {
	var service v1.Service
	service.Name = k.service.ServiceAlias
	service.Labels = map[string]string{
		"service_type": "stateful",
		"name":         k.service.ServiceAlias + "ServiceStateful",
	}
	var serviceports []v1.ServicePort
	for _, p := range ports {
		var servicePort v1.ServicePort
		servicePort.Protocol = "TCP"
		servicePort.TargetPort = intstr.FromInt(p.ContainerPort)
		servicePort.Port = int32(p.MappingPort)
		servicePort.Name = fmt.Sprintf("%d-port", p.ID)
		if servicePort.Port == 0 {
			servicePort.Port = int32(p.ContainerPort)
		}
		serviceports = append(serviceports, servicePort)
	}

	spec := v1.ServiceSpec{
		Ports:     serviceports,
		Selector:  map[string]string{"name": k.service.ServiceAlias},
		ClusterIP: "None",
	}
	service.Spec = spec
	//before k8s 1.8 version, set Annotations for Service.PublishNotReadyAddresses
	service.Annotations = map[string]string{"service.alpha.kubernetes.io/tolerate-unready-endpoints": "true"}
	return &service
}
