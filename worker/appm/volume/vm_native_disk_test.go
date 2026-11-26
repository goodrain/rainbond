package volume

import (
	"errors"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

// capability_id: rainbond.vm-import.native-disk-formats
func TestNativeDiskRegistryImportsUseCDI(t *testing.T) {
	for _, format := range []string{"qcow2", "raw", "img", "vmdk", "vdi", "vhd", "vhdx"} {
		t.Run(format, func(t *testing.T) {
			claim := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "imported-root"}}
			cfg := &vmDiskImportConfig{VolumeName: "disk", Format: format, SourceType: "registry", ImageURL: "goodrain.me/team/native:build-one"}
			volume, template, manual, err := buildVMVolumeSource(claim, nil, nil, "/disk", cfg)
			if err != nil || template == nil || manual || volume.DataVolume == nil || volume.ContainerDisk != nil {
				t.Fatalf("native disk must use a persistent CDI import: %v", err)
			}
			source := template.Spec.Source
			if source == nil || source.Registry == nil || source.HTTP != nil || source.Blank != nil ||
				source.Registry.PullMethod == nil || *source.Registry.PullMethod != cdiv1.RegistryPullNode {
				t.Fatalf("native disk must reach CDI via node pull: %#v", source)
			}
			if cfg.Format != format || volume.DataVolume.Name != claim.Name {
				t.Fatal("source format and persistent disk identity must be preserved")
			}
		})
	}
}

// capability_id: rainbond.vm-import.native-disk-formats
func TestDesktopDiskImportsCannotBypassBuilderValidation(t *testing.T) {
	claim := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "imported-root"}}
	for _, format := range []string{"vmdk", "vdi", "vhd", "vhdx"} {
		for _, declared := range []string{format, "qcow2"} {
			cfg := &vmDiskImportConfig{VolumeName: "disk", Format: declared, SourceType: "http", ImageURL: "https://example.invalid/source." + format}
			_, template, manual, err := buildVMVolumeSource(claim, nil, nil, "/disk", cfg)
			if !errors.Is(err, ErrInvalidVMRegistryImport) || template != nil || manual {
				t.Fatalf("%s input cannot bypass validation as %s: %v", format, declared, err)
			}
		}
		cfg := &vmDiskImportConfig{VolumeName: "disk", Format: format + ".gz", SourceType: "registry", ImageURL: "goodrain.me/team/native:build-one"}
		if _, err := buildVMDiskImportDataVolumeTemplate(claim, nil, nil, *cfg); !errors.Is(err, ErrInvalidVMRegistryImport) {
			t.Fatalf("wrapped %s must be unpacked before validation: %v", format, err)
		}
	}
}
