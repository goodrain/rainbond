package thirdcomponent

import (
	"testing"

	"github.com/goodrain/rainbond/pkg/apis/rainbond/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

func TestStaticEndpointPreservesBothTransports(t *testing.T) {
	component := &v1alpha1.ThirdComponent{Spec: v1alpha1.ThirdComponentSpec{
		Ports:          []*v1alpha1.ComponentPort{{Port: 53}},
		EndpointSource: v1alpha1.ThirdComponentEndpointSource{StaticEndpoints: []*v1alpha1.ThirdComponentEndpoint{{Address: "192.0.2.1:5353"}}},
	}}
	service := corev1.Service{Spec: corev1.ServiceSpec{Ports: []corev1.ServicePort{
		{Name: "tcp-53", Port: 53, Protocol: corev1.ProtocolTCP},
		{Name: "udp-53", Port: 53, Protocol: corev1.ProtocolUDP},
	}}}
	endpoints := createEndpointsOnlyOnePort(component, service, []*v1alpha1.ThirdComponentEndpointStatus{{Address: "192.0.2.1:5353", Status: v1alpha1.EndpointReady}})
	if len(endpoints.Subsets) != 1 || len(endpoints.Subsets[0].Ports) != 2 {
		t.Fatalf("missing transport: %#v", endpoints)
	}
	for _, port := range endpoints.Subsets[0].Ports {
		if port.Port != 5353 {
			t.Fatalf("lost endpoint port: %#v", port)
		}
	}
}
