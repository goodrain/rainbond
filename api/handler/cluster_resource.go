package handler

import (
	"context"

	"github.com/goodrain/rainbond/pkg/component/k8s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// ClusterResourceHandler defines handler methods for cluster-scoped Kubernetes resource operations
type ClusterResourceHandler interface {
	ListClusterResources(group, version, resource string) (*unstructured.UnstructuredList, error)
	GetClusterResource(group, version, resource, name string) (*unstructured.Unstructured, error)
}

// NewClusterResourceHandler creates a new ClusterResourceHandler
func NewClusterResourceHandler() ClusterResourceHandler {
	return &clusterResourceAction{
		dynamicClient: k8s.Default().DynamicClient,
	}
}

// clusterResourceAction implements ClusterResourceHandler
type clusterResourceAction struct {
	dynamicClient dynamic.Interface
}

// ListClusterResources lists all resources of a specific cluster-scoped type
func (h *clusterResourceAction) ListClusterResources(group, version, resource string) (*unstructured.UnstructuredList, error) {
	gvr := schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: resource,
	}

	return h.dynamicClient.Resource(gvr).List(context.Background(), metav1.ListOptions{})
}

// GetClusterResource gets a specific cluster-scoped resource by name
func (h *clusterResourceAction) GetClusterResource(group, version, resource, name string) (*unstructured.Unstructured, error) {
	gvr := schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: resource,
	}

	return h.dynamicClient.Resource(gvr).Get(context.Background(), name, metav1.GetOptions{})
}
