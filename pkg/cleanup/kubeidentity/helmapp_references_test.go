package kubeidentity

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/goodrain/rainbond/pkg/apis/rainbond/v1alpha1"
	guard "github.com/goodrain/rainbond/pkg/cleanup"
	rainfake "github.com/goodrain/rainbond/pkg/generated/clientset/versioned/fake"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ktesting "k8s.io/client-go/testing"
)

// capability_id: rainbond.cleanup.deferred-helmapp-references
func TestHelmAppReferencesRequireSettledRelease(t *testing.T) {
	for _, scenario := range []string{"current", "pending-version", "pending-overrides", "unconfigured", "missing-release", "pending-release", "newer-pending", "changed", "chart-mismatch", "missing-condition", "conflicting-release", "empty", "config-mismatch"} {
		t.Run(scenario, func(t *testing.T) {
			app := &v1alpha1.HelmApp{ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "team", UID: "app-uid", ResourceVersion: "1"}, Spec: v1alpha1.HelmAppSpec{TemplateName: "template", Version: "1.0.0", PreStatus: v1alpha1.HelmAppPreStatusConfigured, Overrides: []string{"image.tag=current"}}, Status: v1alpha1.HelmAppStatus{CurrentVersion: "1.0.0", Overrides: []string{"image.tag=current"}}}
			for _, kind := range []v1alpha1.HelmAppConditionType{v1alpha1.HelmAppChartReady, v1alpha1.HelmAppPreInstalled, v1alpha1.HelmAppInstalled} {
				app.Status.UpdateConditionStatus(kind, corev1.ConditionTrue)
			}
			releases := []guard.HelmReleaseIdentity{{Name: "app", Namespace: "team", Revision: 1, Chart: "template", ChartVersion: "1.0.0", Status: "deployed"}}
			switch scenario {
			case "pending-version":
				app.Spec.Version = "2.0.0"
			case "pending-overrides":
				app.Spec.Overrides = []string{"image.tag=new"}
			case "unconfigured":
				app.Spec.PreStatus = v1alpha1.HelmAppPreStatusNotConfigured
			case "missing-release":
				releases = nil
			case "pending-release":
				releases[0].Status = "pending-upgrade"
			case "chart-mismatch":
				releases[0].Chart = "other-template"
			case "missing-condition":
				app.Status.UpdateConditionStatus(v1alpha1.HelmAppInstalled, corev1.ConditionFalse)
			case "conflicting-release":
				other := releases[0]
				other.Status = "failed"
				releases = append(releases, other)
			case "newer-pending":
				newer := releases[0]
				newer.Revision = 2
				newer.Status = "pending-upgrade"
				releases = append(releases, newer)
			}
			objects := []runtime.Object{app}
			if scenario == "empty" {
				objects = nil
			}
			valuesProof := guard.InspectHelmReleaseReferences(base64.StdEncoding.EncodeToString([]byte(`{"name":"app","namespace":"team","version":1,"config":{"image":{"tag":"current"}}}`)), "app", "team", 1)
			if scenario == "config-mismatch" {
				valuesProof = guard.InspectHelmReleaseReferences(base64.StdEncoding.EncodeToString([]byte(`{"name":"app","namespace":"team","version":1,"config":{"image":{"tag":"old"}}}`)), "app", "team", 1)
			}
			for i := range releases {
				releases[i].ValueHashes = valuesProof.HelmReleases[0].ValueHashes
			}
			client := rainfake.NewSimpleClientset(objects...)
			calls := 0
			client.PrependReactor("list", "helmapps", func(action ktesting.Action) (bool, runtime.Object, error) {
				calls++
				value, err := client.Tracker().List(schema.GroupVersionResource{Group: "rainbond.io", Version: "v1alpha1", Resource: "helmapps"}, schema.GroupVersionKind{Group: "rainbond.io", Version: "v1alpha1", Kind: "HelmApp"}, "")
				if err != nil {
					return true, nil, err
				}
				list := value.(*v1alpha1.HelmAppList)
				list.ResourceVersion = "snapshot"
				if scenario == "changed" && calls == 2 {
					list.Items[0].ResourceVersion = "2"
				}
				return true, list, nil
			})
			result, err := ReadHelmAppReferenceInventory(context.Background(), client, releases)
			if scenario == "current" || scenario == "empty" {
				if err != nil || !result.Complete {
					t.Fatal("settled release rejected", err)
				}
			} else if err == nil && result.Complete {
				t.Fatal("unresolved desired images ignored")
			}
		})
	}
}
