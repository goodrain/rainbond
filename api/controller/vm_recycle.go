package controller

import (
	"net/http"

	"github.com/goodrain/rainbond/api/handler"
	httputil "github.com/goodrain/rainbond/util/http"
)

func (t *TenantStruct) VMRecycleRecovery(w http.ResponseWriter, r *http.Request) {
	id, ok := snapshotServiceID(w, r)
	if !ok {
		return
	}
	result, err := handler.GetServiceManager().GetVMRecycleRecovery(r.Context(), id)
	if err != nil {
		restoreHTTPError(w, r, err)
		return
	}
	httputil.ReturnSuccess(r, w, result)
}
