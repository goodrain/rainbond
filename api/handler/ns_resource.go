package handler

import (
	"bytes"
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/discovery"

	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/pkg/component/k8s"
)

// NsResourceInfo is the list item response for a namespace-scoped resource
type NsResourceInfo struct {
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	APIVersion string `json:"api_version"`
	Status     string `json:"status"`
	Replicas   string `json:"replicas,omitempty"`
	Source     string `json:"source"`
	CreatedAt  string `json:"created_at"`
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
	return NsResourceInfo{
		Name:       obj.GetName(),
		Kind:       obj.GetKind(),
		APIVersion: obj.GetAPIVersion(),
		Source:     source,
		CreatedAt:  obj.GetCreationTimestamp().String(),
		Status:     computeNsResourceStatus(obj),
	}
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

var nsResourceHandler *NsResourceHandler

// GetNsResourceHandler returns the singleton NsResourceHandler
func GetNsResourceHandler() *NsResourceHandler {
	if nsResourceHandler == nil {
		nsResourceHandler = &NsResourceHandler{}
	}
	return nsResourceHandler
}
