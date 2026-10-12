package kubeidentity

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"

	corev1 "k8s.io/api/core/v1"
)

func TestPlatformDefaultsRemainReferencedWithoutActiveBuilds(t *testing.T) {
	result := platformHelperEnvironmentReferences("rbd-chaos", nil, nil)
	if !result.Complete {
		t.Fatal("default configuration incomplete")
	}
	joined := "\n" + strings.Join(result.Images, "\n") + "\n"
	for _, image := range []string{"goodrain.me/runner:latest-amd64", "goodrain.me/builder:latest-arm64", "goodrain.me/ubuntu-noble-builder:0.0.98", "goodrain.me/run-jammy-full:0.1.141"} {
		if !strings.Contains(joined, "\n"+image+"\n") {
			t.Fatal("required build image missing", image)
		}
	}
	worker := platformHelperEnvironmentReferences("rbd-worker", nil, nil)
	if !worker.Complete || len(worker.Images) != 2 {
		t.Fatal("mesh/probe defaults missing", worker)
	}
}
func TestPlatformHelperOverridesNeverExposeOtherEnvironmentValues(t *testing.T) {
	result := platformHelperEnvironmentReferences("rbd-chaos", []corev1.EnvVar{{Name: "BUILD_IMAGE_REPOSTORY_DOMAIN", Value: "registry.local/team"}, {Name: "RUNNER_IMAGE_NAME", Value: "custom-runner:v3"}, {Name: "UNRELATED_AUTH", Value: "fixture-private-value"}}, nil)
	joined := strings.Join(result.Images, " ")
	if !result.Complete || !strings.Contains(joined, "registry.local/team/custom-runner:v3") || strings.Contains(joined, "fixture-private-value") {
		t.Fatal("incorrect configured references")
	}
	if platformHelperEnvironmentReferences("rbd-worker", []corev1.EnvVar{{Name: "TCPMESH_DEFAULT_IMAGE_NAME", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{Key: "image"}}}}, nil).Complete {
		t.Fatal("unresolved image source accepted")
	}
}

func TestBuilderDaemonSetRollbackHelperReferences(t *testing.T) {
	podSpec := func(tag string) corev1.PodSpec {
		return corev1.PodSpec{Containers: []corev1.Container{{Name: "rbd-chaos", Env: []corev1.EnvVar{{Name: "CNB_BUILDER_IMAGE", Value: "registry.local/cnb:" + tag}}}}}
	}
	labels := map[string]string{"name": "rbd-chaos"}
	daemon := &appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Name: "rbd-chaos", Namespace: "system", Labels: labels}, Spec: appsv1.DaemonSetSpec{Template: corev1.PodTemplateSpec{Spec: podSpec("current")}}}
	old, _ := json.Marshal(&appsv1.DaemonSet{Spec: appsv1.DaemonSetSpec{Template: corev1.PodTemplateSpec{Spec: podSpec("rollback")}}})
	revision := &appsv1.ControllerRevision{ObjectMeta: metav1.ObjectMeta{Name: "old", Namespace: "system", Labels: labels}, Data: runtime.RawExtension{Raw: old}}
	client := fake.NewSimpleClientset(daemon, revision)
	client.PrependReactor("list", "*", func(action ktesting.Action) (bool, runtime.Object, error) {
		kind := map[string]string{"pods": "Pod", "deployments": "Deployment", "replicasets": "ReplicaSet", "daemonsets": "DaemonSet", "controllerrevisions": "ControllerRevision"}[action.GetResource().Resource]
		result, err := client.Tracker().List(action.GetResource(), schema.GroupVersionKind{Group: action.GetResource().Group, Version: "v1", Kind: kind}, action.GetNamespace())
		if err == nil {
			accessor, _ := meta.ListAccessor(result)
			accessor.SetResourceVersion("snapshot")
		}
		return true, result, err
	})
	component := &unstructured.Unstructured{Object: map[string]interface{}{"apiVersion": "rainbond.io/v1alpha1", "kind": "RbdComponent", "metadata": map[string]interface{}{"name": "rbd-chaos", "namespace": "system", "uid": "component", "resourceVersion": "1"}}}
	result, err := readComponentHelperReferences(context.Background(), client, dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), component), "system", "rbd-chaos")
	if err != nil || !result.Complete {
		t.Fatal(result, err)
	}
	joined := strings.Join(result.Images, " ")
	if !strings.Contains(joined, "registry.local/cnb:current") || !strings.Contains(joined, "registry.local/cnb:rollback") {
		t.Fatal("lost daemon rollback references")
	}
}
