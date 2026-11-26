package volume

import (
	"errors"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

// capability_id: rainbond.vm-import.vmdk-validated-registry
func TestVMDKDiskImportRequiresPublishedRegistryArtifact(t *testing.T) {
	for _, tt := range []struct {
		name, source, format, image string
	}{
		{"unspecified source", "", " VMDK ", "https://images.example.invalid/source.vmdk"},
		{"direct HTTP", "http", "vmdk", "https://images.example.invalid/source.vmdk"},
		{"HTTP artifact", "http-artifact", "vmdk", "registry.example.invalid/team/source:v1"},
		{"HTTP URL declared registry", "registry", "vmdk", "https://images.example.invalid/source.vmdk"},
		{"wrapped registry VMDK gzip", "registry", "vmdk.gz", "registry.example.invalid/team/source:v1"},
		{"wrapped registry VMDK xz", "registry", "vmdk.xz", "registry.example.invalid/team/source:v1"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			claim := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "manual-root"}}
			_, template, manual, err := buildVMVolumeSource(claim,
				map[string]string{"service_id": "service-a"}, map[string]string{"volume_name": "disk"}, "/disk",
				&vmDiskImportConfig{VolumeName: "disk", Format: tt.format, SourceType: tt.source, ImageURL: tt.image})
			if !errors.Is(err, ErrInvalidVMRegistryImport) {
				t.Fatalf("unpublished or invalid VMDK must abort VM creation, got %v", err)
			}
			if template != nil || manual {
				t.Fatal("rejected input must not create an import or fall back to a blank/manual disk")
			}
		})
	}
}

func TestVMDKHTTPImportCannotBypassPublication(t *testing.T) {
	for _, input := range []string{
		"https://images.example.invalid/disk.VMDK?download=disk.qcow2",
		"https://images.example.invalid/disk%2Evmdk",
		"https://images.example.invalid/disk.vmdk.gz",
		"https://images.example.invalid/disk.vmdk.xz",
	} {
		claim := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "manual-root"}}
		_, err := buildVMDiskImportDataVolumeTemplate(claim, nil, nil, vmDiskImportConfig{
			VolumeName: "disk", ImageURL: input, Format: "qcow2", SourceType: "http"})
		if !errors.Is(err, ErrInvalidVMRegistryImport) {
			t.Fatalf("original VMDK HTTP source must be rejected independently of the declared runtime format: %s: %v", input, err)
		}
		if !strings.Contains(err.Error(), "磁盘镜像入口") || strings.Contains(err.Error(), "QCOW2") || strings.Contains(err.Error(), "converted") {
			t.Fatalf("error must direct users to publish through the disk image entry without requiring QCOW2 conversion: %v", err)
		}
	}
}

func TestVMDKRegistryImportPreservesSourceProvenance(t *testing.T) {
	for _, format := range []string{"vmdk", " VMDK ", "qcow2"} {
		t.Run(format, func(t *testing.T) {
			claim := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "manual-root"}}
			const image = "registry.example.invalid/team/vm-service-a:build-a"
			const provenance = "https://images.example.invalid/Windows%2010.vmdk"
			cfg := vmDiskImportConfig{VolumeName: "disk", Format: format, SourceType: "registry",
				ImageURL: image, SourceURI: provenance}
			volume, template, manual, err := buildVMVolumeSource(claim,
				map[string]string{"service_id": "service-a"}, map[string]string{"volume_name": "disk"}, "/disk", &cfg)
			if err != nil || template == nil || manual || volume.DataVolume == nil {
				t.Fatalf("published registry image must use a persistent imported disk: %v", err)
			}
			source := template.Spec.Source
			if source == nil || source.Registry == nil || source.HTTP != nil || source.Blank != nil ||
				source.Registry.URL == nil || *source.Registry.URL != "docker://"+image ||
				source.Registry.PullMethod == nil || *source.Registry.PullMethod != cdiv1.RegistryPullNode {
				t.Fatalf("published VMDK artifact must be imported by CDI node pull: %#v", source)
			}
			if volume.Name != claim.Name || volume.DataVolume.Name != claim.Name || cfg.SourceURI != provenance {
				t.Fatal("logical disk identity and source provenance must remain unchanged")
			}
		})
	}
}
