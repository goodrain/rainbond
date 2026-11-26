package conversion

import (
	"errors"
	"fmt"
	"testing"

	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/worker/appm/volume"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kv "kubevirt.io/api/core/v1"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

// capability_id: rainbond.vm-import.vmdk-validated-registry
func TestCreateVolumesVMDKImportUsesValidatedRegistryWithoutBlankFallback(t *testing.T) {
	for _, tt := range []struct {
		name, source, image string
		accepted            bool
	}{
		{"published registry", "registry", "registry.example.invalid/team/source:v1", true},
		{"direct HTTP", "http", "https://images.example.invalid/source.vmdk", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			as := newVMAppServiceForImportValidationTest()
			manager := vmImportConversionManager{
				volumeDao: vmImportVolumeDao{volumes: []*dbmodel.TenantServiceVolume{{
					Model: dbmodel.Model{ID: 11}, ServiceID: "service-1", VolumeName: "disk",
					VolumePath: "/disk", VolumeType: "nfs-storage", AccessMode: "RWX", VolumeCapacity: 20,
				}}},
				mountDao: vmImportMountDao{},
				attributeDao: vmImportAttributeDao{attribute: &dbmodel.ComponentK8sAttributes{
					AttributeValue: fmt.Sprintf(`{"disk":{"volume_name":"disk","image_url":%q,"source_type":%q,"format":"vmdk"}}`, tt.image, tt.source),
				}},
			}
			define, err := createVolumes(as, &dbmodel.VersionInfo{ServiceID: "service-1"}, nil, nil, manager)
			if !tt.accepted {
				if !errors.Is(err, volume.ErrInvalidVMRegistryImport) {
					t.Fatalf("direct VMDK input must abort the complete VM definition: %v", err)
				}
				if len(as.GetClaims()) != 0 || len(as.GetClaimsManually()) != 0 {
					t.Fatal("failed VMDK import must not materialize a blank disk")
				}
				return
			}
			if err != nil {
				t.Fatalf("published VMDK must reach CDI: %v", err)
			}
			volumes, templates := define.GetVMVolume(), define.GetVMDataVolumeTemplates()
			if len(volumes) != 1 || volumes[0].DataVolume == nil || volumes[0].ContainerDisk != nil ||
				len(templates) != 1 || templates[0].Spec.Source == nil || templates[0].Spec.Source.Registry == nil ||
				templates[0].Spec.Source.Blank != nil || templates[0].Spec.Source.HTTP != nil ||
				templates[0].Spec.Source.Registry.PullMethod == nil || *templates[0].Spec.Source.Registry.PullMethod != cdi.RegistryPullNode {
				t.Fatalf("published VMDK must produce one persistent registry import, volumes=%#v templates=%#v", volumes, templates)
			}
			if len(as.GetClaimsManually()) != 0 {
				t.Fatal("published VMDK must not fall back to a manually provisioned disk")
			}
		})
	}
}

// capability_id: rainbond.vm-runtime.vmdk-persistent-root
// capability_id: rainbond.vm-import.native-disk-formats
func TestNativeDiskSourcesUseImportedPersistentRoot(t *testing.T) {
	for _, format := range []string{"qcow2", "raw", "img", "vmdk", "vdi", "vhd", "vhdx"} {
		t.Run(format, func(t *testing.T) {

			image := "docker://registry.example.invalid/team/published:build-a"
			templates := []kv.DataVolumeTemplateSpec{{ObjectMeta: metav1.ObjectMeta{Name: "manual-root",
				Annotations: map[string]string{"volume_name": "disk"}},
				Spec: cdi.DataVolumeSpec{Source: &cdi.DataVolumeSource{Registry: &cdi.DataVolumeSourceRegistry{URL: &image}}}}}
			volumes := []kv.Volume{{Name: "manual-root", VolumeSource: kv.VolumeSource{
				DataVolume: &kv.DataVolumeSource{Name: "manual-root"}}}}
			attrs := map[string]string{vmBootSourceFormatKey: format, "vm_boot_mode": "uefi"}
			path := resolveVMBootPath(attrs, templates)
			if path != vmBootPathImportedRootDisk || validateVMBootPath(attrs, path) != nil {
				t.Fatalf("disk source requires its imported persistent root, got %q", path)
			}
			prepared, preparedTemplates, blank := prepareVMImageBootVolumes(path, "source."+format, volumes, templates)
			if len(prepared) != 1 || prepared[0].ContainerDisk != nil || prepared[0].DataVolume == nil ||
				prepared[0].DataVolume.Name != "manual-root" || len(preparedTemplates) != 1 || blank != "" {
				t.Fatal("disk must boot from its imported persistent disk, without an installer/containerDisk fallback")
			}
			domain := &kv.DomainSpec{}
			applyVMBootModeForPath(domain, attrs, path)
			device := vmImageDiskDevice(attrs)
			if domain.Firmware == nil || domain.Firmware.Bootloader == nil || domain.Firmware.Bootloader.EFI == nil ||
				device.Disk == nil || device.Disk.Bus != kv.DiskBusSATA || device.CDRom != nil {
				t.Fatal("Windows disk metadata must retain UEFI and the SATA first-boot disk path")
			}
			if missing := resolveVMBootPath(attrs, nil); missing != vmBootPathMissingRootImport || validateVMBootPath(attrs, missing) == nil {
				t.Fatal("missing disk import must not silently boot another disk")
			}

		})
	}
}
