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

	"github.com/goodrain/rainbond/pkg/component/k8s"
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

func (h *ClusterResourceHandler) GetResource(group, version, resource, name string) (*unstructured.Unstructured, error) {
	if err := validateGVRParams(group, version, resource); err != nil {
		return nil, err
	}
	gvr := schema.GroupVersionResource{Group: group, Version: version, Resource: resource}
	return k8s.Default().DynamicClient.Resource(gvr).Get(context.Background(), name, metav1.GetOptions{})
}

func (h *ClusterResourceHandler) CreateResource(group, version, resource string, yamlBody []byte) (*unstructured.Unstructured, error) {
	if err := validateGVRParams(group, version, resource); err != nil {
		return nil, err
	}
	obj := &unstructured.Unstructured{}
	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(yamlBody), 4096)
	if err := decoder.Decode(obj); err != nil {
		return nil, fmt.Errorf("invalid YAML: %v", err)
	}
	gvr := schema.GroupVersionResource{Group: group, Version: version, Resource: resource}
	return k8s.Default().DynamicClient.Resource(gvr).Create(context.Background(), obj, metav1.CreateOptions{})
}

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
