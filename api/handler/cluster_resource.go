package handler

import (
	"bytes"
	"context"
	"fmt"

	"github.com/goodrain/rainbond/db"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"

	"github.com/goodrain/rainbond/pkg/component/k8s"
	httputil "github.com/goodrain/rainbond/util/http"
)

// ResourceTypeInfo describes a K8s resource type
type ResourceTypeInfo struct {
	Group    string   `json:"group"`
	Version  string   `json:"version"`
	Kind     string   `json:"kind"`
	Resource string   `json:"resource"`
	Verbs    []string `json:"verbs"`
}

// ClusterResourceHandler handles cluster-scoped K8s resource operations
type ClusterResourceHandler struct{}

var clusterResourceDynamicClient = func() dynamic.Interface { return k8s.Default().DynamicClient }

// ListResourceTypes lists discoverable cluster resource types.
func (h *ClusterResourceHandler) ListResourceTypes() ([]ResourceTypeInfo, error) {
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
			if !r.Namespaced && !containsSlash(r.Name) {
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

// ListResources lists objects for a validated cluster resource type.
func (h *ClusterResourceHandler) ListResources(group, version, resource string) ([]unstructured.Unstructured, error) {
	if err := validateGVRParams(group, version, resource); err != nil {
		return nil, err
	}
	gvr := schema.GroupVersionResource{Group: group, Version: version, Resource: resource}
	list, err := k8s.Default().DynamicClient.Resource(gvr).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

// GetResource reads one cluster-scoped object.
func (h *ClusterResourceHandler) GetResource(group, version, resource, name string) (*unstructured.Unstructured, error) {
	if err := validateGVRParams(group, version, resource); err != nil {
		return nil, err
	}
	gvr := schema.GroupVersionResource{Group: group, Version: version, Resource: resource}
	return k8s.Default().DynamicClient.Resource(gvr).Get(context.Background(), name, metav1.GetOptions{})
}

// CreateResource coordinates a cluster-scoped creation with cleanup.
func (h *ClusterResourceHandler) CreateResource(group, version, resource string, yamlBody []byte) (out *unstructured.Unstructured, resultErr error) {
	if err := validateGVRParams(group, version, resource); err != nil {
		return nil, err
	}
	obj := &unstructured.Unstructured{}
	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(yamlBody), 4096)
	if err := decoder.Decode(obj); err != nil {
		return nil, httputil.NewErrBadRequest(fmt.Errorf("invalid YAML: %v", err))
	}
	dynamicClient := clusterResourceDynamicClient()
	if dynamicClient == nil {
		return nil, fmt.Errorf("kubernetes dynamic client is not initialized")
	}
	gvr := schema.GroupVersionResource{Group: group, Version: version, Resource: resource}
	admission, err := admitWorkload(db.GetManager().DB(), yamlBody)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := admission.finish(true); err != nil && resultErr == nil {
			resultErr = err
		}
	}()
	admission.observe(true, nil)
	result, err := dynamicClient.Resource(gvr).Create(context.Background(), obj, metav1.CreateOptions{})
	admission.observe(false, err)
	if err != nil && (errors.IsInvalid(err) || errors.IsBadRequest(err)) {
		return nil, httputil.NewErrBadRequest(err)
	}
	return result, err
}

// UpdateResource coordinates a cluster-scoped update with cleanup.
func (h *ClusterResourceHandler) UpdateResource(group, version, resource, name string, yamlBody []byte) (out *unstructured.Unstructured, resultErr error) {
	if err := validateGVRParams(group, version, resource); err != nil {
		return nil, err
	}
	dynamicClient := clusterResourceDynamicClient()
	if dynamicClient == nil {
		return nil, fmt.Errorf("kubernetes dynamic client is not initialized")
	}
	gvr := schema.GroupVersionResource{Group: group, Version: version, Resource: resource}
	obj := &unstructured.Unstructured{}
	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(yamlBody), 4096)
	if err := decoder.Decode(obj); err != nil {
		return nil, httputil.NewErrBadRequest(fmt.Errorf("invalid YAML: %v", err))
	}
	if obj.GetResourceVersion() == "" {
		current, err := dynamicClient.Resource(gvr).Get(context.Background(), name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		obj.SetResourceVersion(current.GetResourceVersion())
	}
	admission, err := admitWorkload(db.GetManager().DB(), yamlBody)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := admission.finish(true); err != nil && resultErr == nil {
			resultErr = err
		}
	}()
	admission.observe(true, nil)
	result, err := dynamicClient.Resource(gvr).Update(context.Background(), obj, metav1.UpdateOptions{})
	admission.observe(false, err)
	if err != nil && (errors.IsInvalid(err) || errors.IsBadRequest(err)) {
		return nil, httputil.NewErrBadRequest(err)
	}
	return result, err
}

// DeleteResource removes one cluster-scoped object.
func (h *ClusterResourceHandler) DeleteResource(group, version, resource, name string) error {
	if err := validateGVRParams(group, version, resource); err != nil {
		return err
	}
	gvr := schema.GroupVersionResource{Group: group, Version: version, Resource: resource}
	err := k8s.Default().DynamicClient.Resource(gvr).Delete(context.Background(), name, metav1.DeleteOptions{})
	if errors.IsNotFound(err) {
		return nil
	}
	return err
}

func validateGVRParams(group, version, resource string) error {
	if version == "" {
		return fmt.Errorf("version is required")
	}
	if resource == "" {
		return fmt.Errorf("resource is required")
	}
	return nil
}

func containsSlash(s string) bool {
	for _, c := range s {
		if c == '/' {
			return true
		}
	}
	return false
}

var clusterResourceHandler *ClusterResourceHandler

// GetClusterResourceHandler returns the singleton ClusterResourceHandler
func GetClusterResourceHandler() *ClusterResourceHandler {
	if clusterResourceHandler == nil {
		clusterResourceHandler = &ClusterResourceHandler{}
	}
	return clusterResourceHandler
}
