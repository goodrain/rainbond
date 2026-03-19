package handler

import (
	"testing"

	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
				require.NoError(t, err)
				return
			}
			require.EqualError(t, err, tt.wantErr)
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
