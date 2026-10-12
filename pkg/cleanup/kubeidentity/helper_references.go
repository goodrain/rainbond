package kubeidentity

import (
	"context"
	"encoding/json"
	"strings"

	guard "github.com/goodrain/rainbond/pkg/cleanup"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

// ReadAPIHelperReferences covers desired configuration, rollback templates and
// live API instances, not just a currently running executor Pod. Only the known
// executor-image variable is inspected; other environment values never escape.
func ReadAPIHelperReferences(ctx context.Context, client kubernetes.Interface, custom dynamic.Interface, namespace string) (guard.RegionReferenceInventory, error) {
	return readComponentHelperReferences(ctx, client, custom, namespace, "rbd-api")
}

// ReadPlatformHelperReferences includes configured and built-in future executors
// for the API, builder and worker, including retained rollout configurations.
func ReadPlatformHelperReferences(ctx context.Context, client kubernetes.Interface, custom dynamic.Interface, namespace string) (guard.RegionReferenceInventory, error) {
	result := guard.RegionReferenceInventory{Complete: true}
	for _, name := range []string{"rbd-api", "rbd-chaos", "rbd-worker"} {
		part, err := readComponentHelperReferences(ctx, client, custom, namespace, name)
		if err != nil {
			return guard.RegionReferenceInventory{}, err
		}
		result = guard.MergeReferenceInventories(result, part)
	}
	return result, nil
}
func readComponentHelperReferences(ctx context.Context, client kubernetes.Interface, custom dynamic.Interface, namespace, componentName string) (guard.RegionReferenceInventory, error) {
	denied := guard.RegionReferenceInventory{}
	if client == nil || custom == nil || namespace == "" {
		return denied, ErrBinding
	}
	component, err := custom.Resource(schema.GroupVersionResource{Group: "rainbond.io", Version: "v1alpha1", Resource: "rbdcomponents"}).Namespace(namespace).Get(ctx, componentName, metav1.GetOptions{})
	if err != nil || component == nil || component.GetName() != componentName || component.GetNamespace() != namespace || component.GetUID() == "" || component.GetResourceVersion() == "" {
		return denied, ErrBinding
	}
	raw, _, err := unstructured.NestedSlice(component.Object, "spec", "env")
	if err != nil || len(raw) > 1000 {
		return denied, ErrBinding
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		return denied, ErrBinding
	}
	var env []corev1.EnvVar
	if json.Unmarshal(encoded, &env) != nil {
		return denied, ErrBinding
	}
	result := platformHelperEnvironmentReferences(componentName, env, nil)
	options := metav1.ListOptions{LabelSelector: "name=" + componentName, Limit: 129}
	deployments, err := client.AppsV1().Deployments(namespace).List(ctx, options)
	if err != nil || deployments == nil || deployments.ResourceVersion == "" || deployments.Continue != "" || (componentName != "rbd-chaos" && len(deployments.Items) == 0) || len(deployments.Items) > 128 {
		return denied, ErrBinding
	}
	replicas, err := client.AppsV1().ReplicaSets(namespace).List(ctx, options)
	if err != nil || replicas == nil || replicas.ResourceVersion == "" || replicas.Continue != "" || len(replicas.Items) > 128 {
		return denied, ErrBinding
	}
	pods, err := client.CoreV1().Pods(namespace).List(ctx, options)
	if err != nil || pods == nil || pods.ResourceVersion == "" || pods.Continue != "" || (componentName == "rbd-api" && len(pods.Items) == 0) || len(pods.Items) > 128 {
		return denied, ErrBinding
	}
	add := func(spec corev1.PodSpec) {
		found := 0
		for _, container := range spec.Containers {
			if container.Name == componentName {
				found++
				result = guard.MergeReferenceInventories(result, platformHelperEnvironmentReferences(componentName, container.Env, container.EnvFrom))
			}
		}
		if found != 1 {
			result.Complete = false
		}
	}
	if componentName == "rbd-chaos" {
		daemons, err := client.AppsV1().DaemonSets(namespace).List(ctx, options)
		if err != nil || daemons == nil || daemons.ResourceVersion == "" || daemons.Continue != "" || len(daemons.Items) == 0 || len(daemons.Items) > 128 {
			return denied, ErrBinding
		}
		for _, item := range daemons.Items {
			add(item.Spec.Template.Spec)
		}
		revisions, err := client.AppsV1().ControllerRevisions(namespace).List(ctx, options)
		if err != nil || revisions == nil || revisions.ResourceVersion == "" || revisions.Continue != "" || len(revisions.Items) > 128 {
			return denied, ErrBinding
		}
		for _, item := range revisions.Items {
			var desired appsv1.DaemonSet
			if len(item.Data.Raw) == 0 || len(item.Data.Raw) > 1<<20 || json.Unmarshal(item.Data.Raw, &desired) != nil {
				return denied, ErrBinding
			}
			add(desired.Spec.Template.Spec)
		}
	}
	for _, item := range deployments.Items {
		add(item.Spec.Template.Spec)
	}
	for _, item := range replicas.Items {
		add(item.Spec.Template.Spec)
	}
	for _, item := range pods.Items {
		add(item.Spec)
	}
	return result, nil
}
func helperEnvironmentReferences(env []corev1.EnvVar, from []corev1.EnvFromSource) guard.RegionReferenceInventory {
	const name = "CLEANUP_NODE_EXECUTOR_IMAGE"
	complete := true
	values := []string{}
	for _, source := range from {
		if strings.HasPrefix(name, source.Prefix) {
			complete = false
		}
	}
	for _, entry := range env {
		if entry.Name != name {
			continue
		}
		if entry.ValueFrom != nil || strings.Contains(entry.Value, "$(") {
			complete = false
			continue
		}
		values = append(values, entry.Value)
	}
	result := guard.ConfiguredImageReferences(values)
	result.Complete = result.Complete && complete
	return result
}
