package handler

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"k8s.io/apimachinery/pkg/api/errors"
	k8smeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"

	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/pkg/component/k8s"
	httputil "github.com/goodrain/rainbond/util/http"
	corev1 "k8s.io/api/core/v1"
)

// NsResourceInfo is the list item response for a namespace-scoped resource
type NsResourceInfo struct {
	Name          string               `json:"name"`
	Kind          string               `json:"kind"`
	APIVersion    string               `json:"api_version"`
	Status        string               `json:"status"`
	Replicas      int64                `json:"replicas,omitempty"`
	ReadyReplicas int64                `json:"ready_replicas,omitempty"`
	Source        string               `json:"source"`
	CreatedAt     string               `json:"created_at"`
	Node          string               `json:"node,omitempty"`
	RestartCount  int32                `json:"restart_count,omitempty"`
	Owner         string               `json:"owner,omitempty"`
	PodIP         string               `json:"pod_ip,omitempty"`
	Type          string               `json:"type,omitempty"`
	ClusterIP     string               `json:"cluster_ip,omitempty"`
	Ports         []corev1.ServicePort `json:"ports,omitempty"`
	Selector      map[string]string    `json:"selector,omitempty"`
	DataCount     int                  `json:"data_count,omitempty"`
	Storage       string               `json:"storage,omitempty"`
	AccessModes   []string             `json:"access_modes,omitempty"`
	StorageClass  string               `json:"storage_class,omitempty"`
	VolumeName    string               `json:"volume_name,omitempty"`
}

// NsResourceCreateSummary is the aggregated create result summary.
type NsResourceCreateSummary struct {
	Total          int  `json:"total"`
	SuccessCount   int  `json:"success_count"`
	FailureCount   int  `json:"failure_count"`
	PartialSuccess bool `json:"partial_success"`
}

// NsResourceCreateResult is the per-document create result.
type NsResourceCreateResult struct {
	Index         int    `json:"index"`
	APIVersion    string `json:"api_version"`
	Kind          string `json:"kind"`
	Name          string `json:"name"`
	Namespace     string `json:"namespace"`
	ResourceScope string `json:"resource_scope"`
	Success       bool   `json:"success"`
	Message       string `json:"message"`
}

// NsResourceCreateResponse is the batch create response.
type NsResourceCreateResponse struct {
	Message string                   `json:"message"`
	Summary NsResourceCreateSummary  `json:"summary"`
	Results []NsResourceCreateResult `json:"results"`
}

// NsResourceHandler handles namespace-scoped K8s resource operations
type NsResourceHandler struct{}

var nsResourceRESTMapper = func() k8smeta.RESTMapper {
	if k8s.Default() == nil {
		return nil
	}
	return k8s.Default().Mapper
}

var nsResourceDynamicClient = func() dynamic.Interface {
	if k8s.Default() == nil {
		return nil
	}
	return k8s.Default().DynamicClient
}

// ListNsResourceTypes returns all namespace-scoped resource types from the cluster
func (h *NsResourceHandler) ListNsResourceTypes() ([]ResourceTypeInfo, error) {
	dc := k8s.Default().Clientset.Discovery()
	_, resList, err := dc.ServerGroupsAndResources()
	if err != nil && !discovery.IsGroupDiscoveryFailedError(err) {
		return nil, err
	}
	var types []ResourceTypeInfo
	for _, rl := range resList {
		gv, parseErr := schema.ParseGroupVersion(rl.GroupVersion)
		if parseErr != nil {
			continue
		}
		for _, r := range rl.APIResources {
			if r.Namespaced && !containsSlash(r.Name) {
				types = append(types, ResourceTypeInfo{
					Group:    gv.Group,
					Version:  gv.Version,
					Kind:     r.Kind,
					Resource: r.Name,
					Verbs:    r.Verbs,
				})
			}
		}
	}
	return types, nil
}

// ListNsResources returns all resources of the given GVR in the tenant namespace
func (h *NsResourceHandler) ListNsResources(tenantName, group, version, resource string) ([]NsResourceInfo, error) {
	if err := validateGVRParams(group, version, resource); err != nil {
		return nil, err
	}
	ns, err := h.getTenantNamespace(tenantName)
	if err != nil {
		return nil, err
	}
	gvr := schema.GroupVersionResource{Group: group, Version: version, Resource: resource}
	list, err := k8s.Default().DynamicClient.Resource(gvr).Namespace(ns).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	var result []NsResourceInfo
	for _, item := range list.Items {
		result = append(result, toNsResourceInfo(item))
	}
	return result, nil
}

// GetNsResource returns a single resource by name from the tenant namespace
func (h *NsResourceHandler) GetNsResource(tenantName, group, version, resource, name string) (*unstructured.Unstructured, error) {
	if err := validateGVRParams(group, version, resource); err != nil {
		return nil, err
	}
	ns, err := h.getTenantNamespace(tenantName)
	if err != nil {
		return nil, err
	}
	gvr := schema.GroupVersionResource{Group: group, Version: version, Resource: resource}
	return k8s.Default().DynamicClient.Resource(gvr).Namespace(ns).Get(context.Background(), name, metav1.GetOptions{})
}

// CreateNsResource creates resources from YAML body, resolving the actual target GVR from each document.
func (h *NsResourceHandler) CreateNsResource(tenantName, source string, yamlBody []byte) (*NsResourceCreateResponse, int, error) {
	if source != "yaml" && source != "manual" {
		source = "manual"
	}
	ns, err := h.getTenantNamespace(tenantName)
	if err != nil {
		return nil, 0, err
	}

	documents, err := decodeNsResourceDocuments(yamlBody)
	if err != nil {
		return nil, 0, err
	}

	mapper := nsResourceRESTMapper()
	if mapper == nil {
		return nil, 0, fmt.Errorf("kubernetes rest mapper is not initialized")
	}
	dynamicClient := nsResourceDynamicClient()
	if dynamicClient == nil {
		return nil, 0, fmt.Errorf("kubernetes dynamic client is not initialized")
	}

	response := &NsResourceCreateResponse{
		Results: make([]NsResourceCreateResult, 0, len(documents)),
	}

	for i, obj := range documents {
		result := NsResourceCreateResult{
			Index:      i + 1,
			APIVersion: obj.GetAPIVersion(),
			Kind:       obj.GetKind(),
			Name:       obj.GetName(),
		}
		h.createSingleNsResource(dynamicClient, mapper, ns, source, obj, &result)
		response.Results = append(response.Results, result)
	}

	response.Summary = buildNsResourceCreateSummary(response.Results)
	response.Message = buildNsResourceCreateMessage(response.Summary)
	return response, nsResourceCreateStatusCode(response.Summary), nil
}

// UpdateNsResource updates a resource in the tenant namespace from YAML body
func (h *NsResourceHandler) UpdateNsResource(tenantName, group, version, resource, name string, yamlBody []byte) (*unstructured.Unstructured, error) {
	if err := validateGVRParams(group, version, resource); err != nil {
		return nil, err
	}
	ns, err := h.getTenantNamespace(tenantName)
	if err != nil {
		return nil, err
	}
	obj := &unstructured.Unstructured{}
	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(yamlBody), 4096)
	if err := decoder.Decode(obj); err != nil {
		return nil, httputil.NewErrBadRequest(fmt.Errorf("invalid YAML: %v", err))
	}
	obj.SetNamespace(ns)
	if obj.GetName() == "" {
		obj.SetName(name)
	}
	gvr := schema.GroupVersionResource{Group: group, Version: version, Resource: resource}
	if obj.GetResourceVersion() == "" {
		current, err := k8s.Default().DynamicClient.Resource(gvr).Namespace(ns).Get(context.Background(), name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		obj.SetResourceVersion(current.GetResourceVersion())
	}
	result, err := k8s.Default().DynamicClient.Resource(gvr).Namespace(ns).Update(context.Background(), obj, metav1.UpdateOptions{})
	if err != nil && (errors.IsInvalid(err) || errors.IsBadRequest(err)) {
		return nil, httputil.NewErrBadRequest(err)
	}
	return result, err
}

// DeleteNsResource deletes a resource by name from the tenant namespace
func (h *NsResourceHandler) DeleteNsResource(tenantName, group, version, resource, name string) error {
	if err := validateGVRParams(group, version, resource); err != nil {
		return err
	}
	ns, err := h.getTenantNamespace(tenantName)
	if err != nil {
		return err
	}
	gvr := schema.GroupVersionResource{Group: group, Version: version, Resource: resource}
	err = k8s.Default().DynamicClient.Resource(gvr).Namespace(ns).Delete(context.Background(), name, metav1.DeleteOptions{})
	if errors.IsNotFound(err) {
		return nil
	}
	return err
}

func (h *NsResourceHandler) getTenantNamespace(tenantName string) (string, error) {
	tenant, err := db.GetManager().TenantDao().GetTenantIDByName(tenantName)
	if err != nil {
		return "", fmt.Errorf("tenant %s not found: %v", tenantName, err)
	}
	if tenant.Namespace != "" {
		return tenant.Namespace, nil
	}
	return tenant.UUID, nil
}

func injectSourceLabel(labels map[string]string, source string) {
	labels["rainbond.io/source"] = source
}

func decodeNsResourceDocuments(yamlBody []byte) ([]*unstructured.Unstructured, error) {
	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(yamlBody), 4096)
	documents := make([]*unstructured.Unstructured, 0)
	for {
		obj := &unstructured.Unstructured{}
		if err := decoder.Decode(obj); err != nil {
			if err == io.EOF {
				break
			}
			return nil, httputil.NewErrBadRequest(fmt.Errorf("invalid YAML: %v", err))
		}
		if len(obj.Object) == 0 {
			continue
		}
		documents = append(documents, obj)
	}
	if len(documents) == 0 {
		return nil, httputil.NewErrBadRequest(fmt.Errorf("invalid YAML: no resource documents found"))
	}
	return documents, nil
}

func (h *NsResourceHandler) createSingleNsResource(dynamicClient dynamic.Interface, mapper k8smeta.RESTMapper, teamNamespace, source string, obj *unstructured.Unstructured, result *NsResourceCreateResult) {
	mapping, err := resolveNsResourceMapping(mapper, obj)
	if err != nil {
		result.Message = err.Error()
		return
	}

	namespaceableClient := dynamicClient.Resource(mapping.Resource)
	resourceClient := dynamic.ResourceInterface(namespaceableClient)
	if mapping.Scope.Name() == k8smeta.RESTScopeNameNamespace {
		namespace := obj.GetNamespace()
		if namespace == "" {
			namespace = teamNamespace
		}
		obj.SetNamespace(namespace)
		result.Namespace = namespace
		result.ResourceScope = "namespaced"
		resourceClient = namespaceableClient.Namespace(namespace)
	} else {
		obj.SetNamespace("")
		result.Namespace = ""
		result.ResourceScope = "cluster"
	}

	labels := obj.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	injectSourceLabel(labels, source)
	obj.SetLabels(labels)

	created, err := resourceClient.Create(context.Background(), obj, metav1.CreateOptions{})
	if err != nil {
		result.Message = err.Error()
		return
	}
	result.Success = true
	result.Name = created.GetName()
	result.APIVersion = created.GetAPIVersion()
	result.Kind = created.GetKind()
	result.Namespace = created.GetNamespace()
	if result.ResourceScope == "" {
		if created.GetNamespace() == "" {
			result.ResourceScope = "cluster"
		} else {
			result.ResourceScope = "namespaced"
		}
	}
	result.Message = "created"
}

func resolveNsResourceMapping(mapper k8smeta.RESTMapper, obj *unstructured.Unstructured) (*k8smeta.RESTMapping, error) {
	if mapper == nil {
		return nil, fmt.Errorf("kubernetes rest mapper is not initialized")
	}
	gvk := obj.GroupVersionKind()
	if gvk.Kind == "" || gvk.Version == "" {
		return nil, fmt.Errorf("resource kind and apiVersion are required")
	}
	mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return nil, err
	}
	return mapping, nil
}

func buildNsResourceCreateSummary(results []NsResourceCreateResult) NsResourceCreateSummary {
	summary := NsResourceCreateSummary{Total: len(results)}
	for _, result := range results {
		if result.Success {
			summary.SuccessCount++
			continue
		}
		summary.FailureCount++
	}
	summary.PartialSuccess = summary.SuccessCount > 0 && summary.FailureCount > 0
	return summary
}

func buildNsResourceCreateMessage(summary NsResourceCreateSummary) string {
	switch {
	case summary.Total == 0:
		return "未解析到可创建的资源"
	case summary.FailureCount == 0:
		return fmt.Sprintf("共创建 %d 个资源，全部成功", summary.Total)
	case summary.SuccessCount == 0:
		return fmt.Sprintf("共创建 %d 个资源，全部失败", summary.Total)
	default:
		return fmt.Sprintf("共创建 %d 个资源，%d 个成功，%d 个失败", summary.Total, summary.SuccessCount, summary.FailureCount)
	}
}

func nsResourceCreateStatusCode(summary NsResourceCreateSummary) int {
	switch {
	case summary.Total == 0:
		return 400
	case summary.FailureCount == 0:
		return 200
	case summary.SuccessCount == 0:
		return 400
	default:
		return 207
	}
}

func detectResourceSource(labels map[string]string) string {
	if labels == nil {
		return "external"
	}
	if v, ok := labels["app.kubernetes.io/managed-by"]; ok && v == "Helm" {
		return "helm"
	}
	if v, ok := labels["rainbond.io/source"]; ok {
		return v
	}
	return "external"
}

func toNsResourceInfo(obj unstructured.Unstructured) NsResourceInfo {
	source := detectResourceSource(obj.GetLabels())
	info := NsResourceInfo{
		Name:       obj.GetName(),
		Kind:       obj.GetKind(),
		APIVersion: obj.GetAPIVersion(),
		Source:     source,
		CreatedAt:  obj.GetCreationTimestamp().String(),
		Status:     computeNsResourceStatus(obj),
	}
	fillNsResourceInfo(&info, obj)
	return info
}

func computeNsResourceStatus(obj unstructured.Unstructured) string {
	switch obj.GetKind() {
	case "Deployment", "StatefulSet":
		available, _, _ := unstructured.NestedInt64(obj.Object, "status", "availableReplicas")
		desired, _, _ := unstructured.NestedInt64(obj.Object, "spec", "replicas")
		if desired > 0 && available == desired {
			return "running"
		}
		return "warning"
	case "DaemonSet":
		ready, _, _ := unstructured.NestedInt64(obj.Object, "status", "numberReady")
		desired, _, _ := unstructured.NestedInt64(obj.Object, "status", "desiredNumberScheduled")
		if desired > 0 && ready == desired {
			return "running"
		}
		return "warning"
	case "Pod":
		phase, _, _ := unstructured.NestedString(obj.Object, "status", "phase")
		if phase == "" {
			return "unknown"
		}
		return phase
	case "PersistentVolumeClaim":
		phase, _, _ := unstructured.NestedString(obj.Object, "status", "phase")
		if phase == "Bound" {
			return "bound"
		}
		return phase
	default:
		return "active"
	}
}

func fillNsResourceInfo(info *NsResourceInfo, obj unstructured.Unstructured) {
	switch obj.GetKind() {
	case "Deployment", "StatefulSet":
		info.Replicas, _, _ = unstructured.NestedInt64(obj.Object, "spec", "replicas")
		info.ReadyReplicas, _, _ = unstructured.NestedInt64(obj.Object, "status", "readyReplicas")
	case "DaemonSet":
		info.Replicas, _, _ = unstructured.NestedInt64(obj.Object, "status", "desiredNumberScheduled")
		info.ReadyReplicas, _, _ = unstructured.NestedInt64(obj.Object, "status", "numberReady")
	case "Pod":
		info.Node, _, _ = unstructured.NestedString(obj.Object, "spec", "nodeName")
		info.PodIP, _, _ = unstructured.NestedString(obj.Object, "status", "podIP")
		for _, owner := range obj.GetOwnerReferences() {
			if owner.Controller != nil && *owner.Controller {
				info.Owner = owner.Name
				break
			}
		}
		statuses, _, _ := unstructured.NestedSlice(obj.Object, "status", "containerStatuses")
		var restartCount int64
		for _, item := range statuses {
			if statusMap, ok := item.(map[string]interface{}); ok {
				switch count := statusMap["restartCount"].(type) {
				case int64:
					restartCount += count
				case int32:
					restartCount += int64(count)
				case float64:
					restartCount += int64(count)
				}
			}
		}
		info.RestartCount = int32(restartCount)
	case "Service":
		info.Type, _, _ = unstructured.NestedString(obj.Object, "spec", "type")
		info.ClusterIP, _, _ = unstructured.NestedString(obj.Object, "spec", "clusterIP")
		info.Selector, _, _ = unstructured.NestedStringMap(obj.Object, "spec", "selector")
		var ports []corev1.ServicePort
		portList, _, _ := unstructured.NestedSlice(obj.Object, "spec", "ports")
		for _, item := range portList {
			if portMap, ok := item.(map[string]interface{}); ok {
				var port corev1.ServicePort
				switch p := portMap["port"].(type) {
				case int64:
					port.Port = int32(p)
				case int32:
					port.Port = p
				case float64:
					port.Port = int32(p)
				}
				if protocol, ok := portMap["protocol"].(string); ok {
					port.Protocol = corev1.Protocol(protocol)
				}
				if targetPort, ok := portMap["targetPort"].(string); ok {
					port.TargetPort = intstrFromString(targetPort)
				} else {
					switch targetPort := portMap["targetPort"].(type) {
					case int64:
						port.TargetPort = intstrFromInt(targetPort)
					case int32:
						port.TargetPort = intstr.FromInt(int(targetPort))
					case float64:
						port.TargetPort = intstr.FromInt(int(targetPort))
					}
				}
				ports = append(ports, port)
			}
		}
		info.Ports = ports
	case "ConfigMap", "Secret":
		data, _, _ := unstructured.NestedMap(obj.Object, "data")
		if len(data) == 0 {
			data, _, _ = unstructured.NestedMap(obj.Object, "stringData")
		}
		info.DataCount = len(data)
	case "PersistentVolumeClaim":
		info.Storage, _, _ = unstructured.NestedString(obj.Object, "spec", "resources", "requests", "storage")
		info.StorageClass, _, _ = unstructured.NestedString(obj.Object, "spec", "storageClassName")
		info.VolumeName, _, _ = unstructured.NestedString(obj.Object, "spec", "volumeName")
		accessModes, _, _ := unstructured.NestedStringSlice(obj.Object, "spec", "accessModes")
		info.AccessModes = accessModes
	}
}

func intstrFromString(value string) intstr.IntOrString {
	return intstr.FromString(value)
}

func intstrFromInt(value int64) intstr.IntOrString {
	return intstr.FromInt(int(value))
}

var nsResourceHandler *NsResourceHandler

// GetNsResourceHandler returns the singleton NsResourceHandler
func GetNsResourceHandler() *NsResourceHandler {
	if nsResourceHandler == nil {
		nsResourceHandler = &NsResourceHandler{}
	}
	return nsResourceHandler
}
