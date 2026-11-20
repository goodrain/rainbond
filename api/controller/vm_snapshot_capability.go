package controller

import (
	"context"
	"net/http"

	"github.com/goodrain/rainbond/api/handler"
	ctxutil "github.com/goodrain/rainbond/api/util/ctx"
	httputil "github.com/goodrain/rainbond/util/http"
)

// ExistingVMSnapshotCapabilityController exposes a component-scoped read-only capability view.
type ExistingVMSnapshotCapabilityController struct {
	inspect func(context.Context, string, string) (*handler.ExistingVMSnapshotCapability, error)
}

// Get uses only middleware-authenticated tenant/service identity, not claim or namespace input.
func (c *ExistingVMSnapshotCapabilityController) Get(w http.ResponseWriter, r *http.Request) {
	tenant, okTenant := r.Context().Value(ctxutil.ContextKey("tenant_id")).(string)
	service, okService := r.Context().Value(ctxutil.ContextKey("service_id")).(string)
	if !okTenant || !okService || tenant == "" || service == "" {
		httputil.ReturnError(r, w, http.StatusUnauthorized, "missing authenticated component scope")
		return
	}
	inspect := c.inspect
	if inspect == nil {
		inspect = handler.GetServiceManager().GetExistingVMSnapshotCapability
	}
	result, err := inspect(r.Context(), tenant, service)
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	httputil.ReturnSuccess(r, w, result)
}

// ExistingVMSnapshotCapability is a GET without event or mutation wrapping.
func (t *TenantStruct) ExistingVMSnapshotCapability(w http.ResponseWriter, r *http.Request) {
	(&ExistingVMSnapshotCapabilityController{}).Get(w, r)
}
