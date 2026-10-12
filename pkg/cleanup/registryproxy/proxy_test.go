package registryproxy

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type testLease struct {
	responses          int
	result             ForwardResult
	finished           int
	finishContextAlive bool
}

func (l *testLease) ObserveResponse(context.Context, ResponseObservation) error {
	l.responses++
	return nil
}
func (l *testLease) Finish(ctx context.Context, r ForwardResult) error {
	l.finished++
	l.result = r
	l.finishContextAlive = ctx.Err() == nil
	return nil
}

type testCoordinator struct {
	calls  int
	denied bool
	lease  *testLease
}

func (c *testCoordinator) Start(_ context.Context, _ *http.Request, _ RequestDescriptor) (RequestLease, error) {
	c.calls++
	if c.denied {
		return nil, errors.New("rejected")
	}
	return c.lease, nil
}

// capability_id: rainbond.cleanup.registry-stream-admission
func TestProxyAcquiresBeforeForwardingAndPreservesRegistryAuth(t *testing.T) {
	lease := &testLease{}
	coordinator := &testCoordinator{lease: lease}
	calls := 0
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if coordinator.calls != 1 {
			t.Error("request reached registry before durable admission")
		}
		if r.Header.Get("Authorization") != "Bearer registry-fixture" {
			t.Error("registry authentication lost")
		}
		if r.Header.Get("Cookie") != "" || r.Header.Get("X-Rainbond-Cleanup-Permit") != "" || r.Header.Get("Idempotency-Key") != "" || r.Header.Get("X-Idempotency-Key") != "" {
			t.Error("unrelated credential forwarded")
		}
		w.WriteHeader(201)
	}))
	defer backend.Close()
	proxy, err := NewProxy(backend.URL, coordinator, nil)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest("PUT", "http://registry.local/v2/team/app/manifests/v1", strings.NewReader(`{"schemaVersion":2}`))
	request.Header.Set("Authorization", "Bearer registry-fixture")
	request.Header.Set("Idempotency-Key", "untrusted-retry-hint")
	request.Header.Set("X-Idempotency-Key", "untrusted-retry-hint")
	request.Header.Set("Cookie", "platform-fixture=not-for-registry")
	request.Header.Set("X-Rainbond-Cleanup-Permit", "opaque-fixture")
	response := httptest.NewRecorder()
	proxy.ServeHTTP(response, request)
	if response.Code != 201 || calls != 1 || lease.responses != 1 || lease.finished != 1 || !lease.result.Complete || lease.result.Status != 201 {
		t.Fatal(response.Code, calls, lease)
	}
}
func TestProxyDenialNeverContactsRegistry(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("denied request forwarded") }))
	defer backend.Close()
	coordinator := &testCoordinator{denied: true}
	proxy, err := NewProxy(backend.URL, coordinator, nil)
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	proxy.ServeHTTP(response, httptest.NewRequest("POST", "/v2/app/blobs/uploads/", nil))
	if response.Code != 503 || coordinator.calls != 1 {
		t.Fatal(response.Code, coordinator.calls)
	}
}
func TestProxyReadDoesNotRequestWriteAdmission(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "manifest") }))
	defer backend.Close()
	coordinator := &testCoordinator{denied: true}
	proxy, err := NewProxy(backend.URL, coordinator, nil)
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	proxy.ServeHTTP(response, httptest.NewRequest("GET", "/v2/app/manifests/v1", nil))
	if response.Code != 200 || response.Body.String() != "manifest" || coordinator.calls != 0 {
		t.Fatal(response.Code, coordinator.calls)
	}
}
func TestProxyIncompleteResponseRetainsUncertainty(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100")
		w.WriteHeader(201)
		fmt.Fprint(w, "short")
	}))
	defer backend.Close()
	lease := &testLease{}
	coordinator := &testCoordinator{lease: lease}
	proxy, err := NewProxy(backend.URL, coordinator, nil)
	if err != nil {
		t.Fatal(err)
	}
	proxy.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("PUT", "/v2/app/manifests/v1", strings.NewReader("manifest")))
	if lease.finished != 1 || lease.result.Complete || !lease.finishContextAlive {
		t.Fatal("truncated response incorrectly finalized", lease)
	}
}
func TestProxyRequiresLoopbackRegistryAndCoordinator(t *testing.T) {
	for _, endpoint := range []string{"http://external.invalid:5000", "http://127.0.0.1:5000/prefix", "http://user@127.0.0.1:5000", "http://127.0.0.1:5000?x=1"} {
		if _, err := NewProxy(endpoint, &testCoordinator{}, nil); err == nil {
			t.Fatal("unsafe upstream accepted", endpoint)
		}
	}
	if _, err := NewProxy("http://127.0.0.1:5000", nil, nil); err == nil {
		t.Fatal("missing coordinator accepted")
	}
}
