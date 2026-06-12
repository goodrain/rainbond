package mirror

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func newSourceStub(t *testing.T, mirrors ...string) string {
	t.Helper()
	entries := ""
	for i, m := range mirrors {
		if i > 0 {
			entries += ","
		}
		entries += fmt.Sprintf(`{"url": %q}`, m)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"version": 1, "mirrors": [%s]}`, entries)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func testConfig(sourceURL string) Config {
	return Config{
		Enabled:         true,
		SourceURLs:      []string{sourceURL},
		RefreshInterval: time.Hour,
		MaxCount:        3,
	}
}

// capability_id: rainbond.builder.dynamic-mirror-refresh
func TestManagerRefreshUpdatesMirrorsAndConfigMap(t *testing.T) {
	alive := newRegistryStub(t, http.StatusOK, 0)
	dead := newRegistryStub(t, http.StatusInternalServerError, 0)
	source := newSourceStub(t, alive, dead)

	kube := fake.NewSimpleClientset()
	m := New(testConfig(source), kube, "rbd-system")

	if err := m.Refresh(context.Background()); err != nil {
		t.Fatalf("refresh failure: %v", err)
	}
	assertStringSlice(t, m.Mirrors(), []string{alive})

	cm, err := kube.CoreV1().ConfigMaps("rbd-system").Get(context.Background(), configMapName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("configmap not persisted: %v", err)
	}
	if cm.Data["mirrors"] != fmt.Sprintf(`["%s"]`, alive) {
		t.Fatalf("unexpected persisted mirrors: %q", cm.Data["mirrors"])
	}
	if cm.Data["updated_at"] == "" {
		t.Fatal("updated_at should be set")
	}
}

func TestManagerRefreshCapsAtMaxCount(t *testing.T) {
	stubs := make([]string, 0, 4)
	for i := 0; i < 4; i++ {
		stubs = append(stubs, newRegistryStub(t, http.StatusOK, 0))
	}
	source := newSourceStub(t, stubs...)
	cfg := testConfig(source)
	cfg.MaxCount = 2

	m := New(cfg, fake.NewSimpleClientset(), "rbd-system")
	if err := m.Refresh(context.Background()); err != nil {
		t.Fatalf("refresh failure: %v", err)
	}
	if got := len(m.Mirrors()); got != 2 {
		t.Fatalf("mirrors = %d, want capped at 2", got)
	}
}

func TestManagerRefreshFailureKeepsLastList(t *testing.T) {
	alive := newRegistryStub(t, http.StatusOK, 0)
	source := newSourceStub(t, alive)

	m := New(testConfig(source), fake.NewSimpleClientset(), "rbd-system")
	if err := m.Refresh(context.Background()); err != nil {
		t.Fatalf("first refresh failure: %v", err)
	}

	m.cfg.SourceURLs = []string{"http://127.0.0.1:1"}
	if err := m.Refresh(context.Background()); err == nil {
		t.Fatal("expected refresh error when source unreachable")
	}
	assertStringSlice(t, m.Mirrors(), []string{alive})
}

func TestManagerRefreshAllDeadClearsList(t *testing.T) {
	alive := newRegistryStub(t, http.StatusOK, 0)
	dead := newRegistryStub(t, http.StatusInternalServerError, 0)

	m := New(testConfig(newSourceStub(t, alive)), fake.NewSimpleClientset(), "rbd-system")
	if err := m.Refresh(context.Background()); err != nil {
		t.Fatalf("first refresh failure: %v", err)
	}

	m.cfg.SourceURLs = []string{newSourceStub(t, dead)}
	if err := m.Refresh(context.Background()); err != nil {
		t.Fatalf("refresh with dead mirrors should not error: %v", err)
	}
	if got := m.Mirrors(); len(got) != 0 {
		t.Fatalf("dead mirrors must clear the list, got %v", got)
	}
}

// capability_id: rainbond.builder.dynamic-mirror-restore
func TestManagerRestoreFromConfigMap(t *testing.T) {
	kube := fake.NewSimpleClientset()
	m := New(testConfig("http://127.0.0.1:1"), kube, "rbd-system")
	m.setMirrors(context.Background(), []string{"https://docker.1ms.run"})

	restored := New(testConfig("http://127.0.0.1:1"), kube, "rbd-system")
	restored.restore(context.Background())
	assertStringSlice(t, restored.Mirrors(), []string{"https://docker.1ms.run"})
}

func TestDisabledManagerReturnsNoMirrors(t *testing.T) {
	cfg := testConfig("http://127.0.0.1:1")
	cfg.Enabled = false
	m := New(cfg, fake.NewSimpleClientset(), "rbd-system")
	if err := m.Refresh(context.Background()); err != nil {
		t.Fatalf("disabled refresh should be a no-op, got %v", err)
	}
	if got := m.Mirrors(); len(got) != 0 {
		t.Fatalf("disabled manager must expose no mirrors, got %v", got)
	}
}

func TestDefaultManagerNilSafe(t *testing.T) {
	if got := (*Manager)(nil).Mirrors(); got != nil {
		t.Fatalf("nil manager should return nil, got %v", got)
	}
}
