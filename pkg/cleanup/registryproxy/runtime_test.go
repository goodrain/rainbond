package registryproxy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	coordination "github.com/goodrain/rainbond/pkg/cleanup"
)

type runtimeBackend struct {
	CoordinationBackend
	observation coordination.StorageObservation
}

func (b *runtimeBackend) RegisterRegistryParticipant(context.Context, string, string, string, string, string) error {
	return nil
}
func (b *runtimeBackend) InspectStorage(context.Context, string, string) (coordination.StorageObservation, error) {
	return b.observation, nil
}
func TestRuntimeChecksBothMarkerAndRegisteredFingerprint(t *testing.T) {
	root := t.TempDir()
	binding := coordination.StorageRegistration{StorageID: "store", Generation: "one", VolumeUID: "volume", RootPath: "/var/lib/registry"}
	if err := InitializeStorageIdentity(root, binding); err != nil {
		t.Fatal(err)
	}
	fingerprint, _ := binding.Fingerprint()
	backend := &runtimeBackend{observation: coordination.StorageObservation{StorageID: "store", Generation: "one", Mode: "collecting", RegistrationFingerprint: fingerprint}}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Docker-Distribution-API-Version", "registry/2.0")
		w.WriteHeader(401)
	}))
	defer upstream.Close()
	runtime, err := NewRuntime(RuntimeConfig{Root: root, Binding: binding, Upstream: upstream.URL, Owner: "instance", Pod: "pod", PodUID: "uid", Backend: backend, PermitKey: func() []byte { return nil }})
	if err != nil {
		t.Fatal(err)
	}
	request := func(path string) int {
		response := httptest.NewRecorder()
		runtime.ServeHTTP(response, httptest.NewRequest("GET", path, nil))
		return response.Code
	}
	if status := request("/readyz"); status != 200 {
		t.Fatal("enrollment is not routable", status)
	}
	backend.observation.RegistrationFingerprint = "wrong"
	if status := request("/readyz"); status != 503 {
		t.Fatal("wrong volume fingerprint ready", status)
	}
	if status := request("/healthz"); status != 200 {
		t.Fatal("dependency failure should not trigger liveness restart", status)
	}
	backend.observation.RegistrationFingerprint = fingerprint
	backend.observation.Mode = "unverified"
	if status := request("/readyz"); status != 503 {
		t.Fatal("unverified store ready", status)
	}
}
