package conversion

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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
