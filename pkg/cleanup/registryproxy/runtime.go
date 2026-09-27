//go:build linux || darwin

package registryproxy

import (
	"context"
	"net/http"
	"net/url"
	"sync"
	"time"

	coordination "github.com/goodrain/rainbond/pkg/cleanup"
)

// RuntimeBackend adds read-only enrollment inspection to request coordination.
type RuntimeBackend interface {
	CoordinationBackend
	InspectStorage(context.Context, string, string) (coordination.StorageObservation, error)
	RegisterRegistryParticipant(context.Context, string, string, string, string, string) error
}

// RuntimeConfig is populated by the installer from verified deployment facts.
type RuntimeConfig struct {
	Root            string
	Binding         coordination.StorageRegistration
	Upstream, Owner string
	Pod, PodUID     string
	Backend         RuntimeBackend
	PermitKey       func() []byte
	Transport       http.RoundTripper
}

// Runtime keeps process liveness independent from dependency readiness.
type Runtime struct {
	config         RuntimeConfig
	proxy          *Proxy
	fingerprint    string
	registrationMu sync.Mutex
	registered     bool
}

// NewRuntime never initializes or rebinds a mount as a side effect of startup.
func NewRuntime(config RuntimeConfig) (*Runtime, error) {
	fingerprint, err := config.Binding.Fingerprint()
	if err != nil || config.Backend == nil || config.Pod == "" || config.PodUID == "" {
		return nil, ErrStorageIdentity
	}
	if err := VerifyStorageIdentity(config.Root, config.Binding); err != nil {
		return nil, err
	}
	coordinator, err := NewCoordinator(CoordinatorConfig{StorageID: config.Binding.StorageID, Generation: config.Binding.Generation, Owner: config.Owner, Backend: config.Backend, PermitKey: config.PermitKey})
	if err != nil {
		return nil, err
	}
	proxy, err := NewProxy(config.Upstream, coordinator, config.Transport)
	if err != nil {
		return nil, err
	}
	return &Runtime{config: config, proxy: proxy, fingerprint: fingerprint}, nil
}
func (r *Runtime) checkBinding(ctx context.Context) error {
	if err := VerifyStorageIdentity(r.config.Root, r.config.Binding); err != nil {
		return err
	}
	observation, err := r.config.Backend.InspectStorage(ctx, r.config.Binding.StorageID, r.config.Binding.Generation)
	if err != nil {
		return err
	}
	if observation.StorageID != r.config.Binding.StorageID || observation.Generation != r.config.Binding.Generation || observation.RegistrationFingerprint != r.fingerprint {
		return ErrStorageIdentity
	}
	switch observation.Mode {
	case "collecting", "ready", "draining", "maintenance", "restoring", "recovery_required":
		return nil
	default:
		return ErrStorageIdentity
	}
}
func (r *Runtime) ensureRegistered(ctx context.Context, force bool) error {
	r.registrationMu.Lock()
	defer r.registrationMu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if r.registered && !force {
		return nil
	}
	err := r.config.Backend.RegisterRegistryParticipant(ctx, r.config.Binding.StorageID, r.config.Binding.Generation, r.config.Owner, r.config.Pod, r.config.PodUID)
	r.registered = err == nil
	return err
}
func (r *Runtime) ready(ctx context.Context) error {
	if err := r.checkBinding(ctx); err != nil {
		return err
	}
	if err := r.ensureRegistered(ctx, true); err != nil {
		return err
	}
	target := r.proxy.upstream.ResolveReference(&url.URL{Path: "/v2/"})
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return err
	}
	client := &http.Client{Transport: r.proxy.transport, Timeout: 2 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if (response.StatusCode != 200 && response.StatusCode != 401) || response.Header.Get("Docker-Distribution-API-Version") != "registry/2.0" {
		return ErrUnsupportedRequest
	}
	return nil
}

// ServeHTTP exposes no configuration setters or unauthenticated cleanup action.
func (r *Runtime) ServeHTTP(w http.ResponseWriter, request *http.Request) {
	if request.URL.Path == "/healthz" || request.URL.Path == "/readyz" {
		if request.Method != http.MethodGet {
			http.Error(w, "method not allowed", 405)
			return
		}
		if request.URL.Path == "/readyz" {
			ctx, cancel := context.WithTimeout(request.Context(), 2*time.Second)
			defer cancel()
			if err := r.ready(ctx); err != nil {
				http.Error(w, "registry coordinator not ready", 503)
				return
			}
		}
		w.WriteHeader(200)
		return
	}
	// Pod-IP access must not bypass identity checks when Kubernetes readiness is
	// false. The request coordinator still decides each mutation's eligibility.
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		ctx, cancel := context.WithTimeout(request.Context(), 2*time.Second)
		err := r.checkBinding(ctx)
		if err == nil {
			err = r.ensureRegistered(ctx, false)
		}
		cancel()
		if err != nil {
			http.Error(w, "registry storage binding unavailable", 503)
			return
		}
	} else if err := VerifyStorageIdentity(r.config.Root, r.config.Binding); err != nil {
		http.Error(w, "registry storage binding unavailable", 503)
		return
	}
	r.proxy.ServeHTTP(w, request)
}
