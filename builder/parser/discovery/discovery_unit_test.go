package discovery

import "testing"

// capability_id: rainbond.source-discovery.unsupported-type
func TestNewDiscoverierUnsupportedType(t *testing.T) {
	info := &Info{Type: "consul"}
	if got := NewDiscoverier(info); got != nil {
		t.Fatalf("expected nil discoverier for unsupported type, got %T", got)
	}
}
