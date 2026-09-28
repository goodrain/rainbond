package helm

import (
	"net/http"
	"sync"

	guard "github.com/goodrain/rainbond/pkg/cleanup"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"
)

type helmMutationState struct{ pending int }
type helmMutationTracker struct {
	operation sync.Mutex
	mu        sync.Mutex
	active    *helmMutationState
}

func (t *helmMutationTracker) begin() *helmMutationState {
	t.operation.Lock()
	t.mu.Lock()
	defer t.mu.Unlock()
	state := &helmMutationState{}
	t.active = state
	return state
}
func (t *helmMutationTracker) end(state *helmMutationState) {
	t.mu.Lock()
	if t.active == state {
		t.active = nil
	}
	t.mu.Unlock()
	t.operation.Unlock()
}
func (t *helmMutationTracker) confirmed(state *helmMutationState) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.active == state && state.pending == 0
}

type helmMutationTransport struct {
	next    http.RoundTripper
	tracker *helmMutationTracker
}

func (t helmMutationTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if request.Method == http.MethodGet || request.Method == http.MethodHead || request.Method == http.MethodOptions {
		return t.next.RoundTrip(request)
	}
	t.tracker.mu.Lock()
	state := t.tracker.active
	if state == nil {
		t.tracker.mu.Unlock()
		return nil, guard.ErrCoordinationChanged
	}
	state.pending++
	t.tracker.mu.Unlock()
	response, err := t.next.RoundTrip(request)
	definitive := false
	if err == nil && response != nil {
		definitive = response.StatusCode >= 200 && response.StatusCode < 300
		switch response.StatusCode {
		case 400, 401, 403, 404, 405, 409, 410, 415, 422, 429:
			definitive = true
		}
	}
	if definitive {
		t.tracker.mu.Lock()
		state.pending--
		t.tracker.mu.Unlock()
	}
	return response, err
}

// The same getter configures Helm's workload clients and release Secret driver.
// No body or authentication header is inspected, retained or logged.
type helmMutationRESTGetter struct {
	genericclioptions.RESTClientGetter
	tracker *helmMutationTracker
}

func (g helmMutationRESTGetter) ToRESTConfig() (*rest.Config, error) {
	config, err := g.RESTClientGetter.ToRESTConfig()
	if err != nil {
		return nil, err
	}
	config = rest.CopyConfig(config)
	config.Wrap(func(next http.RoundTripper) http.RoundTripper {
		return helmMutationTransport{next: next, tracker: g.tracker}
	})
	return config, nil
}
