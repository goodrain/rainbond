package kubeidentity

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	metadatafake "k8s.io/client-go/metadata/fake"
	ktesting "k8s.io/client-go/testing"
)

func TestHelmMetadataSnapshotDoesNotRequestPaginatedStorage(t *testing.T) {
	options := helmMetadataListOptions()
	if options.Limit != 0 || options.Continue != "" {
		t.Fatal("Helm metadata snapshot requested slow storage pagination", options)
	}
}

func TestHelmInventoryReadsOnlyReleaseDataIncludingUnlabelledHistory(t *testing.T) {
	manifest := "apiVersion: v1\nkind: Pod\nspec:\n  containers:\n  - image: goodrain.me/history:v1\n"
	raw, _ := json.Marshal(map[string]interface{}{"name": "app", "namespace": "team", "version": 1, "manifest": manifest})
	release := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "sh.helm.release.v1.app.v1", Namespace: "team", UID: types.UID("release"), ResourceVersion: "7", Labels: map[string]string{"owner": "helm"}}, Data: map[string][]byte{"release": []byte(base64.StdEncoding.EncodeToString(raw))}}
	ordinary := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ordinary", Namespace: "team", UID: types.UID("ordinary"), ResourceVersion: "8"}}
	legacyRaw := strings.ReplaceAll(string(raw), "history:v1", "history:v2")
	legacyRaw = strings.ReplaceAll(legacyRaw, `"version":1`, `"version":2`)
	config := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "sh.helm.release.v1.app.v2", Namespace: "team", UID: "config-release", ResourceVersion: "9"}, Data: map[string]string{"release": base64.StdEncoding.EncodeToString([]byte(legacyRaw))}}
	client := fake.NewSimpleClientset(release, ordinary, config)
	metadataClient := metadatafake.NewSimpleMetadataClient(runtime.NewScheme())
	metadataClient.PrependReactor("list", "*", func(action ktesting.Action) (bool, runtime.Object, error) {
		list := &metav1.List{ListMeta: metav1.ListMeta{ResourceVersion: "snapshot"}}
		if action.GetResource().Resource == "secrets" {
			for _, secret := range []*corev1.Secret{release, ordinary} {
				list.Items = append(list.Items, runtime.RawExtension{Object: &metav1.PartialObjectMetadata{ObjectMeta: secret.ObjectMeta}})
			}
		}
		if action.GetResource().Resource == "configmaps" {
			list.Items = append(list.Items, runtime.RawExtension{Object: &metav1.PartialObjectMetadata{ObjectMeta: config.ObjectMeta}})
		}
		return true, list, nil
	})
	result, err := ReadHelmReferenceInventory(context.Background(), client, metadataClient)
	if err != nil || !result.Complete || len(result.Images) != 2 || result.Images[0] != "goodrain.me/history:v1" || result.Images[1] != "goodrain.me/history:v2" {
		t.Fatal(result, err)
	}
	for _, action := range client.Actions() {
		if action.GetVerb() == "list" {
			selector := action.(ktesting.ListAction).GetListRestrictions().Labels
			if selector == nil || selector.String() != "owner=helm" {
				t.Fatal("read unrelated secret data")
			}
			continue
		}
		if action.GetVerb() != "get" || (action.(ktesting.GetAction).GetName() != release.Name && action.(ktesting.GetAction).GetName() != config.Name) {
			t.Fatal("read unrelated secret data")
		}
	}
	release.ResourceVersion = "replaced"
	if err := client.Tracker().Update(corev1.SchemeGroupVersion.WithResource("secrets"), release.DeepCopy(), release.Namespace); err != nil {
		t.Fatal(err)
	}
	client.PrependReactor("get", "secrets", func(ktesting.Action) (bool, runtime.Object, error) { return true, release.DeepCopy(), nil })
	// Keep the metadata snapshot old to simulate replacement between LIST and GET.
	metadataClient.PrependReactor("list", "secrets", func(ktesting.Action) (bool, runtime.Object, error) {
		m := release.ObjectMeta
		m.ResourceVersion = "7"
		return true, &metav1.List{ListMeta: metav1.ListMeta{ResourceVersion: "snapshot"}, Items: []runtime.RawExtension{{Object: &metav1.PartialObjectMetadata{ObjectMeta: m}}}}, nil
	})
	if result, err := ReadHelmReferenceInventory(context.Background(), client, metadataClient); err == nil || result.Complete {
		t.Fatal("changed record certified")
	}
	metadataClient.PrependReactor("list", "secrets", func(ktesting.Action) (bool, runtime.Object, error) {
		return true, &metav1.List{}, nil
	})
	if _, err := ReadHelmReferenceInventory(context.Background(), client, metadataClient); err == nil || !strings.Contains(err.Error(), "secrets metadata snapshot missing") {
		t.Fatal("missing metadata snapshot was not diagnosable", err)
	}
}
