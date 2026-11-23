package volume

import (
	"errors"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

// capability_id: rainbond.vm-import.vmdk-converted-runtime
func TestVMDKDiskImportRequiresConvertedRuntime(t *testing.T) {
	for _, source := range []string{"", "http", "registry", "http-artifact"} {
		t.Run(source, func(t *testing.T) {
			claim := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "manual-root"}}
			_, template, manual, err := buildVMVolumeSource(claim,
				map[string]string{"service_id": "service-a"}, map[string]string{"volume_name": "disk"}, "/disk",
				&vmDiskImportConfig{VolumeName: "disk", Format: " VMDK ", SourceType: source,
					ImageURL: "https://images.example.invalid/source.vmdk"})
			if !errors.Is(err, ErrInvalidVMRegistryImport) || !strings.Contains(err.Error(), "VMDK") {
				t.Fatalf("unconverted VMDK must abort VM creation, got %v", err)
			}
			if template != nil || manual {
				t.Fatal("rejected input must not create an import or fall back to a blank/manual disk")
			}
		})
	}
}

func TestVMDKHTTPImportCannotUseAnotherFormatToBypassConversion(t *testing.T) {
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
	}
}

func TestConvertedVMDKRegistryImportPreservesSourceProvenance(t *testing.T) {
	claim := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "manual-root"}}
	const image = "registry.example.invalid/team/vm-service-a:build-a"
	const provenance = "https://images.example.invalid/Windows%2010.vmdk"
	cfg := vmDiskImportConfig{VolumeName: "disk", Format: "qcow2", SourceType: "registry",
		ImageURL: image, SourceURI: provenance}
	volume, template, manual, err := buildVMVolumeSource(claim,
		map[string]string{"service_id": "service-a"}, map[string]string{"volume_name": "disk"}, "/disk", &cfg)
	if err != nil || template == nil || manual || volume.DataVolume == nil {
		t.Fatalf("converted registry image must use a persistent imported disk: %v", err)
	}
	source := template.Spec.Source
	if source == nil || source.Registry == nil || source.HTTP != nil || source.Blank != nil ||
		source.Registry.URL == nil || *source.Registry.URL != "docker://"+image ||
		source.Registry.PullMethod == nil || *source.Registry.PullMethod != cdiv1.RegistryPullNode {
		t.Fatalf("source VMDK URI must not bypass Builder conversion: %#v", source)
	}
	if volume.Name != claim.Name || volume.DataVolume.Name != claim.Name || cfg.SourceURI != provenance {
		t.Fatal("logical disk identity and source provenance must remain unchanged")
	}
}
