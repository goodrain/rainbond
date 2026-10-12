package cleanup

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func helmReferenceFixture(t *testing.T, manifest string, compressed bool) string {
	t.Helper()
	raw, err := json.Marshal(map[string]interface{}{"name": "app", "namespace": "team", "version": 2, "manifest": manifest, "hooks": []map[string]string{{"manifest": "apiVersion: batch/v1\nkind: Job\nspec:\n  template:\n    spec:\n      containers:\n      - image: goodrain.me/hooks:v1\n"}}, "config": map[string]string{"fixture-private-value": "must-not-return"}})
	if err != nil {
		t.Fatal(err)
	}
	if compressed {
		var buf bytes.Buffer
		writer := gzip.NewWriter(&buf)
		writer.Write(raw)
		writer.Close()
		raw = buf.Bytes()
	}
	return base64.StdEncoding.EncodeToString(raw)
}
func TestHelmRetainedManifestAndHooksProvideReferencesOnly(t *testing.T) {
	manifest := "apiVersion: apps/v1\nkind: Deployment\nspec:\n  template:\n    spec:\n      containers:\n      - image: goodrain.me/retained:v2\n"
	for _, compressed := range []bool{false, true} {
		result := InspectHelmReleaseReferences(helmReferenceFixture(t, manifest, compressed), "app", "team", 2)
		if !result.Complete || len(result.Images) != 2 || strings.Contains(strings.Join(result.Images, " "), "must-not-return") {
			t.Fatal("missing or leaking retained references", result.Complete, len(result.Images))
		}
	}
}
func TestHelmCorruptForeignAndUnknownReleasesAreNotEmptySuccess(t *testing.T) {
	encoded := helmReferenceFixture(t, "apiVersion: example.test/v1\nkind: UnresolvedWorkload\n", true)
	if InspectHelmReleaseReferences(encoded, "app", "team", 2).Complete {
		t.Fatal("unknown workload ignored")
	}
	for _, value := range []string{"!", base64.StdEncoding.EncodeToString([]byte("{}")), strings.Repeat("a", (1<<20)+1)} {
		if InspectHelmReleaseReferences(value, "app", "team", 2).Complete {
			t.Fatal("invalid record became empty success")
		}
	}
	encoded = helmReferenceFixture(t, "", true)
	for _, scope := range []struct {
		name, namespace string
		revision        int
	}{{"other", "team", 2}, {"app", "other", 2}, {"app", "team", 3}} {
		if InspectHelmReleaseReferences(encoded, scope.name, scope.namespace, scope.revision).Complete {
			t.Fatal("foreign release accepted")
		}
	}
}

func TestHelmReferenceDecompressionIsBounded(t *testing.T) {
	encoded := helmReferenceFixture(t, strings.Repeat("x", 9<<20), true)
	if InspectHelmReleaseReferences(encoded, "app", "team", 2).Complete {
		t.Fatal("oversized expanded record accepted")
	}
}
