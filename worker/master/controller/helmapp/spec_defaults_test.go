package helmapp

import (
	"context"
	"testing"

	"github.com/goodrain/rainbond/pkg/apis/rainbond/v1alpha1"
	rainfake "github.com/goodrain/rainbond/pkg/generated/clientset/versioned/fake"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// capability_id: rainbond.worker.helmapp.preserve-current-spec
func TestHelmAppSpecDefaultsPreserveCurrentReferences(t *testing.T) {
	for _, configured := range []bool{false, true} {
		t.Run(map[bool]string{false: "default", true: "already-configured"}[configured], func(t *testing.T) {
			stale := newTestHelmApp()
			stale.Spec.PreStatus = v1alpha1.HelmAppPreStatusNotConfigured
			current := stale.DeepCopy()
			current.Spec.Version = "2.0.0"
			current.Spec.Overrides = []string{"image.tag=new"}
			current.Spec.PreStatus = ""
			if configured {
				current.Spec.PreStatus = v1alpha1.HelmAppPreStatusConfigured
			}
			client := rainfake.NewSimpleClientset(current)
			app := &App{ctx: context.Background(), rainbondClient: client, helmApp: stale}
			if err := app.UpdateSpec(); err != nil {
				t.Fatal(err)
			}
			got, err := client.RainbondV1alpha1().HelmApps(current.Namespace).Get(context.Background(), current.Name, metav1.GetOptions{})
			if err != nil {
				t.Fatal(err)
			}
			expected := v1alpha1.HelmAppPreStatusNotConfigured
			if configured {
				expected = v1alpha1.HelmAppPreStatusConfigured
			}
			if got.Spec.Version != "2.0.0" || len(got.Spec.Overrides) != 1 || got.Spec.Overrides[0] != "image.tag=new" || got.Spec.PreStatus != expected {
				t.Fatal("setup overwrote current image intent", got.Spec.Version, got.Spec.PreStatus)
			}
		})
	}
}
