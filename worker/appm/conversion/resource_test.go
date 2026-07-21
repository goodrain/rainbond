package conversion

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestCreateResourcesBySettingSkipsLegacyGPUResource(t *testing.T) {
	resources, vmResources := createResourcesBySetting(512, 500, 1000, 1, false)

	if vmResources != nil {
		t.Fatalf("expected non-VM resources, got VM resources %#v", vmResources)
	}
	if resources == nil {
		t.Fatal("expected resources")
	}
	if resources.Limits.Cpu().Cmp(resource.MustParse("1")) != 0 {
		t.Fatalf("expected cpu limit 1, got %s", resources.Limits.Cpu().String())
	}
	if resources.Requests.Cpu().Cmp(resource.MustParse("500m")) != 0 {
		t.Fatalf("expected cpu request 500m, got %s", resources.Requests.Cpu().String())
	}
	if resources.Limits.Memory().Cmp(resource.MustParse("512Mi")) != 0 {
		t.Fatalf("expected memory limit 512Mi, got %s", resources.Limits.Memory().String())
	}
	if _, ok := resources.Limits[corev1.ResourceName("rainbond.com/gpu-mem")]; ok {
		t.Fatal("did not expect legacy rainbond.com/gpu-mem limit")
	}
	if _, ok := resources.Requests[corev1.ResourceName("rainbond.com/gpu-mem")]; ok {
		t.Fatal("did not expect legacy rainbond.com/gpu-mem request")
	}
}
