package controller

import (
	"context"
	"errors"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func generatedService(name, serviceID string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
			Labels: map[string]string{
				"creator":    "Rainbond",
				"service_id": serviceID,
			},
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: map[string]string{"name": "java"},
			Ports: []corev1.ServicePort{
				{
					Name:       "http-8080",
					Protocol:   corev1.ProtocolTCP,
					Port:       8080,
					TargetPort: intstr.FromInt(8080),
				},
			},
		},
	}
}

func TestCreateKubeServiceCreatesMissingService(t *testing.T) {
	client := k8sfake.NewSimpleClientset()
	desired := generatedService("java", "service-1")

	if err := CreateKubeService(client, desired.Namespace, desired); err != nil {
		t.Fatalf("create service: %v", err)
	}

	got, err := client.CoreV1().Services(desired.Namespace).Get(context.Background(), desired.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get created service: %v", err)
	}
	if got.Labels["service_id"] != "service-1" {
		t.Fatalf("expected managed service label, got %#v", got.Labels)
	}
}

// capability_id: rainbond.lifecycle.service-reconcile-preserves-identity
func TestCreateKubeServiceReconcilesOwnedServiceAndPreservesIdentity(t *testing.T) {
	policy := corev1.IPFamilyPolicySingleStack
	existing := generatedService("java", "service-1")
	existing.UID = types.UID("service-uid")
	existing.ResourceVersion = "7"
	existing.Labels["external-label"] = "keep"
	existing.Annotations = map[string]string{"cloud.example.com/allocated": "keep"}
	existing.Spec.Type = corev1.ServiceTypeLoadBalancer
	existing.Spec.ClusterIP = "10.0.0.8"
	existing.Spec.ClusterIPs = []string{"10.0.0.8"}
	existing.Spec.IPFamilies = []corev1.IPFamily{corev1.IPv4Protocol}
	existing.Spec.IPFamilyPolicy = &policy
	existing.Spec.HealthCheckNodePort = 32001
	existing.Spec.Ports[0].NodePort = 30080

	desired := generatedService("java", "service-1")
	desired.Labels["version"] = "v2"
	desired.Annotations = map[string]string{"rainbond.com/tolerate-unready-endpoints": "true"}
	desired.Spec.Type = corev1.ServiceTypeLoadBalancer
	desired.Spec.Selector = map[string]string{"name": "java-v2"}
	desired.Spec.Ports[0].Port = 8081
	desired.Spec.Ports[0].TargetPort = intstr.FromInt(9090)

	client := k8sfake.NewSimpleClientset(existing)
	if err := CreateKubeService(client, desired.Namespace, desired); err != nil {
		t.Fatalf("reconcile service: %v", err)
	}

	got, err := client.CoreV1().Services(desired.Namespace).Get(context.Background(), desired.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get reconciled service: %v", err)
	}
	if got.UID != existing.UID || got.ResourceVersion != existing.ResourceVersion {
		t.Fatalf("expected service identity to be preserved, got uid=%q resourceVersion=%q", got.UID, got.ResourceVersion)
	}
	if got.Spec.ClusterIP != existing.Spec.ClusterIP || len(got.Spec.ClusterIPs) != 1 || got.Spec.ClusterIPs[0] != existing.Spec.ClusterIPs[0] {
		t.Fatalf("expected cluster IP allocation to be preserved, got clusterIP=%q clusterIPs=%v", got.Spec.ClusterIP, got.Spec.ClusterIPs)
	}
	if got.Spec.IPFamilyPolicy == nil || *got.Spec.IPFamilyPolicy != policy || len(got.Spec.IPFamilies) != 1 || got.Spec.IPFamilies[0] != corev1.IPv4Protocol {
		t.Fatalf("expected IP family allocation to be preserved, got policy=%v families=%v", got.Spec.IPFamilyPolicy, got.Spec.IPFamilies)
	}
	if got.Spec.HealthCheckNodePort != existing.Spec.HealthCheckNodePort || got.Spec.Ports[0].NodePort != existing.Spec.Ports[0].NodePort {
		t.Fatalf("expected node port allocations to be preserved, got health=%d nodePort=%d", got.Spec.HealthCheckNodePort, got.Spec.Ports[0].NodePort)
	}
	if got.Spec.Ports[0].Port != 8081 || got.Spec.Ports[0].TargetPort != intstr.FromInt(9090) {
		t.Fatalf("expected desired port configuration, got %#v", got.Spec.Ports[0])
	}
	if got.Spec.Selector["name"] != "java-v2" || got.Labels["version"] != "v2" {
		t.Fatalf("expected desired managed fields, got selector=%v labels=%v", got.Spec.Selector, got.Labels)
	}
	if got.Labels["external-label"] != "keep" || got.Annotations["cloud.example.com/allocated"] != "keep" {
		t.Fatalf("expected external metadata to be preserved, got labels=%v annotations=%v", got.Labels, got.Annotations)
	}
	if got.Annotations["rainbond.com/tolerate-unready-endpoints"] != "true" {
		t.Fatalf("expected desired annotation, got %v", got.Annotations)
	}
}

// capability_id: rainbond.lifecycle.service-reconcile-rejects-foreign-owner
func TestCreateKubeServiceRejectsForeignService(t *testing.T) {
	existing := generatedService("java", "other-service")
	desired := generatedService("java", "service-1")
	client := k8sfake.NewSimpleClientset(existing)

	err := CreateKubeService(client, desired.Namespace, desired)
	if err == nil || !strings.Contains(err.Error(), "other-service") {
		t.Fatalf("expected ownership conflict, got %v", err)
	}
}

func TestCreateKubeServiceRejectsMissingOwnership(t *testing.T) {
	desired := generatedService("java", "service-1")
	delete(desired.Labels, serviceIDLabel)
	client := k8sfake.NewSimpleClientset()

	err := CreateKubeService(client, desired.Namespace, desired)
	if err == nil || !strings.Contains(err.Error(), serviceIDLabel) {
		t.Fatalf("expected missing ownership error, got %v", err)
	}
}

func TestCreateKubeServiceRejectsHeadlessIdentityChange(t *testing.T) {
	existing := generatedService("java", "service-1")
	existing.Spec.ClusterIP = "10.0.0.8"
	desired := generatedService("java", "service-1")
	desired.Spec.ClusterIP = corev1.ClusterIPNone
	client := k8sfake.NewSimpleClientset(existing)

	err := CreateKubeService(client, desired.Namespace, desired)
	if err == nil || !strings.Contains(err.Error(), "cluster IP mode") {
		t.Fatalf("expected immutable cluster IP mode error, got %v", err)
	}
}

func TestCreateKubeServiceReturnsUpdateError(t *testing.T) {
	existing := generatedService("java", "service-1")
	desired := generatedService("java", "service-1")
	client := k8sfake.NewSimpleClientset(existing)
	client.PrependReactor("update", "services", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("update service failed")
	})

	err := CreateKubeService(client, desired.Namespace, desired)
	if err == nil || !strings.Contains(err.Error(), "update service failed") {
		t.Fatalf("expected update error, got %v", err)
	}
}

func TestCreateKubeServiceDropsNodePortWhenChangingToClusterIP(t *testing.T) {
	existing := generatedService("java", "service-1")
	existing.Spec.Type = corev1.ServiceTypeNodePort
	existing.Spec.ClusterIP = "10.0.0.8"
	existing.Spec.Ports[0].NodePort = 30080
	desired := generatedService("java", "service-1")
	client := k8sfake.NewSimpleClientset(existing)

	if err := CreateKubeService(client, desired.Namespace, desired); err != nil {
		t.Fatalf("reconcile service type: %v", err)
	}
	got, err := client.CoreV1().Services(desired.Namespace).Get(context.Background(), desired.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get reconciled service: %v", err)
	}
	if got.Spec.Ports[0].NodePort != 0 {
		t.Fatalf("expected node port to be cleared for ClusterIP service, got %d", got.Spec.Ports[0].NodePort)
	}
}

func TestCreateKubeServiceRetriesUpdateConflict(t *testing.T) {
	existing := generatedService("java", "service-1")
	desired := generatedService("java", "service-1")
	desired.Labels["version"] = "v2"
	client := k8sfake.NewSimpleClientset(existing)
	updateAttempts := 0
	client.PrependReactor("update", "services", func(k8stesting.Action) (bool, runtime.Object, error) {
		updateAttempts++
		if updateAttempts == 1 {
			return true, nil, apierrors.NewConflict(
				schema.GroupResource{Resource: "services"},
				desired.Name,
				errors.New("concurrent update"),
			)
		}
		return false, nil, nil
	})

	if err := CreateKubeService(client, desired.Namespace, desired); err != nil {
		t.Fatalf("reconcile service after conflict: %v", err)
	}
	if updateAttempts != 2 {
		t.Fatalf("expected two update attempts, got %d", updateAttempts)
	}
}

func TestServicePortsShareIdentity(t *testing.T) {
	tests := []struct {
		name     string
		desired  corev1.ServicePort
		existing corev1.ServicePort
		want     bool
	}{
		{
			name:     "matching named TCP ports",
			desired:  corev1.ServicePort{Name: "http", Protocol: corev1.ProtocolTCP},
			existing: corev1.ServicePort{Name: "http", Protocol: corev1.ProtocolTCP},
			want:     true,
		},
		{
			name:     "different protocols",
			desired:  corev1.ServicePort{Name: "dns", Protocol: corev1.ProtocolUDP},
			existing: corev1.ServicePort{Name: "dns", Protocol: corev1.ProtocolTCP},
		},
		{
			name:     "different names",
			desired:  corev1.ServicePort{Name: "http", Protocol: corev1.ProtocolTCP},
			existing: corev1.ServicePort{Name: "https", Protocol: corev1.ProtocolTCP},
		},
		{
			name:     "matching unnamed ports",
			desired:  corev1.ServicePort{Port: 8080, Protocol: corev1.ProtocolTCP},
			existing: corev1.ServicePort{Port: 8080, Protocol: corev1.ProtocolTCP},
			want:     true,
		},
		{
			name:     "different unnamed ports",
			desired:  corev1.ServicePort{Port: 8080, Protocol: corev1.ProtocolTCP},
			existing: corev1.ServicePort{Port: 9090, Protocol: corev1.ProtocolTCP},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := servicePortsShareIdentity(test.desired, test.existing); got != test.want {
				t.Fatalf("expected %v, got %v", test.want, got)
			}
		})
	}
}
