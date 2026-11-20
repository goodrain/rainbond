package v1

import (
	"encoding/json"
	"testing"

	jsonpatch "github.com/evanphx/json-patch"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// capability_id: rainbond.worker.patch.deployment-upgrade-strategy
func TestDeploymentUpgradePatchAppliesStrategyTransitions(t *testing.T) {
	maxSurge := intstr.FromInt(1)
	for _, tc := range []struct {
		name string
		old  appsv1.DeploymentStrategy
		new  appsv1.DeploymentStrategy
		want appsv1.DeploymentStrategyType
	}{
		{"rolling to recreate", appsv1.DeploymentStrategy{Type: appsv1.RollingUpdateDeploymentStrategyType,
			RollingUpdate: &appsv1.RollingUpdateDeployment{MaxSurge: &maxSurge}},
			appsv1.DeploymentStrategy{Type: appsv1.RecreateDeploymentStrategyType}, appsv1.RecreateDeploymentStrategyType},
		{"clear stale rolling settings", appsv1.DeploymentStrategy{Type: appsv1.RollingUpdateDeploymentStrategyType,
			RollingUpdate: &appsv1.RollingUpdateDeployment{MaxSurge: &maxSurge}},
			appsv1.DeploymentStrategy{Type: appsv1.RecreateDeploymentStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDeployment{MaxSurge: &maxSurge}}, appsv1.RecreateDeploymentStrategyType},
		{"recreate to rolling", appsv1.DeploymentStrategy{Type: appsv1.RecreateDeploymentStrategyType},
			appsv1.DeploymentStrategy{Type: appsv1.RollingUpdateDeploymentStrategyType}, appsv1.RollingUpdateDeploymentStrategyType},
	} {
		t.Run(tc.name, func(t *testing.T) {
			old := &appsv1.Deployment{Spec: appsv1.DeploymentSpec{Strategy: tc.old}}
			new := old.DeepCopy()
			new.Spec.Strategy = tc.new
			patch, err := getDeploymentModifiedConfiguration(old, new)
			if err != nil {
				t.Fatal(err)
			}
			if len(patch) == 0 {
				t.Fatal("strategy transition was omitted from the deployment patch")
			}
			original, err := json.Marshal(old)
			if err != nil {
				t.Fatal(err)
			}
			merged, err := jsonpatch.MergePatch(original, patch)
			if err != nil {
				t.Fatal(err)
			}
			var result appsv1.Deployment
			if err := json.Unmarshal(merged, &result); err != nil {
				t.Fatal(err)
			}
			if result.Spec.Strategy.Type != tc.want {
				t.Fatalf("patched strategy = %s, want %s; patch: %s", result.Spec.Strategy.Type, tc.want, patch)
			}
			if tc.want == appsv1.RecreateDeploymentStrategyType && result.Spec.Strategy.RollingUpdate != nil {
				t.Fatalf("patched Recreate retains rollingUpdate: %s", patch)
			}
		})
	}
}

// capability_id: rainbond.worker.patch.deployment-upgrade-strategy
func TestDeploymentUpgradePatchKeepsRollingDefaults(t *testing.T) {
	maxSurge := intstr.FromString("25%")
	old := &appsv1.Deployment{Spec: appsv1.DeploymentSpec{Strategy: appsv1.DeploymentStrategy{
		Type:          appsv1.RollingUpdateDeploymentStrategyType,
		RollingUpdate: &appsv1.RollingUpdateDeployment{MaxSurge: &maxSurge},
	}}}
	new := old.DeepCopy()
	new.Spec.Strategy.RollingUpdate = nil
	patch, err := getDeploymentModifiedConfiguration(old, new)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]interface{}
	if err := json.Unmarshal(patch, &fields); err != nil {
		t.Fatal(err)
	}
	if spec, ok := fields["spec"].(map[string]interface{}); ok && spec["strategy"] != nil {
		t.Fatalf("defaulted rolling settings should not be removed: %s", patch)
	}
}
