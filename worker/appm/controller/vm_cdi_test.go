package controller

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	kubevirtv1 "kubevirt.io/api/core/v1"
	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// capability_id: rainbond.vm-import.cdi-insecure-registry
func TestEnsureVMRegistryImportInsecureRegistriesAppendsRBDHub(t *testing.T) {
	cdiConfig := newTestCDIConfig([]string{"existing.registry:5000"})
	runtimeClient := fake.NewClientBuilder().WithRuntimeObjects(cdiConfig).Build()
	registryURL := "docker://rbd-hub.rbd-system.svc:5000/ceshi:jjj"

	err := ensureVMRegistryImportInsecureRegistries(context.Background(), runtimeClient, []kubevirtv1.DataVolumeTemplateSpec{
		{
			Spec: cdiv1.DataVolumeSpec{
				Source: &cdiv1.DataVolumeSource{
					Registry: &cdiv1.DataVolumeSourceRegistry{URL: &registryURL},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("expected CDIConfig to be updated, got %v", err)
	}

	updated := emptyTestCDIConfig()
	if err := runtimeClient.Get(context.Background(), types.NamespacedName{Name: "config"}, updated); err != nil {
		t.Fatalf("expected updated CDIConfig, got %v", err)
	}
	registries, found, err := unstructured.NestedStringSlice(updated.Object, "spec", "insecureRegistries")
	if err != nil || !found {
		t.Fatalf("expected insecureRegistries, found=%v err=%v object=%#v", found, err, updated.Object)
	}
	assertStringSet(t, registries, []string{"existing.registry:5000", "rbd-hub.rbd-system.svc:5000"})
}

func TestEnsureVMRegistryImportInsecureRegistriesUpdatesCDIResourceConfig(t *testing.T) {
	cdi := newTestCDI([]string{"existing.registry:5000"})
	runtimeClient := fake.NewClientBuilder().WithRuntimeObjects(cdi).Build()
	registryURL := "docker://rbd-hub.rbd-system.svc:5000/ceshi:jjj"

	err := ensureVMRegistryImportInsecureRegistries(context.Background(), runtimeClient, []kubevirtv1.DataVolumeTemplateSpec{
		{
			Spec: cdiv1.DataVolumeSpec{
				Source: &cdiv1.DataVolumeSource{
					Registry: &cdiv1.DataVolumeSourceRegistry{URL: &registryURL},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("expected CDI resource to be updated, got %v", err)
	}

	updated := emptyTestCDI()
	if err := runtimeClient.Get(context.Background(), types.NamespacedName{Name: "cdi"}, updated); err != nil {
		t.Fatalf("expected updated CDI resource, got %v", err)
	}
	registries, found, err := unstructured.NestedStringSlice(updated.Object, "spec", "config", "insecureRegistries")
	if err != nil || !found {
		t.Fatalf("expected spec.config.insecureRegistries, found=%v err=%v object=%#v", found, err, updated.Object)
	}
	assertStringSet(t, registries, []string{"existing.registry:5000", "rbd-hub.rbd-system.svc:5000"})
}

func TestEnsureVMRegistryImportInsecureRegistriesIgnoresExternalRegistry(t *testing.T) {
	registryURL := "docker://registry.example.com/team/root:v1"

	if err := ensureVMRegistryImportInsecureRegistries(context.Background(), nil, []kubevirtv1.DataVolumeTemplateSpec{
		{
			Spec: cdiv1.DataVolumeSpec{
				Source: &cdiv1.DataVolumeSource{
					Registry: &cdiv1.DataVolumeSourceRegistry{URL: &registryURL},
				},
			},
		},
	}); err != nil {
		t.Fatalf("expected external registry to be ignored, got %v", err)
	}
}

func newTestCDIConfig(insecureRegistries []string) *unstructured.Unstructured {
	obj := emptyTestCDIConfig()
	if err := unstructured.SetNestedStringSlice(obj.Object, insecureRegistries, "spec", "insecureRegistries"); err != nil {
		panic(err)
	}
	return obj
}

func newTestCDI(insecureRegistries []string) *unstructured.Unstructured {
	obj := emptyTestCDI()
	if err := unstructured.SetNestedStringSlice(obj.Object, insecureRegistries, "spec", "config", "insecureRegistries"); err != nil {
		panic(err)
	}
	return obj
}

func emptyTestCDIConfig() *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("cdi.kubevirt.io/v1beta1")
	obj.SetKind("CDIConfig")
	obj.SetName("config")
	return obj
}

func emptyTestCDI() *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("cdi.kubevirt.io/v1beta1")
	obj.SetKind("CDI")
	obj.SetName("cdi")
	return obj
}

func assertStringSet(t *testing.T, got, want []string) {
	t.Helper()
	seen := make(map[string]bool, len(got))
	for _, value := range got {
		seen[value] = true
	}
	for _, value := range want {
		if !seen[value] {
			t.Fatalf("expected %q in %#v", value, got)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("expected only %#v, got %#v", want, got)
	}
}
