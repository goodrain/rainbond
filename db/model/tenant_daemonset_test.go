package model

import "testing"

// capability_id: rainbond.service-type.daemonset
func TestServiceTypeDaemonSetClassification(t *testing.T) {
	serviceType := ServiceTypeDaemonSet
	if !serviceType.IsDaemonSet() {
		t.Fatalf("expected daemonset service type to be classified as daemonset")
	}
	if serviceType.IsState() {
		t.Fatalf("expected daemonset service type not to be classified as stateful")
	}
	if serviceType.IsSingleton() {
		t.Fatalf("expected daemonset service type not to be classified as singleton")
	}

	service := &TenantServices{ExtendMethod: ServiceTypeDaemonSet.String()}
	if !service.IsDaemonSet() {
		t.Fatalf("expected tenant service to be classified as daemonset")
	}
}
