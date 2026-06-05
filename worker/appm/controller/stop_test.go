package controller

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestShouldDeleteManifestWithRuntimeClient(t *testing.T) {
	testCases := []struct {
		name     string
		manifest *unstructured.Unstructured
		want     bool
	}{
		{
			name: "nil manifest",
			want: false,
		},
		{
			name:     "skip kubevirt virtual machine",
			manifest: &unstructured.Unstructured{Object: map[string]interface{}{"kind": "VirtualMachine"}},
			want:     false,
		},
		{
			name:     "delete other custom resource",
			manifest: &unstructured.Unstructured{Object: map[string]interface{}{"kind": "VirtualService"}},
			want:     true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldDeleteManifestWithRuntimeClient(tc.manifest); got != tc.want {
				t.Fatalf("expected %v, got %v", tc.want, got)
			}
		})
	}
}
