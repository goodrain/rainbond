package volume

import (
	"testing"

	"github.com/goodrain/rainbond/builder/sourceutil"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

func TestParseVMDiskImportConfigs(t *testing.T) {
	raw := `{"data-1":{"image_url":"https://download/data-1.qcow2","format":"qcow2"}}`

	configs, err := parseVMDiskImportConfigs(raw)
	if err != nil {
		t.Fatalf("expected imports to parse: %v", err)
	}

	cfg, ok := configs["data-1"]
	if !ok {
		t.Fatalf("expected data-1 import config")
	}
	if cfg.VolumeName != "data-1" {
		t.Fatalf("expected normalized volume name data-1, got %q", cfg.VolumeName)
	}
	if cfg.DiskKey != "data-1" {
		t.Fatalf("expected normalized disk key data-1, got %q", cfg.DiskKey)
	}
	if cfg.ImageURL != "https://download/data-1.qcow2" {
		t.Fatalf("unexpected image url: %q", cfg.ImageURL)
	}
}

func TestBuildVMDiskImportDataVolumeTemplate(t *testing.T) {
	storageClassName := "local-path"
	volumeMode := corev1.PersistentVolumeFilesystem
	claim := &corev1.PersistentVolumeClaim{
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			StorageClassName: &storageClassName,
			VolumeMode:       &volumeMode,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("10Gi"),
				},
			},
		},
	}
	claim.Name = "manual-1"

	template := buildVMDiskImportDataVolumeTemplate(
		claim,
		map[string]string{"service_id": "svc-1"},
		map[string]string{"volume_name": "data-1"},
		vmDiskImportConfig{
			VolumeName: "data-1",
			ImageURL:   "https://download/data-1.qcow2",
		},
	)

	if template.Name != "manual-1" {
		t.Fatalf("expected template name manual-1, got %q", template.Name)
	}
	if template.Spec.Source == nil || template.Spec.Source.HTTP == nil {
		t.Fatal("expected http import source")
	}
	if template.Spec.Source.HTTP.URL != "https://download/data-1.qcow2" {
		t.Fatalf("unexpected import url: %q", template.Spec.Source.HTTP.URL)
	}
	if template.Spec.Storage == nil || template.Spec.Storage.StorageClassName == nil {
		t.Fatal("expected storage spec with storage class")
	}
	if *template.Spec.Storage.StorageClassName != "local-path" {
		t.Fatalf("unexpected storage class: %q", *template.Spec.Storage.StorageClassName)
	}
}

func TestResolveVMExportHTTPImportConfigMap(t *testing.T) {
	restoreDynamicClient := sourceutil.SetVMExportDynamicClientProviderForTest(func() dynamic.Interface {
		gvr := schema.GroupVersionResource{
			Group:    "export.kubevirt.io",
			Version:  "v1beta1",
			Resource: "virtualmachineexports",
		}
		scheme := runtime.NewScheme()
		return dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
			scheme,
			map[schema.GroupVersionResource]string{gvr: "VirtualMachineExportList"},
			&unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "export.kubevirt.io/v1beta1",
					"kind":       "VirtualMachineExport",
					"metadata": map[string]interface{}{
						"name":      "virt-export-demo",
						"namespace": "default",
					},
					"spec": map[string]interface{}{
						"tokenSecretRef": "export-token",
					},
					"status": map[string]interface{}{
						"links": map[string]interface{}{
							"internal": map[string]interface{}{
								"cert": "-----BEGIN CERTIFICATE-----\nCERTDATA\n-----END CERTIFICATE-----",
								"volumes": []interface{}{
									map[string]interface{}{
										"name": "disk",
										"formats": []interface{}{
											map[string]interface{}{
												"format": "gzip",
												"url":    "https://virt-export-demo.default.svc/volumes/manual30/disk.img.gz",
											},
										},
									},
								},
							},
						},
					},
				},
			},
		)
	})
	defer restoreDynamicClient()
	restoreSecretGetter := sourceutil.SetVMExportSecretGetterForTest(func(namespace, name string) ([]byte, error) {
		if namespace != "default" || name != "export-token" {
			t.Fatalf("unexpected secret lookup: %s/%s", namespace, name)
		}
		return []byte("secret-token"), nil
	})
	defer restoreSecretGetter()

	configMap, extraHeaders, err := resolveVMExportHTTPImportConfigMap(
		"manual64",
		"https://virt-export-demo.default.svc/volumes/manual30/disk.img.gz",
	)
	if err != nil {
		t.Fatalf("expected vm export auth resolution to succeed: %v", err)
	}
	if configMap == nil || configMap.Name != "manual64-vmexport-ca" {
		t.Fatalf("expected cert configmap, got %#v", configMap)
	}
	if configMap.Data["ca.crt"] == "" {
		t.Fatalf("expected ca.crt data, got %#v", configMap.Data)
	}
	if len(extraHeaders) != 1 || extraHeaders[0] != "x-kubevirt-export-token:secret-token" {
		t.Fatalf("unexpected extra headers: %#v", extraHeaders)
	}
}

func TestBuildVMVolumeSourceUsesBlankDataVolumeForDisk(t *testing.T) {
	storageClassName := "local-path"
	volumeMode := corev1.PersistentVolumeFilesystem
	claim := &corev1.PersistentVolumeClaim{
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			StorageClassName: &storageClassName,
			VolumeMode:       &volumeMode,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("20Gi"),
				},
			},
		},
	}
	claim.Name = "manual-root"

	volume, template, manual := buildVMVolumeSource(
		claim,
		map[string]string{"service_id": "svc-1"},
		map[string]string{"volume_name": "disk"},
		"/disk",
		nil,
	)

	if manual {
		t.Fatal("expected vm root disk to avoid manual pvc provisioning")
	}
	if volume.DataVolume == nil || volume.DataVolume.Name != "manual-root" {
		t.Fatalf("expected data volume source for root disk, got %#v", volume.VolumeSource)
	}
	if template == nil || template.Spec.Source == nil || template.Spec.Source.Blank == nil {
		t.Fatalf("expected blank data volume template for root disk, got %#v", template)
	}
	if template.Spec.Storage == nil || template.Spec.Storage.StorageClassName == nil {
		t.Fatal("expected storage spec on blank data volume template")
	}
	if *template.Spec.Storage.StorageClassName != "local-path" {
		t.Fatalf("unexpected blank data volume storage class: %q", *template.Spec.Storage.StorageClassName)
	}
}

func TestBuildVMVolumeSourceKeepsCDRomAsPVCWithoutImport(t *testing.T) {
	storageClassName := "local-path"
	claim := &corev1.PersistentVolumeClaim{
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			StorageClassName: &storageClassName,
		},
	}
	claim.Name = "manual-cdrom"

	volume, template, manual := buildVMVolumeSource(
		claim,
		map[string]string{"service_id": "svc-1"},
		map[string]string{"volume_name": "cdrom"},
		"/cdrom",
		nil,
	)

	if !manual {
		t.Fatal("expected cdrom volume without import to keep manual pvc provisioning")
	}
	if volume.PersistentVolumeClaim == nil || volume.PersistentVolumeClaim.ClaimName != "manual-cdrom" {
		t.Fatalf("expected pvc-backed cdrom volume, got %#v", volume.VolumeSource)
	}
	if template != nil {
		t.Fatalf("expected no data volume template for pvc-backed cdrom, got %#v", template)
	}
}
