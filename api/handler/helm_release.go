package handler

import (
	"archive/tar"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/pkg/helm"
	"helm.sh/helm/v3/pkg/chart"
	helmrelease "helm.sh/helm/v3/pkg/release"
)

// HelmReleaseHandler handles Helm release operations for a tenant namespace.
// repoFile and repoCache are package-level variables from api/handler/helm.go:60-64,
// accessible here since this file is in the same package.
type HelmReleaseHandler struct{}

const (
	HelmReleaseSourceStore  = "store"
	HelmReleaseSourceRepo   = "repo"
	HelmReleaseSourceOCI    = "oci"
	HelmReleaseSourceUpload = "upload"
)

// HelmReleaseInstallRequest describes all supported install sources.
type HelmReleaseInstallRequest struct {
	SourceType  string `json:"source_type"`
	Namespace   string `json:"namespace"`
	RepoName    string `json:"repo_name"`
	RepoURL     string `json:"repo_url"`
	Chart       string `json:"chart"`
	ChartName   string `json:"chart_name"`
	ChartURL    string `json:"chart_url"`
	Version     string `json:"version"`
	ReleaseName string `json:"release_name"`
	Values      string `json:"values"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	EventID     string `json:"event_id"`
}

type HelmReleaseChartPreview struct {
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	Description string            `json:"description"`
	Icon        string            `json:"icon"`
	Keywords    []string          `json:"keywords"`
	AppVersion  string            `json:"app_version"`
	Values      map[string]string `json:"values"`
	Readme      string            `json:"readme"`
}

func (r *HelmReleaseInstallRequest) Normalize() {
	r.SourceType = strings.TrimSpace(r.SourceType)
	if r.SourceType == "" {
		r.SourceType = HelmReleaseSourceStore
	}
	r.Namespace = strings.TrimSpace(r.Namespace)
	r.Chart = strings.TrimSpace(firstNonEmpty(r.Chart, r.ChartName))
	r.ChartName = strings.TrimSpace(firstNonEmpty(r.ChartName, r.Chart))
	r.ChartURL = strings.TrimSpace(r.ChartURL)
	r.RepoName = strings.TrimSpace(r.RepoName)
	r.RepoURL = strings.TrimSpace(r.RepoURL)
	r.ReleaseName = strings.TrimSpace(r.ReleaseName)
	r.EventID = strings.TrimSpace(r.EventID)
}

func (r *HelmReleaseInstallRequest) Validate() error {
	if r.ReleaseName == "" {
		return fmt.Errorf("release_name is required")
	}
	switch r.SourceType {
	case HelmReleaseSourceStore:
		if r.RepoName == "" {
			return fmt.Errorf("repo_name is required for store source")
		}
		if r.Chart == "" {
			return fmt.Errorf("chart is required for store source")
		}
	case HelmReleaseSourceRepo:
		hasRepoChart := r.RepoURL != "" && r.ChartName != ""
		hasDirectChartURL := r.ChartURL != ""
		if !hasRepoChart && !hasDirectChartURL {
			return fmt.Errorf("repo source requires repo_url and chart_name, or chart_url")
		}
	case HelmReleaseSourceOCI:
		if !strings.HasPrefix(r.ChartURL, "oci://") {
			return fmt.Errorf("oci source requires chart_url with oci:// prefix")
		}
	case HelmReleaseSourceUpload:
		if r.EventID == "" {
			return fmt.Errorf("event_id is required for upload source")
		}
	default:
		return fmt.Errorf("unsupported source_type %q", r.SourceType)
	}
	return nil
}

func (r *HelmReleaseInstallRequest) ValidateForPreview() error {
	switch r.SourceType {
	case HelmReleaseSourceStore:
		if r.RepoName == "" {
			return fmt.Errorf("repo_name is required for store source")
		}
		if r.Chart == "" && r.ChartName == "" {
			return fmt.Errorf("chart is required for store source")
		}
	case HelmReleaseSourceRepo:
		if strings.TrimSpace(firstNonEmpty(r.ChartURL, r.ChartName)) == "" {
			return fmt.Errorf("chart_url is required for repo source")
		}
	case HelmReleaseSourceOCI:
		if !strings.HasPrefix(strings.TrimSpace(r.ChartURL), "oci://") {
			return fmt.Errorf("oci source requires chart_url with oci:// prefix")
		}
	case HelmReleaseSourceUpload:
		if r.EventID == "" {
			return fmt.Errorf("event_id is required for upload source")
		}
	default:
		return fmt.Errorf("unsupported source_type %q", r.SourceType)
	}
	return nil
}

func (h *HelmReleaseHandler) resolveNamespace(tenantName, namespace string) (string, error) {
	if strings.TrimSpace(namespace) != "" {
		return strings.TrimSpace(namespace), nil
	}
	tenant, err := db.GetManager().TenantDao().GetTenantIDByName(tenantName)
	if err != nil {
		return "", fmt.Errorf("tenant %s not found: %v", tenantName, err)
	}
	return helmReleaseNamespace(tenant), nil
}

func (h *HelmReleaseHandler) newHelm(tenantName, namespace string) (*helm.Helm, error) {
	resolvedNamespace, err := h.resolveNamespace(tenantName, namespace)
	if err != nil {
		return nil, err
	}
	return helm.NewHelm(resolvedNamespace, repoFile, repoCache)
}

// ListReleases returns all Helm releases in the tenant's namespace.
func (h *HelmReleaseHandler) ListReleases(tenantName, namespace string) ([]*helmrelease.Release, error) {
	hc, err := h.newHelm(tenantName, namespace)
	if err != nil {
		return nil, err
	}
	return hc.ListReleases()
}

// InstallRelease installs a Helm release from store, repo, OCI or uploaded chart sources.
func (h *HelmReleaseHandler) InstallRelease(tenantName string, req HelmReleaseInstallRequest) (*helmrelease.Release, error) {
	req.Normalize()
	if err := req.Validate(); err != nil {
		return nil, err
	}
	hc, err := h.newHelm(tenantName, req.Namespace)
	if err != nil {
		return nil, err
	}
	switch req.SourceType {
	case HelmReleaseSourceStore:
		return hc.InstallFromRepo(req.RepoName, req.Chart, req.Version, req.ReleaseName, req.Values)
	case HelmReleaseSourceRepo:
		chartRef := firstNonEmpty(req.ChartURL, req.ChartName)
		return hc.InstallFromReference(chartRef, req.RepoURL, req.Version, req.ReleaseName, req.Values, req.Username, req.Password)
	case HelmReleaseSourceOCI:
		return hc.InstallFromReference(req.ChartURL, "", req.Version, req.ReleaseName, req.Values, req.Username, req.Password)
	case HelmReleaseSourceUpload:
		chartPath, chartVersion, err := GetUploadChartPathAndVersion(req.EventID)
		if err != nil {
			return nil, err
		}
		version := req.Version
		if version == "" {
			version = chartVersion
		}
		return hc.InstallFromChartPath(chartPath, version, req.ReleaseName, req.Values)
	default:
		return nil, fmt.Errorf("unsupported source_type %q", req.SourceType)
	}
}

// PreviewChart resolves chart metadata, readme and values files before installation.
func (h *HelmReleaseHandler) PreviewChart(tenantName string, req HelmReleaseInstallRequest) (*HelmReleaseChartPreview, error) {
	req.Normalize()
	if err := req.ValidateForPreview(); err != nil {
		return nil, err
	}
	hc, err := h.newHelm(tenantName, req.Namespace)
	if err != nil {
		return nil, err
	}

	var (
		ch        *chart.Chart
		chartPath string
		version   string
	)

	switch req.SourceType {
	case HelmReleaseSourceStore:
		chartRef := fmt.Sprintf("%s/%s", req.RepoName, req.Chart)
		ch, chartPath, version, err = hc.LoadChartFromReference(chartRef, "", req.Version, "", "")
	case HelmReleaseSourceRepo:
		chartRef := firstNonEmpty(req.ChartURL, req.ChartName)
		ch, chartPath, version, err = hc.LoadChartFromReference(chartRef, req.RepoURL, req.Version, req.Username, req.Password)
	case HelmReleaseSourceOCI:
		ch, chartPath, version, err = hc.LoadChartFromReference(req.ChartURL, "", req.Version, req.Username, req.Password)
	case HelmReleaseSourceUpload:
		chartPath, version, err = GetUploadChartPathAndVersion(req.EventID)
		if err == nil {
			ch, err = hc.LoadChartFromPath(chartPath)
		}
	default:
		err = fmt.Errorf("unsupported source_type %q", req.SourceType)
	}
	if err != nil {
		return nil, err
	}

	values, readme, err := readChartPreviewFiles(chartPath)
	if err != nil {
		return nil, err
	}
	if version == "" && ch != nil && ch.Metadata != nil {
		version = ch.Metadata.Version
	}
	preview := &HelmReleaseChartPreview{
		Version: version,
		Values:  values,
		Readme:  readme,
	}
	if ch != nil && ch.Metadata != nil {
		preview.Name = ch.Metadata.Name
		preview.Description = ch.Metadata.Description
		preview.Icon = ch.Metadata.Icon
		preview.Keywords = ch.Metadata.Keywords
		preview.AppVersion = ch.Metadata.AppVersion
	}
	return preview, nil
}

// UninstallRelease uninstalls a Helm release from the tenant's namespace.
func (h *HelmReleaseHandler) UninstallRelease(tenantName, releaseName, namespace string) error {
	hc, err := h.newHelm(tenantName, namespace)
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func helmReleaseNamespace(tenant *dbmodel.Tenants) string {
	if tenant == nil {
		return ""
	}
	if strings.TrimSpace(tenant.Namespace) != "" {
		return tenant.Namespace
	}
	return tenant.UUID
}

func readChartPreviewFiles(chartPath string) (map[string]string, string, error) {
	stat, err := os.Stat(chartPath)
	if err != nil {
		return nil, "", err
	}
	if stat.IsDir() {
		return readChartPreviewFilesFromDir(chartPath)
	}
	return readChartPreviewFilesFromArchive(chartPath)
}

func readChartPreviewFilesFromDir(chartPath string) (map[string]string, string, error) {
	values := make(map[string]string)
	readme := ""
	err := filepath.Walk(chartPath, func(currentPath string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info == nil || info.IsDir() {
			return walkErr
		}
		relPath, err := filepath.Rel(chartPath, currentPath)
		if err != nil {
			return err
		}
		content, err := os.ReadFile(currentPath)
		if err != nil {
			return err
		}
		if strings.HasSuffix(relPath, "README.md") && readme == "" {
			readme = base64.StdEncoding.EncodeToString(content)
		}
		if strings.HasSuffix(relPath, "values.yaml") {
			values[filepath.ToSlash(relPath)] = base64.StdEncoding.EncodeToString(content)
		}
		return nil
	})
	return values, readme, err
}

func readChartPreviewFilesFromArchive(chartPath string) (map[string]string, string, error) {
	file, err := os.Open(chartPath)
	if err != nil {
		return nil, "", err
	}
	defer file.Close()
	gzr, err := gzip.NewReader(file)
	if err != nil {
		return nil, "", err
	}
	defer gzr.Close()

	values := make(map[string]string)
	readme := ""
	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, "", err
		}
		if header.FileInfo().IsDir() {
			continue
		}
		content, err := io.ReadAll(tr)
		if err != nil {
			return nil, "", err
		}
		if strings.HasSuffix(header.Name, "README.md") && readme == "" {
			readme = base64.StdEncoding.EncodeToString(content)
		}
		if strings.HasSuffix(header.Name, "values.yaml") {
			values[header.Name] = base64.StdEncoding.EncodeToString(content)
		}
	}
	return values, readme, nil
}
