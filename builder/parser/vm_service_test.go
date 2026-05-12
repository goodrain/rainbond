package parser

import (
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/goodrain/rainbond/event"
)

// capability_id: rainbond.vm-run.remote-package-probe
func TestVMServiceParseRemoteURLPrefersHeadProbe(t *testing.T) {
	withDefaultClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.Method {
		case http.MethodHead:
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(http.NoBody),
				Request:    req,
			}, nil
		case http.MethodGet:
			return nil, errors.New("read: connection reset by peer")
		default:
			t.Fatalf("unexpected request method %s", req.Method)
			return nil, nil
		}
	}))

	parser := CreateVMServiceParse("https://example.com/ubuntu.iso", event.GetTestLogger())

	if errors := parser.Parse(); len(errors) != 0 {
		t.Fatalf("expected no parse errors, got %#v", errors)
	}
}

// capability_id: rainbond.vm-run.remote-package-probe-range-fallback
func TestVMServiceParseRemoteURLFallsBackToRangeGet(t *testing.T) {
	withDefaultClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.Method {
		case http.MethodHead:
			return &http.Response{
				StatusCode: http.StatusMethodNotAllowed,
				Body:       io.NopCloser(http.NoBody),
				Request:    req,
			}, nil
		case http.MethodGet:
			if got := req.Header.Get("Range"); got != "bytes=0-0" {
				t.Fatalf("expected Range header bytes=0-0, got %q", got)
			}
			return &http.Response{
				StatusCode: http.StatusPartialContent,
				Body:       io.NopCloser(http.NoBody),
				Request:    req,
			}, nil
		default:
			t.Fatalf("unexpected request method %s", req.Method)
			return nil, nil
		}
	}))

	parser := CreateVMServiceParse("https://example.com/ubuntu.iso", event.GetTestLogger())

	if errors := parser.Parse(); len(errors) != 0 {
		t.Fatalf("expected no parse errors, got %#v", errors)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func withDefaultClient(t *testing.T, transport http.RoundTripper) {
	t.Helper()

	originalClient := http.DefaultClient
	http.DefaultClient = &http.Client{Transport: transport}
	t.Cleanup(func() {
		http.DefaultClient = originalClient
	})
}
