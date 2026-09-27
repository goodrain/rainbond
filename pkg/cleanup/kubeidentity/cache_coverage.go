package kubeidentity

import (
	"context"
	"reflect"
	"sort"

	coordination "github.com/goodrain/rainbond/pkg/cleanup"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
)

// InspectCacheWriterCoverage verifies the live system writer set, its owning
// controller rollout and absence of detached builds. Database startup membership
// is checked separately in the storage transaction; this does not enable writes.
func InspectCacheWriterCoverage(ctx context.Context, client kubernetes.Interface, namespace string, binding coordination.StorageRegistration) ([]coordination.ParticipantRegistration, error) {
	if client == nil || namespace == "" {
		return nil, ErrBinding
	}
	list := func() (*corev1.PodList, error) {
		pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: "name=rbd-chaos", Limit: 65})
		if err != nil || pods.ResourceVersion == "" || pods.Continue != "" || len(pods.Items) == 0 || len(pods.Items) > 64 {
			return nil, ErrBinding
		}
		sort.Slice(pods.Items, func(i, j int) bool { return pods.Items[i].Name < pods.Items[j].Name })
		return pods, nil
	}
	pods, err := list()
	if err != nil {
		return nil, err
	}
	controllers := map[string]*appsv1.DaemonSet{}
	counts := map[string]int32{}
	members := []coordination.ParticipantRegistration{}
	for i := range pods.Items {
		pod := &pods.Items[i]
		if validateDisabledCleaner(pod) != nil || len(pod.OwnerReferences) != 1 {
			return nil, ErrBinding
		}
		owner := pod.OwnerReferences[0]
		if owner.APIVersion != "apps/v1" || owner.Kind != "DaemonSet" || owner.Controller == nil || !*owner.Controller || owner.UID == "" {
			return nil, ErrBinding
		}
		daemon := controllers[owner.Name]
		if daemon == nil {
			daemon, err = client.AppsV1().DaemonSets(namespace).Get(ctx, owner.Name, metav1.GetOptions{})
			if err != nil || daemon.UID != owner.UID || daemon.ResourceVersion == "" || daemon.DeletionTimestamp != nil || daemon.Generation == 0 || daemon.Status.ObservedGeneration != daemon.Generation || daemon.Status.DesiredNumberScheduled == 0 || daemon.Status.CurrentNumberScheduled != daemon.Status.DesiredNumberScheduled || daemon.Status.UpdatedNumberScheduled != daemon.Status.DesiredNumberScheduled || daemon.Status.NumberReady != daemon.Status.DesiredNumberScheduled || daemon.Status.NumberAvailable != daemon.Status.DesiredNumberScheduled || daemon.Status.NumberUnavailable != 0 || daemon.Status.NumberMisscheduled != 0 || len(daemon.Spec.Template.Spec.Containers) != 1 {
				return nil, ErrBinding
			}
			controllers[owner.Name] = daemon
		}
		if daemon.UID != owner.UID {
			return nil, ErrBinding
		}
		if daemon.Spec.Selector == nil || daemon.Spec.Template.Labels["name"] != "rbd-chaos" {
			return nil, ErrBinding
		}
		selector, err := metav1.LabelSelectorAsSelector(daemon.Spec.Selector)
		if err != nil || !selector.Matches(labels.Set(pod.Labels)) || !selector.Matches(labels.Set(daemon.Spec.Template.Labels)) {
			return nil, ErrBinding
		}
		actual, desired := pod.Spec.Containers[0], daemon.Spec.Template.Spec.Containers[0]
		if actual.Image != desired.Image || !reflect.DeepEqual(actual.Command, desired.Command) || !reflect.DeepEqual(actual.Args, desired.Args) {
			return nil, ErrBinding
		}
		am, _, err := effectiveMount(&actual, "/cache/build")
		if err != nil {
			return nil, ErrBinding
		}
		dm, _, err := effectiveMount(&desired, "/cache/build")
		if err != nil || !reflect.DeepEqual(am, dm) {
			return nil, ErrBinding
		}
		volume := func(spec corev1.PodSpec, name string) *corev1.Volume {
			for i := range spec.Volumes {
				if spec.Volumes[i].Name == name {
					return &spec.Volumes[i]
				}
			}
			return nil
		}
		if !reflect.DeepEqual(volume(pod.Spec, am.Name), volume(daemon.Spec.Template.Spec, dm.Name)) {
			return nil, ErrBinding
		}
		counts[owner.Name]++
		observed, err := InspectManagedBuildCache(ctx, client, namespace, pod.Name, string(pod.UID))
		if err != nil {
			return nil, err
		}
		if observed.Mount.VolumeUID == binding.VolumeUID {
			member, err := InspectCacheBuilder(ctx, client, namespace, pod.Name, string(pod.UID), binding)
			if err != nil {
				return nil, err
			}
			members = append(members, member)
		}
	}
	if len(members) == 0 {
		return nil, ErrBinding
	}
	for name, daemon := range controllers {
		if counts[name] != daemon.Status.DesiredNumberScheduled {
			return nil, ErrBinding
		}
	}
	builds, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: "job=codebuild", FieldSelector: "status.phase!=Succeeded,status.phase!=Failed", Limit: 2})
	if err != nil || builds.ResourceVersion == "" || builds.Continue != "" {
		return nil, ErrBinding
	}
	for _, pod := range builds.Items {
		if pod.Status.Phase != corev1.PodSucceeded && pod.Status.Phase != corev1.PodFailed {
			return nil, coordination.ErrCoordinationBusy
		}
	}
	current, err := list()
	if err != nil || len(current.Items) != len(pods.Items) {
		return nil, ErrBinding
	}
	for i := range current.Items {
		if current.Items[i].UID != pods.Items[i].UID || current.Items[i].ResourceVersion != pods.Items[i].ResourceVersion {
			return nil, ErrBinding
		}
	}
	for name, daemon := range controllers {
		current, err := client.AppsV1().DaemonSets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil || current.UID != daemon.UID || current.ResourceVersion != daemon.ResourceVersion {
			return nil, ErrBinding
		}
	}
	return members, nil
}
