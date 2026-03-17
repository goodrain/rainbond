package handler

import (
	"fmt"

	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/pkg/helm"
	helmrelease "helm.sh/helm/v3/pkg/release"
)

// HelmReleaseHandler handles Helm release operations for a tenant namespace.
// repoFile and repoCache are package-level variables from api/handler/helm.go:60-64,
// accessible here since this file is in the same package.
type HelmReleaseHandler struct{}

func (h *HelmReleaseHandler) newHelm(tenantName string) (*helm.Helm, error) {
	tenant, err := db.GetManager().TenantDao().GetTenantIDByName(tenantName)
	if err != nil {
		return nil, fmt.Errorf("tenant %s not found: %v", tenantName, err)
	}
	return helm.NewHelm(tenant.UUID, repoFile, repoCache)
}

// ListReleases returns all Helm releases in the tenant's namespace.
func (h *HelmReleaseHandler) ListReleases(tenantName string) ([]*helmrelease.Release, error) {
	hc, err := h.newHelm(tenantName)
	if err != nil {
		return nil, err
	}
	return hc.ListReleases()
}

// InstallRelease installs a chart from a configured Helm repo into the tenant's namespace.
func (h *HelmReleaseHandler) InstallRelease(tenantName, repoName, chart, version, releaseName, valuesYAML string) (*helmrelease.Release, error) {
	hc, err := h.newHelm(tenantName)
	if err != nil {
		return nil, err
	}
	return hc.InstallFromRepo(repoName, chart, version, releaseName, valuesYAML)
}

// UninstallRelease uninstalls a Helm release from the tenant's namespace.
func (h *HelmReleaseHandler) UninstallRelease(tenantName, releaseName string) error {
	hc, err := h.newHelm(tenantName)
	if err != nil {
		return err
	}
	return hc.Uninstall(releaseName)
}

var helmReleaseHandler *HelmReleaseHandler

// GetHelmReleaseHandler returns the singleton HelmReleaseHandler.
func GetHelmReleaseHandler() *HelmReleaseHandler {
	if helmReleaseHandler == nil {
		helmReleaseHandler = &HelmReleaseHandler{}
	}
	return helmReleaseHandler
}
