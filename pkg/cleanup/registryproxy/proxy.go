package registryproxy

import (
	"context"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// ResponseObservation contains only protocol identity, never upload-state query
// credentials or authorization headers.
type ResponseObservation struct {
	Status        int
	UploadID      string
	LocationPath  string
	ContentDigest string
}

// ForwardResult distinguishes a complete response from an ambiguous transfer.
type ForwardResult struct {
	Status   int
	Complete bool
}

// RequestLease is a durable admission owned by a concrete Registry request.
// Implementations retain upload-session admissions across chunk requests.
type RequestLease interface {
	ObserveResponse(context.Context, ResponseObservation) error
	Finish(context.Context, ForwardResult) error
}

// RequestCoordinator must persist admission before returning a lease. It must
// consume exact-target deletion grants and track active upload continuations.
type RequestCoordinator interface {
	Start(context.Context, *http.Request, RequestDescriptor) (RequestLease, error)
}

// ErrAdmissionDenied reports an invalid participant or cleanup permit.
var ErrAdmissionDenied = errors.New("registry coordination authorization denied")

// ErrAdmissionBusy reports a conflicting cleanup or maintenance operation.
var ErrAdmissionBusy = errors.New("registry coordination scope busy")

// Proxy forwards covered Registry v2 requests to a loopback-only upstream.
type Proxy struct {
	upstream    *url.URL
	coordinator RequestCoordinator
	transport   http.RoundTripper
}

// NewProxy requires a coordinator and an unambiguous local Registry origin.
func NewProxy(endpoint string, coordinator RequestCoordinator, transport http.RoundTripper) (*Proxy, error) {
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") || (u.Scheme != "http" && u.Scheme != "https") || coordinator == nil {
		return nil, ErrUnsupportedRequest
	}
	ip := net.ParseIP(u.Hostname())
	if ip == nil || !ip.IsLoopback() {
		return nil, ErrUnsupportedRequest
	}
	if transport == nil {
		transport = &http.Transport{DialContext: (&net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}).DialContext, ResponseHeaderTimeout: 30 * time.Second, IdleConnTimeout: 90 * time.Second, ExpectContinueTimeout: time.Second, MaxIdleConns: 100}
	}
	return &Proxy{upstream: u, coordinator: coordinator, transport: transport}, nil
}

type observedBody struct {
	io.ReadCloser
	ended, failed bool
}

func (b *observedBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if err == io.EOF {
		b.ended = true
	} else if err != nil {
		b.failed = true
	}
	return n, err
}

type safeProxyLog struct{}

func (safeProxyLog) Write(p []byte) (int, error) {
	logrus.Warn("Registry proxy stream failed; coordination may require verification")
	return len(p), nil
}

// ServeHTTP never forwards a mutation before admission. Reads can continue when
// the control plane is unavailable, and no Region credential is sent upstream.
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	description, err := ClassifyRequest(r.Method, r.URL)
	if err != nil {
		http.Error(w, "unsupported registry request", http.StatusMethodNotAllowed)
		return
	}
	if values := r.Header.Values("Authorization"); len(values) > 1 {
		http.Error(w, "unsupported registry authorization", 403)
		return
	}
	if auth := r.Header.Get("Authorization"); auth != "" {
		scheme, _, ok := strings.Cut(auth, " ")
		if !ok || (!strings.EqualFold(scheme, "Basic") && !strings.EqualFold(scheme, "Bearer")) {
			http.Error(w, "unsupported registry authorization", 403)
			return
		}
	}
	if r.Context().Err() != nil {
		return
	}
	var lease RequestLease
	if description.Mutating {
		lease, err = p.coordinator.Start(r.Context(), r, description)
		if err != nil || lease == nil {
			status := http.StatusServiceUnavailable
			if errors.Is(err, ErrAdmissionDenied) {
				status = http.StatusForbidden
			}
			if errors.Is(err, ErrAdmissionBusy) {
				status = http.StatusConflict
			}
			http.Error(w, "registry coordination rejected request", status)
			return
		}
	}
	status := 0
	returned, failed := false, false
	var body *observedBody
	if lease != nil {
		defer func() {
			// Client cancellation must not prevent durable uncertainty from being saved.
			ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 5*time.Second)
			defer cancel()
			complete := returned && !failed && body != nil && body.ended && !body.failed && r.Context().Err() == nil
			if err := lease.Finish(ctx, ForwardResult{Status: status, Complete: complete}); err != nil {
				logrus.Warn("Registry request finalization pending; retained coordination must be verified")
			}
		}()
	}
	proxy := httputil.NewSingleHostReverseProxy(p.upstream)
	proxy.Transport = p.transport
	proxy.ErrorLog = log.New(safeProxyLog{}, "", 0)
	proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, _ error) {
		failed = true
		http.Error(w, "registry upstream unavailable", http.StatusBadGateway)
	}
	proxy.ModifyResponse = func(response *http.Response) error {
		status = response.StatusCode
		body = &observedBody{ReadCloser: response.Body, ended: response.ContentLength == 0 || response.Body == http.NoBody}
		response.Body = body
		if lease == nil {
			return nil
		}
		observation := ResponseObservation{Status: status, UploadID: response.Header.Get("Docker-Upload-UUID"), ContentDigest: response.Header.Get("Docker-Content-Digest")}
		if observation.UploadID != "" && !uploadReference.MatchString(observation.UploadID) {
			return ErrUnsupportedRequest
		}
		if location := response.Header.Get("Location"); location != "" {
			u, err := url.Parse(location)
			if err != nil {
				return ErrUnsupportedRequest
			}
			// Only the path is used as evidence. Opaque upload query state is neither
			// persisted nor logged; the original Registry response remains unchanged.
			observation.LocationPath = u.Path
		}
		return lease.ObserveResponse(r.Context(), observation)
	}
	forwarded := r.Clone(r.Context())
	forwarded.Header.Del("Cookie")
	forwarded.Header.Del("Proxy-Authorization")
	// Caller hints must not make the transport replay Registry mutations.
	forwarded.Header.Del("Idempotency-Key")
	forwarded.Header.Del("X-Idempotency-Key")
	forwarded.Header.Del("X-Rainbond-Cleanup-Permit")
	proxy.ServeHTTP(w, forwarded)
	returned = true
}
