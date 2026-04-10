package volume

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
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
