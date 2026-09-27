package registryproxy

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/url"
	"time"

	coordination "github.com/goodrain/rainbond/pkg/cleanup"
)

// CoordinationBackend is the authenticated Region coordination contract.
type CoordinationBackend interface {
	Acquire(context.Context, coordination.CoordinationRequest) (bool, error)
	Finish(context.Context, coordination.CoordinationRequest, bool) error
	BindRegistryUpload(context.Context, coordination.CoordinationRequest, string, string) error
	LookupRegistryUpload(context.Context, string, string, string, string) (coordination.CoordinationRequest, error)
	AcquireUploadRequest(context.Context, coordination.CoordinationRequest, coordination.CoordinationRequest, bool) (bool, error)
	RecordUploadRequest(context.Context, coordination.CoordinationRequest, coordination.CoordinationRequest, string) error
	BeginDeletionAttempt(context.Context, coordination.CoordinationRequest) error
	CompleteDeletionAttempt(context.Context, coordination.CoordinationRequest, string) error
}

var _ CoordinationBackend = (*coordination.CoordinationClient)(nil)

// CoordinatorConfig binds a sidecar instance to one verified storage generation.
// The runtime must complete participant and physical-storage verification before
// becoming ready; this constructor does not establish those deployment facts.
type CoordinatorConfig struct {
	StorageID, Generation, Owner string
	Backend                      CoordinationBackend
	PermitKey                    func() []byte
	Now                          func() time.Time
}

// Coordinator maintains request and multipart leases through the Region API.
type Coordinator struct{ config CoordinatorConfig }

// NewCoordinator rejects missing identities or dependencies.
func NewCoordinator(config CoordinatorConfig) (*Coordinator, error) {
	if config.StorageID == "" || config.Generation == "" || config.Owner == "" || config.Backend == nil || config.PermitKey == nil {
		return nil, ErrUnsupportedRequest
	}
	if config.Now == nil {
		config.Now = time.Now
	}
	return &Coordinator{config: config}, nil
}
func admissionError(err error) error {
	switch {
	case errors.Is(err, coordination.ErrCoordinationBusy), errors.Is(err, coordination.ErrCoordinationChanged), errors.Is(err, coordination.ErrCoordinationUncertain):
		return ErrAdmissionBusy
	case errors.Is(err, coordination.ErrCoordinationDenied), errors.Is(err, coordination.ErrCoordinationNotFound):
		return ErrAdmissionDenied
	default:
		return err
	}
}
func (c *Coordinator) newRequest(r *http.Request, d RequestDescriptor) (coordination.CoordinationRequest, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return coordination.CoordinationRequest{}, err
	}
	id := "registry-" + hex.EncodeToString(random[:])
	fingerprint := sha256.Sum256([]byte(r.Method + "\x00" + r.URL.EscapedPath() + "\x00" + r.URL.RawQuery + "\x00" + id))
	scope := d.Repository
	if d.MountFrom != "" && d.MountFrom != d.Repository {
		scope = "*"
	}
	return coordination.CoordinationRequest{StorageID: c.config.StorageID, Generation: c.config.Generation, Owner: c.config.Owner, OperationID: id, Kind: "producer", Scope: scope, Fingerprint: hex.EncodeToString(fingerprint[:])}, nil
}

// Start admits one request and never interprets an idempotent acknowledgement
// as a fresh forwarding grant.
func (c *Coordinator) Start(ctx context.Context, r *http.Request, d RequestDescriptor) (RequestLease, error) {
	actual, err := ClassifyRequest(r.Method, r.URL)
	if err != nil || actual != d || !d.Mutating {
		return nil, ErrAdmissionDenied
	}
	lease := &coordinatedLease{backend: c.config.Backend, description: d, expectedDigest: r.URL.Query().Get("digest")}
	if d.Kind == "manifest_delete" {
		permit, err := coordination.VerifyRegistryDeletionPermit(c.config.PermitKey(), r.Header.Get("X-Rainbond-Cleanup-Permit"), c.config.Now())
		if err != nil || permit.StorageID != c.config.StorageID || permit.Generation != c.config.Generation || permit.Scope != d.Repository || permit.Target != d.Target {
			return nil, ErrAdmissionDenied
		}
		if err := c.config.Backend.BeginDeletionAttempt(ctx, permit); err != nil {
			return nil, admissionError(err)
		}
		lease.binding = permit
		return lease, nil
	}
	binding, err := c.newRequest(r, d)
	if err != nil {
		return nil, err
	}
	lease.binding = binding
	if d.Kind == "upload_part" || d.Kind == "upload_complete" || d.Kind == "upload_abort" {
		parent, err := c.config.Backend.LookupRegistryUpload(ctx, c.config.StorageID, c.config.Generation, d.Repository, d.Target)
		if err != nil {
			return nil, admissionError(err)
		}
		// Continuations use the actual repository, even if a cross-repository mount
		// conservatively acquired a global parent before falling back to upload.
		binding.Scope = d.Repository
		lease.binding = binding
		lease.parent = &parent
		created, err := c.config.Backend.AcquireUploadRequest(ctx, parent, binding, d.Kind != "upload_part")
		if err != nil {
			return nil, admissionError(err)
		}
		if !created {
			return nil, ErrAdmissionBusy
		}
		return lease, nil
	}
	created, err := c.config.Backend.Acquire(ctx, binding)
	if err != nil {
		return nil, admissionError(err)
	}
	if !created {
		return nil, ErrAdmissionBusy
	}
	return lease, nil
}

type coordinatedLease struct {
	backend          CoordinationBackend
	description      RequestDescriptor
	binding          coordination.CoordinationRequest
	parent           *coordination.CoordinationRequest
	expectedDigest   string
	responseObserved bool
	responseStatus   int
	uploadBound      bool
}

func (l *coordinatedLease) ObserveResponse(ctx context.Context, o ResponseObservation) error {
	l.responseObserved = true
	l.responseStatus = o.Status
	if l.description.Kind == "upload_start" && o.Status == http.StatusAccepted {
		if o.UploadID == "" {
			return ErrUnsupportedRequest
		}
		location, err := ClassifyRequest(http.MethodGet, &url.URL{Path: o.LocationPath})
		if err != nil || location.Repository != l.description.Repository || location.Target != o.UploadID {
			return ErrUnsupportedRequest
		}
		if err := l.backend.BindRegistryUpload(ctx, l.binding, l.description.Repository, o.UploadID); err != nil {
			return err
		}
		l.uploadBound = true
	}
	if l.description.Kind == "upload_part" && o.Status == http.StatusAccepted && o.UploadID != l.description.Target {
		return ErrUnsupportedRequest
	}
	if l.description.Kind == "upload_complete" && o.Status == http.StatusCreated && o.ContentDigest != l.expectedDigest {
		return ErrUnsupportedRequest
	}
	return nil
}
func knownRejection(status int) bool {
	return status == 400 || status == 401 || status == 403 || status == 404 || status == 405 || status == 409 || status == 412 || status == 416
}
func (l *coordinatedLease) Finish(ctx context.Context, result ForwardResult) error {
	complete := result.Complete && l.responseObserved && result.Status == l.responseStatus
	if l.description.Kind == "manifest_delete" {
		outcome := "unknown"
		if complete && result.Status == http.StatusAccepted {
			outcome = "applied"
		} else if complete && knownRejection(result.Status) {
			outcome = "rejected"
		}
		return l.backend.CompleteDeletionAttempt(ctx, l.binding, outcome)
	}
	if l.parent != nil {
		expected := http.StatusAccepted
		if l.description.Kind == "upload_complete" {
			expected = http.StatusCreated
		}
		if l.description.Kind == "upload_abort" {
			expected = http.StatusNoContent
		}
		outcome := "unknown"
		if complete && result.Status == expected {
			outcome = "succeeded"
		} else if complete && knownRejection(result.Status) {
			outcome = "rejected"
		}
		return l.backend.RecordUploadRequest(ctx, *l.parent, l.binding, outcome)
	}
	if l.description.Kind == "upload_start" && result.Status == http.StatusAccepted {
		if complete && l.uploadBound {
			return nil
		} // parent remains durable between parts
		return l.backend.Finish(ctx, l.binding, false)
	}
	known := complete && (result.Status == http.StatusCreated || knownRejection(result.Status))
	return l.backend.Finish(ctx, l.binding, known)
}
