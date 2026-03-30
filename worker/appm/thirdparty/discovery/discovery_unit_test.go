package discovery

import (
	"testing"

	"github.com/eapache/channels"
	"github.com/goodrain/rainbond/db/model"
)

// capability_id: rainbond.worker.appm.discovery.unsupported-type
func TestNewDiscoverierUnsupportedType(t *testing.T) {
	updateCh := channels.NewRingChannel(8)
	stopCh := make(chan struct{})
	defer close(stopCh)

	_, err := NewDiscoverier(&model.ThirdPartySvcDiscoveryCfg{
		Type: "consul",
	}, updateCh, stopCh)
	if err == nil {
		t.Fatal("expected unsupported discovery type error")
	}
}
