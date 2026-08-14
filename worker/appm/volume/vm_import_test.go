package volume

import (
	"testing"

	"github.com/goodrain/rainbond/builder"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

// capability_id: rainbond.vm-import.registry-reference-validation
func TestNormalizeVMRegistryImportURLRejectsInvalidReference(t *testing.T) {
	originalRegistryDomain := builder.REGISTRYDOMAIN
	builder.REGISTRYDOMAIN = "goodrain.me"
	defer func() {
		builder.REGISTRYDOMAIN = originalRegistryDomain
	}()

	tests := []struct {
		name     string
		imageURL string
		wantURL  string
		wantErr  bool
	}{
		{
			name:     "valid fully qualified registry image",
			imageURL: "docker://registry.example.com/team/windows-root:v1",
			wantURL:  "docker://registry.example.com/team/windows-root:v1",
		},
		{
			name:     "valid short internal registry image",
			imageURL: "ceshi:dbserver-tongyong",
			wantURL:  "docker://goodrain.me/ceshi:dbserver-tongyong",
		},
		{
			name:     "parentheses in image tag",
			imageURL: "ceshi:DBServer(TongYong)",
			wantErr:  true,
		},
		{
			name:     "non registry scheme",
			imageURL: "https://example.com/windows-root.qcow2",
			wantErr:  true,
		},
		{
			name:    "empty image reference",
			wantErr: true,
		},
		{
			name:     "parent traversal segment",
			imageURL: "ceshi/../dbserver:latest",
			wantErr:  true,
		},
		{
			name:     "current traversal segment",
			imageURL: "ceshi/./dbserver:latest",
			wantErr:  true,
		},
		{
			name:     "empty path segment",
			imageURL: "ceshi//dbserver:latest",
			wantErr:  true,
		},
		{
			name:     "leading slash",
			imageURL: "/ceshi:dbserver-tongyong",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotURL, err := normalizeVMRegistryImportURL(tt.imageURL)
			if (err != nil) != tt.wantErr {
				t.Fatalf("normalizeVMRegistryImportURL(%q) error = %v, wantErr %t", tt.imageURL, err, tt.wantErr)
			}
			if err == nil && gotURL != tt.wantURL {
				t.Fatalf("normalizeVMRegistryImportURL(%q) = %q, want %q", tt.imageURL, gotURL, tt.wantURL)
			}
		})
	}
}

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

	template, err := buildVMDiskImportDataVolumeTemplate(
		claim,
		map[string]string{"service_id": "svc-1"},
		map[string]string{"volume_name": "data-1"},
		vmDiskImportConfig{
			VolumeName: "data-1",
			ImageURL:   "https://download/data-1.qcow2",
		},
	)
	if err != nil {
		t.Fatalf("build import data volume template: %v", err)
	}

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

	template, err := buildVMDiskImportDataVolumeTemplate(
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
	if err != nil {
		t.Fatalf("build registry import data volume template: %v", err)
	}

	if template.Spec.Source == nil || template.Spec.Source.Registry == nil {
		t.Fatalf("expected registry import source, got %#v", template.Spec.Source)
	}
	if template.Spec.Source.Registry.URL == nil || *template.Spec.Source.Registry.URL != "docker://registry.example.com/team/windows-root:v1" {
		t.Fatalf("unexpected registry import url: %#v", template.Spec.Source.Registry.URL)
	}
	if template.Spec.Source.Registry.PullMethod == nil || *template.Spec.Source.Registry.PullMethod != cdiv1.RegistryPullNode {
		t.Fatalf("expected registry pull method node, got %#v", template.Spec.Source.Registry.PullMethod)
	}
	if template.Spec.Source.Registry.SecretRef != nil {
		t.Fatalf("did not expect registry import to use CDI secretRef, got %#v", template.Spec.Source.Registry.SecretRef)
	}
}

func TestBuildVMRegistryImportDataVolumeTemplateAddsDockerSchemeWhenMissing(t *testing.T) {
	storageClassName := "nfs-storage"
	claim := &corev1.PersistentVolumeClaim{}
	claim.Name = "manual-root"
	claim.Spec.StorageClassName = &storageClassName

	template, err := buildVMDiskImportDataVolumeTemplate(
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
	if err != nil {
		t.Fatalf("build registry import data volume template: %v", err)
	}

	if template.Spec.Source == nil || template.Spec.Source.Registry == nil || template.Spec.Source.Registry.URL == nil {
		t.Fatalf("expected registry import source, got %#v", template.Spec.Source)
	}
	if *template.Spec.Source.Registry.URL != "docker://registry.example.com/team/windows-root:v1" {
		t.Fatalf("expected docker scheme to be added, got %q", *template.Spec.Source.Registry.URL)
	}
}

func TestBuildVMRegistryImportDataVolumeTemplatePrefixesInternalRegistryForShortImage(t *testing.T) {
	origRegistryDomain := builder.REGISTRYDOMAIN
	builder.REGISTRYDOMAIN = "goodrain.me"
	defer func() {
		builder.REGISTRYDOMAIN = origRegistryDomain
	}()

	storageClassName := "nfs-storage"
	claim := &corev1.PersistentVolumeClaim{}
	claim.Name = "manual-root"
	claim.Spec.StorageClassName = &storageClassName

	template, err := buildVMDiskImportDataVolumeTemplate(
		claim,
		map[string]string{"service_id": "svc-vm"},
		map[string]string{"volume_name": "disk"},
		vmDiskImportConfig{
			VolumeName: "disk",
			ImageURL:   "ceshi:vava",
			SourceType: "registry",
			Format:     "qcow2",
		},
	)
	if err != nil {
		t.Fatalf("build registry import data volume template: %v", err)
	}

	if template.Spec.Source == nil || template.Spec.Source.Registry == nil || template.Spec.Source.Registry.URL == nil {
		t.Fatalf("expected registry import source, got %#v", template.Spec.Source)
	}
	if *template.Spec.Source.Registry.URL != "docker://goodrain.me/ceshi:vava" {
		t.Fatalf("expected short internal image to use default registry host, got %q", *template.Spec.Source.Registry.URL)
	}
	if template.Spec.Source.Registry.PullMethod == nil || *template.Spec.Source.Registry.PullMethod != cdiv1.RegistryPullNode {
		t.Fatalf("expected internal registry import to use node pull method, got %#v", template.Spec.Source.Registry.PullMethod)
	}
	if template.Spec.Source.Registry.SecretRef != nil {
		t.Fatalf("did not expect internal registry import to use CDI secretRef, got %#v", template.Spec.Source.Registry.SecretRef)
	}
}

func TestBuildVMArtifactImportDataVolumeTemplateUsesHTTPArtifactService(t *testing.T) {
	storageClassName := "nfs-storage"
	claim := &corev1.PersistentVolumeClaim{}
	claim.Name = "manual-root"
	claim.Spec.StorageClassName = &storageClassName

	template, err := buildVMDiskImportDataVolumeTemplate(
		claim,
		map[string]string{"service_id": "svc-vm"},
		map[string]string{"volume_name": "disk"},
		vmDiskImportConfig{
			VolumeName: "disk",
			ImageURL:   "goodrain.me/team/windows-root:v1",
			SourceType: "http-artifact",
			Format:     "raw.gz",
		},
	)
	if err != nil {
		t.Fatalf("build artifact import data volume template: %v", err)
	}

	if template.Spec.Source == nil || template.Spec.Source.HTTP == nil {
		t.Fatalf("expected http import source, got %#v", template.Spec.Source)
	}
	if template.Spec.Source.Registry != nil {
		t.Fatalf("did not expect registry import source for http artifact, got %#v", template.Spec.Source.Registry)
	}
	if template.Spec.Source.HTTP.URL != "http://vm-artifact-manual-root/disk.img.gz" {
		t.Fatalf("unexpected artifact import url: %q", template.Spec.Source.HTTP.URL)
	}
	if template.Annotations["rainbond.com/vm-artifact-image"] != "goodrain.me/team/windows-root:v1" {
		t.Fatalf("expected artifact image annotation, got %#v", template.Annotations)
	}
	if template.Annotations["rainbond.com/vm-artifact-service"] != "vm-artifact-manual-root" {
		t.Fatalf("expected artifact service annotation, got %#v", template.Annotations)
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

	volume, template, manual, err := buildVMVolumeSource(
		claim,
		map[string]string{"service_id": "svc-1"},
		map[string]string{"volume_name": "disk"},
		"/disk",
		nil,
	)
	if err != nil {
		t.Fatalf("build VM volume source: %v", err)
	}

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

	volume, template, manual, err := buildVMVolumeSource(
		claim,
		map[string]string{"service_id": "svc-1"},
		map[string]string{"volume_name": "data-1"},
		"/disk-1",
		nil,
	)
	if err != nil {
		t.Fatalf("build VM volume source: %v", err)
	}

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

	volume, template, manual, err := buildVMVolumeSource(
		claim,
		map[string]string{"service_id": "svc-1"},
		map[string]string{"volume_name": "cdrom"},
		"/cdrom",
		nil,
	)
	if err != nil {
		t.Fatalf("build VM volume source: %v", err)
	}

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
