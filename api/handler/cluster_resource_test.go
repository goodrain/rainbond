package handler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

// getFakeDynamicClient creates a fake dynamic client for testing
func getFakeDynamicClient() dynamic.Interface {
	scheme := runtime.NewScheme()

	// Create a fake ClusterRole for testing
	clusterRole := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "rbac.authorization.k8s.io/v1",
			"kind":       "ClusterRole",
			"metadata": map[string]interface{}{
				"name": "admin",
			},
			"rules": []interface{}{},
		},
	}

	return dynamicfake.NewSimpleDynamicClient(scheme, clusterRole)
}

func TestListClusterResources(t *testing.T) {
	handler := &clusterResourceAction{
		dynamicClient: getFakeDynamicClient(),
	}

	// Test listing ClusterRole
	resources, err := handler.ListClusterResources("rbac.authorization.k8s.io", "v1", "clusterroles")
	assert.NoError(t, err)
	assert.NotNil(t, resources)
	assert.Equal(t, 1, len(resources.Items))
}

func TestGetClusterResource(t *testing.T) {
	handler := &clusterResourceAction{
		dynamicClient: getFakeDynamicClient(),
	}

	// Test getting single ClusterRole
	resource, err := handler.GetClusterResource("rbac.authorization.k8s.io", "v1", "clusterroles", "admin")
	assert.NoError(t, err)
	assert.NotNil(t, resource)
	assert.Equal(t, "admin", resource.GetName())
}
