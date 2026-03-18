package handler

import (
	"bytes"
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/discovery"

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

// NsResourceHandler handles namespace-scoped K8s resource operations
type NsResourceHandler struct{}

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

// CreateNsResource creates a resource in the tenant namespace from YAML body, injecting source label
func (h *NsResourceHandler) CreateNsResource(tenantName, group, version, resource, source string, yamlBody []byte) (*unstructured.Unstructured, error) {
	if err := validateGVRParams(group, version, resource); err != nil {
		return nil, err
	}
	if source != "yaml" && source != "manual" {
		source = "manual"
	}
	ns, err := h.getTenantNamespace(tenantName)
	if err != nil {
		return nil, err
	}
	obj := &unstructured.Unstructured{}
	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(yamlBody), 4096)
	if err := decoder.Decode(obj); err != nil {
		return nil, fmt.Errorf("invalid YAML: %v", err)
	}
	labels := obj.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	injectSourceLabel(labels, source)
	obj.SetLabels(labels)
	obj.SetNamespace(ns)
	gvr := schema.GroupVersionResource{Group: group, Version: version, Resource: resource}
	return k8s.Default().DynamicClient.Resource(gvr).Namespace(ns).Create(context.Background(), obj, metav1.CreateOptions{})
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
