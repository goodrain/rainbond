package kubeidentity

import (
	"context"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
)

func TestAPIHelperReferencesIncludeDesiredRollbackAndRunningImages(t *testing.T) {
	spec := func(tag string) corev1.PodSpec {
		return corev1.PodSpec{Containers: []corev1.Container{{Name: "rbd-api", Env: []corev1.EnvVar{{Name: "CLEANUP_NODE_EXECUTOR_IMAGE", Value: "goodrain.me/helper:" + tag}, {Name: "UNRELATED_AUTH", Value: "fixture-private-value"}}}}}
	}
	labels := map[string]string{"name": "rbd-api"}
	deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "rbd-api", Namespace: "system", Labels: labels}, Spec: appsv1.DeploymentSpec{Template: corev1.PodTemplateSpec{Spec: spec("deploy")}}}
	replica := &appsv1.ReplicaSet{ObjectMeta: metav1.ObjectMeta{Name: "old-api", Namespace: "system", Labels: labels}, Spec: appsv1.ReplicaSetSpec{Template: corev1.PodTemplateSpec{Spec: spec("rollback")}}}
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "system", Labels: labels}, Spec: spec("running")}
	client := fake.NewSimpleClientset(deployment, replica, pod)
	client.PrependReactor("list", "*", func(action ktesting.Action) (bool, runtime.Object, error) {
		kind := map[string]string{"pods": "Pod", "deployments": "Deployment", "replicasets": "ReplicaSet"}[action.GetResource().Resource]
		result, err := client.Tracker().List(action.GetResource(), schema.GroupVersionKind{Group: action.GetResource().Group, Version: "v1", Kind: kind}, action.GetNamespace())
		if err == nil {
			accessor, _ := meta.ListAccessor(result)
			accessor.SetResourceVersion("snapshot")
		}
		return true, result, err
	})
	component := &unstructured.Unstructured{Object: map[string]interface{}{"apiVersion": "rainbond.io/v1alpha1", "kind": "RbdComponent", "metadata": map[string]interface{}{"name": "rbd-api", "namespace": "system", "uid": "api-config", "resourceVersion": "1"}, "spec": map[string]interface{}{"env": []interface{}{map[string]interface{}{"name": "CLEANUP_NODE_EXECUTOR_IMAGE", "value": "goodrain.me/helper:desired"}}}}}
	custom := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), component)
	result, err := ReadAPIHelperReferences(context.Background(), client, custom, "system")
	if err != nil || !result.Complete || len(result.Images) != 4 || strings.Contains(strings.Join(result.Images, " "), "fixture-private-value") {
		t.Fatal("incomplete or leaking configured references", result, err)
	}
}
func TestAPIHelperDoesNotReadSecretBackedEnvironment(t *testing.T) {
	for _, result := range []bool{
		helperEnvironmentReferences([]corev1.EnvVar{{Name: "CLEANUP_NODE_EXECUTOR_IMAGE", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{Key: "image"}}}}, nil).Complete,
		helperEnvironmentReferences(nil, []corev1.EnvFromSource{{SecretRef: &corev1.SecretEnvSource{}}}).Complete,
		helperEnvironmentReferences([]corev1.EnvVar{{Name: "CLEANUP_NODE_EXECUTOR_IMAGE", Value: "$(REGISTRY)/helper:v1"}}, nil).Complete,
	} {
		if result {
			t.Fatal("unresolved image source became complete")
		}
	}
}
