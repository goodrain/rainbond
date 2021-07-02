package server

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	k8sutil "github.com/goodrain/rainbond/util/k8s"
	"github.com/goodrain/rainbond/worker/server/pb"
	"github.com/goodrain/rainbond/worker/util"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/reference"
)

var (
	// ErrAppServiceNotFound app service not found error, happens when haven't find any matched data when looking up with a service id
	ErrAppServiceNotFound = errors.New("app service not found")
	// ErrPodNotFound pod not found error, happens when haven't find any matched data when looking up with a pod name
	ErrPodNotFound = errors.New("pod not found")
)

// GetPodDetail returns detail information of the pod based on pod name.
func (r *RuntimeServer) GetPodDetail(ctx context.Context, req *pb.GetPodDetailReq) (podDetail *pb.PodDetail, err error) {
	pod, err := r.getPodByName(req.Sid, req.PodName)
	if err != nil {
		logrus.Errorf("name: %s; error getting pod: %v", req.PodName, err)
		return
	}
	if pod == nil {
		err = ErrPodNotFound
		return
	}

	// describe pod
	podDetail = &pb.PodDetail{}
	podDetail.Name = pod.Name
	podDetail.Version = func() string {
		labels := pod.GetLabels()
		if labels == nil {
			return ""
		}
		return labels["version"]
	}()
	if pod.Status.StartTime != nil {
		podDetail.StartTime = pod.Status.StartTime.Time.Format(time.RFC3339)
	}
	podDetail.InitContainers = make([]*pb.PodContainer, len(pod.Spec.InitContainers))
	podDetail.Containers = make([]*pb.PodContainer, len(pod.Spec.Containers))
	podDetail.Status = &pb.PodStatus{}
	podDetail.Events = []*pb.PodEvent{}

	if pod.Spec.NodeName != "" {
		podDetail.Node = pod.Spec.NodeName
	}
	if pod.Status.StartTime != nil {
		podDetail.StartTime = pod.Status.StartTime.Time.Format(time.RFC3339)
	}
	podDetail.NodeIp = pod.Status.HostIP
	podDetail.Ip = pod.Status.PodIP

	events := r.listPodEventsByPod(pod)
	if len(events) != 0 {
		podDetail.Events = append(podDetail.Events, events...)
	}

	if len(pod.Spec.InitContainers) != 0 {
		describeContainers(pod.Spec.InitContainers, pod.Status.InitContainerStatuses, &podDetail.InitContainers)
	}
	describeContainers(pod.Spec.Containers, pod.Status.ContainerStatuses, &podDetail.Containers)

	util.DescribePodStatus(r.clientset, pod, podDetail.Status, k8sutil.DefListEventsByPod)

	return podDetail, nil
}

func (r *RuntimeServer) getPodByName(namespace, name string) (*corev1.Pod, error) {
	return r.store.GetPod(namespace, name)
}

// GetPodEvents -
func (r *RuntimeServer) listPodEventsByPod(pod *corev1.Pod) []*pb.PodEvent {
	ref, err := reference.GetReference(scheme.Scheme, pod)
	if err != nil {
		logrus.Errorf("Unable to construct reference to '%#v': %v", pod, err)
		// custome event
		event := &pb.PodEvent{
			Type:    "Warning", // TODO: use k8s enum
			Reason:  fmt.Sprintf("error getting pod events."),
			Message: err.Error(),
		}
		return []*pb.PodEvent{event}
	}
	ref.Kind = ""
	if _, isMirrorPod := pod.Annotations[corev1.MirrorPodAnnotationKey]; isMirrorPod {
		ref.UID = types.UID(pod.Annotations[corev1.MirrorPodAnnotationKey])
	}
	events, _ := r.clientset.CoreV1().Events(pod.GetNamespace()).Search(scheme.Scheme, ref)
	podEvents := DescribeEvents(events)
	return podEvents
}

// GetPodEventsByName -
func (r RuntimeServer) listPodEventsByName(name, namespace string) []*pb.PodEvent {
	eventsInterface := r.clientset.CoreV1().Events(namespace)
	selector := eventsInterface.GetFieldSelector(&name, &namespace, nil, nil)
	options := metav1.ListOptions{FieldSelector: selector.String()}
	events, err := eventsInterface.List(context.Background(), options)
	if err == nil && len(events.Items) > 0 {
		podEvents := DescribeEvents(events)
		return podEvents
	}
	if err != nil {
		// custome event
		event := &pb.PodEvent{
			Type:    "Warning", // TODO: use k8s enum
			Reason:  fmt.Sprintf("error getting pod events."),
			Message: err.Error(),
		}
		return []*pb.PodEvent{event}
	}
	return nil
}

func describeContainers(containers []corev1.Container, containerStatuses []corev1.ContainerStatus, podContainers *[]*pb.PodContainer) {
	statuses := map[string]corev1.ContainerStatus{}
	for _, status := range containerStatuses {
		statuses[status.Name] = status
	}

	for idx, container := range containers {
		status, ok := statuses[container.Name]
		pc := &pb.PodContainer{
			Image: container.Image,
		}
		if ok {
			describeContainerState(status, pc)
		}
		describeContainerResource(container, pc)
		pcs := *podContainers
		pcs[idx] = pc
	}
}

func describeContainerState(status corev1.ContainerStatus, podContainer *pb.PodContainer) {
	describeStatus(status.State, podContainer)
	// TODO: LastTerminationState
	// TODO: Ready
	// TODO: Restart Count
}

func describeStatus(state corev1.ContainerState, podContainer *pb.PodContainer) {
	switch {
	case state.Running != nil:
		podContainer.State = "Running"
		podContainer.Started = state.Running.StartedAt.Time.Format(time.RFC3339)
	case state.Waiting != nil:
		podContainer.State = "Waiting"
		if state.Waiting.Reason != "" {
			podContainer.Reason = state.Waiting.Reason
		}
	case state.Terminated != nil:
		podContainer.State = "Terminated"
		if state.Terminated.Reason != "" {
			podContainer.Reason = state.Terminated.Reason
		}
	default:
		podContainer.State = "Waiting"
	}
}

func describeContainerResource(container corev1.Container, pc *pb.PodContainer) {
	resources := container.Resources
	for _, name := range SortedResourceNames(resources.Limits) {
		quantity := resources.Limits[name]
		if name == "memory" {
			pc.LimitMemory = quantity.String()
		}
		if name == "cpu" {
			pc.LimitCpu = quantity.String()
		}
	}
	for _, name := range SortedResourceNames(resources.Requests) {
		quantity := resources.Requests[name]
		if name == "memory" {
			pc.RequestMemory = quantity.String()
		}
		if name == "cpu" {
			pc.RequestCpu = quantity.String()
		}
	}
}

// SortableResourceNames -
type SortableResourceNames []corev1.ResourceName

func (list SortableResourceNames) Len() int {
	return len(list)
}

func (list SortableResourceNames) Swap(i, j int) {
	list[i], list[j] = list[j], list[i]
}

func (list SortableResourceNames) Less(i, j int) bool {
	return list[i] < list[j]
}

// SortedResourceNames returns the sorted resource names of a resource list.
func SortedResourceNames(list corev1.ResourceList) []corev1.ResourceName {
	resources := make([]corev1.ResourceName, 0, len(list))
	for res := range list {
		resources = append(resources, res)
	}
	sort.Sort(SortableResourceNames(resources))
	return resources
}
