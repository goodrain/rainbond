package kubeidentity

import (
	"context"
	"reflect"
	"slices"

	"github.com/goodrain/rainbond/pkg/apis/rainbond/v1alpha1"
	guard "github.com/goodrain/rainbond/pkg/cleanup"
	"github.com/goodrain/rainbond/pkg/generated/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ReadHelmAppReferenceInventory prevents deferred chart changes from being
// mistaken for absent references. Rendered release images are read separately;
// this verifies the desired state is represented by the latest deployed release.
// No chart values, overrides or repository credentials are returned.
func ReadHelmAppReferenceInventory(ctx context.Context, client versioned.Interface, releases []guard.HelmReleaseIdentity) (guard.RegionReferenceInventory, error) {
	denied := guard.RegionReferenceInventory{}
	if client == nil || len(releases) > 1024 {
		return denied, ErrBinding
	}
	result := guard.RegionReferenceInventory{Complete: true, Images: []string{}}
	latest := map[string]guard.HelmReleaseIdentity{}
	ambiguous := map[string]bool{}
	for _, release := range releases {
		key := release.Namespace + "/" + release.Name
		if previous, ok := latest[key]; !ok || release.Revision > previous.Revision {
			latest[key] = release
			ambiguous[key] = false
		} else if release.Revision == previous.Revision && !reflect.DeepEqual(release, previous) {
			ambiguous[key] = true
		}
	}
	versions := map[string]string{}
	cursor, snapshot := "", ""
	cursors := map[string]bool{}
	for pages := 0; ; pages++ {
		if pages >= 32 || ctx.Err() != nil {
			return denied, ErrBinding
		}
		apps, err := client.RainbondV1alpha1().HelmApps(metav1.NamespaceAll).List(ctx, metav1.ListOptions{Limit: 64, Continue: cursor})
		if err != nil || apps == nil || apps.ResourceVersion == "" || (snapshot != "" && snapshot != apps.ResourceVersion) {
			return denied, ErrBinding
		}
		snapshot = apps.ResourceVersion
		for _, app := range apps.Items {
			uid := string(app.UID)
			if uid == "" || app.Name == "" || app.Namespace == "" || app.ResourceVersion == "" || versions[uid] != "" || len(versions) >= 1024 {
				return denied, ErrBinding
			}
			versions[uid] = app.ResourceVersion
			key := app.Namespace + "/" + app.Name
			release, found := latest[key]
			if app.DeletionTimestamp != nil || app.Spec.PreStatus != v1alpha1.HelmAppPreStatusConfigured || app.Spec.Version == "" || app.Spec.TemplateName == "" || app.Spec.Version != app.Status.CurrentVersion || !slices.Equal(app.Spec.Overrides, app.Status.Overrides) || !found || ambiguous[key] || release.Status != "deployed" || release.Chart != app.Spec.TemplateName || release.ChartVersion != app.Spec.Version || !guard.HelmReleaseMatchesOverrides(release, app.Spec.Overrides) {
				result.Complete = false
			}
			for _, condition := range []v1alpha1.HelmAppConditionType{v1alpha1.HelmAppChartReady, v1alpha1.HelmAppPreInstalled, v1alpha1.HelmAppInstalled} {
				if !app.Status.IsConditionTrue(condition) {
					result.Complete = false
				}
			}
		}
		if apps.Continue == "" {
			break
		}
		if cursors[apps.Continue] {
			return denied, ErrBinding
		}
		cursors[apps.Continue] = true
		cursor = apps.Continue
	}
	cursor, snapshot = "", ""
	cursors = map[string]bool{}
	for pages := 0; ; pages++ {
		if pages >= 32 || ctx.Err() != nil {
			return denied, ErrBinding
		}
		current, err := client.RainbondV1alpha1().HelmApps(metav1.NamespaceAll).List(ctx, metav1.ListOptions{Limit: 64, Continue: cursor})
		if err != nil || current == nil || current.ResourceVersion == "" || (snapshot != "" && snapshot != current.ResourceVersion) {
			return denied, ErrBinding
		}
		snapshot = current.ResourceVersion
		for _, app := range current.Items {
			uid := string(app.UID)
			if versions[uid] == "" || versions[uid] != app.ResourceVersion {
				return denied, ErrBinding
			}
			delete(versions, uid)
		}
		if current.Continue == "" {
			break
		}
		if cursors[current.Continue] {
			return denied, ErrBinding
		}
		cursors[current.Continue] = true
		cursor = current.Continue
	}
	if len(versions) != 0 {
		return denied, ErrBinding
	}
	return result, nil
}
