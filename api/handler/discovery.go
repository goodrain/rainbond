package handler

import (
	"strings"

	"github.com/goodrain/rainbond/pkg/component/k8s"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
)

// DiscoveryHandler defines handler methods for Kubernetes Discovery API operations
type DiscoveryHandler interface {
	GetClusterScopedResourceTypes() ([]ResourceType, error)
}

// NewDiscoveryHandler creates a new DiscoveryHandler
func NewDiscoveryHandler() DiscoveryHandler {
	k8sComp := k8s.Default()
	var client kubernetes.Interface
	if k8sComp.TestClientset != nil {
		client = k8sComp.TestClientset
	} else {
		client = k8sComp.Clientset
	}
	return &discoveryAction{
		k8sClient: client,
	}
}

// discoveryAction implements DiscoveryHandler
type discoveryAction struct {
	k8sClient kubernetes.Interface
}

// ResourceType represents a Kubernetes resource type
type ResourceType struct {
	Group      string   `json:"group"`
	Version    string   `json:"version"`
	Kind       string   `json:"kind"`
	Name       string   `json:"name"`
	Namespaced bool     `json:"namespaced"`
	Verbs      []string `json:"verbs"`
}

// GetClusterScopedResourceTypes queries Kubernetes Discovery API
// and returns all cluster-scoped resource types (excluding namespaced resources and subresources)
func (d *discoveryAction) GetClusterScopedResourceTypes() ([]ResourceType, error) {
	discoveryClient := d.k8sClient.Discovery()
	_, apiResourceLists, err := discoveryClient.ServerGroupsAndResources()
	if err != nil {
		// Partial errors can be ignored (permission issues)
		if !discovery.IsGroupDiscoveryFailedError(err) {
			return nil, err
		}
	}

	resourceTypes := make([]ResourceType, 0)
	for _, apiResourceList := range apiResourceLists {
		gv, err := schema.ParseGroupVersion(apiResourceList.GroupVersion)
		if err != nil {
			continue // Skip malformed GroupVersion
		}

		for _, apiResource := range apiResourceList.APIResources {
			// Filter: cluster-scoped + not subresource + supports list
			if !apiResource.Namespaced &&
				!strings.Contains(apiResource.Name, "/") &&
				contains(apiResource.Verbs, "list") {

				resourceTypes = append(resourceTypes, ResourceType{
					Group:      gv.Group,
					Version:    gv.Version,
					Kind:       apiResource.Kind,
					Name:       apiResource.Name,
					Namespaced: apiResource.Namespaced,
					Verbs:      apiResource.Verbs,
				})
			}
		}
	}
	return resourceTypes, nil
}

// contains checks if a string slice contains a specific item
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
