package middleware

// capability_id: rainbond.vm-snapshot.http-bounded-contract

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/goodrain/rainbond/pkg/component/k8s"
	"github.com/goodrain/rainbond/pkg/generated/clientset/versioned"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func TestSnapshotCommandPreservesLicenseBoundaryWithoutEarlyEventSuccess(t *testing.T) {
	component := k8s.Default()
	if component == nil {
		component = k8s.New()
	}
	previousRainbond, previousCore := component.RainbondClient, component.Clientset
	t.Cleanup(func() {
		component.RainbondClient = previousRainbond
		component.Clientset = previousCore
		InvalidateRSALicenseCache()
	})
	for _, enterprise := range []bool{false, true} {
		t.Run(map[bool]string{false: "open-source-accepted", true: "enterprise-denied"}[enterprise], func(t *testing.T) {
			InvalidateRSALicenseCache()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if enterprise && r.URL.Path == "/apis/rainbond.io/v1alpha1/rbdplugins/rainbond-enterprise-base" {
					_, _ = w.Write([]byte(`{"apiVersion":"rainbond.io/v1alpha1","kind":"RBDPlugin","metadata":{"name":"rainbond-enterprise-base"}}`))
					return
				}
				w.WriteHeader(404)
				_, _ = w.Write([]byte(`{"kind":"Status","apiVersion":"v1","status":"Failure","reason":"NotFound","code":404}`))
			}))
			defer server.Close()
			config := &rest.Config{Host: server.URL}
			component.RainbondClient = versioned.NewForConfigOrDie(config)
			component.Clientset = kubernetes.NewForConfigOrDie(config)
			calls := 0
			wrapped := SnapshotCommand(func(w http.ResponseWriter, r *http.Request) { calls++; w.WriteHeader(202) })
			w := httptest.NewRecorder()
			wrapped(w, httptest.NewRequest("POST", "/vm-snapshots", nil))
			want := 202
			if enterprise {
				want = 412
			}
			if w.Code != want || calls != map[bool]int{false: 1, true: 0}[enterprise] {
				t.Fatalf("license status=%d calls=%d", w.Code, calls)
			}
		})
	}
}
