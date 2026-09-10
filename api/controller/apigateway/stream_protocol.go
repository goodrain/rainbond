package apigateway

import (
	"fmt"
	"strconv"
	"strings"

	apimodel "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/util/portprotocol"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func streamPorts(protocol string, port int32, target intstr.IntOrString, nodePort int32) []corev1.ServicePort {
	var ports []corev1.ServicePort
	for _, transport := range portprotocol.Transports(protocol) {
		ports = append(ports, corev1.ServicePort{Name: fmt.Sprintf("%s-%d", strings.ToLower(string(transport)), port), Protocol: transport, Port: port, TargetPort: target, NodePort: nodePort})
	}
	return ports
}

func streamRouteSummary(service corev1.Service) apimodel.TCPRouteServicePort {
	item := apimodel.TCPRouteServicePort{ServiceName: service.Name, ServiceAlias: service.Labels["service_alias"], ServiceID: service.Labels["service_id"], AppID: service.Labels["app_id"], BackendServiceName: service.Annotations["rainbond.com/backend-service"]}
	if len(service.Spec.Ports) == 0 {
		return item
	}
	item.ServicePort = service.Spec.Ports[0]
	item.Name = service.Name
	item.ContainerPort = item.Port
	if port, err := strconv.Atoi(service.Labels["port"]); err == nil {
		item.ContainerPort = int32(port)
	}
	seen := map[corev1.Protocol]bool{}
	for _, port := range service.Spec.Ports {
		protocol := port.Protocol
		if protocol == "" {
			protocol = corev1.ProtocolTCP
		}
		if !seen[protocol] {
			item.Protocols = append(item.Protocols, protocol)
			seen[protocol] = true
		}
	}
	item.Protocol = corev1.Protocol(strings.ToUpper(portprotocol.Canonical(item.Protocols)))
	return item
}

func backendSupportsStream(service *corev1.Service, port int32, protocol string) bool {
	available := []corev1.Protocol{}
	for _, p := range service.Spec.Ports {
		if p.Port == port {
			available = append(available, p.Protocol)
		}
	}
	return len(available) > 0 && portprotocol.Allows(portprotocol.Canonical(available), protocol)
}
