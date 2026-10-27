package podevent

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	k8stesting "k8s.io/client-go/testing"
)

// capability_id: rainbond.worker.pod-check-leader-lifecycle
func TestHandleStopsAcrossLeaderTerms(t *testing.T) {
	client := fake.NewSimpleClientset()
	listed := make(chan struct{}, 10)
	client.PrependReactor("list", "pods", func(k8stesting.Action) (bool, runtime.Object, error) {
		listed <- struct{}{}
		return true, &corev1.PodList{}, nil
	})
	p := &PodEvent{
		clientset:  client,
		stopCh:     make(chan struct{}),
		podEventCh: make(chan *corev1.Pod),
	}
	defer close(p.stopCh)

	for term := 0; term < 4; term++ {
		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)
		ticks := make(chan time.Time, 1)
		done := make(chan struct{})
		go func() {
			defer close(done)
			p.handle(ctx, ticks)
		}()

		ticks <- time.Now()
		waitForLifecycleSignal(t, listed, "periodic list")
		cancel()
		waitForLifecycleSignal(t, done, "old leader handler to exit")

		// The old handler must not consume ticks after its leader term ends.
		ticks <- time.Now()
		// A queued tick must also be ignored if an already-canceled term starts.
		p.handle(ctx, ticks)
		if got := len(client.Actions()); got != term+1 {
			t.Fatalf("after leader term %d: got %d API calls, want %d", term, got, term+1)
		}
	}
}

// capability_id: rainbond.worker.pod-check-leader-lifecycle
func TestHandleCancelsInFlightProbeRequests(t *testing.T) {
	for _, operation := range []string{"get", "list"} {
		for _, stop := range []string{"leadership", "controller"} {
			t.Run(operation+"/"+stop, func(t *testing.T) {
				started := make(chan struct{})
				requestCanceled := make(chan struct{})
				release := make(chan struct{})
				var calls atomic.Int32
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if calls.Add(1) != 1 {
						http.Error(w, "unexpected additional request", http.StatusInternalServerError)
						return
					}
					wantPath := "/api/v1/pods"
					if operation == "get" {
						wantPath = "/api/v1/namespaces/default/pods/example"
					}
					if r.URL.Path != wantPath {
						t.Errorf("request path = %q, want %q", r.URL.Path, wantPath)
					}
					close(started)
					select {
					case <-r.Context().Done():
						close(requestCanceled)
					case <-release:
					}
				}))
				// Release the server even if a cancellation assertion fails.
				defer server.Close()
				defer close(release)
				client, err := kubernetes.NewForConfig(&rest.Config{Host: server.URL})
				if err != nil {
					t.Fatal(err)
				}
				p := &PodEvent{
					clientset:  client,
					stopCh:     make(chan struct{}),
					podEventCh: make(chan *corev1.Pod),
				}
				if operation == "get" {
					p.recentPods.Store("default/example", &corev1.Pod{})
				}
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				ticks := make(chan time.Time, 1)
				done := make(chan struct{})
				go func() {
					defer close(done)
					p.handle(ctx, ticks)
				}()

				ticks <- time.Now()
				waitForLifecycleSignal(t, started, "probe request to start")
				if stop == "leadership" {
					cancel()
				} else {
					close(p.stopCh)
				}
				waitForLifecycleSignal(t, requestCanceled, "in-flight HTTP request cancellation")
				waitForLifecycleSignal(t, done, "handler to exit")
				if got := calls.Load(); got != 1 {
					t.Fatalf("got %d requests; canceled GET must not be followed by LIST", got)
				}
				if operation == "get" {
					if _, ok := p.recentPods.Load("default/example"); !ok {
						t.Fatal("leadership/controller cancellation removed a cached pod")
					}
				}
			})
		}
	}
}

// capability_id: rainbond.worker.pod-check-leader-lifecycle
func TestHandleWaitsForPreviousLeaderTerm(t *testing.T) {
	firstRequest := make(chan struct{})
	secondRequest := make(chan struct{})
	release := make(chan struct{})
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	var calls atomic.Int32
	client, err := kubernetes.NewForConfig(&rest.Config{
		Host: "http://example.invalid",
		WrapTransport: func(http.RoundTripper) http.RoundTripper {
			return probeRoundTripper(func(*http.Request) (*http.Response, error) {
				switch calls.Add(1) {
				case 1:
					close(firstRequest)
					// Simulate work that has not yet observed leader cancellation.
					<-release
				case 2:
					close(secondRequest)
				default:
					t.Error("unexpected additional request")
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(`{"apiVersion":"v1","kind":"PodList","items":[]}`)),
				}, nil
			})
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	p := &PodEvent{
		clientset:  client,
		stopCh:     make(chan struct{}),
		podEventCh: make(chan *corev1.Pod),
	}
	defer close(p.stopCh)
	firstCtx, cancelFirst := context.WithCancel(context.Background())
	defer cancelFirst()
	secondCtx, cancelSecond := context.WithCancel(context.Background())
	defer cancelSecond()
	firstTicks := make(chan time.Time, 1)
	secondTicks := make(chan time.Time, 1)
	firstDone := make(chan struct{})
	secondDone := make(chan struct{})
	go func() {
		defer close(firstDone)
		p.handle(firstCtx, firstTicks)
	}()
	firstTicks <- time.Now()
	waitForLifecycleSignal(t, firstRequest, "first leader request")
	cancelFirst()
	go func() {
		defer close(secondDone)
		p.handle(secondCtx, secondTicks)
	}()
	secondTicks <- time.Now()
	select {
	case <-secondRequest:
		t.Error("new leader issued a probe while the previous handler was still running")
	case <-time.After(100 * time.Millisecond):
	}
	close(release)
	waitForLifecycleSignal(t, firstDone, "previous leader handler to finish")
	waitForLifecycleSignal(t, secondRequest, "new leader probe after previous handler exits")
	cancelSecond()
	waitForLifecycleSignal(t, secondDone, "new leader handler to exit")
}

type probeRoundTripper func(*http.Request) (*http.Response, error)

func (f probeRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func waitForLifecycleSignal(t *testing.T, signal <-chan struct{}, description string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for %s", description)
	}
}
