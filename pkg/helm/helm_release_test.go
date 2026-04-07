package helm

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	helmchart "helm.sh/helm/v3/pkg/chart"
	helmrelease "helm.sh/helm/v3/pkg/release"
	helmpkgrepo "helm.sh/helm/v3/pkg/repo"
	helmtime "helm.sh/helm/v3/pkg/time"
)

// capability_id: rainbond.helm-release.values-yaml
func TestParseValuesYAML(t *testing.T) {
	vals, err := parseValuesYAML("replicaCount: 2\nservice:\n  type: ClusterIP\n")
	assert.NoError(t, err)
	assert.Equal(t, float64(2), vals["replicaCount"])
	service, ok := vals["service"].(map[string]interface{})
	if assert.True(t, ok) {
		assert.Equal(t, "ClusterIP", service["type"])
	}

	_, err = parseValuesYAML("service: [")
	assert.Error(t, err)
}

// capability_id: rainbond.helm-release.oci-reference-normalize
func TestNormalizeOCIChartReference(t *testing.T) {
	chartRef, version := normalizeOCIChartReference("oci://example.com/charts/demo:1.2.3", "")
	assert.Equal(t, "oci://example.com/charts/demo", chartRef)
	assert.Equal(t, "1.2.3", version)

	chartRef, version = normalizeOCIChartReference("oci://example.com/charts/demo:1.2.3", "9.9.9")
	assert.Equal(t, "oci://example.com/charts/demo", chartRef)
	assert.Equal(t, "9.9.9", version)

	chartRef, version = normalizeOCIChartReference("bitnami/nginx", "")
	assert.Equal(t, "bitnami/nginx", chartRef)
	assert.Equal(t, "", version)
}

// capability_id: rainbond.helm-release.installable-check
func TestCheckIfInstallable(t *testing.T) {
	assert.NoError(t, checkIfInstallable(&helmchart.Chart{
		Metadata: &helmchart.Metadata{Type: "application"},
	}))
	assert.NoError(t, checkIfInstallable(&helmchart.Chart{
		Metadata: &helmchart.Metadata{Type: ""},
	}))
	assert.Error(t, checkIfInstallable(&helmchart.Chart{
		Metadata: &helmchart.Metadata{Type: "library"},
	}))
}

// capability_id: rainbond.helm-release.history-summary
func TestGetReleaseHistory(t *testing.T) {
	now := time.Now().UTC()
	releases := []*helmrelease.Release{
		{
			Version: 1,
			Chart: &helmchart.Chart{
				Metadata: &helmchart.Metadata{
					Name:       "demo",
					Version:    "0.1.0",
					AppVersion: "1.0.0",
				},
			},
			Info: &helmrelease.Info{
				Status:       helmrelease.StatusSuperseded,
				Description:  "old release",
				LastDeployed: helmtime.Time{Time: now.Add(-time.Hour)},
			},
		},
		{
			Version: 2,
			Chart: &helmchart.Chart{
				Metadata: &helmchart.Metadata{
					Name:       "demo",
					Version:    "0.2.0",
					AppVersion: "1.1.0",
				},
			},
			Info: &helmrelease.Info{
				Status:       helmrelease.StatusDeployed,
				Description:  "current release",
				LastDeployed: helmtime.Time{Time: now},
			},
		},
	}

	history := getReleaseHistory(releases)
	if assert.Len(t, history, 2) {
		assert.Equal(t, 2, history[0].Revision)
		assert.Equal(t, "deployed", history[0].Status)
		assert.Equal(t, "demo-0.2.0", history[0].Chart)
		assert.Equal(t, "1.1.0", history[0].AppVersion)
		assert.Equal(t, "current release", history[0].Description)
		assert.Equal(t, 1, history[1].Revision)
		assert.Equal(t, "superseded", history[1].Status)
	}
}

// capability_id: rainbond.helm-release.chart-name-format
func TestFormatChartName(t *testing.T) {
	assert.Equal(t, "MISSING", formatChartName(nil))
	assert.Equal(t, "MISSING", formatChartName(&helmchart.Chart{}))
	assert.Equal(t, "demo-1.2.3", formatChartName(&helmchart.Chart{
		Metadata: &helmchart.Metadata{Name: "demo", Version: "1.2.3"},
	}))
}

// capability_id: rainbond.helm-release.app-version-format
func TestFormatAppVersion(t *testing.T) {
	assert.Equal(t, "MISSING", formatAppVersion(nil))
	assert.Equal(t, "MISSING", formatAppVersion(&helmchart.Chart{}))
	assert.Equal(t, "8.0.0", formatAppVersion(&helmchart.Chart{
		Metadata: &helmchart.Metadata{AppVersion: "8.0.0"},
	}))
}

// capability_id: rainbond.helm-release.strip-kube-version
func TestRemoveKubeVersionFromChart(t *testing.T) {
	child := &helmchart.Chart{
		Metadata: &helmchart.Metadata{Name: "child", KubeVersion: ">=1.28.0"},
	}
	parent := &helmchart.Chart{
		Metadata: &helmchart.Metadata{Name: "parent", KubeVersion: ">=1.27.0"},
	}
	parent.SetDependencies(child)

	removeKubeVersionFromChart(parent)

	assert.Equal(t, "", parent.Metadata.KubeVersion)
	assert.Equal(t, "", child.Metadata.KubeVersion)
}

// capability_id: rainbond.helm-repo.requested-filter
func TestCheckRequestedRepos(t *testing.T) {
	valid := []*helmpkgrepo.Entry{
		{Name: "bitnami"},
		{Name: "goodrain"},
	}

	assert.NoError(t, checkRequestedRepos([]string{"bitnami"}, valid))
	assert.Error(t, checkRequestedRepos([]string{"missing"}, valid))
}

// capability_id: rainbond.helm-repo.requested-filter
func TestIsRepoRequested(t *testing.T) {
	assert.True(t, isRepoRequested("bitnami", []string{"bitnami", "goodrain"}))
	assert.False(t, isRepoRequested("missing", []string{"bitnami", "goodrain"}))
}
