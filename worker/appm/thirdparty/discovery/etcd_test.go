package discovery

import (
	"testing"

	"github.com/eapache/channels"
	"github.com/goodrain/rainbond/db/model"
)

// capability_id: rainbond.worker.appm.discovery.etcd-config
func TestNewEtcdAndFetchGuard(t *testing.T) {
	cfg := &model.ThirdPartySvcDiscoveryCfg{
		Type:      model.DiscorveryTypeEtcd.String(),
		ServiceID: "svc-1",
		Servers:   "http://127.0.0.1:2379,http://127.0.0.1:2380",
		Key:       "/foobar/eps",
		Username:  "user",
		Password:  "pass",
	}
	updateCh := channels.NewRingChannel(8)
	stopCh := make(chan struct{})
	defer close(stopCh)

	d, ok := NewEtcd(cfg, updateCh, stopCh).(*etcd)
	if !ok {
		t.Fatal("expected *etcd")
	}
	if len(d.endpoints) != 2 || d.endpoints[0] != "http://127.0.0.1:2379" || d.endpoints[1] != "http://127.0.0.1:2380" {
		t.Fatalf("unexpected endpoints: %#v", d.endpoints)
	}
	if d.sid != "svc-1" || d.key != "/foobar/eps" || d.username != "user" || d.password != "pass" {
		t.Fatalf("unexpected config: %+v", d)
	}
	if _, err := d.Fetch(); err == nil {
		t.Fatal("expected fetch guard error without client")
	}
}

// capability_id: rainbond.worker.appm.discovery.etcd-config
func TestNewDiscoverierAndCloseNil(t *testing.T) {
	cfg := &model.ThirdPartySvcDiscoveryCfg{
		Type: model.DiscorveryTypeEtcd.String(),
	}
	updateCh := channels.NewRingChannel(8)
	stopCh := make(chan struct{})
	defer close(stopCh)

	discoverier, err := NewDiscoverier(cfg, updateCh, stopCh)
	if err != nil || discoverier == nil {
		t.Fatalf("expected discoverier, err=%v", err)
	}

	d := &etcd{}
	if err := d.Close(); err != nil {
		t.Fatalf("expected nil-safe close, got %v", err)
	}
}
