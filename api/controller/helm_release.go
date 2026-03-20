package controller

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/api/handler"
	httputil "github.com/goodrain/rainbond/util/http"
)

// HelmReleaseController handles HTTP requests for Helm releases.
type HelmReleaseController struct{}

// ListReleases lists all Helm releases in the tenant's namespace.
func (c *HelmReleaseController) ListReleases(w http.ResponseWriter, r *http.Request) {
	tenantName := chi.URLParam(r, "tenant_name")
	namespace := strings.TrimSpace(r.URL.Query().Get("namespace"))
	list, err := handler.GetHelmReleaseHandler().ListReleases(tenantName, namespace)
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	httputil.ReturnSuccess(r, w, map[string]interface{}{"list": list, "total": len(list)})
}

type installReleaseReq struct {
	handler.HelmReleaseInstallRequest
}

type rollbackReleaseReq struct {
	handler.HelmReleaseRollbackRequest
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

// PreviewChart resolves chart metadata and values before installation.
func (c *HelmReleaseController) PreviewChart(w http.ResponseWriter, r *http.Request) {
	tenantName := chi.URLParam(r, "tenant_name")
	var req installReleaseReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	req.Normalize()
	if err := req.ValidateForPreview(); err != nil {
		httputil.ReturnBcodeError(r, w, httputil.NewErrBadRequest(err))
		return
	}
	preview, err := handler.GetHelmReleaseHandler().PreviewChart(tenantName, req.HelmReleaseInstallRequest)
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	httputil.ReturnSuccess(r, w, preview)
}

// GetReleaseHistory lists all revisions for the target release.
func (c *HelmReleaseController) GetReleaseHistory(w http.ResponseWriter, r *http.Request) {
	tenantName := chi.URLParam(r, "tenant_name")
	releaseName := chi.URLParam(r, "release_name")
	namespace := strings.TrimSpace(r.URL.Query().Get("namespace"))
	list, err := handler.GetHelmReleaseHandler().GetReleaseHistory(tenantName, releaseName, namespace)
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	httputil.ReturnSuccess(r, w, map[string]interface{}{"list": list, "total": len(list)})
}

// UpgradeRelease upgrades an existing Helm release using the target chart source in request body.
func (c *HelmReleaseController) UpgradeRelease(w http.ResponseWriter, r *http.Request) {
	tenantName := chi.URLParam(r, "tenant_name")
	releaseName := chi.URLParam(r, "release_name")
	var req installReleaseReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	req.ReleaseName = releaseName
	req.Normalize()
	if err := req.Validate(); err != nil {
		httputil.ReturnBcodeError(r, w, httputil.NewErrBadRequest(err))
		return
	}
	rel, err := handler.GetHelmReleaseHandler().UpgradeRelease(tenantName, releaseName, req.HelmReleaseInstallRequest)
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	httputil.ReturnSuccess(r, w, rel)
}

// RollbackRelease rolls an existing Helm release back to a previous revision.
func (c *HelmReleaseController) RollbackRelease(w http.ResponseWriter, r *http.Request) {
	tenantName := chi.URLParam(r, "tenant_name")
	releaseName := chi.URLParam(r, "release_name")
	namespace := strings.TrimSpace(r.URL.Query().Get("namespace"))
	var req rollbackReleaseReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	if err := req.Validate(); err != nil {
		httputil.ReturnBcodeError(r, w, httputil.NewErrBadRequest(err))
		return
	}
	if err := handler.GetHelmReleaseHandler().RollbackRelease(tenantName, releaseName, namespace, req.Revision); err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}

// UninstallRelease removes a Helm release from the tenant's namespace.
func (c *HelmReleaseController) UninstallRelease(w http.ResponseWriter, r *http.Request) {
	tenantName := chi.URLParam(r, "tenant_name")
	releaseName := chi.URLParam(r, "release_name")
	namespace := strings.TrimSpace(r.URL.Query().Get("namespace"))
	if err := handler.GetHelmReleaseHandler().UninstallRelease(tenantName, releaseName, namespace); err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}

// GetHelmReleaseController returns a new HelmReleaseController.
func GetHelmReleaseController() *HelmReleaseController {
	return &HelmReleaseController{}
}
