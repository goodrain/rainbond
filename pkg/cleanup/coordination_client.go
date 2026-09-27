package cleanup

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ErrCoordinationUnavailable never includes an upstream URL, credential or body.
var ErrCoordinationUnavailable = errors.New("cleanup coordination unavailable")

// ErrCoordinationNotFound means no matching durable operation was recorded.
var ErrCoordinationNotFound = errors.New("cleanup coordination operation not found")

// ErrCoordinationDenied indicates that the internal participant was rejected.
var ErrCoordinationDenied = errors.New("cleanup coordination authorization denied")

// CoordinationClient is used by participants without direct Region DB access.
// It does not retry writes or follow redirects with the Region credential.
type CoordinationClient struct {
	base  string
	token string
	http  *http.Client
}

// NewCoordinationClient binds one trusted Region API origin. Plain HTTP must be
// explicitly allowed for a trusted internal deployment or an isolated test.
func NewCoordinationClient(endpoint, token string, allowHTTP bool, transports ...http.RoundTripper) (*CoordinationClient, error) {
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") || (u.Scheme != "https" && !(allowHTTP && u.Scheme == "http")) || token == "" || strings.ContainsAny(token, " \t\r\n") {
		return nil, ErrCoordinationChanged
	}
	if len(transports) > 1 {
		return nil, ErrCoordinationChanged
	}
	var transport http.RoundTripper
	if len(transports) == 1 {
		transport = transports[0]
		if transport == nil {
			return nil, ErrCoordinationChanged
		}
	}
	return &CoordinationClient{base: u.Scheme + "://" + u.Host, token: token, http: &http.Client{Transport: transport, Timeout: 4 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}

type coordinationResponse struct {
	Msg  string `json:"msg"`
	Bean struct {
		NodeJob           *NodeJobProgress     `json:"node_job"`
		GCJob             *GCJobProgress       `json:"gc_job"`
		Protocol          int                  `json:"protocol"`
		NewlyAdmitted     *bool                `json:"newly_admitted"`
		Recorded          *bool                `json:"recorded"`
		State             string               `json:"state"`
		Permit            string               `json:"permit"`
		Binding           *CoordinationRequest `json:"binding"`
		Storage           *StorageObservation  `json:"storage"`
		Registration      *StorageRegistration `json:"registration"`
		RegistryContainer string               `json:"registry_container"`
		Repository        string               `json:"repository"`
		UploadID          string               `json:"upload_id"`
	} `json:"bean"`
}

func (c *CoordinationClient) call(ctx context.Context, r CoordinationRequest, action string, body interface{}) (coordinationResponse, error) {
	var result coordinationResponse
	if !r.valid() {
		return result, ErrCoordinationChanged
	}
	path := "/v2/cleanup/stores/" + url.PathEscape(r.StorageID) + "/operations"
	if action != "" {
		path += "/" + url.PathEscape(r.OperationID) + "/" + action
	}
	return c.callPath(ctx, path, body)
}
func (c *CoordinationClient) callPath(ctx context.Context, path string, body interface{}) (coordinationResponse, error) {
	var result coordinationResponse
	raw, err := json.Marshal(body)
	if err != nil {
		return result, ErrCoordinationChanged
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+path, bytes.NewReader(raw))
	if err != nil {
		return result, ErrCoordinationChanged
	}
	req.GetBody = nil
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Token "+c.token)
	response, err := c.http.Do(req)
	if err != nil {
		return result, ErrCoordinationUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode == 401 || response.StatusCode == 403 {
		return result, ErrCoordinationDenied
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, (64<<10)+1))
	if err != nil || len(data) > 64<<10 {
		return result, ErrCoordinationUnavailable
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	if decoder.Decode(&result) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return result, ErrCoordinationUnavailable
	}
	if response.StatusCode == 404 && result.Msg == "COORDINATION_NOT_FOUND" {
		return result, ErrCoordinationNotFound
	}
	if response.StatusCode != http.StatusOK {
		if response.StatusCode == 409 {
			switch result.Msg {
			case "COORDINATION_UNCERTAIN":
				return result, ErrCoordinationUncertain
			case "COORDINATION_CHANGED":
				return result, ErrCoordinationChanged
			default:
				return result, ErrCoordinationBusy
			}
		}
		return result, ErrCoordinationUnavailable
	}
	if result.Bean.Protocol != 1 {
		return result, ErrCoordinationUnavailable
	}
	return result, nil
}

// Acquire returns true only for an explicitly acknowledged new admission.
func (c *CoordinationClient) Acquire(ctx context.Context, r CoordinationRequest) (bool, error) {
	if r.Kind == "gc" {
		return false, ErrCoordinationChanged
	}
	response, err := c.call(ctx, r, "", r)
	if err != nil {
		return false, err
	}
	if response.Bean.NewlyAdmitted == nil {
		return false, ErrCoordinationUnavailable
	}
	return *response.Bean.NewlyAdmitted, nil
}

// Finish records a confirmed or uncertain outcome; it never releases uncertainty.
func (c *CoordinationClient) Finish(ctx context.Context, r CoordinationRequest, confirmed bool) error {
	if r.Kind == "gc" {
		return ErrCoordinationChanged
	}
	body := struct {
		CoordinationRequest
		Confirmed bool `json:"confirmed"`
	}{r, confirmed}
	response, err := c.call(ctx, r, "finish", body)
	if err != nil {
		return err
	}
	if response.Bean.Recorded == nil || !*response.Bean.Recorded {
		return ErrCoordinationUnavailable
	}
	return nil
}

// Inspect observes the exact previously bound operation without resubmission.
func (c *CoordinationClient) Inspect(ctx context.Context, r CoordinationRequest) (string, error) {
	response, err := c.call(ctx, r, "inspect", r)
	if err != nil {
		return "", err
	}
	switch response.Bean.State {
	case "active", "executing", "applied", "rejected", "uncertain", "finished", "draining", "exclusive", "restore_pending", "restoring":
		return response.Bean.State, nil
	default:
		return "", ErrCoordinationUnavailable
	}
}

// RequestMaintenance closes admission without granting GC execution.
func (c *CoordinationClient) RequestMaintenance(ctx context.Context, r CoordinationRequest) (bool, error) {
	response, err := c.call(ctx, r, "maintenance/request", r)
	if err != nil {
		return false, err
	}
	if response.Bean.NewlyAdmitted == nil {
		return false, ErrCoordinationUnavailable
	}
	return *response.Bean.NewlyAdmitted, nil
}
func (c *CoordinationClient) record(ctx context.Context, r CoordinationRequest, action string, body interface{}) error {
	response, err := c.call(ctx, r, action, body)
	if err != nil {
		return err
	}
	if response.Bean.Recorded == nil || !*response.Bean.Recorded {
		return ErrCoordinationUnavailable
	}
	return nil
}

// EnterMaintenance obtains the one-time execution grant after draining writers.
func (c *CoordinationClient) EnterMaintenance(ctx context.Context, r CoordinationRequest) error {
	return c.record(ctx, r, "maintenance/enter", r)
}

// CancelDrain cancels maintenance only before any GC execution grant.
func (c *CoordinationClient) CancelDrain(ctx context.Context, r CoordinationRequest) error {
	return c.record(ctx, r, "maintenance/cancel", r)
}

// CompleteMaintenanceWork records the observed GC process outcome.
func (c *CoordinationClient) CompleteMaintenanceWork(ctx context.Context, r CoordinationRequest, outcome string) error {
	return c.record(ctx, r, "maintenance/complete", struct {
		CoordinationRequest
		Outcome string `json:"outcome"`
	}{r, outcome})
}

// RecordMaintenanceMeasurement persists an observation without granting execution
// or restoring writes. Only a trusted executor should report observations.
func (c *CoordinationClient) RecordMaintenanceMeasurement(ctx context.Context, r CoordinationRequest, phase string, measurement StorageMeasurement) error {
	if r.Kind != "gc" || (phase != "before" && phase != "after") || !IsValidStorageMeasurement(measurement) || measurement.StorageID != r.StorageID || measurement.Generation != r.Generation {
		return ErrCoordinationChanged
	}
	return c.record(ctx, r, "maintenance/measurement", struct {
		CoordinationRequest
		Phase       string             `json:"phase"`
		Measurement StorageMeasurement `json:"measurement"`
	}{r, phase, measurement})
}

// BeginRestore durably records native restoration intent.
func (c *CoordinationClient) BeginRestore(ctx context.Context, r CoordinationRequest) error {
	return c.record(ctx, r, "maintenance/restore", r)
}

// FinishRestore cannot turn an uncertain restoration into success by retrying.
func (c *CoordinationClient) FinishRestore(ctx context.Context, r CoordinationRequest, confirmed bool) error {
	return c.record(ctx, r, "maintenance/restored", struct {
		CoordinationRequest
		Confirmed bool `json:"confirmed"`
	}{r, confirmed})
}

// RegistryPermit requests delegation only for the original active manifest target.
func (c *CoordinationClient) RegistryPermit(ctx context.Context, r CoordinationRequest) (string, error) {
	response, err := c.call(ctx, r, "registry-permit", r)
	if err != nil {
		return "", err
	}
	if response.Bean.Permit == "" {
		return "", ErrCoordinationUnavailable
	}
	return response.Bean.Permit, nil
}

// BeginDeletionAttempt consumes a durable one-time Registry forwarding grant.
func (c *CoordinationClient) BeginDeletionAttempt(ctx context.Context, r CoordinationRequest) error {
	return c.record(ctx, r, "attempt", r)
}

// CompleteDeletionAttempt records the observed response while retaining the fence.
func (c *CoordinationClient) CompleteDeletionAttempt(ctx context.Context, r CoordinationRequest, outcome string) error {
	return c.record(ctx, r, "attempt/complete", struct {
		CoordinationRequest
		Outcome string `json:"outcome"`
	}{r, outcome})
}

// BindRegistryUpload durably associates the admitted request with its upload ID.
func (c *CoordinationClient) BindRegistryUpload(ctx context.Context, r CoordinationRequest, repository, id string) error {
	return c.record(ctx, r, "upload", struct {
		CoordinationRequest
		Repository string `json:"repository"`
		UploadID   string `json:"upload_id"`
	}{r, repository, id})
}

// LookupRegistryUpload resolves a previously admitted upload after process restart.
func (c *CoordinationClient) LookupRegistryUpload(ctx context.Context, storage, generation, repository, id string) (CoordinationRequest, error) {
	if _, err := registryUploadKey(storage, generation, repository, id); err != nil {
		return CoordinationRequest{}, err
	}
	body := struct {
		Generation string `json:"generation"`
		Repository string `json:"repository"`
		UploadID   string `json:"upload_id"`
	}{generation, repository, id}
	response, err := c.callPath(ctx, "/v2/cleanup/stores/"+url.PathEscape(storage)+"/uploads/lookup", body)
	if err != nil {
		return CoordinationRequest{}, err
	}
	binding := response.Bean.Binding
	if binding == nil {
		return CoordinationRequest{}, ErrCoordinationUnavailable
	}
	binding.StorageID = storage
	if !binding.valid() || binding.Kind != "producer" || binding.Generation != generation || (binding.Scope != "*" && binding.Scope != repository) || response.Bean.Repository != repository || response.Bean.UploadID != id {
		return CoordinationRequest{}, ErrCoordinationUnavailable
	}
	return *binding, nil
}

// AcquireUploadRequest permits only a request belonging to an existing session.
func (c *CoordinationClient) AcquireUploadRequest(ctx context.Context, parent, r CoordinationRequest, closing bool) (bool, error) {
	if !r.valid() || r.StorageID != parent.StorageID || r.Generation != parent.Generation {
		return false, ErrCoordinationChanged
	}
	body := struct {
		Parent  CoordinationRequest `json:"parent"`
		Request CoordinationRequest `json:"request"`
		Closing bool                `json:"closing"`
	}{parent, r, closing}
	response, err := c.call(ctx, parent, "upload/requests", body)
	if err != nil {
		return false, err
	}
	if response.Bean.NewlyAdmitted == nil {
		return false, ErrCoordinationUnavailable
	}
	return *response.Bean.NewlyAdmitted, nil
}

// FinishUploadRequest records a part outcome and preserves the parent lifecycle.
func (c *CoordinationClient) FinishUploadRequest(ctx context.Context, parent, r CoordinationRequest, confirmed bool) error {
	outcome := "unknown"
	if confirmed {
		outcome = "succeeded"
	}
	return c.RecordUploadRequest(ctx, parent, r, outcome)
}

// RecordUploadRequest records an actual success, rejection or unknown result.
func (c *CoordinationClient) RecordUploadRequest(ctx context.Context, parent, r CoordinationRequest, outcome string) error {
	body := struct {
		Parent  CoordinationRequest `json:"parent"`
		Request CoordinationRequest `json:"request"`
		Outcome string              `json:"outcome"`
	}{parent, r, outcome}
	return c.record(ctx, parent, "upload/requests/finish", body)
}

// InspectStorage verifies the response belongs to the requested generation.
func (c *CoordinationClient) InspectStorage(ctx context.Context, storage, generation string) (StorageObservation, error) {
	if !coordinationIdentity.MatchString(storage) || !coordinationIdentity.MatchString(generation) {
		return StorageObservation{}, ErrCoordinationChanged
	}
	response, err := c.callPath(ctx, "/v2/cleanup/stores/"+url.PathEscape(storage)+"/status", struct {
		Generation string `json:"generation"`
	}{generation})
	if err != nil {
		return StorageObservation{}, err
	}
	observed := response.Bean.Storage
	if observed == nil || observed.StorageID != storage || observed.Generation != generation {
		return StorageObservation{}, ErrCoordinationChanged
	}
	return *observed, nil
}

// RegistryPreparation is configuration data, not permission to begin cleanup.
type RegistryPreparation struct {
	Registration      StorageRegistration
	Storage           StorageObservation
	RegistryContainer string
}

// PrepareRegistry obtains the physical identity selected by the control plane.
func (c *CoordinationClient) PrepareRegistry(ctx context.Context, pod, uid string) (RegistryPreparation, error) {
	if pod == "" || len(pod) > 253 || uid == "" || len(uid) > 64 {
		return RegistryPreparation{}, ErrCoordinationChanged
	}
	response, err := c.callPath(ctx, "/v2/cleanup/registry/prepare", struct {
		Pod    string `json:"pod"`
		PodUID string `json:"pod_uid"`
	}{pod, uid})
	if err != nil {
		return RegistryPreparation{}, err
	}
	binding, storage := response.Bean.Registration, response.Bean.Storage
	if binding == nil || storage == nil || response.Bean.RegistryContainer == "" {
		return RegistryPreparation{}, ErrCoordinationUnavailable
	}
	fingerprint, err := binding.Fingerprint()
	if err != nil || storage.StorageID != binding.StorageID || storage.Generation != binding.Generation || storage.RegistrationFingerprint != fingerprint {
		return RegistryPreparation{}, ErrCoordinationChanged
	}
	return RegistryPreparation{Registration: *binding, Storage: *storage, RegistryContainer: response.Bean.RegistryContainer}, nil
}

// RegisterRegistryParticipant submits only locators, not claimed container facts.
func (c *CoordinationClient) RegisterRegistryParticipant(ctx context.Context, storage, generation, owner, pod, uid string) error {
	if !coordinationIdentity.MatchString(storage) || !coordinationIdentity.MatchString(generation) {
		return ErrCoordinationChanged
	}
	body := struct {
		Generation string `json:"generation"`
		Owner      string `json:"owner"`
		Pod        string `json:"pod"`
		PodUID     string `json:"pod_uid"`
	}{generation, owner, pod, uid}
	response, err := c.callPath(ctx, "/v2/cleanup/stores/"+url.PathEscape(storage)+"/participants/registry", body)
	if err != nil {
		return err
	}
	if response.Bean.Recorded == nil || !*response.Bean.Recorded {
		return ErrCoordinationUnavailable
	}
	return nil
}
