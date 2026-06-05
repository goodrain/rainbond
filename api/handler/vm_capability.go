package handler

import (
	"context"
	"sort"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

var (
	kubeVirtGVR = schema.GroupVersionResource{
		Group:    "kubevirt.io",
		Version:  "v1",
		Resource: "kubevirts",
	}
)

type VMCapability struct {
	ChunkUploadSupported bool     `json:"chunk_upload_supported"`
	GPUSupported         bool     `json:"gpu_supported"`
	USBSupported         bool     `json:"usb_supported"`
	GPUResources         []string `json:"gpu_resources"`
	USBResources         []string `json:"usb_resources"`
}

func BuildVMCapabilities(dynamicClient dynamic.Interface) (*VMCapability, error) {
	capabilities := &VMCapability{
		ChunkUploadSupported: true,
	}
	if dynamicClient == nil {
		return capabilities, nil
	}

	gpuResources, usbResources, err := listPermittedHostDeviceResources(dynamicClient)
	if err != nil {
		return nil, err
	}
	capabilities.GPUResources = gpuResources
	capabilities.USBResources = usbResources
	capabilities.GPUSupported = len(gpuResources) > 0
	capabilities.USBSupported = len(usbResources) > 0

	return capabilities, nil
}

func listPermittedHostDeviceResources(dynamicClient dynamic.Interface) ([]string, []string, error) {
	list, err := dynamicClient.Resource(kubeVirtGVR).Namespace(metav1.NamespaceAll).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, nil, nil
		}
		return nil, nil, err
	}

	var gpuResources []string
	var usbResources []string
	for _, item := range list.Items {
		gpuResources = append(gpuResources, extractResourceNames(item.Object, "spec", "configuration", "permittedHostDevices", "pciHostDevices")...)
		gpuResources = append(gpuResources, extractResourceNames(item.Object, "spec", "configuration", "permittedHostDevices", "mediatedDevices")...)
		usbResources = append(usbResources, extractResourceNames(item.Object, "spec", "configuration", "permittedHostDevices", "usb")...)
	}

	return uniqueSortedStrings(gpuResources), uniqueSortedStrings(usbResources), nil
}

func extractResourceNames(obj map[string]interface{}, fields ...string) []string {
	values, found, err := unstructured.NestedSlice(obj, fields...)
	if err != nil || !found {
		return nil
	}
	var resources []string
	for _, value := range values {
		device, ok := value.(map[string]interface{})
		if !ok {
			continue
		}
		resourceName, _, err := unstructured.NestedString(device, "resourceName")
		if err != nil || resourceName == "" {
			continue
		}
		resources = append(resources, resourceName)
	}
	return resources
}

func uniqueSortedStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	uniq := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		uniq[value] = struct{}{}
	}
	result := make([]string, 0, len(uniq))
	for value := range uniq {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
