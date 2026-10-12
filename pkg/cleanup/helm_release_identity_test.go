package cleanup

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

// capability_id: rainbond.cleanup.helm-release-identity
func TestHelmReleaseIdentityDoesNotExposeChartConfiguration(t *testing.T) {
	raw := []byte(`{"name":"app","namespace":"team","version":3,"manifest":"","chart":{"metadata":{"name":"template","version":"1.2.3"},"values":{"private":"must-not-return"}},"info":{"status":"deployed"},"config":{"private":"must-not-return"}}`)
	result := InspectHelmReleaseReferences(base64.StdEncoding.EncodeToString(raw), "app", "team", 3)
	if !result.Complete || len(result.HelmReleases) != 1 || result.HelmReleases[0].Chart != "template" || result.HelmReleases[0].ChartVersion != "1.2.3" || result.HelmReleases[0].Status != "deployed" {
		t.Fatal("missing release identity")
	}
	encoded, err := json.Marshal(result)
	if err != nil || strings.Contains(string(encoded), "template") || strings.Contains(string(encoded), "must-not-return") {
		t.Fatal("internal metadata escaped inventory response")
	}
}

// capability_id: rainbond.cleanup.helm-release-values-proof
func TestHelmReleaseOverridesRequireActualConfig(t *testing.T) {
	raw := []byte(`{"name":"app","namespace":"team","version":3,"manifest":"","config":{"image":{"tag":"old"},"replicas":2}}`)
	result := InspectHelmReleaseReferences(base64.StdEncoding.EncodeToString(raw), "app", "team", 3)
	if len(result.HelmReleases) != 1 {
		t.Fatal("missing identity")
	}
	release := result.HelmReleases[0]
	if !HelmReleaseMatchesOverrides(release, []string{"image.tag=old"}) || !HelmReleaseMatchesOverrides(release, []string{"replicas=2"}) || HelmReleaseMatchesOverrides(release, []string{"image.tag=new"}) {
		t.Fatal("actual config not enforced")
	}
}
