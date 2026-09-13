package conversion

import (
	"fmt"
	"testing"

	"k8s.io/apimachinery/pkg/util/validation"

	"github.com/goodrain/rainbond/db/model"
	appv1 "github.com/goodrain/rainbond/worker/appm/types/v1"
	corev1 "k8s.io/api/core/v1"
)

func TestMixedInternalAndHeadlessPorts(t *testing.T) {
	as := &appv1.AppService{}
	as.ServiceAlias = "dns"
	as.SetTenant(&corev1.Namespace{})
	builder := &AppServiceBuild{service: &model.TenantServices{ServiceAlias: "dns"}, appService: as}
	ports := []*model.TenantServicesPort{{ContainerPort: 53, Protocol: "tcp+udp", K8sServiceName: "dns"}}
	for _, svc := range []*corev1.Service{builder.createInnerService(ports), builder.createStatefulService(ports)} {
		for key, value := range svc.Labels {
			if errors := validation.IsValidLabelValue(value); len(errors) > 0 {
				t.Fatalf("invalid label %s=%s: %v", key, value, errors)
			}
		}
		if len(svc.Spec.Ports) != 2 {
			t.Fatalf("expected two transports: %#v", svc.Spec.Ports)
		}
		if svc.Spec.Ports[0].Protocol != corev1.ProtocolTCP || svc.Spec.Ports[1].Protocol != corev1.ProtocolUDP {
			t.Fatalf("wrong protocols: %#v", svc.Spec.Ports)
		}
		if svc.Spec.Ports[0].Name == svc.Spec.Ports[1].Name {
			t.Fatal("duplicate port names")
		}
	}
}

func TestRestoreThreeIndependentMappingProtocols(t *testing.T) {
	as := &appv1.AppService{}
	as.ServiceAlias = "grf9ce55"
	as.SetTenant(&corev1.Namespace{})
	builder := &AppServiceBuild{service: &model.TenantServices{ServiceAlias: as.ServiceAlias}, appService: as}
	port := &model.TenantServicesPort{ContainerPort: 53, Protocol: "tcp+udp", K8sServiceName: "demo-2048"}
	for _, tc := range []struct {
		port     int
		protocol string
		count    int
	}{{30010, "tcp", 1}, {30020, "udp", 1}, {30030, "tcp+udp", 2}} {
		svc := builder.nodePortService(as, port, &model.TCPRule{ContainerPort: 53, Port: tc.port, Protocol: tc.protocol})
		if want := fmt.Sprintf("demo-2048-%d", tc.port); svc.Name != want {
			t.Fatalf("expected configured service name %q, got %q", want, svc.Name)
		}
		if svc.Spec.Selector["service_alias"] != "grf9ce55" {
			t.Fatalf("naming must not change the pod selector: %#v", svc.Spec.Selector)
		}
		if len(svc.Spec.Ports) != tc.count {
			t.Fatalf("wrong transport count: %#v", svc.Spec.Ports)
		}
		for _, sp := range svc.Spec.Ports {
			if int(sp.NodePort) != tc.port {
				t.Fatalf("lost external port: %#v", sp)
			}
		}
		if tc.protocol == "udp" && svc.Spec.Ports[0].Protocol != corev1.ProtocolUDP {
			t.Fatal("restored UDP as TCP")
		}
	}
}

func TestNodePortServiceNameFallsBackToLegacyAlias(t *testing.T) {
	as := &appv1.AppService{}
	as.ServiceAlias = "grf9ce55"
	as.SetTenant(&corev1.Namespace{})
	builder := &AppServiceBuild{service: &model.TenantServices{ServiceAlias: as.ServiceAlias}, appService: as}
	service := builder.nodePortService(as, &model.TenantServicesPort{ContainerPort: 8081},
		&model.TCPRule{ContainerPort: 8081, Port: 30001, Protocol: "udp"})
	if service.Name != "grf9ce55-30001" {
		t.Fatalf("legacy port without a configured service name lost its alias: %s", service.Name)
	}
}
