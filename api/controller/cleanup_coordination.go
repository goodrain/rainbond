package controller

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/config/configs"
	"github.com/goodrain/rainbond/db"
	guard "github.com/goodrain/rainbond/pkg/cleanup"
	"github.com/goodrain/rainbond/pkg/cleanup/kubeidentity"
	"github.com/goodrain/rainbond/pkg/component/k8s"
	httputil "github.com/goodrain/rainbond/util/http"
	"github.com/jinzhu/gorm"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/kubernetes"
)

// CleanupCoordinationHandler is an internal Region API. Routes must always use
// CleanupIdentity, requiring a configured token or verified platform mutual TLS.
// This API records coordination only; it never performs deletion or enables an
// unverified store. Owner identities come from trusted Region participants.
type CleanupCoordinationHandler struct {
	nodeSettings        func() kubeidentity.NodeJobSettings
	inventorySettings   func() kubeidentity.NodeInventorySettings
	inspectManagedCache func(context.Context, string, string) (kubeidentity.ManagedCachePreparation, error)
	gcTarget            func() (kubernetes.Interface, string, string, error)
	database            func() *gorm.DB
	permitKey           func() []byte
	inspectRegistry     func(context.Context, string, string) (kubeidentity.RegistryPreparation, error)
	inspectParticipant  func(context.Context, string, string, string, guard.StorageRegistration) (guard.ParticipantRegistration, error)
}

// NewCleanupCoordinationHandler uses the Region database manager.
func NewCleanupCoordinationHandler() *CleanupCoordinationHandler {
	return &CleanupCoordinationHandler{inventorySettings: systemNodeInventorySettings, nodeSettings: systemNodeJobSettings, database: func() *gorm.DB { return db.GetManager().DB() }, permitKey: func() []byte { return []byte(os.Getenv("TOKEN")) }, inspectRegistry: inspectSystemRegistry, inspectManagedCache: inspectSystemManagedCache, inspectParticipant: inspectSystemRegistryParticipant, gcTarget: systemRegistryInspectionTarget}
}

// DiscoverStores locates enrolled storage for authenticated platform producers.
// It neither registers new storage nor enables deletion readiness.
func (h *CleanupCoordinationHandler) DiscoverStores(w http.ResponseWriter, r *http.Request) {
	var body struct{}
	if !coordinationDecode(w, r, &body) {
		return
	}
	stores, err := guard.DiscoverStores(h.database())
	if err != nil {
		coordinationError(w, r, err)
		return
	}
	httputil.ReturnSuccess(r, w, struct {
		Protocol int                   `json:"protocol"`
		Stores   []guard.StoreIdentity `json:"stores"`
	}{1, stores})
}

func coordinationError(w http.ResponseWriter, r *http.Request, err error) {
	status, code := http.StatusServiceUnavailable, "COORDINATION_UNAVAILABLE"
	switch {
	case errors.Is(err, guard.ErrCoordinationDenied):
		status, code = 403, "COORDINATION_DENIED"
	case errors.Is(err, guard.ErrCoordinationBusy):
		status, code = 409, "COORDINATION_BUSY"
	case errors.Is(err, guard.ErrCoordinationChanged):
		status, code = 409, "COORDINATION_CHANGED"
	case errors.Is(err, guard.ErrCoordinationUncertain):
		status, code = 409, "COORDINATION_UNCERTAIN"
	case errors.Is(err, gorm.ErrRecordNotFound):
		status, code = 404, "COORDINATION_NOT_FOUND"
	}
	httputil.ReturnError(r, w, status, code)
}
func coordinationDecode(w http.ResponseWriter, r *http.Request, value interface{}) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	decoder.DisallowUnknownFields()
	if decoder.Decode(value) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		httputil.ReturnError(r, w, 400, "INVALID_COORDINATION_REQUEST")
		return false
	}
	return true
}
func coordinationScopeFromRoute(w http.ResponseWriter, r *http.Request, request *guard.CoordinationRequest) bool {
	request.StorageID = chi.URLParam(r, "storage_id")
	if id := chi.URLParam(r, "operation_id"); id != "" && request.OperationID != id {
		httputil.ReturnError(r, w, 400, "INVALID_COORDINATION_SCOPE")
		return false
	}
	return true
}

// Acquire records an admission bound to the server-routed store.
func (h *CleanupCoordinationHandler) Acquire(w http.ResponseWriter, r *http.Request) {
	var request guard.CoordinationRequest
	if !coordinationDecode(w, r, &request) || !coordinationScopeFromRoute(w, r, &request) {
		return
	}
	if err := r.Context().Err(); err != nil {
		coordinationError(w, r, err)
		return
	}
	created, err := guard.AcquireOperation(h.database(), request)
	if err != nil {
		coordinationError(w, r, err)
		return
	}
	httputil.ReturnSuccess(r, w, struct {
		Protocol      int  `json:"protocol"`
		NewlyAdmitted bool `json:"newly_admitted"`
	}{1, created})
}

// Finish records an original operation outcome without unlocking uncertain work.
func (h *CleanupCoordinationHandler) Finish(w http.ResponseWriter, r *http.Request) {
	var body struct {
		guard.CoordinationRequest
		Confirmed *bool `json:"confirmed"`
	}
	if !coordinationDecode(w, r, &body) || !coordinationScopeFromRoute(w, r, &body.CoordinationRequest) {
		return
	}
	if body.Confirmed == nil {
		httputil.ReturnError(r, w, 400, "CONFIRMATION_REQUIRED")
		return
	}
	if err := r.Context().Err(); err != nil {
		coordinationError(w, r, err)
		return
	}
	if err := guard.FinishOperation(h.database(), body.CoordinationRequest, *body.Confirmed); err != nil {
		coordinationError(w, r, err)
		return
	}
	httputil.ReturnSuccess(r, w, struct {
		Protocol int  `json:"protocol"`
		Recorded bool `json:"recorded"`
	}{1, true})
}

// Inspect returns only the exact bound operation state.
func (h *CleanupCoordinationHandler) Inspect(w http.ResponseWriter, r *http.Request) {
	var request guard.CoordinationRequest
	if !coordinationDecode(w, r, &request) || !coordinationScopeFromRoute(w, r, &request) {
		return
	}
	if err := r.Context().Err(); err != nil {
		coordinationError(w, r, err)
		return
	}
	state, err := guard.InspectOperation(h.database(), request)
	if err != nil {
		coordinationError(w, r, err)
		return
	}
	httputil.ReturnSuccess(r, w, struct {
		Protocol int    `json:"protocol"`
		State    string `json:"state"`
	}{1, state})
}

// RequestMaintenance closes admission but does not start GC.
func (h *CleanupCoordinationHandler) RequestMaintenance(w http.ResponseWriter, r *http.Request) {
	var request guard.CoordinationRequest
	if !coordinationDecode(w, r, &request) || !coordinationScopeFromRoute(w, r, &request) {
		return
	}
	if err := r.Context().Err(); err != nil {
		coordinationError(w, r, err)
		return
	}
	created, err := guard.RequestMaintenance(h.database(), request)
	if err != nil {
		coordinationError(w, r, err)
		return
	}
	httputil.ReturnSuccess(r, w, struct {
		Protocol      int  `json:"protocol"`
		NewlyAdmitted bool `json:"newly_admitted"`
	}{1, created})
}
func (h *CleanupCoordinationHandler) maintenanceTransition(w http.ResponseWriter, r *http.Request, transition func(*gorm.DB, guard.CoordinationRequest) error) {
	var request guard.CoordinationRequest
	if !coordinationDecode(w, r, &request) || !coordinationScopeFromRoute(w, r, &request) {
		return
	}
	if err := r.Context().Err(); err != nil {
		coordinationError(w, r, err)
		return
	}
	if err := transition(h.database(), request); err != nil {
		coordinationError(w, r, err)
		return
	}
	httputil.ReturnSuccess(r, w, struct {
		Protocol int  `json:"protocol"`
		Recorded bool `json:"recorded"`
	}{1, true})
}

// EnterMaintenance grants one execution after all earlier operations drain.
func (h *CleanupCoordinationHandler) EnterMaintenance(w http.ResponseWriter, r *http.Request) {
	h.maintenanceTransition(w, r, guard.EnterMaintenance)
}

// BeginRestore records intent; it does not declare the native storage writable.
func (h *CleanupCoordinationHandler) BeginRestore(w http.ResponseWriter, r *http.Request) {
	h.maintenanceTransition(w, r, guard.BeginMaintenanceRestore)
}

// CancelDrain only cancels a maintenance request before GC has been granted.
func (h *CleanupCoordinationHandler) CancelDrain(w http.ResponseWriter, r *http.Request) {
	h.maintenanceTransition(w, r, guard.CancelMaintenanceDrain)
}

// CompleteMaintenanceWork records the trusted executor's observed process outcome.
func (h *CleanupCoordinationHandler) CompleteMaintenanceWork(w http.ResponseWriter, r *http.Request) {
	var body struct {
		guard.CoordinationRequest
		Outcome string `json:"outcome"`
	}
	if !coordinationDecode(w, r, &body) || !coordinationScopeFromRoute(w, r, &body.CoordinationRequest) {
		return
	}
	if err := r.Context().Err(); err != nil {
		coordinationError(w, r, err)
		return
	}
	if err := guard.CompleteMaintenanceWork(h.database(), body.CoordinationRequest, body.Outcome); err != nil {
		coordinationError(w, r, err)
		return
	}
	httputil.ReturnSuccess(r, w, struct {
		Protocol int  `json:"protocol"`
		Recorded bool `json:"recorded"`
	}{1, true})
}

// RecordMaintenanceMeasurement accepts immutable observations from an internal
// executor; observations alone never complete GC or restore storage writes.
func (h *CleanupCoordinationHandler) RecordMaintenanceMeasurement(w http.ResponseWriter, r *http.Request) {
	var body struct {
		guard.CoordinationRequest
		Phase       string                   `json:"phase"`
		Measurement guard.StorageMeasurement `json:"measurement"`
	}
	if !coordinationDecode(w, r, &body) || !coordinationScopeFromRoute(w, r, &body.CoordinationRequest) {
		return
	}
	if err := r.Context().Err(); err != nil {
		coordinationError(w, r, err)
		return
	}
	if err := guard.RecordMaintenanceMeasurement(h.database(), body.CoordinationRequest, body.Phase, body.Measurement); err != nil {
		coordinationError(w, r, err)
		return
	}
	httputil.ReturnSuccess(r, w, struct {
		Protocol int  `json:"protocol"`
		Recorded bool `json:"recorded"`
	}{1, true})
}

// FinishRestore records successful native restoration or persistent uncertainty.
func (h *CleanupCoordinationHandler) FinishRestore(w http.ResponseWriter, r *http.Request) {
	var body struct {
		guard.CoordinationRequest
		Confirmed *bool `json:"confirmed"`
	}
	if !coordinationDecode(w, r, &body) || !coordinationScopeFromRoute(w, r, &body.CoordinationRequest) {
		return
	}
	if body.Confirmed == nil {
		httputil.ReturnError(r, w, 400, "CONFIRMATION_REQUIRED")
		return
	}
	if err := r.Context().Err(); err != nil {
		coordinationError(w, r, err)
		return
	}
	if err := guard.FinishMaintenanceRestore(h.database(), body.CoordinationRequest, *body.Confirmed); err != nil {
		coordinationError(w, r, err)
		return
	}
	httputil.ReturnSuccess(r, w, struct {
		Protocol int  `json:"protocol"`
		Recorded bool `json:"recorded"`
	}{1, true})
}

// RegistryReferenceInventory returns advisory image identities for a ready store.
func (h *CleanupCoordinationHandler) RegistryReferenceInventory(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Generation string `json:"generation"`
	}
	if !coordinationDecode(w, r, &body) {
		return
	}
	if err := r.Context().Err(); err != nil {
		coordinationError(w, r, err)
		return
	}
	storage := chi.URLParam(r, "storage_id")
	result, err := guard.ReadRegionReferenceInventory(h.database(), storage, body.Generation)
	if err != nil {
		coordinationError(w, r, err)
		return
	}
	httputil.ReturnSuccess(r, w, struct {
		Protocol   int    `json:"protocol"`
		StorageID  string `json:"storage_id"`
		Generation string `json:"generation"`
		guard.RegionReferenceInventory
	}{1, storage, body.Generation, result})
}

// RegistryReferences audits retained Region records while the selected scope is held.
func (h *CleanupCoordinationHandler) RegistryReferences(w http.ResponseWriter, r *http.Request) {
	var body struct {
		guard.CoordinationRequest
		Tags []string `json:"tags"`
	}
	if !coordinationDecode(w, r, &body) || !coordinationScopeFromRoute(w, r, &body.CoordinationRequest) {
		return
	}
	if err := r.Context().Err(); err != nil {
		coordinationError(w, r, err)
		return
	}
	result, err := guard.AuditRegionManifestReferences(h.database(), body.CoordinationRequest, body.Tags)
	if err != nil {
		coordinationError(w, r, err)
		return
	}
	httputil.ReturnSuccess(r, w, struct {
		Protocol int `json:"protocol"`
		guard.RegionReferenceAudit
	}{1, result})
}

// RegistryPermit issues a short-lived credential for an active immutable target.
func (h *CleanupCoordinationHandler) RegistryPermit(w http.ResponseWriter, r *http.Request) {
	var request guard.CoordinationRequest
	if !coordinationDecode(w, r, &request) || !coordinationScopeFromRoute(w, r, &request) {
		return
	}
	if h.permitKey == nil {
		coordinationError(w, r, guard.ErrCoordinationDenied)
		return
	}
	state, err := guard.InspectOperation(h.database(), request)
	if err != nil {
		coordinationError(w, r, err)
		return
	}
	if state != "active" {
		coordinationError(w, r, guard.ErrCoordinationUncertain)
		return
	}
	permit, err := guard.IssueRegistryDeletionPermit(h.permitKey(), request, time.Now())
	if err != nil {
		coordinationError(w, r, err)
		return
	}
	httputil.ReturnSuccess(r, w, struct {
		Protocol int    `json:"protocol"`
		Permit   string `json:"permit"`
	}{1, permit})
}

// BeginAttempt consumes the target's one-time execution grant.
func (h *CleanupCoordinationHandler) BeginAttempt(w http.ResponseWriter, r *http.Request) {
	h.maintenanceTransition(w, r, guard.BeginDeletionAttempt)
}

// CompleteAttempt records the observed transport result without releasing scope.
func (h *CleanupCoordinationHandler) CompleteAttempt(w http.ResponseWriter, r *http.Request) {
	var body struct {
		guard.CoordinationRequest
		Outcome string `json:"outcome"`
	}
	if !coordinationDecode(w, r, &body) || !coordinationScopeFromRoute(w, r, &body.CoordinationRequest) {
		return
	}
	if err := guard.CompleteDeletionAttempt(h.database(), body.CoordinationRequest, body.Outcome); err != nil {
		coordinationError(w, r, err)
		return
	}
	httputil.ReturnSuccess(r, w, struct {
		Protocol int  `json:"protocol"`
		Recorded bool `json:"recorded"`
	}{1, true})
}

// BindUpload persists the Registry's exact upload identity before forwarding it.
func (h *CleanupCoordinationHandler) BindUpload(w http.ResponseWriter, r *http.Request) {
	var body struct {
		guard.CoordinationRequest
		Repository string `json:"repository"`
		UploadID   string `json:"upload_id"`
	}
	if !coordinationDecode(w, r, &body) || !coordinationScopeFromRoute(w, r, &body.CoordinationRequest) {
		return
	}
	if err := guard.BindRegistryUpload(h.database(), body.CoordinationRequest, body.Repository, body.UploadID); err != nil {
		coordinationError(w, r, err)
		return
	}
	httputil.ReturnSuccess(r, w, struct {
		Protocol int  `json:"protocol"`
		Recorded bool `json:"recorded"`
	}{1, true})
}

// LookupUpload resolves an existing upload under the server-selected store.
func (h *CleanupCoordinationHandler) LookupUpload(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Generation string `json:"generation"`
		Repository string `json:"repository"`
		UploadID   string `json:"upload_id"`
	}
	if !coordinationDecode(w, r, &body) {
		return
	}
	binding, err := guard.LookupRegistryUpload(h.database(), chi.URLParam(r, "storage_id"), body.Generation, body.Repository, body.UploadID)
	if err != nil {
		coordinationError(w, r, err)
		return
	}
	httputil.ReturnSuccess(r, w, struct {
		Protocol   int                       `json:"protocol"`
		Binding    guard.CoordinationRequest `json:"binding"`
		Repository string                    `json:"repository"`
		UploadID   string                    `json:"upload_id"`
	}{1, binding, body.Repository, body.UploadID})
}

// AcquireUpload admits only a continuation of a recorded upload session.
func (h *CleanupCoordinationHandler) AcquireUpload(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Parent  guard.CoordinationRequest `json:"parent"`
		Request guard.CoordinationRequest `json:"request"`
		Closing *bool                     `json:"closing"`
	}
	if !coordinationDecode(w, r, &body) || !coordinationScopeFromRoute(w, r, &body.Parent) {
		return
	}
	body.Request.StorageID = body.Parent.StorageID
	if body.Closing == nil {
		httputil.ReturnError(r, w, 400, "UPLOAD_PHASE_REQUIRED")
		return
	}
	created, err := guard.AcquireUploadRequest(h.database(), body.Parent, body.Request, *body.Closing)
	if err != nil {
		coordinationError(w, r, err)
		return
	}
	httputil.ReturnSuccess(r, w, struct {
		Protocol      int  `json:"protocol"`
		NewlyAdmitted bool `json:"newly_admitted"`
	}{1, created})
}

// FinishUpload keeps the upload parent until the closing request is confirmed.
func (h *CleanupCoordinationHandler) FinishUpload(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Parent  guard.CoordinationRequest `json:"parent"`
		Request guard.CoordinationRequest `json:"request"`
		Outcome string                    `json:"outcome"`
	}
	if !coordinationDecode(w, r, &body) || !coordinationScopeFromRoute(w, r, &body.Parent) {
		return
	}
	body.Request.StorageID = body.Parent.StorageID
	if body.Outcome == "" {
		httputil.ReturnError(r, w, 400, "CONFIRMATION_REQUIRED")
		return
	}
	if err := guard.RecordUploadRequest(h.database(), body.Parent, body.Request, body.Outcome); err != nil {
		coordinationError(w, r, err)
		return
	}
	httputil.ReturnSuccess(r, w, struct {
		Protocol int  `json:"protocol"`
		Recorded bool `json:"recorded"`
	}{1, true})
}

// StorageStatus exposes identity and enrollment state, never a writable override.
func (h *CleanupCoordinationHandler) StorageStatus(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Generation string `json:"generation"`
	}
	if !coordinationDecode(w, r, &body) {
		return
	}
	observation, err := guard.InspectStorage(h.database(), chi.URLParam(r, "storage_id"), body.Generation)
	if err != nil {
		coordinationError(w, r, err)
		return
	}
	httputil.ReturnSuccess(r, w, struct {
		Protocol int                      `json:"protocol"`
		Storage  guard.StorageObservation `json:"storage"`
	}{1, observation})
}

func systemRegistryInspectionTarget() (kubernetes.Interface, string, string, error) {
	component := k8s.Default()
	configuration := configs.Default()
	if component == nil || component.Clientset == nil || configuration.PublicConfig == nil || configuration.ServerConfig == nil {
		return nil, "", "", kubeidentity.ErrBinding
	}
	namespace := configuration.PublicConfig.RbdNamespace
	endpoint, err := url.Parse(configuration.ServerConfig.RbdHub)
	if err != nil || endpoint.User != nil || namespace == "" {
		return nil, "", "", kubeidentity.ErrBinding
	}
	host := endpoint.Hostname()
	service := strings.Split(host, ".")[0]
	if host != service && host != service+"."+namespace && host != service+"."+namespace+".svc" && host != service+"."+namespace+".svc.cluster.local" {
		return nil, "", "", kubeidentity.ErrBinding
	}
	return component.Clientset, namespace, service, nil
}
func inspectSystemRegistry(ctx context.Context, pod, uid string) (kubeidentity.RegistryPreparation, error) {
	client, namespace, service, err := systemRegistryInspectionTarget()
	if err != nil {
		return kubeidentity.RegistryPreparation{}, err
	}
	return kubeidentity.InspectNativeRegistry(ctx, client, namespace, service, pod, uid)
}
func inspectSystemRegistryParticipant(ctx context.Context, pod, uid, owner string, binding guard.StorageRegistration) (guard.ParticipantRegistration, error) {
	client, namespace, service, err := systemRegistryInspectionTarget()
	if err != nil {
		return guard.ParticipantRegistration{}, err
	}
	return kubeidentity.InspectRegistryParticipant(ctx, client, namespace, service, pod, uid, owner, binding)
}

var registryPodUID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]{0,63}$`)

// PrepareRegistry derives identity from the configured system registry, never
// from caller-provided volume IDs, generations, namespaces or filesystem paths.
func (h *CleanupCoordinationHandler) PrepareRegistry(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Pod    string `json:"pod"`
		PodUID string `json:"pod_uid"`
	}
	if !coordinationDecode(w, r, &body) {
		return
	}
	if len(validation.IsDNS1123Subdomain(body.Pod)) != 0 || !registryPodUID.MatchString(body.PodUID) {
		httputil.ReturnError(r, w, 400, "INVALID_REGISTRY_POD")
		return
	}
	if h.inspectRegistry == nil {
		coordinationError(w, r, guard.ErrCoordinationUnavailable)
		return
	}
	observed, err := h.inspectRegistry(r.Context(), body.Pod, body.PodUID)
	if err != nil {
		coordinationError(w, r, guard.ErrCoordinationUnavailable)
		return
	}
	binding, err := guard.ProvisionRegistryStorage(h.database(), observed.Mount.VolumeUID, observed.Root)
	if err != nil {
		coordinationError(w, r, err)
		return
	}
	status, err := guard.InspectStorage(h.database(), binding.StorageID, binding.Generation)
	if err != nil {
		coordinationError(w, r, err)
		return
	}
	httputil.ReturnSuccess(r, w, struct {
		Protocol     int                       `json:"protocol"`
		Registration guard.StorageRegistration `json:"registration"`
		Storage      guard.StorageObservation  `json:"storage"`
		Container    string                    `json:"registry_container"`
	}{1, binding, status, observed.Container})
}

// RegisterRegistryParticipant accepts only runtime facts verified by the core.
func (h *CleanupCoordinationHandler) RegisterRegistryParticipant(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Generation string `json:"generation"`
		Owner      string `json:"owner"`
		Pod        string `json:"pod"`
		PodUID     string `json:"pod_uid"`
	}
	if !coordinationDecode(w, r, &body) {
		return
	}
	if len(validation.IsDNS1123Subdomain(body.Pod)) != 0 || !registryPodUID.MatchString(body.PodUID) || body.Owner == "" || len(body.Owner) > 128 {
		httputil.ReturnError(r, w, 400, "INVALID_PARTICIPANT_REQUEST")
		return
	}
	if h.inspectParticipant == nil {
		coordinationError(w, r, guard.ErrCoordinationUnavailable)
		return
	}
	binding, err := guard.StorageBinding(h.database(), chi.URLParam(r, "storage_id"), body.Generation)
	if err != nil {
		coordinationError(w, r, err)
		return
	}
	observed, err := h.inspectParticipant(r.Context(), body.Pod, body.PodUID, body.Owner, binding)
	if err != nil {
		coordinationError(w, r, guard.ErrCoordinationUnavailable)
		return
	}
	fingerprint, _ := binding.Fingerprint()
	if observed.StorageID != binding.StorageID || observed.Generation != binding.Generation || observed.Owner != body.Owner || observed.PodUID != body.PodUID || observed.Role != "registry-ingress" || observed.BindingFingerprint != fingerprint {
		coordinationError(w, r, guard.ErrCoordinationChanged)
		return
	}
	if err := guard.RegisterParticipant(h.database(), observed); err != nil {
		coordinationError(w, r, err)
		return
	}
	httputil.ReturnSuccess(r, w, struct {
		Protocol int  `json:"protocol"`
		Recorded bool `json:"recorded"`
	}{1, true})
}

func inspectSystemManagedCache(ctx context.Context, pod, uid string) (kubeidentity.ManagedCachePreparation, error) {
	client, namespace, _, err := systemRegistryInspectionTarget()
	if err != nil {
		return kubeidentity.ManagedCachePreparation{}, err
	}
	return kubeidentity.InspectManagedBuildCacheSource(ctx, client, namespace, pod, uid)
}

// PrepareManagedCache enrolls only observed system build-cache storage. It
// never accepts filesystem paths or grants deletion readiness.
func (h *CleanupCoordinationHandler) PrepareManagedCache(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Pod    string `json:"pod"`
		PodUID string `json:"pod_uid"`
	}
	if !coordinationDecode(w, r, &body) {
		return
	}
	if len(validation.IsDNS1123Subdomain(body.Pod)) != 0 || !registryPodUID.MatchString(body.PodUID) {
		httputil.ReturnError(r, w, 400, "INVALID_MANAGED_CACHE_POD")
		return
	}
	if h.inspectManagedCache == nil {
		coordinationError(w, r, guard.ErrCoordinationUnavailable)
		return
	}
	observed, err := h.inspectManagedCache(r.Context(), body.Pod, body.PodUID)
	if err != nil || observed.NodeUID == "" {
		coordinationError(w, r, guard.ErrCoordinationUnavailable)
		return
	}
	binding, err := guard.ProvisionManagedCacheStorage(h.database(), observed.Mount.VolumeUID, observed.Root)
	if err != nil {
		coordinationError(w, r, err)
		return
	}
	status, quiescent, err := h.certifyCacheWriters(r.Context(), binding, nil)
	if err != nil {
		coordinationError(w, r, err)
		return
	}
	httputil.ReturnSuccess(r, w, struct {
		Protocol     int                       `json:"protocol"`
		Quiescent    bool                      `json:"quiescent"`
		Registration guard.StorageRegistration `json:"registration"`
		Storage      guard.StorageObservation  `json:"storage"`
		NodeUID      string                    `json:"node_uid"`
		NodeName     string                    `json:"node_name"`
	}{1, quiescent, binding, status, observed.NodeUID, observed.Mount.NodeName})
}
