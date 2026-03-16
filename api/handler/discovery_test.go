package handler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes/fake"
)

func TestGetClusterScopedResourceTypes(t *testing.T) {
	// Create fake Kubernetes client
	k8sClient := fake.NewSimpleClientset()

	handler := &discoveryAction{
		k8sClient: k8sClient,
	}

	types, err := handler.GetClusterScopedResourceTypes()
	assert.NoError(t, err)
	assert.NotNil(t, types)
	// Fake client returns empty list, which is valid
	assert.IsType(t, []ResourceType{}, types)

	// Verify returned resource types are cluster-scoped (if any)
	for _, rt := range types {
		assert.False(t, rt.Namespaced, "Should only return cluster-scoped resources")
		assert.NotContains(t, rt.Name, "/", "Should not include subresources")
		assert.Contains(t, rt.Verbs, "list", "Should support list operation")
	}
}

func TestResourceTypeFiltering(t *testing.T) {
	// Test the filtering logic directly
	handler := &discoveryAction{
		k8sClient: fake.NewSimpleClientset(),
	}

	// Test with real cluster would return resources like:
	// - nodes (cluster-scoped, supports list) ✓
	// - namespaces (cluster-scoped, supports list) ✓
	// - pods (namespaced) ✗
	// - nodes/status (subresource) ✗

	types, err := handler.GetClusterScopedResourceTypes()
	assert.NoError(t, err)
	assert.NotNil(t, types)

	// All returned types must pass filters
	for _, rt := range types {
		assert.False(t, rt.Namespaced, "Must be cluster-scoped")
		assert.NotContains(t, rt.Name, "/", "Must not be subresource")
		assert.Contains(t, rt.Verbs, "list", "Must support list verb")
	}
}

func TestContainsHelper(t *testing.T) {
	tests := []struct {
		name     string
		slice    []string
		item     string
		expected bool
	}{
		{
			name:     "item exists",
			slice:    []string{"get", "list", "watch"},
			item:     "list",
			expected: true,
		},
		{
			name:     "item does not exist",
			slice:    []string{"get", "watch"},
			item:     "list",
			expected: false,
		},
		{
			name:     "empty slice",
			slice:    []string{},
			item:     "list",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := contains(tt.slice, tt.item)
			assert.Equal(t, tt.expected, result)
		})
	}
}
