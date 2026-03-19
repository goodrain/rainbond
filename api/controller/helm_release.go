package controller

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/api/handler"
	httputil "github.com/goodrain/rainbond/util/http"
)

// HelmReleaseController handles HTTP requests for Helm releases.
type HelmReleaseController struct{}

// ListReleases lists all Helm releases in the tenant's namespace.
func (c *HelmReleaseController) ListReleases(w http.ResponseWriter, r *http.Request) {
	tenantName := chi.URLParam(r, "tenant_name")
	list, err := handler.GetHelmReleaseHandler().ListReleases(tenantName)
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	httputil.ReturnSuccess(r, w, map[string]interface{}{"list": list, "total": len(list)})
}

type installReleaseReq struct {
	handler.HelmReleaseInstallRequest
}

// InstallRelease installs a Helm chart into the tenant's namespace.
func (c *HelmReleaseController) InstallRelease(w http.ResponseWriter, r *http.Request) {
	tenantName := chi.URLParam(r, "tenant_name")
	var req installReleaseReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	req.Normalize()
	if err := req.Validate(); err != nil {
		httputil.ReturnBcodeError(r, w, httputil.NewErrBadRequest(err))
		return
	}
	rel, err := handler.GetHelmReleaseHandler().InstallRelease(tenantName, req.HelmReleaseInstallRequest)
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	httputil.ReturnSuccess(r, w, rel)
}

// UninstallRelease removes a Helm release from the tenant's namespace.
func (c *HelmReleaseController) UninstallRelease(w http.ResponseWriter, r *http.Request) {
	tenantName := chi.URLParam(r, "tenant_name")
	releaseName := chi.URLParam(r, "release_name")
	if err := handler.GetHelmReleaseHandler().UninstallRelease(tenantName, releaseName); err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}

// GetHelmReleaseController returns a new HelmReleaseController.
func GetHelmReleaseController() *HelmReleaseController {
	return &HelmReleaseController{}
}
