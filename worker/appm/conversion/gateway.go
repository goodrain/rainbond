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
	"strconv"
	"strings"

	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/gateway/annotations/parser"
	"github.com/goodrain/rainbond/util/k8s"
	v1 "github.com/goodrain/rainbond/worker/appm/types/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	betav1 "k8s.io/api/networking/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
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

	k8s, err := builder.Build()
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

	logrus.Infof("TenantServiceRegist --------------------%v", as)
	return nil
}

//AppServiceBuild has the ability to build k8s service, ingress and secret
type AppServiceBuild struct {
	serviceID, eventID string
	tenant             *model.Tenants
	service            *model.TenantServices
	appService         *v1.AppService
	replicationType    string
	dbmanager          db.Manager
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
func (a *AppServiceBuild) Build() (*v1.K8sResources, error) {
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
	var secrets []*corev1.Secret
	if len(ports) > 0 {
		for i := range ports {
			port := ports[i]
			if *port.IsInnerService {
				switch a.appService.GovernanceMode {
				case model.GovernanceModeKubernetesNativeService:
					services = append(services, a.createKubernetesNativeService(port))
				default:
					services = append(services, a.createInnerService(port))
				}
			}
			if *port.IsOuterService {
				service := a.createOuterService(port)
				services = append(services, service)
				relContainerPort := pp[int32(port.ContainerPort)]
				if relContainerPort == 0 {
					relContainerPort = port.ContainerPort
				}
				ings, secrs, err := a.ApplyRules(port.ServiceID, relContainerPort, port.ContainerPort, service)
				if err != nil {
					logrus.Errorf("error applying rules: %s", err.Error())
					return nil, err
				}
				ingresses = append(ingresses, ings...)
				if secrs != nil {
					secrets = append(secrets, secrs...)
				}
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

	logrus.Infof("AppServiceBuild build ingresses %v", ingresses)
	return &v1.K8sResources{
		Services:  services,
		Secrets:   secrets,
		Ingresses: ingresses,
	}, nil
}

// ApplyRules applies http rules and tcp rules
func (a AppServiceBuild) ApplyRules(serviceID string, containerPort, pluginContainerPort int,
	service *corev1.Service) ([]interface{}, []*corev1.Secret, error) {
	var ingresses []interface{}
	var secrets []*corev1.Secret
	httpRules, err := a.dbmanager.HTTPRuleDao().GetHTTPRuleByServiceIDAndContainerPort(serviceID, containerPort)
	if err != nil {
		logrus.Infof("Can't get HTTPRule corresponding to ServiceID(%s): %v", serviceID, err)
	}
	// create http ingresses
	logrus.Debugf("find %d count http rule", len(httpRules))
	if len(httpRules) > 0 {
		for _, httpRule := range httpRules {
			ingress, sec, err := a.applyHTTPRule(httpRule, containerPort, pluginContainerPort, service)
			if err != nil {
				logrus.Errorf("Unexpected error occurred while applying http rule: %v", err)
				// skip the failed rule
				continue
			}
			//logrus.Debugf("create ingress %s", ingress.Name)
			ingresses = append(ingresses, ingress)
			if sec != nil {
				secrets = append(secrets, sec)
			}

		}
	}

	// create tcp ingresses
	tcpRules, err := a.dbmanager.TCPRuleDao().GetTCPRuleByServiceIDAndContainerPort(serviceID, containerPort)
	if err != nil {
		logrus.Infof("Can't get TCPRule corresponding to ServiceID(%s): %v", serviceID, err)
	}
	if len(tcpRules) > 0 {
		for _, tcpRule := range tcpRules {
			ing, err := a.applyTCPRule(tcpRule, service, a.appService.GetNamespace())
			if err != nil {
				logrus.Errorf("Unexpected error occurred while applying tcp rule: %v", err)
				// skip the failed rule
				continue
			}
			ingresses = append(ingresses, ing)
		}
	}

	return ingresses, secrets, nil
}

// applyTCPRule applies stream rule into ingress
func (a *AppServiceBuild) applyHTTPRule(rule *model.HTTPRule, containerPort, pluginContainerPort int,
	service *corev1.Service) (ingress interface{}, sec *corev1.Secret, err error) {
	//deal with empty path and domain
	path := strings.Replace(rule.Path, " ", "", -1)
	if path == "" {
		path = "/"
	}
	domain := strings.Replace(rule.Domain, " ", "", -1)
	if domain == "" {
		domain = createDefaultDomain(a.tenant.Name, a.service.ServiceAlias, containerPort)
	}
	// create ingress
	labels := a.appService.GetCommonLabels()
	name := rule.UUID
	namespace := a.appService.GetNamespace()
	serviceName := service.Name

	logrus.Infof("applyHTTPRule serviceName %s", serviceName)

	// certificate
	sec, err = a.createSecret(rule, name, namespace, labels)
	if err != nil {
		return nil, nil, err
	}

	logrus.Infof("applyHTTPRule createSecret %v:", sec)
	// parse annotations
	annotations, err := a.parseAnnotations(rule)
	if err != nil {
		return nil, nil, err
	}
	logrus.Infof("applyHTTPRule annotations %v:", annotations)

	if k8s.IsHighVersion() {
		ntwIngress := createNtwIngress(domain, path, name, namespace, serviceName, labels, pluginContainerPort)
		if sec != nil {
			ntwIngress.Spec.TLS = []networkingv1.IngressTLS{
				{
					Hosts:      []string{domain},
					SecretName: sec.Name,
				},
			}
		}
		ntwIngress.SetAnnotations(annotations)
		logrus.Infof("applyHTTPRule ntwIngress: %v", ntwIngress)
		return ntwIngress, sec, nil
	}

	beatIngress := createBetaIngress(domain, path, name, namespace, serviceName, labels, pluginContainerPort)
	if sec != nil {
		beatIngress.Spec.TLS = []betav1.IngressTLS{
			{
				Hosts:      []string{domain},
				SecretName: sec.Name,
			},
		}
	}
	beatIngress.SetAnnotations(annotations)
	return beatIngress, sec, nil

}

// applyTCPRule applies stream rule into ingress
func (a *AppServiceBuild) applyTCPRule(rule *model.TCPRule, service *corev1.Service, namespace string) (ingress interface{}, err error) {
	//parse annotations
	annos := make(map[string]string)
	annos[parser.GetAnnotationWithPrefix("l4-enable")] = "true"
	annos[parser.GetAnnotationWithPrefix("l4-host")] = rule.IP
	annos[parser.GetAnnotationWithPrefix("l4-port")] = fmt.Sprintf("%v", rule.Port)

	// create ingress
	objectMeta := createIngressMeta(rule.UUID, namespace, a.appService.GetCommonLabels())
	if k8s.IsHighVersion() {
		nwkIngress := &networkingv1.Ingress{
			ObjectMeta: objectMeta,
			Spec: networkingv1.IngressSpec{
				DefaultBackend: &networkingv1.IngressBackend{
					Service: &networkingv1.IngressServiceBackend{
						Name: service.Name,
						Port: networkingv1.ServiceBackendPort{
							Number: service.Spec.Ports[0].Port,
						},
					},
				},
			},
		}
		nwkIngress.SetAnnotations(annos)
		return nwkIngress, nil
	}

	betaIngress := &betav1.Ingress{
		ObjectMeta: objectMeta,
		Spec: betav1.IngressSpec{
			Backend: &betav1.IngressBackend{
				ServiceName: service.Name,
				ServicePort: intstr.FromInt(int(service.Spec.Ports[0].Port)),
			},
		},
	}
	betaIngress.SetAnnotations(annos)
	return betaIngress, nil
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
		if !isOut && *port.IsInnerService {
			return a.createInnerService(port), nil
		}
		if isOut && *port.IsOuterService {
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

func (a *AppServiceBuild) createKubernetesNativeService(port *model.TenantServicesPort) *corev1.Service {
	svc := a.createInnerService(port)
	svc.Name = port.K8sServiceName
	if svc.Name == "" {
		svc.Name = fmt.Sprintf("%s-%d", a.appService.GetK8sWorkloadName(), port.ContainerPort)
	}
	return svc
}

func (a *AppServiceBuild) createInnerService(port *model.TenantServicesPort) *corev1.Service {
	var service corev1.Service
	service.Name = port.K8sServiceName
	if service.Name == "" {
		service.Name = fmt.Sprintf("%s-%d-%d", a.appService.GetK8sWorkloadName(), port.ID, port.ContainerPort)
	}
	service.Namespace = a.appService.GetNamespace()
	service.Labels = a.appService.GetCommonLabels(map[string]string{
		"service_type":  "inner",
		"name":          a.service.ServiceAlias + "Service",
		"port_protocol": port.Protocol,
		"service_port":  strconv.Itoa(port.ContainerPort),
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
	servicePort.Name = generateSVCPortName(port.Protocol, port.ContainerPort)
	servicePort.TargetPort = intstr.FromInt(port.ContainerPort)
	servicePort.Port = int32(port.MappingPort)
	if servicePort.Port == 0 {
		servicePort.Port = int32(port.ContainerPort)
	}
	spec := corev1.ServiceSpec{
		Ports: []corev1.ServicePort{servicePort},
	}
	if a.appService.ServiceKind != model.ServiceKindThirdParty {
		spec.Selector = map[string]string{"name": a.service.ServiceAlias}
	}
	service.Spec = spec
	return &service
}

func (a *AppServiceBuild) createOuterService(port *model.TenantServicesPort) *corev1.Service {
	var service corev1.Service
	service.Name = fmt.Sprintf("%s-%d-%dout", a.appService.GetK8sWorkloadName(), port.ID, port.ContainerPort)
	service.Namespace = a.appService.GetNamespace()
	service.Labels = a.appService.GetCommonLabels(map[string]string{
		"service_type":  "outer",
		"name":          a.service.ServiceAlias + "ServiceOUT",
		"tenant_name":   a.tenant.Name,
		"protocol":      port.Protocol,
		"port_protocol": port.Protocol,
		"service_port":  strconv.Itoa(port.ContainerPort),
		"event_id":      a.eventID,
		"version":       a.service.DeployVersion,
	})
	if a.service.Replicas <= 1 {
		service.Labels["rainbond.com/tolerate-unready-endpoints"] = "true"
	}
	var servicePort corev1.ServicePort
	servicePort.Protocol = conversionPortProtocol(port.Protocol)
	servicePort.TargetPort = intstr.FromInt(port.ContainerPort)
	servicePort.Name = generateSVCPortName(port.Protocol, port.ContainerPort)
	servicePort.Port = int32(port.ContainerPort)
	portType := corev1.ServiceTypeClusterIP
	spec := corev1.ServiceSpec{
		Ports: []corev1.ServicePort{servicePort},
		Type:  portType,
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

func (a *AppServiceBuild) parseAnnotations(rule *model.HTTPRule) (map[string]string, error) {
	annos := make(map[string]string)
	// weight
	if rule.Weight > 1 {
		annos[parser.GetAnnotationWithPrefix("weight")] = fmt.Sprintf("%d", rule.Weight)
	}
	// header
	if rule.Header != "" {
		annos[parser.GetAnnotationWithPrefix("header")] = rule.Header
	}
	// cookie
	if rule.Cookie != "" {
		annos[parser.GetAnnotationWithPrefix("cookie")] = rule.Cookie
	}

	// path-rewrite
	if rule.PathRewrite {
		annos[parser.GetAnnotationWithPrefix("path-rewrite")] = "true"
	}
	httpRuleRewrites, err := a.dbmanager.HTTPRuleRewriteDao().ListByHTTPRuleID(rule.UUID)
	if err != nil {
		return nil, err
	}
	for i, rewrite := range httpRuleRewrites {
		annos[parser.GetAnnotationWithPrefix(fmt.Sprintf("rewrite-%d-regex", i))] = rewrite.Regex
		annos[parser.GetAnnotationWithPrefix(fmt.Sprintf("rewrite-%d-replacement", i))] = rewrite.Replacement
		annos[parser.GetAnnotationWithPrefix(fmt.Sprintf("rewrite-%d-flag", i))] = rewrite.Flag
	}

	// rule extension
	ruleExtensions, err := a.dbmanager.RuleExtensionDao().GetRuleExtensionByRuleID(rule.UUID)
	if err != nil {
		return nil, err
	}
	for _, extension := range ruleExtensions {
		switch extension.Key {
		case string(model.HTTPToHTTPS):
			if rule.CertificateID == "" {
				logrus.Warningf("enable force-ssl-redirect, but with no certificate. rule id is: %s", rule.UUID)
				break
			}
			annos[parser.GetAnnotationWithPrefix("force-ssl-redirect")] = "true"
		case string(model.LBType):
			if strings.HasPrefix(extension.Value, "upstream-hash-by") {
				s := strings.Split(extension.Value, ":")
				if len(s) < 2 {
					logrus.Warningf("invalid extension value for upstream-hash-by: %s", extension.Value)
					break
				}
				annos[parser.GetAnnotationWithPrefix("upstream-hash-by")] = s[1]
				break
			}
			annos[parser.GetAnnotationWithPrefix("lb-type")] = extension.Value

		default:
			logrus.Warnf("Unexpected RuleExtension Key: %s", extension.Key)
		}
	}

	configs, err := db.GetManager().GwRuleConfigDao().ListByRuleID(rule.UUID)
	if err != nil {
		return nil, err
	}
	if len(configs) > 0 {
		for _, cfg := range configs {
			annos[parser.GetAnnotationWithPrefix(cfg.Key)] = cfg.Value
		}
	}
	return annos, nil
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

func createNtwIngress(domain, path, name, namespace, serviceName string, labels map[string]string, pluginContainerPort int) *networkingv1.Ingress {
	logrus.Infof("start create networking ingress,serviceName is %s", serviceName)
	return &networkingv1.Ingress{
		ObjectMeta: createIngressMeta(name, namespace, labels),
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					Host: domain,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     path,
									PathType: k8s.IngressPathType(networkingv1.PathTypeExact),
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: serviceName,
											Port: networkingv1.ServiceBackendPort{
												Number: int32(pluginContainerPort),
											},
										},
									},
								},
							},
						},
					},
				},
			},
			DefaultBackend: &networkingv1.IngressBackend{
				Service: &networkingv1.IngressServiceBackend{
					Name: serviceName,
					Port: networkingv1.ServiceBackendPort{
						Number: int32(pluginContainerPort),
					},
				},
			},
		},
	}
}

func createBetaIngress(domain, path, name, namespace, serviceName string, labels map[string]string, pluginContainerPort int) *betav1.Ingress {
	logrus.Infof("start create beta ingress,serviceName is %s", serviceName)
	betaIngress := &betav1.Ingress{
		ObjectMeta: createIngressMeta(name, namespace, labels),
		Spec: betav1.IngressSpec{
			Rules: []betav1.IngressRule{
				{
					Host: domain,
					IngressRuleValue: betav1.IngressRuleValue{
						HTTP: &betav1.HTTPIngressRuleValue{
							Paths: []betav1.HTTPIngressPath{
								{
									Path: path,
									Backend: betav1.IngressBackend{
										ServiceName: serviceName,
										ServicePort: intstr.FromInt(pluginContainerPort),
									},
								},
							},
						},
					},
				},
			},
		},
	}
	return betaIngress
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
