package kubeblocks

import "testing"

// capability_id: rainbond.kubeblocks.component-selector
func TestGenerateKubeBlocksSelector(t *testing.T) {
	selector := GenerateKubeBlocksSelector("demo-mysql")
	if selector["app.kubernetes.io/instance"] != "demo" || selector["apps.kubeblocks.io/component-name"] != "mysql" {
		t.Fatalf("unexpected selector: %#v", selector)
	}
	if selector["kubeblocks.io/role"] != "primary" {
		t.Fatalf("expected primary role selector, got %#v", selector)
	}

	selector = GenerateKubeBlocksSelector("demo-rabbitmq")
	if _, ok := selector["kubeblocks.io/role"]; ok {
		t.Fatalf("did not expect role selector for peer component, got %#v", selector)
	}
}
