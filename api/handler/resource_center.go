package handler

import (
	"context"
	"sort"

	"github.com/goodrain/rainbond/pkg/component/k8s"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type WorkloadDetailSummary struct {
	Name          string            `json:"name"`
	Kind          string            `json:"kind"`
	Namespace     string            `json:"namespace"`
	Status        string            `json:"status"`
	Replicas      int64             `json:"replicas,omitempty"`
	ReadyReplicas int64             `json:"ready_replicas,omitempty"`
	CreatedAt     string            `json:"created_at"`
	Selector      map[string]string `json:"selector,omitempty"`
}

type WorkloadDetail struct {
	Summary   WorkloadDetailSummary      `json:"summary"`
	Workload  *unstructured.Unstructured `json:"workload"`
	Pods      []corev1.Pod               `json:"pods"`
	Services  []corev1.Service           `json:"services"`
	Ingresses []networkingv1.Ingress     `json:"ingresses"`
}

type PodContainerInfo struct {
	Name         string `json:"name"`
	Image        string `json:"image,omitempty"`
	Ready        bool   `json:"ready"`
	RestartCount int32  `json:"restart_count,omitempty"`
}

type PodDetailSummary struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Phase     string `json:"phase,omitempty"`
	NodeName  string `json:"node_name,omitempty"`
	PodIP     string `json:"pod_ip,omitempty"`
	CreatedAt string `json:"created_at"`
}

type PodResourceDetail struct {
	Summary    PodDetailSummary       `json:"summary"`
	Pod        *corev1.Pod            `json:"pod"`
	Detail     interface{}            `json:"detail,omitempty"`
	Containers []PodContainerInfo     `json:"containers"`
	Services   []corev1.Service       `json:"services"`
	Ingresses  []networkingv1.Ingress `json:"ingresses"`
}

type ResourceEventInfo struct {
	Type          string `json:"type"`
	Reason        string `json:"reason"`
	Message       string `json:"message"`
	Count         int32  `json:"count"`
	LastTimestamp string `json:"last_timestamp"`
}

type ResourceCenterHandler struct{}

func (h *ResourceCenterHandler) GetWorkloadDetail(tenantName, group, version, resource, name string) (*WorkloadDetail, error) {
	if err := validateGVRParams(group, version, resource); err != nil {
		return nil, err
	}
	ns, err := GetNsResourceHandler().getTenantNamespace(tenantName)
	if err != nil {
		return nil, err
	}
	gvr := schema.GroupVersionResource{Group: group, Version: version, Resource: resource}
	workload, err := k8s.Default().DynamicClient.Resource(gvr).Namespace(ns).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	selector := extractWorkloadSelector(workload)
	pods, err := h.listWorkloadPods(ns, selector)
	if err != nil {
		return nil, err
	}
	services, err := h.listRelatedServices(ns, selector)
	if err != nil {
		return nil, err
	}
	ingresses, err := h.listRelatedIngresses(ns, services)
	if err != nil {
		return nil, err
	}

	summary := WorkloadDetailSummary{
		Name:          workload.GetName(),
		Kind:          workload.GetKind(),
		Namespace:     ns,
		Status:        computeNsResourceStatus(*workload),
		CreatedAt:     workload.GetCreationTimestamp().String(),
		Selector:      selector,
		Replicas:      extractWorkloadReplicas(workload),
		ReadyReplicas: extractWorkloadReadyReplicas(workload),
	}

	return &WorkloadDetail{
		Summary:   summary,
		Workload:  workload,
		Pods:      pods,
		Services:  services,
		Ingresses: ingresses,
	}, nil
}

func (h *ResourceCenterHandler) GetPodDetail(tenantName, podName string) (*PodResourceDetail, error) {
	ns, err := GetNsResourceHandler().getTenantNamespace(tenantName)
	if err != nil {
		return nil, err
	}
	pod, err := k8s.Default().Clientset.CoreV1().Pods(ns).Get(context.Background(), podName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	detail, err := GetPodHandler().PodDetail(ns, podName)
	if err != nil {
		return nil, err
	}
	services, err := h.listRelatedServices(ns, pod.Labels)
	if err != nil {
		return nil, err
	}
	ingresses, err := h.listRelatedIngresses(ns, services)
	if err != nil {
		return nil, err
	}

	containers := make([]PodContainerInfo, 0, len(pod.Spec.Containers))
	statusByName := make(map[string]corev1.ContainerStatus, len(pod.Status.ContainerStatuses))
	for _, status := range pod.Status.ContainerStatuses {
		statusByName[status.Name] = status
	}
	for _, container := range pod.Spec.Containers {
		info := PodContainerInfo{
			Name:  container.Name,
			Image: container.Image,
		}
		if status, ok := statusByName[container.Name]; ok {
			info.Ready = status.Ready
			info.RestartCount = status.RestartCount
		}
		containers = append(containers, info)
	}

	return &PodResourceDetail{
		Summary: PodDetailSummary{
			Name:      pod.Name,
			Namespace: pod.Namespace,
			Phase:     string(pod.Status.Phase),
			NodeName:  pod.Spec.NodeName,
			PodIP:     pod.Status.PodIP,
			CreatedAt: pod.CreationTimestamp.String(),
		},
		Pod:        pod,
		Detail:     detail,
		Containers: containers,
		Services:   services,
		Ingresses:  ingresses,
	}, nil
}

func (h *ResourceCenterHandler) ListEvents(tenantName, namespace, kind, name string) ([]ResourceEventInfo, error) {
	ns := namespace
	if ns == "" {
		var err error
		ns, err = GetNsResourceHandler().getTenantNamespace(tenantName)
		if err != nil {
			return nil, err
		}
	}
	selector := fields.Set{
		"involvedObject.kind": kind,
		"involvedObject.name": name,
	}.String()
	list, err := k8s.Default().Clientset.CoreV1().Events(ns).List(context.Background(), metav1.ListOptions{
		FieldSelector: selector,
	})
	if err != nil {
		return nil, err
	}
	items := make([]ResourceEventInfo, 0, len(list.Items))
	for _, event := range list.Items {
		items = append(items, toResourceEventInfo(event))
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].LastTimestamp > items[j].LastTimestamp
	})
	return items, nil
}

func (h *ResourceCenterHandler) listWorkloadPods(namespace string, selector map[string]string) ([]corev1.Pod, error) {
	if len(selector) == 0 {
		return []corev1.Pod{}, nil
	}
	list, err := k8s.Default().Clientset.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: labels.Set(selector).String(),
	})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

func (h *ResourceCenterHandler) listRelatedServices(namespace string, selector map[string]string) ([]corev1.Service, error) {
	list, err := k8s.Default().Clientset.CoreV1().Services(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	var matched []corev1.Service
	for _, svc := range list.Items {
		if labelsMatchSelector(svc.Spec.Selector, selector) {
			matched = append(matched, svc)
		}
	}
	return matched, nil
}

func (h *ResourceCenterHandler) listRelatedIngresses(namespace string, services []corev1.Service) ([]networkingv1.Ingress, error) {
	if len(services) == 0 {
		return []networkingv1.Ingress{}, nil
	}
	serviceNames := make(map[string]struct{}, len(services))
	for _, svc := range services {
		serviceNames[svc.Name] = struct{}{}
	}
	list, err := k8s.Default().Clientset.NetworkingV1().Ingresses(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	var matched []networkingv1.Ingress
	for _, ing := range list.Items {
		for _, serviceName := range collectIngressServiceNames(ing) {
			if _, ok := serviceNames[serviceName]; ok {
				matched = append(matched, ing)
				break
			}
		}
	}
	return matched, nil
}

func extractWorkloadSelector(workload *unstructured.Unstructured) map[string]string {
	if selector, ok, _ := unstructured.NestedStringMap(workload.Object, "spec", "selector", "matchLabels"); ok && len(selector) > 0 {
		return selector
	}
	if selector, ok, _ := unstructured.NestedStringMap(workload.Object, "spec", "jobTemplate", "spec", "template", "metadata", "labels"); ok && len(selector) > 0 {
		return selector
	}
	if selector, ok, _ := unstructured.NestedStringMap(workload.Object, "spec", "template", "metadata", "labels"); ok && len(selector) > 0 {
		return selector
	}
	return nil
}

func extractWorkloadReplicas(workload *unstructured.Unstructured) int64 {
	switch workload.GetKind() {
	case "DaemonSet":
		value, _, _ := unstructured.NestedInt64(workload.Object, "status", "desiredNumberScheduled")
		return value
	case "CronJob":
		active, _, _ := unstructured.NestedSlice(workload.Object, "status", "active")
		return int64(len(active))
	default:
		value, _, _ := unstructured.NestedInt64(workload.Object, "spec", "replicas")
		return value
	}
}

func extractWorkloadReadyReplicas(workload *unstructured.Unstructured) int64 {
	switch workload.GetKind() {
	case "DaemonSet":
		value, _, _ := unstructured.NestedInt64(workload.Object, "status", "numberReady")
		return value
	case "CronJob":
		active, _, _ := unstructured.NestedSlice(workload.Object, "status", "active")
		return int64(len(active))
	default:
		value, _, _ := unstructured.NestedInt64(workload.Object, "status", "readyReplicas")
		return value
	}
}

func labelsMatchSelector(selector map[string]string, resourceLabels map[string]string) bool {
	if len(selector) == 0 || len(resourceLabels) == 0 {
		return false
	}
	for key, value := range selector {
		if resourceLabels[key] != value {
			return false
		}
	}
	return true
}

func collectIngressServiceNames(ingress networkingv1.Ingress) []string {
	names := make([]string, 0, 4)
	if ingress.Spec.DefaultBackend != nil && ingress.Spec.DefaultBackend.Service != nil {
		names = append(names, ingress.Spec.DefaultBackend.Service.Name)
	}
	for _, rule := range ingress.Spec.Rules {
		if rule.HTTP == nil {
			continue
		}
		for _, path := range rule.HTTP.Paths {
			if path.Backend.Service != nil {
				names = append(names, path.Backend.Service.Name)
			}
		}
	}
	return uniqueStrings(names)
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func toResourceEventInfo(event corev1.Event) ResourceEventInfo {
	lastTimestamp := event.LastTimestamp.String()
	if lastTimestamp == "" || lastTimestamp == "<nil>" {
		lastTimestamp = event.EventTime.String()
	}
	if lastTimestamp == "" || lastTimestamp == "<nil>" {
		lastTimestamp = event.FirstTimestamp.String()
	}
	return ResourceEventInfo{
		Type:          event.Type,
		Reason:        event.Reason,
		Message:       event.Message,
		Count:         event.Count,
		LastTimestamp: lastTimestamp,
	}
}

var resourceCenterHandler *ResourceCenterHandler

func GetResourceCenterHandler() *ResourceCenterHandler {
	if resourceCenterHandler == nil {
		resourceCenterHandler = &ResourceCenterHandler{}
	}
	return resourceCenterHandler
}
