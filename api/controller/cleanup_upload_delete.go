package controller

import (
	"context"
	"net/http"
	"regexp"
	"time"

	"github.com/goodrain/rainbond/db/model"
	guard "github.com/goodrain/rainbond/pkg/cleanup"
	"github.com/goodrain/rainbond/pkg/component/storage"
	httputil "github.com/goodrain/rainbond/util/http"
	"github.com/jinzhu/gorm"
)

var uploadInspectionOperation = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)
var uploadInspectionFingerprint = regexp.MustCompile(`^[a-f0-9]{64}$`)

func systemUploadStorageBinding() (string, error) { return storage.Default().UploadChunkBinding() }
func systemDeleteUploadChunks(id string) error {
	configured := storage.Default()
	if configured == nil || configured.StorageCli == nil {
		return guard.ErrCoordinationUnavailable
	}
	return configured.StorageCli.CleanupChunks(id)
}

func (h *CleanupCoordinationHandler) verifyUploadWriters(ctx context.Context, database *gorm.DB) error {
	if h.inspectReferenceWriters == nil {
		return guard.ErrCoordinationUnavailable
	}
	writers, err := h.inspectReferenceWriters(ctx)
	if err != nil {
		return guard.ErrCoordinationUnavailable
	}
	ready, err := guard.UploadWriterCoverageRegistered(database, writers)
	if err != nil || !ready {
		return guard.ErrCoordinationUnavailable
	}
	return nil
}

// DeleteUploadChunks is internal to the trusted Console. Its route requires
// CleanupIdentity; Console must separately validate enterprise ownership and
// the user's confirmed selection. No caller-controlled filesystem path is used.
func (h *CleanupCoordinationHandler) DeleteUploadChunks(w http.ResponseWriter, r *http.Request) {
	var body struct {
		OperationID        string `json:"operation_id"`
		EventID            string `json:"event_id"`
		SessionID          string `json:"session_id"`
		StateFingerprint   string `json:"state_fingerprint"`
		StorageFingerprint string `json:"storage_fingerprint"`
		IdleDays           int    `json:"idle_days"`
	}
	if !coordinationDecode(w, r, &body) {
		return
	}
	if body.IdleDays < 1 || body.IdleDays > 365 || !uploadInventoryEventID.MatchString(body.EventID) || !uploadInventoryEventID.MatchString(body.SessionID) {
		httputil.ReturnError(r, w, 400, "INVALID_UPLOAD_SELECTION")
		return
	}
	if h.database == nil || h.uploadStorageBinding == nil || h.deleteUploadChunks == nil {
		coordinationError(w, r, guard.ErrCoordinationUnavailable)
		return
	}
	database := h.database()
	if database == nil {
		coordinationError(w, r, guard.ErrCoordinationUnavailable)
		return
	}
	verify := func() error {
		if err := h.verifyUploadWriters(r.Context(), database); err != nil {
			return err
		}
		binding, err := h.uploadStorageBinding()
		if err != nil || binding == "" || binding != body.StorageFingerprint {
			return guard.ErrCoordinationChanged
		}
		return nil
	}
	if err := verify(); err != nil {
		coordinationError(w, r, err)
		return
	}
	var session model.UploadSession
	if err := database.Where("id = ? AND event_id = ?", body.SessionID, body.EventID).First(&session).Error; err != nil {
		coordinationError(w, r, guard.ErrCoordinationChanged)
		return
	}
	if session.ExpiresAt.IsZero() || session.ExpiresAt.After(time.Now().Add(-time.Duration(body.IdleDays)*24*time.Hour)) {
		coordinationError(w, r, guard.ErrCoordinationChanged)
		return
	}
	err := guard.DeleteExpiredUploadChunks(database, body.OperationID, body.EventID, body.SessionID, body.StateFingerprint, func(id string) error {
		// Recheck current deployment and configured storage after the durable tombstone.
		if err := verify(); err != nil {
			return err
		}
		return h.deleteUploadChunks(id)
	})
	if err != nil {
		coordinationError(w, r, err)
		return
	}
	httputil.ReturnSuccess(r, w, map[string]interface{}{"protocol": 1, "operation_id": body.OperationID, "event_id": body.EventID, "session_id": body.SessionID, "state": "deleted"})
}

// InspectUploadChunks reads the authoritative receipt for one original
// operation. It never invokes the retirement or deletion paths.
func (h *CleanupCoordinationHandler) InspectUploadChunks(w http.ResponseWriter, r *http.Request) {
	var body struct {
		OperationID        string `json:"operation_id"`
		EventID            string `json:"event_id"`
		SessionID          string `json:"session_id"`
		StateFingerprint   string `json:"state_fingerprint"`
		StorageFingerprint string `json:"storage_fingerprint"`
	}
	if !coordinationDecode(w, r, &body) {
		return
	}
	if !uploadInspectionOperation.MatchString(body.OperationID) || !uploadInventoryEventID.MatchString(body.EventID) ||
		!uploadInventoryEventID.MatchString(body.SessionID) || !uploadInspectionFingerprint.MatchString(body.StateFingerprint) ||
		!uploadInspectionFingerprint.MatchString(body.StorageFingerprint) {
		httputil.ReturnError(r, w, 400, "INVALID_UPLOAD_INSPECTION")
		return
	}
	if h.database == nil || h.uploadStorageBinding == nil || h.measureUploadChunks == nil {
		coordinationError(w, r, guard.ErrCoordinationUnavailable)
		return
	}
	database := h.database()
	if database == nil {
		coordinationError(w, r, guard.ErrCoordinationUnavailable)
		return
	}
	state := "unknown"
	binding, err := h.uploadStorageBinding()
	if err == nil && binding != "" && binding == body.StorageFingerprint {
		state, err = guard.InspectExpiredUploadDeletion(database, body.OperationID, body.EventID, body.SessionID, body.StateFingerprint,
			func(id string) (int64, int, error) {
				usage, measureErr := h.measureUploadChunks(r.Context(), id)
				return usage.Bytes, usage.Objects, measureErr
			})
		if err != nil {
			coordinationError(w, r, err)
			return
		}
	}
	httputil.ReturnSuccess(r, w, map[string]interface{}{"protocol": 1, "operation_id": body.OperationID, "event_id": body.EventID, "session_id": body.SessionID, "state": state})
}
