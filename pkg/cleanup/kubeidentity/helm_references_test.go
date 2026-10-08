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
	ktesting "k8s.io/client-go/testing"
)

func TestHelmReleaseListFiltersWithoutPagination(t *testing.T) {
	options := helmReleaseListOptions()
	if options.LabelSelector != "owner=helm" || options.Limit != 0 || options.Continue != "" {
		t.Fatal("Helm release list did not use the bounded standard label query", options)
	}
}

func TestHelmInventoryReadsOnlyStandardLabeledReleaseData(t *testing.T) {
	manifest := "apiVersion: v1\nkind: Pod\nspec:\n  containers:\n  - image: goodrain.me/history:v1\n"
	raw, _ := json.Marshal(map[string]interface{}{"name": "app", "namespace": "team", "version": 1, "manifest": manifest})
	release := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "sh.helm.release.v1.app.v1", Namespace: "team", UID: types.UID("release"), ResourceVersion: "7", Labels: map[string]string{"owner": "helm"}}, Data: map[string][]byte{"release": []byte(base64.StdEncoding.EncodeToString(raw))}}
	ordinary := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ordinary", Namespace: "team", UID: types.UID("ordinary"), ResourceVersion: "8"}}
	legacyRaw := strings.ReplaceAll(string(raw), "history:v1", "history:v2")
	legacyRaw = strings.ReplaceAll(legacyRaw, `"version":1`, `"version":2`)
	config := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "sh.helm.release.v1.app.v2", Namespace: "team", UID: "config-release", ResourceVersion: "9"}, Data: map[string]string{"release": base64.StdEncoding.EncodeToString([]byte(legacyRaw))}}
	client := fake.NewSimpleClientset(release, ordinary, config)
	client.PrependReactor("list", "secrets", func(ktesting.Action) (bool, runtime.Object, error) {
		return true, &corev1.SecretList{ListMeta: metav1.ListMeta{ResourceVersion: "snapshot"}, Items: []corev1.Secret{*release.DeepCopy()}}, nil
	})
	client.PrependReactor("list", "configmaps", func(ktesting.Action) (bool, runtime.Object, error) {
		return true, &corev1.ConfigMapList{ListMeta: metav1.ListMeta{ResourceVersion: "snapshot"}}, nil
	})
	result, err := ReadHelmReferenceInventory(context.Background(), client)
	if err != nil || !result.Complete || len(result.Images) != 1 || result.Images[0] != "goodrain.me/history:v1" {
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
		if action.GetVerb() != "list" {
			t.Fatal("read unrelated secret data")
		}
	}
}
