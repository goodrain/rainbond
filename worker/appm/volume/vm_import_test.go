package volume

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

// capability_id: rainbond.vm-import.registry-datavolume
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
	if cfg.SourceType != "http" {
		t.Fatalf("expected source type http, got %q", cfg.SourceType)
	}
}

func TestParseVMDiskImportConfigsInfersRegistrySourceType(t *testing.T) {
	raw := `{"disk":{"image_url":"docker://registry.example.com/team/windows-root:v1","format":"qcow2"}}`

	configs, err := parseVMDiskImportConfigs(raw)
	if err != nil {
		t.Fatalf("expected imports to parse: %v", err)
	}

	cfg, ok := configs["disk"]
	if !ok {
		t.Fatalf("expected disk import config")
	}
	if cfg.SourceType != "registry" {
		t.Fatalf("expected source type registry, got %q", cfg.SourceType)
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

func TestBuildVMRegistryImportDataVolumeTemplate(t *testing.T) {
	storageClassName := "nfs-storage"
	volumeMode := corev1.PersistentVolumeFilesystem
	claim := &corev1.PersistentVolumeClaim{
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany},
			StorageClassName: &storageClassName,
			VolumeMode:       &volumeMode,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("80Gi"),
				},
			},
		},
	}
	claim.Name = "manual-root"

	template := buildVMDiskImportDataVolumeTemplate(
		claim,
		map[string]string{"service_id": "svc-vm"},
		map[string]string{"volume_name": "disk"},
		vmDiskImportConfig{
			VolumeName: "disk",
			ImageURL:   "docker://registry.example.com/team/windows-root:v1",
			SourceType: "registry",
			Format:     "qcow2",
		},
	)

	if template.Spec.Source == nil || template.Spec.Source.Registry == nil {
		t.Fatalf("expected registry import source, got %#v", template.Spec.Source)
	}
	if template.Spec.Source.Registry.URL == nil || *template.Spec.Source.Registry.URL != "docker://registry.example.com/team/windows-root:v1" {
		t.Fatalf("unexpected registry import url: %#v", template.Spec.Source.Registry.URL)
	}
	if template.Spec.Source.Registry.PullMethod == nil || *template.Spec.Source.Registry.PullMethod != cdiv1.RegistryPullPod {
		t.Fatalf("expected registry pull method pod, got %#v", template.Spec.Source.Registry.PullMethod)
	}
}

func TestBuildVMRegistryImportDataVolumeTemplateAddsDockerSchemeWhenMissing(t *testing.T) {
	storageClassName := "nfs-storage"
	claim := &corev1.PersistentVolumeClaim{}
	claim.Name = "manual-root"
	claim.Spec.StorageClassName = &storageClassName

	template := buildVMDiskImportDataVolumeTemplate(
		claim,
		map[string]string{"service_id": "svc-vm"},
		map[string]string{"volume_name": "disk"},
		vmDiskImportConfig{
			VolumeName: "disk",
			ImageURL:   "registry.example.com/team/windows-root:v1",
			SourceType: "registry",
			Format:     "qcow2",
		},
	)

	if template.Spec.Source == nil || template.Spec.Source.Registry == nil || template.Spec.Source.Registry.URL == nil {
		t.Fatalf("expected registry import source, got %#v", template.Spec.Source)
	}
	if *template.Spec.Source.Registry.URL != "docker://registry.example.com/team/windows-root:v1" {
		t.Fatalf("expected docker scheme to be added, got %q", *template.Spec.Source.Registry.URL)
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

func TestBuildVMVolumeSourceUsesBlankDataVolumeForIndexedDiskPath(t *testing.T) {
	storageClassName := "nfs-storage"
	volumeMode := corev1.PersistentVolumeFilesystem
	claim := &corev1.PersistentVolumeClaim{
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany},
			StorageClassName: &storageClassName,
			VolumeMode:       &volumeMode,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("20Gi"),
				},
			},
		},
	}
	claim.Name = "manual-data-1"

	volume, template, manual := buildVMVolumeSource(
		claim,
		map[string]string{"service_id": "svc-1"},
		map[string]string{"volume_name": "data-1"},
		"/disk-1",
		nil,
	)

	if manual {
		t.Fatal("expected indexed vm disk path to use data volume template")
	}
	if volume.DataVolume == nil || volume.DataVolume.Name != "manual-data-1" {
		t.Fatalf("expected data volume source for indexed vm disk, got %#v", volume.VolumeSource)
	}
	if template == nil || template.Spec.Source == nil || template.Spec.Source.Blank == nil {
		t.Fatalf("expected blank data volume template for indexed vm disk, got %#v", template)
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
