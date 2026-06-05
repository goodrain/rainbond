package controller

import (
	"encoding/json"
	"net/http"

	"github.com/goodrain/rainbond/api/handler"
	ctxutil "github.com/goodrain/rainbond/api/util/ctx"
	httputil "github.com/goodrain/rainbond/util/http"
)

type VMSnapshotController struct {
	createSnapshot func(serviceID string, req *handler.VMSnapshotRequest) (*handler.VMSnapshotStatus, error)
}

var defaultVMSnapshotController = &VMSnapshotController{}

func GetVMSnapshotController() *VMSnapshotController {
	return defaultVMSnapshotController
}

func (c *VMSnapshotController) CreateVMSnapshot(w http.ResponseWriter, r *http.Request) {
	serviceID := r.Context().Value(ctxutil.ContextKey("service_id")).(string)
	var reqBody handler.VMSnapshotRequest
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		httputil.ReturnError(r, w, http.StatusBadRequest, "invalid request body")
		return
	}
	if reqBody.Name == "" {
		httputil.ReturnError(r, w, http.StatusBadRequest, "snapshot name is required")
		return
	}
	createSnapshot := c.createSnapshot
	if createSnapshot == nil {
		createSnapshot = handler.GetServiceManager().CreateVMSnapshot
	}
	status, err := createSnapshot(serviceID, &reqBody)
	if err != nil {
		httputil.ReturnError(r, w, http.StatusInternalServerError, err.Error())
		return
	}
	httputil.ReturnSuccess(r, w, status)
}

func (t *TenantStruct) CreateVMSnapshot(w http.ResponseWriter, r *http.Request) {
	GetVMSnapshotController().CreateVMSnapshot(w, r)
}
