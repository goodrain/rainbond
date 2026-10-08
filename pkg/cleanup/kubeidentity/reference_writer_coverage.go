package kubeidentity

import (
	"context"
	"reflect"
	"sort"

	guard "github.com/goodrain/rainbond/pkg/cleanup"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
)

// InspectReferenceWriterCoverage observes all four platform writer roles and
// rejects incomplete rollouts. It returns runtime identities, not registration
// evidence or permission to delete; callers must also verify durable proofs.
func InspectReferenceWriterCoverage(ctx context.Context, client kubernetes.Interface, namespace string) ([]guard.ReferenceWriter, error) {
	if client == nil || namespace == "" {
		return nil, ErrBinding
	}
	roles := map[string]string{"rbd-api": "api", "rbd-worker": "worker", "rbd-chaos": "builder", "rbd-app-ui": "console"}
	list := func() (*corev1.PodList, error) {
		pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: "name in (rbd-api,rbd-worker,rbd-chaos,rbd-app-ui)", Limit: 257})
		if err != nil || pods == nil || pods.Continue != "" || len(pods.Items) < 4 || len(pods.Items) > 256 {
			return nil, ErrBinding
		}
		sort.Slice(pods.Items, func(i, j int) bool { return pods.Items[i].Name < pods.Items[j].Name })
		return pods, nil
	}
	pods, err := list()
	if err != nil {
		return nil, err
	}
	counts := map[string]int32{}
	desired := map[string]int32{}
	checks := []func() bool{}
	writers := []guard.ReferenceWriter{}
	for i := range pods.Items {
		pod := &pods.Items[i]
		name := pod.Labels["name"]
		role := roles[name]
		if role == "" || len(pod.OwnerReferences) != 1 {
			return nil, ErrBinding
		}
		owner := pod.OwnerReferences[0]
		if owner.APIVersion != "apps/v1" || owner.Controller == nil || !*owner.Controller || owner.UID == "" {
			return nil, ErrBinding
		}
		var template corev1.PodTemplateSpec
		var selector *metav1.LabelSelector
		if name == "rbd-chaos" {
			if owner.Kind != "DaemonSet" || owner.Name != name {
				return nil, ErrBinding
			}
			d, e := client.AppsV1().DaemonSets(namespace).Get(ctx, name, metav1.GetOptions{})
			if e != nil || d.UID != owner.UID || d.ResourceVersion == "" || d.DeletionTimestamp != nil || d.Generation < 1 || d.Status.ObservedGeneration != d.Generation || d.Status.DesiredNumberScheduled < 1 || d.Status.CurrentNumberScheduled != d.Status.DesiredNumberScheduled || d.Status.UpdatedNumberScheduled != d.Status.DesiredNumberScheduled || d.Status.NumberReady != d.Status.DesiredNumberScheduled || d.Status.NumberAvailable != d.Status.DesiredNumberScheduled || d.Status.NumberUnavailable != 0 || d.Status.NumberMisscheduled != 0 {
				return nil, ErrBinding
			}
			template, selector = d.Spec.Template, d.Spec.Selector
			desired[name] = d.Status.DesiredNumberScheduled
			checks = append(checks, func() bool {
				current, e := client.AppsV1().DaemonSets(namespace).Get(ctx, d.Name, metav1.GetOptions{})
				return e == nil && current.UID == d.UID && current.ResourceVersion == d.ResourceVersion
			})
		} else {
			if owner.Kind != "ReplicaSet" {
				return nil, ErrBinding
			}
			rs, e := client.AppsV1().ReplicaSets(namespace).Get(ctx, owner.Name, metav1.GetOptions{})
			if e != nil || rs.UID != owner.UID || rs.ResourceVersion == "" || rs.DeletionTimestamp != nil || len(rs.OwnerReferences) != 1 {
				return nil, ErrBinding
			}
			parent := rs.OwnerReferences[0]
			if parent.APIVersion != "apps/v1" || parent.Kind != "Deployment" || parent.Name != name || parent.Controller == nil || !*parent.Controller || parent.UID == "" {
				return nil, ErrBinding
			}
			d, e := client.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
			if e != nil || d.UID != parent.UID || d.ResourceVersion == "" || d.DeletionTimestamp != nil || d.Generation < 1 || d.Status.ObservedGeneration != d.Generation || d.Spec.Replicas == nil || *d.Spec.Replicas < 1 || d.Status.Replicas != *d.Spec.Replicas || d.Status.UpdatedReplicas != *d.Spec.Replicas || d.Status.ReadyReplicas != *d.Spec.Replicas || d.Status.AvailableReplicas != *d.Spec.Replicas || d.Status.UnavailableReplicas != 0 {
				return nil, ErrBinding
			}
			if !reflect.DeepEqual(rs.Spec.Template.Spec.Containers, d.Spec.Template.Spec.Containers) {
				return nil, ErrBinding
			}
			template, selector = d.Spec.Template, d.Spec.Selector
			desired[name] = *d.Spec.Replicas
			checks = append(checks, func() bool {
				current, e := client.AppsV1().Deployments(namespace).Get(ctx, d.Name, metav1.GetOptions{})
				return e == nil && current.UID == d.UID && current.ResourceVersion == d.ResourceVersion
			}, func() bool {
				current, e := client.AppsV1().ReplicaSets(namespace).Get(ctx, rs.Name, metav1.GetOptions{})
				return e == nil && current.UID == rs.UID && current.ResourceVersion == rs.ResourceVersion
			})
		}
		if selector == nil || template.Labels["name"] != name {
			return nil, ErrBinding
		}
		selected, e := metav1.LabelSelectorAsSelector(selector)
		if e != nil || !selected.Matches(labels.Set(pod.Labels)) {
			return nil, ErrBinding
		}
		main := func(containers []corev1.Container) *corev1.Container {
			var result *corev1.Container
			for j := range containers {
				if containers[j].Name == name {
					if result != nil {
						return nil
					}
					result = &containers[j]
				}
			}
			return result
		}
		actual, wanted := main(pod.Spec.Containers), main(template.Spec.Containers)
		if actual == nil || wanted == nil || actual.Image != wanted.Image || !reflect.DeepEqual(actual.Command, wanted.Command) || !reflect.DeepEqual(actual.Args, wanted.Args) || !reflect.DeepEqual(actual.Env, wanted.Env) || !reflect.DeepEqual(actual.EnvFrom, wanted.EnvFrom) {
			return nil, ErrBinding
		}
		ready := false
		for _, s := range pod.Status.ContainerStatuses {
			if s.Name == name {
				ready = s.Ready
			}
		}
		if !ready {
			return nil, ErrBinding
		}
		writer, e := InspectReferenceWriter(ctx, client, namespace, pod.Name, string(pod.UID), role)
		if e != nil {
			return nil, e
		}
		writers = append(writers, writer)
		counts[name]++
	}
	for name := range roles {
		if desired[name] < 1 || counts[name] != desired[name] {
			return nil, ErrBinding
		}
	}
	current, err := list()
	if err != nil || len(current.Items) != len(pods.Items) {
		return nil, ErrBinding
	}
	for i := range pods.Items {
		if pods.Items[i].UID != current.Items[i].UID || pods.Items[i].ResourceVersion != current.Items[i].ResourceVersion {
			return nil, ErrBinding
		}
	}
	for _, check := range checks {
		if !check() {
			return nil, ErrBinding
		}
	}
	return writers, nil
}
