package mirror

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// capability_id: rainbond.builder.dynamic-mirror-probe
func TestProbeFiltersAndSortsByLatency(t *testing.T) {
	slowOK := newRegistryStub(t, http.StatusOK, 300*time.Millisecond)
	fastOK := newRegistryStub(t, http.StatusOK, 0)
	dead := newRegistryStub(t, http.StatusInternalServerError, 0)

	got := Probe(context.Background(), []string{slowOK, fastOK, dead}, 2*time.Second)

	assertStringSlice(t, got, []string{fastOK, slowOK})
}

// token 认证类 mirror（如 daocloud）对匿名 manifest 请求回 401，但真实拉取时
// resolver 会走 token 流程，因此 401 必须判活。
func TestProbeTokenAuthMirrorIsAlive(t *testing.T) {
	authChallenge := newRegistryStub(t, http.StatusUnauthorized, 0)
	got := Probe(context.Background(), []string{authChallenge}, time.Second)
	assertStringSlice(t, got, []string{authChallenge})
}

// manifest 路径挂起不响应的“假活”源要在探活超时内排除
// （docker.xuanyuan.me 卡死构建事故的场景之一）。
func TestProbeManifestStallTreatedAsDead(t *testing.T) {
	stalled := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second) // longer than probe timeout
	}))
	t.Cleanup(stalled.Close)
	got := Probe(context.Background(), []string{stalled.URL}, 500*time.Millisecond)
	if len(got) != 0 {
		t.Fatalf("stalled mirror must be treated as dead, got %v", got)
	}
}

func TestProbeUnreachableHostDropped(t *testing.T) {
	got := Probe(context.Background(), []string{"http://127.0.0.1:1"}, 500*time.Millisecond)
	if len(got) != 0 {
		t.Fatalf("expected no alive mirrors, got %v", got)
	}
}

func TestProbeEmptyInput(t *testing.T) {
	if got := Probe(context.Background(), nil, time.Second); len(got) != 0 {
		t.Fatalf("expected empty result, got %v", got)
	}
}

// newRegistryStub serves the probe manifest path with the given status after
// an artificial delay and returns the server base URL.
func newRegistryStub(t *testing.T, status int, delay time.Duration) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != probeManifestPath {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		time.Sleep(delay)
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}
