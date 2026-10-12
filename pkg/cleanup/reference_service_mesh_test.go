package cleanup

import (
	"encoding/json"
	"testing"

	"github.com/goodrain/rainbond/db/model"
)

// capability_id: rainbond.cleanup.native-service-mesh-references
func TestSavedServiceMeshRequiresKnownNativeProvider(t *testing.T) {
	for _, tc := range []struct {
		name, provider string
		extra          bool
		complete       bool
	}{
		{"native", model.GovernanceModeKubernetesNativeService, false, true},
		{"builtin", model.GovernanceModeBuildInServiceMesh, false, true},
		{"external", model.GovernanceModeIstioServiceMesh, false, false},
		{"custom", "custom.mesh", false, false},
		{"missing", "", false, false},
		{"unknown-shape", model.GovernanceModeBuildInServiceMesh, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			object := map[string]interface{}{"apiVersion": "rainbond.io/v1alpha1", "kind": "ServiceMesh", "metadata": map[string]interface{}{"name": "app"}, "provisioner": tc.provider, "selector": map[string]interface{}{"app_id": "owned"}}
			if tc.extra {
				object["spec"] = map[string]interface{}{"image": "goodrain.me/hidden:v1"}
			}
			raw, _ := json.Marshal(object)
			images := []string{}
			complete := inspectSavedWorkload(string(raw), func(image string) { images = append(images, image) })
			if complete != tc.complete || len(images) != 0 {
				t.Fatal("incorrect custom resource evidence", complete, images)
			}
		})
	}
}
