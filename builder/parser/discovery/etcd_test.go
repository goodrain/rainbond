package discovery

import "testing"

// capability_id: rainbond.source-discovery.etcd-config
func TestNewEtcdAndFetchGuard(t *testing.T) {
	info := &Info{
		Type:     "etcd",
		Servers:  []string{"http://127.0.0.1:2379"},
		Key:      "/services/demo",
		Username: "user",
		Password: "pass",
	}

	d, ok := NewEtcd(info).(*etcd)
	if !ok {
		t.Fatal("expected *etcd")
	}
	if len(d.endpoints) != 1 || d.endpoints[0] != "http://127.0.0.1:2379" {
		t.Fatalf("unexpected endpoints: %#v", d.endpoints)
	}
	if d.key != "/services/demo" || d.username != "user" || d.password != "pass" {
		t.Fatalf("unexpected etcd config: %+v", d)
	}

	if _, err := d.Fetch(); err == nil {
		t.Fatal("expected fetch guard error without client")
	}
}

// capability_id: rainbond.source-discovery.etcd-config
func TestNewDiscoverierAndCloseNil(t *testing.T) {
	info := &Info{Type: "ETCD", Servers: []string{"http://127.0.0.1:2379"}}
	if d := NewDiscoverier(info); d == nil {
		t.Fatal("expected etcd discoverier")
	}

	d := &etcd{}
	if err := d.Close(); err != nil {
		t.Fatalf("expected nil-safe close, got %v", err)
	}
}
