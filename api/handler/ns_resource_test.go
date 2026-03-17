package handler

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetNsResourceHandlerSingleton(t *testing.T) {
	h1 := GetNsResourceHandler()
	h2 := GetNsResourceHandler()
	assert.NotNil(t, h1)
	assert.Equal(t, h1, h2)
}

func TestInjectSourceLabelYaml(t *testing.T) {
	labels := map[string]string{}
	injectSourceLabel(labels, "yaml")
	assert.Equal(t, "yaml", labels["rainbond.io/source"])
}

func TestInjectSourceLabelManual(t *testing.T) {
	labels := map[string]string{}
	injectSourceLabel(labels, "manual")
	assert.Equal(t, "manual", labels["rainbond.io/source"])
}

func TestDetectResourceSource(t *testing.T) {
	tests := []struct {
		labels   map[string]string
		expected string
	}{
		{map[string]string{"app.kubernetes.io/managed-by": "Helm"}, "helm"},
		{map[string]string{"rainbond.io/source": "yaml"}, "yaml"},
		{map[string]string{"rainbond.io/source": "manual"}, "manual"},
		{map[string]string{}, "external"},
		{nil, "external"},
	}
	for _, tt := range tests {
		result := detectResourceSource(tt.labels)
		assert.Equal(t, tt.expected, result)
	}
}
