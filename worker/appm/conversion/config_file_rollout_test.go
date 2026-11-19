package conversion

import (
	"bytes"
	"testing"

	appmtypes "github.com/goodrain/rainbond/worker/appm/types/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// capability_id: rainbond.config-file.rollout-on-content-change
func TestOwnedConfigFileContentChangesDeploymentTemplate(t *testing.T) {
	makeApp := func(content string) *appmtypes.AppService {
		annotations := configFilePodAnnotations(nil, []*corev1.ConfigMap{{
			ObjectMeta: metav1.ObjectMeta{
				Name: "stable-config", Annotations: map[string]string{appmtypes.ConfigFileScopeAnnotation: appmtypes.ConfigFileScopeOwned},
			},
			Data: map[string]string{"app.conf": content},
		}})
		app := &appmtypes.AppService{UpgradePatch: make(map[string][]byte)}
		replicas := int32(1)
		app.SetDeployment(&appsv1.Deployment{Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Annotations: annotations}},
		}})
		return app
	}
	previous, desired := makeApp("old"), makeApp("new")
	if err := previous.SetUpgradePatch(desired); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(desired.UpgradePatch["deployment"], []byte("rainbond.io/config-file-checksum")) {
		t.Fatalf("expected a Pod template checksum in the patch, got %s", desired.UpgradePatch["deployment"])
	}
	unchanged := makeApp("new")
	if err := desired.SetUpgradePatch(unchanged); err != nil {
		t.Fatal(err)
	}
	if string(unchanged.UpgradePatch["deployment"]) != "{}" {
		t.Fatalf("expected an empty patch for unchanged content, got %s", unchanged.UpgradePatch["deployment"])
	}
}
