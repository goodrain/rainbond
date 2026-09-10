package apigateway

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestStreamPortsShareNodePortWithDistinctNames(t *testing.T) {
	ports := streamPorts("tcp+udp", 53, intstr.FromInt(53), 30030)
	if len(ports) != 2 {
		t.Fatalf("want TCP and UDP, got %v", ports)
	}
	if ports[0].Name == ports[1].Name {
		t.Fatal("mixed ports must have unique names")
	}
	for _, p := range ports {
		if p.NodePort != 30030 || p.Port != 53 || p.TargetPort.IntVal != 53 {
			t.Fatalf("incorrect mapping: %v", p)
		}
	}
}
func TestStreamRouteSummaryPreservesIdentityAndProtocols(t *testing.T) {
	svc := corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "dns-30030", Labels: map[string]string{"service_id": "component", "port": "53"}}, Spec: corev1.ServiceSpec{Ports: []corev1.ServicePort{
		{Name: "tcp-53", Protocol: corev1.ProtocolTCP, Port: 53, NodePort: 30030},
		{Name: "udp-53", Protocol: corev1.ProtocolUDP, Port: 53, NodePort: 30030},
	}}}
	got := streamRouteSummary(svc)
	if got.Name != svc.Name || got.ServiceName != svc.Name || got.Protocol != "TCP+UDP" || len(got.Protocols) != 2 {
		t.Fatalf("unexpected summary: %#v", got)
	}
	if got.NodePort != 30030 || got.ContainerPort != 53 {
		t.Fatalf("incorrect ports: %#v", got)
	}
}
