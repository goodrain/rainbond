package handler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

func TestBuildVMCapabilities(t *testing.T) {
	scheme := runtime.NewScheme()
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		networkAttachmentDefinitionGVR:       "NetworkAttachmentDefinitionList",
		networkAttachmentDefinitionCompatGVR: "NetworkAttachmentDefinitionList",
		kubeVirtGVR:                          "KubeVirtList",
	},
		&unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "k8s.cni.cncf.io/v1",
				"kind":       "NetworkAttachmentDefinition",
				"metadata": map[string]interface{}{
					"name":      "bridge-test",
					"namespace": "tenant-a",
				},
			},
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
	assert.Equal(t, []string{"random", "fixed"}, capabilities.NetworkModes)
	assert.True(t, capabilities.GPUSupported)
	assert.True(t, capabilities.USBSupported)
	assert.Len(t, capabilities.Networks, 1)
	assert.Equal(t, VMNetworkCapability{
		Name:      "bridge-test",
		Namespace: "tenant-a",
	}, capabilities.Networks[0])
	assert.Equal(t, []string{"gpu.example.com/A10", "nvidia.com/T4"}, capabilities.GPUResources)
	assert.Equal(t, []string{"kubevirt.io/usb-a"}, capabilities.USBResources)
}

func TestBuildVMCapabilitiesWithoutOptionalResources(t *testing.T) {
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{
		networkAttachmentDefinitionGVR:       "NetworkAttachmentDefinitionList",
		networkAttachmentDefinitionCompatGVR: "NetworkAttachmentDefinitionList",
		kubeVirtGVR:                          "KubeVirtList",
	})
	capabilities, err := BuildVMCapabilities(client)
	assert.NoError(t, err)
	assert.Equal(t, []string{"random"}, capabilities.NetworkModes)
	assert.False(t, capabilities.GPUSupported)
	assert.False(t, capabilities.USBSupported)
	assert.Empty(t, capabilities.Networks)
	assert.Empty(t, capabilities.GPUResources)
	assert.Empty(t, capabilities.USBResources)
}

func TestBuildVMCapabilitiesSortsAndDeduplicates(t *testing.T) {
	scheme := runtime.NewScheme()
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		networkAttachmentDefinitionGVR:       "NetworkAttachmentDefinitionList",
		networkAttachmentDefinitionCompatGVR: "NetworkAttachmentDefinitionList",
		kubeVirtGVR:                          "KubeVirtList",
	},
		&unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "k8s.cni.cncf.io/v1",
				"kind":       "NetworkAttachmentDefinition",
				"metadata": map[string]interface{}{
					"name":      "b-net",
					"namespace": metav1.NamespaceDefault,
				},
			},
		},
		&unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "k8s.cni.cncf.io/v1",
				"kind":       "NetworkAttachmentDefinition",
				"metadata": map[string]interface{}{
					"name":      "a-net",
					"namespace": metav1.NamespaceDefault,
				},
			},
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
	assert.Equal(t, []VMNetworkCapability{
		{Name: "a-net", Namespace: metav1.NamespaceDefault},
		{Name: "b-net", Namespace: metav1.NamespaceDefault},
	}, capabilities.Networks)
	assert.Equal(t, []string{"nvidia.com/T4"}, capabilities.GPUResources)
	assert.Equal(t, []string{"kubevirt.io/usb-a", "kubevirt.io/usb-b"}, capabilities.USBResources)
}
