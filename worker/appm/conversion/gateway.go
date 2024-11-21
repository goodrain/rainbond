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

// 文件说明：
// 该文件包含与 Kubernetes 资源相关的服务构建功能，包括服务、Ingress 和 Secret 的创建和应用。
// 它定义了 `AppServiceBuild` 类型及其方法，用于根据服务 ID 和其他信息构建和配置 Kubernetes 服务。
// 文件中提供的功能包括服务、Ingress 和 Secret 的创建、规则应用、插件端口映射等。
// 主要的类型和函数如下：
// - `AppServiceBuild`：服务构建器类型，包含构建 Kubernetes 资源所需的属性和方法。
// - `AppServiceBuilder`：用于创建 `AppServiceBuild` 实例的工厂方法。
// - `Build`：构建 Kubernetes 服务、Ingress 和 Secret 的方法。
// - `ApplyRules`：应用 HTTP 和 TCP 规则的方法。
// - `CreateUpstreamPluginMappingPort`：检查和创建插件端口映射的方法。
// - `CreateUpstreamPluginMappingService`：在服务中添加插件映射标签的方法。
// - `BuildOnPort`：在指定端口上创建服务的方法。
// - `createServiceAnnotations`：创建服务注释的方法。
// 本文件还包含对服务、Ingress 和 Secret 的创建、配置及其与 HTTP/TCP 规则的应用逻辑。

package conversion

import (
	"context"
	"fmt"
	v2 "github.com/apache/apisix-ingress-controller/pkg/kube/apisix/apis/config/v2"
	"github.com/goodrain/rainbond/api/util"
	k8s2 "github.com/goodrain/rainbond/pkg/component/k8s"
	"github.com/twinj/uuid"
	"os"
	"strconv"
	"strings"

	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/model"
	v1 "github.com/goodrain/rainbond/worker/appm/types/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// createDefaultDomain create default domain
func createDefaultDomain(tenantName, serviceAlias string, servicePort int) string {
	exDomain := os.Getenv("EX_DOMAIN")
	if exDomain == "" {
		return ""
	}
	if strings.Contains(exDomain, ":") {
		exDomain = strings.Split(exDomain, ":")[0]
	}
	if exDomain[0] == '.' {
		exDomain = exDomain[1:]
	}
	exDomain = strings.TrimSpace(exDomain)
	return fmt.Sprintf("%d.%s.%s.%s", servicePort, serviceAlias, tenantName, exDomain)
}

// TenantServiceRegist conv inner and outer service regist
func TenantServiceRegist(as *v1.AppService, dbmanager db.Manager) error {
	builder, err := AppServiceBuilder(as.ServiceID, string(as.ServiceType), dbmanager, as)
	if err != nil {
		logrus.Error("create k8s service builder error.", err.Error())
		return err
	}

	k8s, err := builder.Build(as)
	if err != nil {
		logrus.Error("error creating app service: ", err.Error())
		return err
	}
	if k8s == nil {
		return nil
	}
	for _, service := range k8s.Services {
		as.SetService(service)
	}
	for _, ing := range k8s.Ingresses {
		as.SetIngress(ing)
	}
	for _, sec := range k8s.Secrets {
		as.SetSecret(sec)
	}
	for _, route := range k8s.ApiSixRoute {
		_, createErr := k8s2.Default().ApiSixClient.ApisixV2().
			ApisixRoutes(as.GetNamespace()).
			Create(context.Background(), route, metav1.CreateOptions{})
		if createErr != nil {
			logrus.Errorf("failed to create ApisixRoute %s: %v\n", route.Name, createErr)
		} else {
			logrus.Infof("successfully created ApisixRoute %s.\n", route.Name)
		}
	}
	return nil
}

// AppServiceBuild has the ability to build k8s service, ingress and secret
type AppServiceBuild struct {
	serviceID, eventID string
	tenant             *model.Tenants
	service            *model.TenantServices
	appService         *v1.AppService
	replicationType    string
	dbmanager          db.Manager
}

// AppServiceBuilder returns a AppServiceBuild
func AppServiceBuilder(serviceID, replicationType string, dbmanager db.Manager, as *v1.AppService) (*AppServiceBuild, error) {
	service, err := dbmanager.TenantServiceDao().GetServiceByID(serviceID)
	if err != nil {
		return nil, fmt.Errorf("find service error. %v", err.Error())
	}
	if service == nil {
		return nil, fmt.Errorf("did not find the TenantService corresponding to ServiceID(%s)", serviceID)
	}

	tenant, err := dbmanager.TenantDao().GetTenantByUUID(service.TenantID)
	if err != nil {
		return nil, fmt.Errorf("find tenant error. %v", err.Error())
	}
	if tenant == nil {
		return nil, fmt.Errorf("did not find the Tenant corresponding to ServiceID(%s)", serviceID)
	}

	return &AppServiceBuild{
		serviceID:       serviceID,
		dbmanager:       dbmanager,
		service:         service,
		tenant:          tenant,
		replicationType: replicationType,
		appService:      as,
	}, nil
}

// Build builds service, ingress and secret for each port
func (a *AppServiceBuild) Build(as *v1.AppService) (*v1.K8sResources, error) {
	ports, err := a.dbmanager.TenantServicesPortDao().GetPortsByServiceID(a.serviceID)
	if err != nil {
		return nil, fmt.Errorf("find service port from db error %s", err.Error())
	}
	crt, err := checkUpstreamPluginRelation(a.serviceID, a.dbmanager)
	if err != nil {
		return nil, fmt.Errorf("get service upstream plugin relation error, %s", err.Error())
	}
	pp := make(map[int32]int)
	if crt {
		pluginPorts, err := a.dbmanager.TenantServicesStreamPluginPortDao().GetPluginMappingPorts(
			a.serviceID)
		if err != nil {
			return nil, fmt.Errorf("find upstream plugin mapping port error, %s", err.Error())
		}
		ports, pp, err = a.CreateUpstreamPluginMappingPort(ports, pluginPorts)
		if err != nil {
			logrus.Errorf("create mapping port failure %s", err.Error())
		}
	}

	var services []*corev1.Service
	var ingresses []interface{}
	var apiSixRoutes []*v2.ApisixRoute
	var secrets []*corev1.Secret
	var innerService []*model.TenantServicesPort

	if len(ports) > 0 {
		for i := range ports {
			port := ports[i]
			if *port.IsInnerService {
				innerService = append(innerService, port)
			}
			if *port.IsOuterService {
				route := a.generateOuterDomain(as, port)
				if route != nil {
					apiSixRoutes = append(apiSixRoutes, route)
				}
			}
		}
	}
	if len(innerService) > 0 {
		services = append(services, a.createInnerService(innerService))
	}
	// build stateful service
	if a.replicationType == model.TypeStatefulSet {
		services = append(services, a.createStatefulService(ports))
	}
	if crt {
		services, _ = a.CreateUpstreamPluginMappingService(services, pp)
	}

	return &v1.K8sResources{
		Services:    services,
		Secrets:     secrets,
		Ingresses:   ingresses,
		ApiSixRoute: apiSixRoutes,
	}, nil
}

// CreateUpstreamPluginMappingPort 检查是否存在upstream插件，接管入口网络
func (a *AppServiceBuild) CreateUpstreamPluginMappingPort(
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

// CreateUpstreamPluginMappingService 增加service plugin mapport 标签
func (a *AppServiceBuild) CreateUpstreamPluginMappingService(services []*corev1.Service,
	pp map[int32]int) ([]*corev1.Service, error) {
	for _, service := range services {
		logrus.Debugf("map is %v, port is %v, origin_port is %d",
			pp,
			service.Spec.Ports[0].Port,
			pp[service.Spec.Ports[0].Port])
		service.Labels["origin_port"] = fmt.Sprintf("%d", pp[service.Spec.Ports[0].Port])
	}
	return services, nil
}

// BuildOnPort 指定端口创建Service
func (a *AppServiceBuild) BuildOnPort(p int, isOut bool) (*corev1.Service, error) {
	port, err := a.dbmanager.TenantServicesPortDao().GetPort(a.serviceID, p)
	if err != nil {
		return nil, fmt.Errorf("find service port from db error %s", err.Error())
	}
	if port != nil {
		if !isOut && *port.IsInnerService {
			return a.createInnerService([]*model.TenantServicesPort{port}), nil
		}
	}
	return nil, fmt.Errorf("tenant service port %d is not exist", p)
}

// createServiceAnnotations create service annotation
func (a *AppServiceBuild) createServiceAnnotations() map[string]string {
	var annotations = make(map[string]string)
	if a.service.Replicas <= 1 {
		annotations["rainbond.com/tolerate-unready-endpoints"] = "true"
	}
	return annotations
}

func (a *AppServiceBuild) createInnerService(ports []*model.TenantServicesPort) *corev1.Service {
	var service corev1.Service
	service.Name = ports[0].K8sServiceName
	service.Namespace = a.appService.GetNamespace()
	service.Labels = a.appService.GetCommonLabels(map[string]string{
		"service_type": "inner",
		"name":         a.service.ServiceAlias + "Service",
		"version":      a.service.DeployVersion,
	})
	if a.service.Replicas <= 1 {
		service.Labels["rainbond.com/tolerate-unready-endpoints"] = "true"
	}
	service.Annotations = a.createServiceAnnotations()
	var servicePorts []corev1.ServicePort
	for _, port := range ports {
		var servicePort corev1.ServicePort
		if port.Protocol == "udp" {
			servicePort.Protocol = "UDP"
		} else {
			servicePort.Protocol = "TCP"
		}
		servicePort.Name = generateSVCPortName(port.Protocol, port.ContainerPort)
		servicePort.TargetPort = intstr.FromInt(port.ContainerPort)
		servicePort.Port = int32(port.MappingPort)
		if servicePort.Port == 0 {
			servicePort.Port = int32(port.ContainerPort)
		}
		portProtocol := fmt.Sprintf("port_protocol_%v", servicePort.Port)
		service.Labels[portProtocol] = port.Protocol
		servicePorts = append(servicePorts, servicePort)
	}
	spec := corev1.ServiceSpec{
		Ports: servicePorts,
	}
	if a.appService.ServiceKind != model.ServiceKindThirdParty {
		spec.Selector = map[string]string{"name": a.service.ServiceAlias}
	}
	service.Spec = spec
	return &service
}

func (a *AppServiceBuild) createStatefulService(ports []*model.TenantServicesPort) *corev1.Service {
	var service corev1.Service
	service.Name = a.appService.GetK8sWorkloadName()
	service.Namespace = a.appService.GetNamespace()
	service.Labels = a.appService.GetCommonLabels(map[string]string{
		"service_type": "stateful",
		"name":         a.service.ServiceAlias + "ServiceStateful",
	})
	var serviceports []corev1.ServicePort
	for _, p := range ports {
		var servicePort corev1.ServicePort
		servicePort.Protocol = "TCP"
		servicePort.TargetPort = intstr.FromInt(p.ContainerPort)
		servicePort.Port = int32(p.MappingPort)
		servicePort.Name = generateSVCPortName(string(servicePort.Protocol), p.ContainerPort)
		if servicePort.Port == 0 {
			servicePort.Port = int32(p.ContainerPort)
		}
		serviceports = append(serviceports, servicePort)
	}
	spec := corev1.ServiceSpec{
		Ports:                    serviceports,
		Selector:                 map[string]string{"name": a.service.ServiceAlias},
		ClusterIP:                "None",
		PublishNotReadyAddresses: true,
	}
	service.Spec = spec
	service.Annotations = map[string]string{"service.alpha.kubernetes.io/tolerate-unready-endpoints": "true"}
	return &service
}

func (a *AppServiceBuild) createSecret(rule *model.HTTPRule, name, namespace string, labels map[string]string) (*corev1.Secret, error) {
	if rule.CertificateID == "" {
		return nil, nil
	}
	cert, err := a.dbmanager.CertificateDao().GetCertificateByID(rule.CertificateID)
	if err != nil {
		return nil, fmt.Errorf("cant not get certificate by id(%s): %v", rule.CertificateID, err)
	}
	if cert == nil || strings.TrimSpace(cert.Certificate) == "" || strings.TrimSpace(cert.PrivateKey) == "" {
		return nil, fmt.Errorf("rule id: %s; certificate not found", rule.UUID)
	}
	return &corev1.Secret{
		ObjectMeta: createIngressMeta(name, namespace, labels),
		Data: map[string][]byte{
			"tls.crt": []byte(cert.Certificate),
			"tls.key": []byte(cert.PrivateKey),
		},
		Type: corev1.SecretTypeOpaque,
	}, nil
}

func createIngressMeta(name, namespace string, labels map[string]string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      name,
		Namespace: namespace,
		Labels:    labels,
	}
}

func generateSVCPortName(protocol string, containerPort int) string {
	protocols := map[string]struct{}{
		"http":  {},
		"https": {},
		"tcp":   {},
		"grpc":  {},
		"udp":   {},
		"mysql": {},
	}
	if _, ok := protocols[strings.ToLower(protocol)]; !ok {
		protocol = "tcp"
	}
	return fmt.Sprintf("%s-%d", strings.ToLower(protocol), containerPort)
}

func (a *AppServiceBuild) generateOuterDomain(as *v1.AppService, port *model.TenantServicesPort) (outerRoutes *v2.ApisixRoute) {
	httpRules, err := a.dbmanager.HTTPRuleDao().GetHTTPRuleByServiceIDAndContainerPort(as.ServiceID, port.ContainerPort)
	if err != nil {
		logrus.Infof("Can't get HTTPRule corresponding to ServiceID(%s): %v", as.ServiceID, err)
	}
	// create http ingresses
	logrus.Debugf("find %d count http rule", len(httpRules))
	if len(httpRules) > 0 {
		httpRule := httpRules[0]
		routes, err := k8s2.Default().ApiSixClient.ApisixV2().ApisixRoutes(as.GetNamespace()).List(
			context.Background(),
			metav1.ListOptions{
				LabelSelector: as.ServiceAlias + "=service_alias" + ",port=" + strconv.Itoa(port.ContainerPort),
			},
		)
		if err != nil {
			logrus.Errorf("generate outer domain list apisixRoute failure: %v", err)
		} else {
			if routes != nil && len(routes.Items) > 0 {
				logrus.Infof("%v route num > 0, not create", as.ServiceAlias)
			} else {
				// 创建 label
				labels := make(map[string]string)
				labels["creator"] = "Rainbond"
				labels["port"] = strconv.Itoa(port.ContainerPort)
				labels["component_sort"] = as.ServiceAlias
				labels["app_id"] = as.AppID
				labels[as.ServiceAlias] = "service_alias"
				labels[httpRule.Domain] = "host"

				routeName := httpRule.Domain + "/*"
				routeName = strings.ReplaceAll(routeName, "/", "p-p")
				routeName = strings.ReplaceAll(routeName, "*", "s-s")
				weight := 100
				apisixRouteHTTP := v2.ApisixRouteHTTP{
					Name: uuid.NewV4().String()[0:8],
					Match: v2.ApisixRouteHTTPMatch{
						Paths: []string{"/*"},
						Hosts: []string{httpRule.Domain},
					},
					Backends: []v2.ApisixRouteHTTPBackend{
						{
							ServiceName: port.K8sServiceName,
							ServicePort: intstr.FromInt(port.ContainerPort),
							Weight:      &weight,
						},
					},
					Authentication: v2.ApisixRouteAuthentication{
						Enable:  false,
						Type:    "basicAuth",
						KeyAuth: v2.ApisixRouteAuthenticationKeyAuth{},
					},
				}
				outerRoutes = &v2.ApisixRoute{
					TypeMeta: metav1.TypeMeta{
						Kind:       util.ApisixRoute,
						APIVersion: util.APIVersion,
					},
					ObjectMeta: metav1.ObjectMeta{
						Labels:       labels,
						Name:         routeName,
						GenerateName: "rbd",
					},
					Spec: v2.ApisixRouteSpec{
						IngressClassName: "apisix",
						HTTP: []v2.ApisixRouteHTTP{
							apisixRouteHTTP,
						},
					},
				}
			}
		}
	}
	return
}
