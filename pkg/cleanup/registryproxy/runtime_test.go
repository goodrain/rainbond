package registryproxy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	coordination "github.com/goodrain/rainbond/pkg/cleanup"
)

type runtimeBackend struct {
	CoordinationBackend
	observation coordination.StorageObservation
}

type runtimeRoundTrip func(*http.Request) (*http.Response, error)

func (f runtimeRoundTrip) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }

func TestRegistryUpstreamReadyRequiresRegistryIdentityOrAuthChallenge(t *testing.T) {
	for _, tc := range []struct {
		name      string
		status    int
		version   string
		challenge string
		ready     bool
	}{
		{name: "version", status: http.StatusOK, version: "registry/2.0", ready: true},
		{name: "basic challenge", status: http.StatusUnauthorized, challenge: `Basic realm="Registry Realm"`, ready: true},
		{name: "bearer challenge", status: http.StatusUnauthorized, challenge: `Bearer realm="https://auth.example/token"`, ready: true},
		{name: "missing identity", status: http.StatusOK},
		{name: "missing challenge", status: http.StatusUnauthorized},
		{name: "unsupported challenge", status: http.StatusUnauthorized, challenge: `Digest realm="other"`},
		{name: "unavailable", status: http.StatusServiceUnavailable, version: "registry/2.0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			header := http.Header{}
			header.Set("Docker-Distribution-API-Version", tc.version)
			header.Set("WWW-Authenticate", tc.challenge)
			if ready := registryUpstreamReady(&http.Response{StatusCode: tc.status, Header: header}); ready != tc.ready {
				t.Fatalf("unexpected readiness %v", ready)
			}
		})
	}
}

func TestRuntimeStorageObservationRequiresExactShortLivedPermit(t *testing.T) {
	root := t.TempDir()
	binding := coordination.StorageRegistration{StorageID: "store", Generation: "one", VolumeUID: "volume", RootPath: "/var/lib/registry"}
	if err := InitializeStorageIdentity(root, binding); err != nil {
		t.Fatal(err)
	}
	fingerprint, _ := binding.Fingerprint()
	backend := &runtimeBackend{observation: coordination.StorageObservation{StorageID: "store", Generation: "one", Mode: "collecting", RegistrationFingerprint: fingerprint}}
	upstreamCalls := 0
	transport := runtimeRoundTrip(func(*http.Request) (*http.Response, error) {
		upstreamCalls++
		return &http.Response{StatusCode: 500, Body: http.NoBody, Header: http.Header{}}, nil
	})
	key := []byte(strings.Repeat("observation-key-", 3))
	runtime, err := NewRuntime(RuntimeConfig{Root: root, Binding: binding, Upstream: "http://127.0.0.1:5000", Owner: "instance", Pod: "pod", PodUID: "uid", Backend: backend, PermitKey: func() []byte { return key }, Transport: transport})
	if err != nil {
		t.Fatal(err)
	}
	permit, err := coordination.IssueStorageObservationPermit(key, binding, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	request := func(method, token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/storagez", nil)
		if token != "" {
			req.Header.Set("X-Rainbond-Storage-Observation", token)
		}
		response := httptest.NewRecorder()
		runtime.ServeHTTP(response, req)
		return response
	}
	response := request(http.MethodGet, permit)
	var measured coordination.StorageMeasurement
	if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &measured) != nil || measured.StorageID != binding.StorageID || measured.Generation != binding.Generation || measured.BindingFingerprint != fingerprint || measured.TotalBytes == 0 {
		t.Fatal("verified storage observation failed", response.Code, response.Body.String())
	}
	if request(http.MethodGet, "").Code != 403 || request(http.MethodGet, permit+"x").Code != 403 || request(http.MethodPost, permit).Code != 405 || upstreamCalls != 0 {
		t.Fatal("storage observation bypassed permit or reached Registry", upstreamCalls)
	}
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
		w.Header().Set("WWW-Authenticate", `Basic realm="Registry Realm"`)
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
