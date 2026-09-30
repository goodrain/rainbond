package controller

import (
	"net/http"
	"time"

	"github.com/go-chi/chi"
	guard "github.com/goodrain/rainbond/pkg/cleanup"
	httputil "github.com/goodrain/rainbond/util/http"
)

// StorageObservationPermit issues only a short-lived read-only measurement
// permit for a server-registered storage binding.
func (h *CleanupCoordinationHandler) StorageObservationPermit(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Generation string `json:"generation"`
	}
	if !coordinationDecode(w, r, &body) {
		return
	}
	if h.database == nil || h.permitKey == nil {
		coordinationError(w, r, guard.ErrCoordinationUnavailable)
		return
	}
	binding, err := guard.StorageBinding(h.database(), chi.URLParam(r, "storage_id"), body.Generation)
	if err != nil || !guard.IsRegistryStorageBinding(binding) {
		if err == nil {
			err = guard.ErrCoordinationChanged
		}
		coordinationError(w, r, err)
		return
	}
	fingerprint, err := binding.Fingerprint()
	if err != nil {
		coordinationError(w, r, err)
		return
	}
	permit, err := guard.IssueStorageObservationPermit(h.permitKey(), binding, time.Now())
	if err != nil {
		coordinationError(w, r, err)
		return
	}
	httputil.ReturnSuccess(r, w, struct {
		Protocol           int    `json:"protocol"`
		Permit             string `json:"permit"`
		BindingFingerprint string `json:"binding_fingerprint"`
	}{1, permit, fingerprint})
}
