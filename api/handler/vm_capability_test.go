package handler

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

// capability_id: rainbond.vm-capabilities.password-bootstrap-env-gate
func TestBuildVMCapabilitiesPasswordBootstrapEnvGate(t *testing.T) {
	tests := []struct {
		name  string
		value string
		unset bool
		want  bool
	}{
		{name: "unset", unset: true, want: false},
		{name: "empty", value: "", want: false},
		{name: "false", value: "false", want: false},
		{name: "numeric one", value: "1", want: false},
		{name: "uppercase", value: "TRUE", want: false},
		{name: "exact true", value: "true", want: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.unset {
				t.Setenv("VM_PASSWORD_BOOTSTRAP_ENABLED", "true")
				if err := os.Unsetenv("VM_PASSWORD_BOOTSTRAP_ENABLED"); err != nil {
					t.Fatal("unset capability environment: ", err)
				}
			} else {
				t.Setenv("VM_PASSWORD_BOOTSTRAP_ENABLED", test.value)
			}

			got, err := BuildVMCapabilities(nil)
			if err != nil {
				t.Fatal("build capabilities failed")
			}
			if got.PasswordBootstrapSupported != test.want {
				t.Fatalf("password capability gate mismatch: got %t, want %t", got.PasswordBootstrapSupported, test.want)
			}
		})
	}
}

func TestBuildVMCapabilities(t *testing.T) {
	scheme := runtime.NewScheme()
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		kubeVirtGVR: "KubeVirtList",
	},
		&unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "kubevirt.io/v1",
				"kind":       "KubeVirt",
				"metadata": map[string]interface{}{
					"name":      "kubevirt",
					"namespace": "kubevirt",
				},
				"spec": map[string]interface{}{
					"configuration": map[string]interface{}{
						"permittedHostDevices": map[string]interface{}{
							"pciHostDevices": []interface{}{
								map[string]interface{}{"resourceName": "nvidia.com/T4"},
							},
							"mediatedDevices": []interface{}{
								map[string]interface{}{"resourceName": "gpu.example.com/A10"},
							},
							"usb": []interface{}{
								map[string]interface{}{"resourceName": "kubevirt.io/usb-a"},
							},
						},
					},
				},
			},
		},
	)

	capabilities, err := BuildVMCapabilities(client)
	assert.NoError(t, err)
	assert.True(t, capabilities.ChunkUploadSupported)
	assert.True(t, capabilities.GPUSupported)
	assert.True(t, capabilities.USBSupported)
	assert.Equal(t, []string{"gpu.example.com/A10", "nvidia.com/T4"}, capabilities.GPUResources)
	assert.Equal(t, []string{"kubevirt.io/usb-a"}, capabilities.USBResources)
}

func TestBuildVMCapabilitiesWithoutOptionalResources(t *testing.T) {
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{
		kubeVirtGVR: "KubeVirtList",
	})
	capabilities, err := BuildVMCapabilities(client)
	assert.NoError(t, err)
	assert.False(t, capabilities.GPUSupported)
	assert.False(t, capabilities.USBSupported)
	assert.Empty(t, capabilities.GPUResources)
	assert.Empty(t, capabilities.USBResources)
}

func TestBuildVMCapabilitiesDeduplicatesDeviceResources(t *testing.T) {
	scheme := runtime.NewScheme()
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		kubeVirtGVR: "KubeVirtList",
	},
		&unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "kubevirt.io/v1",
				"kind":       "KubeVirt",
				"metadata": map[string]interface{}{
					"name":      "kubevirt",
					"namespace": "kubevirt",
				},
				"spec": map[string]interface{}{
					"configuration": map[string]interface{}{
						"permittedHostDevices": map[string]interface{}{
							"pciHostDevices": []interface{}{
								map[string]interface{}{"resourceName": "nvidia.com/T4"},
								map[string]interface{}{"resourceName": "nvidia.com/T4"},
							},
							"usb": []interface{}{
								map[string]interface{}{"resourceName": "kubevirt.io/usb-b"},
								map[string]interface{}{"resourceName": "kubevirt.io/usb-a"},
							},
						},
					},
				},
			},
		},
	)

	capabilities, err := BuildVMCapabilities(client)
	assert.NoError(t, err)
	assert.Equal(t, []string{"nvidia.com/T4"}, capabilities.GPUResources)
	assert.Equal(t, []string{"kubevirt.io/usb-a", "kubevirt.io/usb-b"}, capabilities.USBResources)
}
