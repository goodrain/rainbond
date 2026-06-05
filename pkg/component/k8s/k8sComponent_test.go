package k8s

// capability_id: rainbond.k8s.scheme-registers-kubevirt-vm

import (
	"testing"

	kubevirtv1 "kubevirt.io/api/core/v1"
)

func TestSchemeRegistersKubeVirtVirtualMachine(t *testing.T) {
	t.Helper()

	_, _, err := scheme.ObjectKinds(&kubevirtv1.VirtualMachine{})
	if err != nil {
		t.Fatalf("expected kubevirt VirtualMachine to be registered in scheme: %v", err)
	}
}
