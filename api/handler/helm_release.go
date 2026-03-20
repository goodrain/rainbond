package handler

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	rbdmodel "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/pkg/component/k8s"
	"github.com/goodrain/rainbond/pkg/helm"
	"github.com/goodrain/rainbond/util/constants"
	httputil "github.com/goodrain/rainbond/util/http"
	"helm.sh/helm/v3/pkg/chart"
	helmrelease "helm.sh/helm/v3/pkg/release"
	k8sapierrors "k8s.io/apimachinery/pkg/api/errors"
	k8sapimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"
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
	SourceType        string `json:"source_type"`
	Namespace         string `json:"namespace"`
	RepoName          string `json:"repo_name"`
	RepoURL           string `json:"repo_url"`
	Chart             string `json:"chart"`
	ChartName         string `json:"chart_name"`
	ChartURL          string `json:"chart_url"`
	Version           string `json:"version"`
	ReleaseName       string `json:"release_name"`
	Values            string `json:"values"`
	Username          string `json:"username"`
	Password          string `json:"password"`
	EventID           string `json:"event_id"`
	AllowChartReplace bool   `json:"allow_chart_replace"`
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

type HelmReleaseSummary struct {
	Name         string `json:"name"`
	Chart        string `json:"chart"`
	ChartVersion string `json:"chart_version"`
	AppVersion   string `json:"app_version"`
	Status       string `json:"status"`
	Version      int    `json:"version"`
	Namespace    string `json:"namespace"`
	Updated      string `json:"updated"`
}

type HelmReleaseHistoryItem struct {
	Revision     int    `json:"revision"`
	Chart        string `json:"chart"`
	ChartVersion string `json:"chart_version"`
	AppVersion   string `json:"app_version"`
	Status       string `json:"status"`
	Description  string `json:"description"`
	Updated      string `json:"updated"`
}

type HelmReleaseDetailSummary struct {
	Name         string `json:"name"`
	Namespace    string `json:"namespace"`
	Status       string `json:"status"`
	Chart        string `json:"chart"`
	ChartVersion string `json:"chart_version"`
	AppVersion   string `json:"app_version"`
	Revision     int    `json:"revision"`
	Description  string `json:"description"`
	Updated      string `json:"updated"`
	Values       string `json:"values"`
}

type HelmReleaseDetail struct {
	Summary   *HelmReleaseDetailSummary `json:"summary"`
	Workloads []NsResourceInfo          `json:"workloads"`
	Services  []NsResourceInfo          `json:"services"`
	Others    []NsResourceInfo          `json:"others"`
	History   []*HelmReleaseHistoryItem `json:"history"`
}

type HelmReleaseRollbackRequest struct {
	Revision int `json:"revision"`
}

var helmReleaseResourceTargets = []schema.GroupVersionResource{
	{Group: "apps", Version: "v1", Resource: "deployments"},
	{Group: "apps", Version: "v1", Resource: "statefulsets"},
	{Group: "apps", Version: "v1", Resource: "daemonsets"},
	{Group: "batch", Version: "v1", Resource: "jobs"},
	{Group: "batch", Version: "v1", Resource: "cronjobs"},
	{Group: "", Version: "v1", Resource: "services"},
	{Group: "", Version: "v1", Resource: "configmaps"},
	{Group: "", Version: "v1", Resource: "secrets"},
	{Group: "", Version: "v1", Resource: "serviceaccounts"},
	{Group: "", Version: "v1", Resource: "persistentvolumeclaims"},
	{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "roles"},
	{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "rolebindings"},
	{Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"},
	{Group: "autoscaling", Version: "v2", Resource: "horizontalpodautoscalers"},
	{Group: "gateway.networking.k8s.io", Version: "v1beta1", Resource: "gateways"},
	{Group: "gateway.networking.k8s.io", Version: "v1beta1", Resource: "httproutes"},
	{Group: "rollouts.kruise.io", Version: "v1alpha1", Resource: "rollouts"},
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

func (r *HelmReleaseRollbackRequest) Validate() error {
	if r.Revision <= 0 {
		return fmt.Errorf("revision must be greater than 0")
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
func (h *HelmReleaseHandler) ListReleases(tenantName, namespace string) ([]*HelmReleaseSummary, error) {
	hc, err := h.newHelm(tenantName, namespace)
	if err != nil {
		return nil, err
	}
	releases, err := hc.ListReleases()
	if err != nil {
		return nil, err
	}
	summaries := make([]*HelmReleaseSummary, 0, len(releases))
	for _, release := range releases {
		summaries = append(summaries, summarizeHelmRelease(release))
	}
	return summaries, nil
}

// GetReleaseHistory returns the Helm revision history for the given release.
func (h *HelmReleaseHandler) GetReleaseHistory(tenantName, releaseName, namespace string) ([]*HelmReleaseHistoryItem, error) {
	hc, err := h.newHelm(tenantName, namespace)
	if err != nil {
		return nil, err
	}
	history, err := hc.History(releaseName)
	if err != nil {
		return nil, err
	}
	return summarizeHelmReleaseHistory(history), nil
}

// GetReleaseDetail returns the Helm release status, values, history and related namespace resources.
func (h *HelmReleaseHandler) GetReleaseDetail(tenantName, releaseName, namespace string) (*HelmReleaseDetail, error) {
	hc, err := h.newHelm(tenantName, namespace)
	if err != nil {
		return nil, err
	}
	release, err := hc.Status(releaseName)
	if err != nil {
		return nil, err
	}
	history, err := hc.History(releaseName)
	if err != nil {
		return nil, err
	}
	resources, err := h.listReleaseResources(tenantName, releaseName, release.Namespace)
	if err != nil {
		return nil, err
	}
	workloads, services, others := splitHelmReleaseResources(resources)
	return &HelmReleaseDetail{
		Summary:   summarizeHelmReleaseDetail(release),
		Workloads: workloads,
		Services:  services,
		Others:    others,
		History:   summarizeHelmReleaseHistory(history),
	}, nil
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

	ch, chartPath, version, err := h.loadTargetChart(hc, req)
	if err != nil {
		return nil, wrapHelmChartPreviewSourceError(err)
	}

	values, readme, err := readChartPreviewFiles(chartPath)
	if err != nil {
		return nil, wrapHelmChartPreviewSourceError(err)
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

// UpgradeRelease upgrades an existing Helm release using a newly specified chart source.
func (h *HelmReleaseHandler) UpgradeRelease(tenantName, releaseName string, req HelmReleaseInstallRequest) (*helmrelease.Release, error) {
	req.Normalize()
	req.ReleaseName = releaseName
	if err := req.Validate(); err != nil {
		return nil, err
	}
	hc, err := h.newHelm(tenantName, req.Namespace)
	if err != nil {
		return nil, err
	}
	currentRelease, err := hc.Status(releaseName)
	if err != nil {
		return nil, err
	}
	targetChart, chartPath, version, err := h.loadTargetChart(hc, req)
	if err != nil {
		return nil, err
	}
	if err := validateUpgradeChartName(currentRelease, targetChart, req.AllowChartReplace); err != nil {
		return nil, err
	}
	return hc.UpgradeFromChartPath(chartPath, version, releaseName, req.Values)
}

// RollbackRelease rolls the given release back to a previous revision.
func (h *HelmReleaseHandler) RollbackRelease(tenantName, releaseName, namespace string, revision int) error {
	hc, err := h.newHelm(tenantName, namespace)
	if err != nil {
		return err
	}
	return hc.Rollback(releaseName, revision)
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

func summarizeHelmRelease(release *helmrelease.Release) *HelmReleaseSummary {
	summary := &HelmReleaseSummary{}
	if release == nil {
		return summary
	}

	summary.Name = release.Name
	summary.Version = release.Version
	summary.Namespace = release.Namespace

	if release.Chart != nil && release.Chart.Metadata != nil {
		summary.Chart = release.Chart.Metadata.Name
		summary.ChartVersion = release.Chart.Metadata.Version
		summary.AppVersion = release.Chart.Metadata.AppVersion
	}

	if release.Info != nil {
		summary.Status = release.Info.Status.String()
		if !release.Info.LastDeployed.Time.IsZero() {
			summary.Updated = release.Info.LastDeployed.Time.UTC().Format(time.RFC3339)
		}
	}

	return summary
}

func summarizeHelmReleaseDetail(release *helmrelease.Release) *HelmReleaseDetailSummary {
	summary := &HelmReleaseDetailSummary{}
	if release == nil {
		return summary
	}
	summary.Name = release.Name
	summary.Namespace = release.Namespace
	summary.Revision = release.Version
	summary.Values = marshalHelmReleaseValues(release.Config)
	if release.Chart != nil && release.Chart.Metadata != nil {
		summary.Chart = release.Chart.Metadata.Name
		summary.ChartVersion = release.Chart.Metadata.Version
		summary.AppVersion = release.Chart.Metadata.AppVersion
	}
	if release.Info != nil {
		summary.Status = release.Info.Status.String()
		summary.Description = release.Info.Description
		if !release.Info.LastDeployed.Time.IsZero() {
			summary.Updated = release.Info.LastDeployed.Time.UTC().Format(time.RFC3339)
		}
	}
	return summary
}

func summarizeHelmReleaseHistory(history helm.ReleaseHistory) []*HelmReleaseHistoryItem {
	items := make([]*HelmReleaseHistoryItem, 0, len(history))
	for _, item := range history {
		chartName := item.ChartName
		chartVersion := item.ChartVersion
		if chartName == "" || chartVersion == "" {
			fallbackChartName, fallbackChartVersion := splitHelmChartLabel(item.Chart)
			if chartName == "" {
				chartName = fallbackChartName
			}
			if chartVersion == "" {
				chartVersion = fallbackChartVersion
			}
		}
		items = append(items, &HelmReleaseHistoryItem{
			Revision:     item.Revision,
			Chart:        chartName,
			ChartVersion: chartVersion,
			AppVersion:   item.AppVersion,
			Status:       item.Status,
			Description:  item.Description,
			Updated:      formatHelmReleaseTime(item.Updated.Time),
		})
	}
	return items
}

func marshalHelmReleaseValues(values map[string]interface{}) string {
	if len(values) == 0 {
		return ""
	}
	data, err := yaml.Marshal(values)
	if err != nil {
		return ""
	}
	return string(data)
}

func isHelmReleaseResource(labels map[string]string, releaseName string) bool {
	if len(labels) == 0 || strings.TrimSpace(releaseName) == "" {
		return false
	}
	return labels[constants.ResourceManagedByLabel] == "Helm" &&
		labels[constants.ResourceInstanceLabel] == releaseName
}

func splitHelmReleaseResources(resources []NsResourceInfo) ([]NsResourceInfo, []NsResourceInfo, []NsResourceInfo) {
	workloads := make([]NsResourceInfo, 0)
	services := make([]NsResourceInfo, 0)
	others := make([]NsResourceInfo, 0)
	for _, resource := range resources {
		switch resource.Kind {
		case rbdmodel.Deployment, rbdmodel.StateFulSet, "DaemonSet", rbdmodel.Job, rbdmodel.CronJob, rbdmodel.Rollout:
			workloads = append(workloads, resource)
		case rbdmodel.Service:
			services = append(services, resource)
		default:
			others = append(others, resource)
		}
	}
	return workloads, services, others
}

func (h *HelmReleaseHandler) listReleaseResources(tenantName, releaseName, namespace string) ([]NsResourceInfo, error) {
	ns := strings.TrimSpace(namespace)
	if ns == "" {
		resolvedNamespace, err := h.resolveNamespace(tenantName, namespace)
		if err != nil {
			return nil, err
		}
		ns = resolvedNamespace
	}
	resources := make([]NsResourceInfo, 0, 16)
	for _, gvr := range helmReleaseResourceTargets {
		list, err := k8s.Default().DynamicClient.Resource(gvr).Namespace(ns).List(context.Background(), metav1.ListOptions{})
		if err != nil {
			if k8sapimeta.IsNoMatchError(err) || k8sapierrors.IsNotFound(err) {
				continue
			}
			return nil, err
		}
		for _, item := range list.Items {
			if !isHelmReleaseResource(item.GetLabels(), releaseName) {
				continue
			}
			resources = append(resources, toNsResourceInfo(item))
		}
	}
	sort.Slice(resources, func(i, j int) bool {
		if resources[i].Kind == resources[j].Kind {
			return resources[i].Name < resources[j].Name
		}
		return resources[i].Kind < resources[j].Kind
	})
	return resources, nil
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

func formatHelmReleaseTime(ts time.Time) string {
	if ts.IsZero() {
		return ""
	}
	return ts.UTC().Format(time.RFC3339)
}

func splitHelmChartLabel(chart string) (string, string) {
	value := strings.TrimSpace(chart)
	if value == "" {
		return "", ""
	}
	for index := len(value) - 1; index >= 0; index-- {
		if value[index] != '-' {
			continue
		}
		if index+1 >= len(value) || (value[index+1] < '0' || value[index+1] > '9') {
			continue
		}
		return value[:index], value[index+1:]
	}
	return value, ""
}

func validateUpgradeChartName(currentRelease *helmrelease.Release, targetChart *chart.Chart, allowChartReplace bool) error {
	if currentRelease == nil || currentRelease.Chart == nil || currentRelease.Chart.Metadata == nil {
		return nil
	}
	if targetChart == nil || targetChart.Metadata == nil {
		return nil
	}
	currentName := strings.TrimSpace(currentRelease.Chart.Metadata.Name)
	targetName := strings.TrimSpace(targetChart.Metadata.Name)
	if currentName == "" || targetName == "" {
		return nil
	}
	if currentName != targetName {
		if allowChartReplace {
			return nil
		}
		return fmt.Errorf("upgrade chart name %q does not match current release chart %q", targetName, currentName)
	}
	return nil
}

func (h *HelmReleaseHandler) loadTargetChart(hc *helm.Helm, req HelmReleaseInstallRequest) (*chart.Chart, string, string, error) {
	switch req.SourceType {
	case HelmReleaseSourceStore:
		chartRef := fmt.Sprintf("%s/%s", req.RepoName, req.Chart)
		return hc.LoadChartFromReference(chartRef, "", req.Version, "", "")
	case HelmReleaseSourceRepo:
		chartRef := firstNonEmpty(req.ChartURL, req.ChartName)
		return hc.LoadChartFromReference(chartRef, req.RepoURL, req.Version, req.Username, req.Password)
	case HelmReleaseSourceOCI:
		return hc.LoadChartFromReference(req.ChartURL, "", req.Version, req.Username, req.Password)
	case HelmReleaseSourceUpload:
		chartPath, version, err := GetUploadChartPathAndVersion(req.EventID)
		if err != nil {
			return nil, "", "", err
		}
		ch, err := hc.LoadChartFromPath(chartPath)
		return ch, chartPath, version, err
	default:
		return nil, "", "", fmt.Errorf("unsupported source_type %q", req.SourceType)
	}
}

func wrapHelmChartPreviewSourceError(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := err.(httputil.ErrBadRequest); ok {
		return err
	}
	return httputil.NewErrBadRequest(err)
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
