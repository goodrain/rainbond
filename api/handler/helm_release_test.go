package handler

import (
	"testing"
	"time"

	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/stretchr/testify/assert"
	helmchart "helm.sh/helm/v3/pkg/chart"
	helmrelease "helm.sh/helm/v3/pkg/release"
	helmtime "helm.sh/helm/v3/pkg/time"
)

func TestGetHelmReleaseHandlerSingleton(t *testing.T) {
	h1 := GetHelmReleaseHandler()
	h2 := GetHelmReleaseHandler()
	assert.NotNil(t, h1)
	assert.Equal(t, h1, h2)
}

func TestHelmReleaseInstallRequestNormalizeDefaults(t *testing.T) {
	req := HelmReleaseInstallRequest{
		RepoName:    "bitnami",
		ChartName:   "nginx",
		ReleaseName: "demo",
	}

	req.Normalize()

	assert.Equal(t, HelmReleaseSourceStore, req.SourceType)
	assert.Equal(t, "nginx", req.Chart)
	assert.Equal(t, "nginx", req.ChartName)
}

func TestHelmReleaseInstallRequestValidate(t *testing.T) {
	cases := []struct {
		name    string
		req     HelmReleaseInstallRequest
		wantErr string
	}{
		{
			name: "store ok",
			req: HelmReleaseInstallRequest{
				SourceType:  HelmReleaseSourceStore,
				RepoName:    "bitnami",
				Chart:       "nginx",
				ReleaseName: "demo",
			},
		},
		{
			name: "repo needs repo or direct chart url",
			req: HelmReleaseInstallRequest{
				SourceType:  HelmReleaseSourceRepo,
				ReleaseName: "demo",
			},
			wantErr: "repo source requires repo_url and chart_name, or chart_url",
		},
		{
			name: "oci prefix required",
			req: HelmReleaseInstallRequest{
				SourceType:  HelmReleaseSourceOCI,
				ChartURL:    "https://charts.example.com/nginx.tgz",
				ReleaseName: "demo",
			},
			wantErr: "oci source requires chart_url with oci:// prefix",
		},
		{
			name: "upload requires event id",
			req: HelmReleaseInstallRequest{
				SourceType:  HelmReleaseSourceUpload,
				ReleaseName: "demo",
			},
			wantErr: "event_id is required for upload source",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			tt.req.Normalize()
			err := tt.req.Validate()
			if tt.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			if assert.Error(t, err) {
				assert.Equal(t, tt.wantErr, err.Error())
			}
		})
	}
}

func TestHelmReleaseNamespaceUsesTenantNamespaceWhenPresent(t *testing.T) {
	tenant := &dbmodel.Tenants{
		Namespace: "team-namespace",
		UUID:      "tenant-uuid",
	}

	namespace := helmReleaseNamespace(tenant)

	assert.Equal(t, "team-namespace", namespace)
}

func TestHelmReleaseNamespaceFallsBackToTenantUUID(t *testing.T) {
	tenant := &dbmodel.Tenants{
		UUID: "tenant-uuid",
	}

	namespace := helmReleaseNamespace(tenant)

	assert.Equal(t, "tenant-uuid", namespace)
}

func TestResolveHelmReleaseNamespaceUsesExplicitNamespace(t *testing.T) {
	namespace, err := GetHelmReleaseHandler().resolveNamespace("demo-team", "demo-namespace")

	assert.NoError(t, err)
	assert.Equal(t, "demo-namespace", namespace)
}

func TestResolveHelmReleaseNamespaceFallsBackToTenantNamespace(t *testing.T) {
	tenantDao := &testTenantDao{
		tenant: &dbmodel.Tenants{
			Name:      "demo-team",
			UUID:      "tenant-uuid",
			Namespace: "tenant-namespace",
		},
	}
	db.SetTestManager(testManager{tenantDao: tenantDao})
	defer db.SetTestManager(nil)

	namespace, err := GetHelmReleaseHandler().resolveNamespace("demo-team", "")

	assert.NoError(t, err)
	assert.Equal(t, "demo-team", tenantDao.requestedFor)
	assert.Equal(t, "tenant-namespace", namespace)
}

func TestSummarizeHelmReleaseBuildsStableDTO(t *testing.T) {
	release := &helmrelease.Release{
		Name:      "demo-release",
		Version:   3,
		Namespace: "demo-namespace",
		Chart: &helmchart.Chart{
			Metadata: &helmchart.Metadata{
				Name:       "mysql",
				Version:    "9.4.2",
				AppVersion: "8.0.36",
			},
		},
		Info: &helmrelease.Info{
			Status:       helmrelease.StatusDeployed,
			LastDeployed: helmtime.Time{Time: time.Date(2026, 3, 20, 9, 30, 0, 0, time.UTC)},
		},
	}

	summary := summarizeHelmRelease(release)

	assert.Equal(t, "demo-release", summary.Name)
	assert.Equal(t, "mysql", summary.Chart)
	assert.Equal(t, "9.4.2", summary.ChartVersion)
	assert.Equal(t, "8.0.36", summary.AppVersion)
	assert.Equal(t, "deployed", summary.Status)
	assert.Equal(t, 3, summary.Version)
	assert.Equal(t, "demo-namespace", summary.Namespace)
	assert.Equal(t, "2026-03-20T09:30:00Z", summary.Updated)
}
