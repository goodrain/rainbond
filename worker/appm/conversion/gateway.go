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

package conversion

import (
	"fmt"
	"os"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/gateway/annotations/parser"
	"github.com/twinj/uuid"
	corev1 "k8s.io/api/core/v1"
	extensions "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/worker/appm/types/v1"
)

//createDefaultDomain create default domain
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

//TenantServiceRegist conv inner and outer service regist
func TenantServiceRegist(as *v1.AppService, dbmanager db.Manager) error {
	builder, err := AppServiceBuilder(as.ServiceID, string(as.ServiceType), dbmanager, as)
	if err != nil {
		logrus.Error("create k8s service builder error.", err.Error())
		return err
	}

	svcs, ings, secs, err := builder.Build()
	if err != nil {
		logrus.Error("build k8s services error.", err.Error())
		return err
	}
	for _, service := range svcs {
		as.SetService(service)
	}
	for _, ing := range ings {
		as.SetIngress(ing)
	}
	for _, sec := range secs {
		as.SetSecrets(sec)
	}

	return nil
}

//AppServiceBuild has the ability to build k8s service, ingress and secret
type AppServiceBuild struct {
	serviceID, eventID string
	service            *model.TenantServices
	tenant             *model.Tenants
	dbmanager          db.Manager
	logger             event.Logger
	replicationType    string
	appService         *v1.AppService
}

//AppServiceBuilder returns a AppServiceBuild
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

//Build builds service, ingress and secret for each port
func (a *AppServiceBuild) Build() ([]*corev1.Service, []*extensions.Ingress, []*corev1.Secret, error) {
	ports, err := a.dbmanager.TenantServicesPortDao().GetPortsByServiceID(a.serviceID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("find service port from db error %s", err.Error())
	}
	crt, err := a.checkUpstreamPluginRelation()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("get service upstream plugin relation error, %s", err.Error())
	}
	pp := make(map[int32]int)
	if crt {
		pluginPorts, err := a.dbmanager.TenantServicesStreamPluginPortDao().GetPluginMappingPorts(
			a.serviceID,
			model.UpNetPlugin,
		)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("find upstream plugin mapping port error, %s", err.Error())
		}
		ports, pp, err = a.CreateUpstreamPluginMappingPort(ports, pluginPorts)
	}
	if err != nil {
		return nil, nil, nil, fmt.Errorf("create upstream port error, %s", err.Error())
	}
	var services []*corev1.Service
	var ingresses []*extensions.Ingress
	var secrets []*corev1.Secret
	if ports != nil && len(ports) > 0 {
		for i := range ports {
			port := ports[i]
			if port.IsInnerService {
				services = append(services, a.createInnerService(port))
			}
			if port.IsOuterService {
				service := a.createOuterService(port)

				ings, secret, err := a.ApplyRules(port, service)
				ingresses = append(ingresses, ings...)
				secrets = append(secrets, secret)
				if err != nil {
					return nil, nil, nil, err
				}

				services = append(services, service)
			}
		}
	}

	// build stateful service
	if a.replicationType == model.TypeStatefulSet {
		services = append(services, a.createStatefulService(ports))
	}
	if crt {
		services, _ = a.CreateUpstreamPluginMappingService(services, pp)
	}

	return services, ingresses, secrets, nil
}

func (a AppServiceBuild) ApplyRules(port *model.TenantServicesPort, service *corev1.Service) ([]*extensions.Ingress, *corev1.Secret, error) {
	httpRule, err := a.dbmanager.HttpRuleDao().GetHttpRuleByServiceIDAndContainerPort(port.ServiceID,
		port.ContainerPort)
	if err != nil {
		logrus.Infof("Can't get HttpRule corresponding to ServiceID(%s): %v", port.ServiceID, err)
	}
	tcpRule, err := a.dbmanager.TcpRuleDao().GetTcpRuleByServiceIDAndContainerPort(port.ServiceID,
		port.ContainerPort)
	if err != nil {
		logrus.Infof("Can't get TcpRule corresponding to ServiceID(%s): %v", port.ServiceID, err)
	}
	if httpRule == nil && tcpRule == nil {
		return nil, nil, fmt.Errorf("Can't find HttpRule or TcpRule for Outer Service(%s)", port.ServiceID)
	}

	// create ingresses
	var ingresses []*extensions.Ingress
	var secret *corev1.Secret
	// http
	if httpRule != nil {
		ing, sec, err := a.applyHttpRule(httpRule, port, service)
		if err != nil {
			return nil, nil, err
		}
		ingresses = append(ingresses, ing)
		secret = sec
	}

	// tcp
	if tcpRule != nil {
		mappingPort, err := a.dbmanager.TenantServiceLBMappingPortDao().CreateTenantServiceLBMappingPort(
			a.serviceID, port.ContainerPort)
		ing, err := applyTcpRule(tcpRule, service, string(mappingPort.Port), a.tenant.UUID)
		if err != nil {
			return nil, nil, err
		}
		ingresses = append(ingresses, ing)
	}

	return ingresses, secret, nil
}

// applyTcpRule applies stream rule into ingress
func (a *AppServiceBuild) applyHttpRule(rule *model.HttpRule, port *model.TenantServicesPort,
	service *corev1.Service) (ing *extensions.Ingress, sec *corev1.Secret, err error) {
	if err != nil {
		return nil, nil, err
	}

	domain := strings.Replace(rule.Domain, " ", "", -1)
	if domain == "" {
		domain = createDefaultDomain(a.tenant.Name, a.service.ServiceAlias, port.ContainerPort)
	}
	path := strings.Replace(rule.Path, " ", "", -1)
	if path == "" {
		path = "/"
	}

	ing = &extensions.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      genIngName("l7", service.Name, path),
			Namespace: a.tenant.UUID,
		},
		Spec: extensions.IngressSpec{
			Rules: []extensions.IngressRule{
				{
					Host: domain,
					IngressRuleValue: extensions.IngressRuleValue{
						HTTP: &extensions.HTTPIngressRuleValue{
							Paths: []extensions.HTTPIngressPath{
								{
									Path: path,
									Backend: extensions.IngressBackend{
										ServiceName: service.Name,
										ServicePort: intstr.FromInt(port.ContainerPort),
									},
								},
							},
						},
					},
				},
			},
		},
	}
	annos := make(map[string]string)
	// load balancer type
	annos[parser.GetAnnotationWithPrefix("load-balancer-type")] = string(rule.LoadBalancerType)
	// header
	if rule.Header != "" {
		annos[parser.GetAnnotationWithPrefix("header")] = rule.Header
	}
	// cookie
	if rule.Cookie != "" {
		annos[parser.GetAnnotationWithPrefix("cookie")] = rule.Cookie
	}
	// certificate
	if rule.CertificateID != "" {
		cert, err := a.dbmanager.CertificateDao().GetCertificateByID(rule.CertificateID)
		if err != nil {
			return nil, nil, fmt.Errorf("Cant not get certificate by id(%s): %v", rule.CertificateID, err)
		}
		// create secret
		sec = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cert.CertificateName,
				Namespace: a.tenant.UUID,
			},
			Data: map[string][]byte{
				"tls.crt": []byte(cert.Certificate),
				"tls.key": []byte(cert.PrivateKey),
			},
			Type: corev1.SecretTypeOpaque,
		}
		ing.Spec.TLS = []extensions.IngressTLS{
			{
				Hosts:      []string{domain},
				SecretName: sec.Name,
			},
		}
	}
	// rule extension

	ruleExtensions, err := a.dbmanager.RuleExtensionDao().GetRuleExtensionByServiceID(a.serviceID)
	if err != nil {
		return nil, nil, err
	}
	for _, extension := range ruleExtensions {
		switch extension.Value {
		case model.HttpToHttpsEV:
			annos[parser.GetAnnotationWithPrefix("force-ssl-redirect")] = "true"
		default:
			logrus.Warnf("Unexpected RuleExtension Value: %s", extension.Value)
		}
	}
	ing.SetAnnotations(annos)

	return ing, sec, nil
}

// applyTcpRule applies stream rule into ingress
func applyTcpRule(
	rule *model.TcpRule,
	service *corev1.Service,
	mappingPort string,
	namespace string) (ing *extensions.Ingress, err error) {
	ing = &extensions.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      genIngName("l4", service.Name, ""),
			Namespace: namespace,
		},
		Spec: extensions.IngressSpec{
			Backend: &extensions.IngressBackend{
				ServiceName: service.Name,
				ServicePort: intstr.FromInt(int(service.Spec.Ports[0].Port)),
			},
		},
	}
	annos := make(map[string]string)
	annos[parser.GetAnnotationWithPrefix("load-balancer-type")] = string(rule.LoadBalancerType)
	annos[parser.GetAnnotationWithPrefix("l4-enable")] = "true"
	annos[parser.GetAnnotationWithPrefix("l4-host")] = rule.IP
	if err != nil {
		return nil, err
	}
	annos[parser.GetAnnotationWithPrefix("l4-port")] = mappingPort
	ing.SetAnnotations(annos)

	return ing, nil
}

// genIngName generates a Ingress name
func genIngName(t string, serviceName string, path string) string {
	if path == "" {
		return fmt.Sprintf("%s-%s--%s", t, serviceName, uuid.NewV4().String()[0:8])
	} else {
		return fmt.Sprintf("%s-%s-%s-%s", t, serviceName, path, uuid.NewV4().String()[0:8])
	}
}

func (a *AppServiceBuild) checkUpstreamPluginRelation() (bool, error) {
	return a.dbmanager.TenantServicePluginRelationDao().CheckSomeModelPluginByServiceID(
		a.serviceID,
		model.UpNetPlugin)
}

//CreateUpstreamPluginMappingPort 检查是否存在upstream插件，接管入口网络
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

//CreateUpstreamPluginMappingService 增加service plugin mapport 标签
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

//BuildOnPort 指定端口创建Service
func (a *AppServiceBuild) BuildOnPort(p int, isOut bool) (*corev1.Service, error) {
	port, err := a.dbmanager.TenantServicesPortDao().GetPort(a.serviceID, p)
	if err != nil {
		return nil, fmt.Errorf("find service port from db error %s", err.Error())
	}
	if port != nil {
		if !isOut && port.IsInnerService {
			return a.createInnerService(port), nil
		}
		if isOut && port.IsOuterService {
			return a.createOuterService(port), nil
		}
	}
	return nil, fmt.Errorf("tenant service port %d is not exist", p)
}

//createServiceAnnotations create service annotation
func (a *AppServiceBuild) createServiceAnnotations() map[string]string {
	var annotations = make(map[string]string)
	if a.service.Replicas <= 1 {
		annotations["rainbond.com/tolerate-unready-endpoints"] = "true"
	}
	return annotations
}

func (a *AppServiceBuild) createInnerService(port *model.TenantServicesPort) *corev1.Service {
	var service corev1.Service
	service.Name = fmt.Sprintf("service-%d-%d", port.ID, port.ContainerPort)
	service.Labels = a.appService.GetCommonLabels(map[string]string{
		"service_type":  "inner",
		"name":          a.service.ServiceAlias + "Service",
		"port_protocol": port.Protocol,
		"creator":       "RainBond",
		"service_id":    a.service.ServiceID,
		"version":       a.service.DeployVersion,
	})
	if a.service.Replicas <= 1 {
		service.Labels["rainbond.com/tolerate-unready-endpoints"] = "true"
	}
	service.Annotations = a.createServiceAnnotations()
	var servicePort corev1.ServicePort
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
	spec := corev1.ServiceSpec{
		Ports:    []corev1.ServicePort{servicePort},
		Selector: map[string]string{"name": a.service.ServiceAlias},
	}
	service.Spec = spec
	return &service
}

func (a *AppServiceBuild) createOuterService(port *model.TenantServicesPort) *corev1.Service {
	var service corev1.Service
	service.Name = fmt.Sprintf("service-%d-%dout", port.ID, port.ContainerPort)
	service.Labels = a.appService.GetCommonLabels(map[string]string{
		"service_type":  "outer",
		"name":          a.service.ServiceAlias + "ServiceOUT",
		"tenant_name":   a.tenant.Name,
		"domain":        a.service.Autodomain(a.tenant.Name, port.ContainerPort),
		"protocol":      port.Protocol,
		"port_protocol": port.Protocol,
		"event_id":      a.eventID,
		"service_id":    a.service.ServiceID,
		"version":       a.service.DeployVersion,
	})
	if a.service.Replicas <= 1 {
		service.Labels["rainbond.com/tolerate-unready-endpoints"] = "true"
	}
	service.Annotations = a.createServiceAnnotations()
	//if port.Protocol == "stream" { //stream 协议获取映射端口
	if port.Protocol != "http" { //stream 协议获取映射端口
		mapPort, err := a.dbmanager.TenantServiceLBMappingPortDao().GetTenantServiceLBMappingPort(a.serviceID, port.ContainerPort)
		if err != nil {
			logrus.Error("get tenant service lb map port error", err.Error())
			service.Labels["lbmap_port"] = "0"
		} else {
			service.Labels["lbmap_port"] = fmt.Sprintf("%d", mapPort.Port)
		}
	}
	var servicePort corev1.ServicePort
	//TODO: udp, tcp
	if port.Protocol == "udp" {
		servicePort.Protocol = "UDP"
	} else {
		servicePort.Protocol = "TCP"
	}
	servicePort.TargetPort = intstr.FromInt(port.ContainerPort)
	servicePort.Port = int32(port.ContainerPort)
	var portType corev1.ServiceType
	if os.Getenv("CUR_NET") == "midonet" {
		portType = corev1.ServiceTypeNodePort
	} else {
		portType = corev1.ServiceTypeClusterIP
	}
	spec := corev1.ServiceSpec{
		Ports:    []corev1.ServicePort{servicePort},
		Selector: map[string]string{"name": a.service.ServiceAlias},
		Type:     portType,
	}
	service.Spec = spec
	return &service
}

func (a *AppServiceBuild) createStatefulService(ports []*model.TenantServicesPort) *corev1.Service {
	var service corev1.Service
	service.Name = a.service.ServiceName
	service.Labels = map[string]string{
		"service_type": "stateful",
		"name":         a.service.ServiceAlias + "ServiceStateful",
		"creator":      "RainBond",
		"service_id":   a.service.ServiceID,
	}
	var serviceports []corev1.ServicePort
	for _, p := range ports {
		var servicePort corev1.ServicePort
		servicePort.Protocol = "TCP"
		servicePort.TargetPort = intstr.FromInt(p.ContainerPort)
		servicePort.Port = int32(p.MappingPort)
		servicePort.Name = fmt.Sprintf("%d-port", p.ID)
		if servicePort.Port == 0 {
			servicePort.Port = int32(p.ContainerPort)
		}
		serviceports = append(serviceports, servicePort)
	}

	spec := corev1.ServiceSpec{
		Ports:     serviceports,
		Selector:  map[string]string{"name": a.service.ServiceAlias},
		ClusterIP: "None",
	}
	service.Spec = spec
	//before k8s 1.8 version, set Annotations for Service.PublishNotReadyAddresses
	service.Annotations = map[string]string{"service.alpha.kubernetes.io/tolerate-unready-endpoints": "true"}
	return &service
}
