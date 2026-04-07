package handler

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// capability_id: rainbond.cluster-resource.validate-gvr
func TestValidateGVRParams(t *testing.T) {
	tests := []struct {
		name     string
		version  string
		resource string
		wantErr  bool
	}{
		{"all filled", "v1", "namespaces", false},
		{"missing version", "", "storageclasses", true},
		{"missing resource", "v1", "", true},
		{"both empty", "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateGVRParams("", tt.version, tt.resource)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// capability_id: rainbond.cluster-resource.detect-subresource
func TestContainsSlash(t *testing.T) {
	assert.True(t, containsSlash("pods/log"))
	assert.False(t, containsSlash("pods"))
}

// capability_id: rainbond.cluster-resource.handler-singleton
func TestGetClusterResourceHandlerSingleton(t *testing.T) {
	h1 := GetClusterResourceHandler()
	h2 := GetClusterResourceHandler()
	assert.Equal(t, h1, h2)
}
