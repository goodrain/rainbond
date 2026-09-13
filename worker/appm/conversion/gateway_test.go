package conversion

import (
	"testing"

	"github.com/goodrain/rainbond/db/model"
	appsv1 "github.com/goodrain/rainbond/worker/appm/types/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGeneratedHTTPRouteUsesCommonIdentityLabels(t *testing.T) {
	appService := &appsv1.AppService{
		AppServiceBase: appsv1.AppServiceBase{
			TenantID:     "tenant-id",
			TenantName:   "tenant-name",
			AppID:        "region-app-id",
			ServiceID:    "service-id",
			ServiceAlias: "service-alias",
		},
	}

	labels := generatedHTTPRouteLabels(appService, 8080, "example.com")
	want := map[string]string{
		"creator":        "Rainbond",
		"tenant_id":      "tenant-id",
		"tenant_name":    "tenant-name",
		"app_id":         "region-app-id",
		"service_id":     "service-id",
		"service_alias":  "service-alias",
		"port":           "8080",
		"component_sort": "service-alias",
		"service-alias":  "service_alias",
		"example.com":    "host",
	}
	for key, expected := range want {
		if got := labels[key]; got != expected {
			t.Errorf("label %s = %q; want %q", key, got, expected)
		}
	}
}

// capability_id: rainbond.worker.appm.gateway.reassign-conflicting-nodeport
func TestReassignAllocatedNodePort(t *testing.T) {
	t.Setenv("MIN_LB_PORT", "30000")
	t.Setenv("MAX_LB_PORT", "30010")

	services := []corev1.Service{
		{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "other-namespace",
				Name:      "other-service",
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{{NodePort: 30000}},
			},
		},
	}

	nodePort, changed := reassignAllocatedNodePort(30000, "target-namespace", "grffb6e5-30000", services)

	if !changed {
		t.Fatal("expected conflicting nodePort to be reassigned")
	}
	if nodePort != 30001 {
		t.Fatalf("nodePort = %d, expected 30001", nodePort)
	}
}

// capability_id: rainbond.worker.appm.gateway.reassign-conflicting-nodeport
func TestReassignAllocatedNodePortKeepsCurrentServicePort(t *testing.T) {
	t.Setenv("MIN_LB_PORT", "30000")
	t.Setenv("MAX_LB_PORT", "30010")

	services := []corev1.Service{
		{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "target-namespace",
				Name:      "grffb6e5-30000",
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{{NodePort: 30000}},
			},
		},
	}

	nodePort, changed := reassignAllocatedNodePort(30000, "target-namespace", "grffb6e5-30000", services)

	if changed {
		t.Fatal("expected nodePort owned by the current service to be kept")
	}
	if nodePort != 30000 {
		t.Fatalf("nodePort = %d, expected 30000", nodePort)
	}
}

// capability_id: rainbond.worker.appm.gateway.vm-nodeport-service-uses-local-external-traffic-policy
func TestOuterServiceExternalTrafficPolicyForVM(t *testing.T) {
	service := &model.TenantServices{ExtendMethod: model.ServiceTypeVM.String()}

	if got := outerServiceExternalTrafficPolicy(service); got != corev1.ServiceExternalTrafficPolicyLocal {
		t.Fatalf("expected VM outer service traffic policy %q, got %q", corev1.ServiceExternalTrafficPolicyLocal, got)
	}
}

// capability_id: rainbond.worker.appm.gateway.vm-nodeport-service-uses-local-external-traffic-policy
func TestOuterServiceExternalTrafficPolicyForNonVM(t *testing.T) {
	service := &model.TenantServices{ExtendMethod: model.ServiceTypeStatelessMultiple.String()}

	if got := outerServiceExternalTrafficPolicy(service); got != corev1.ServiceExternalTrafficPolicyCluster {
		t.Fatalf("expected non-VM outer service traffic policy %q, got %q", corev1.ServiceExternalTrafficPolicyCluster, got)
	}
}
