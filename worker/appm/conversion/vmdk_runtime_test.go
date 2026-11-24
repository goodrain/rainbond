package conversion

import (
	"errors"
	"testing"

	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/worker/appm/volume"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kv "kubevirt.io/api/core/v1"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

func TestCreateVolumesRejectsUnconvertedVMDKWithoutBlankFallback(t *testing.T) {
	as := newVMAppServiceForImportValidationTest()
	manager := vmImportConversionManager{
		volumeDao: vmImportVolumeDao{volumes: []*dbmodel.TenantServiceVolume{{
			Model: dbmodel.Model{ID: 11}, ServiceID: "service-1", VolumeName: "disk",
			VolumePath: "/disk", VolumeType: "nfs-storage", AccessMode: "RWX", VolumeCapacity: 20,
		}}},
		mountDao: vmImportMountDao{},
		attributeDao: vmImportAttributeDao{attribute: &dbmodel.ComponentK8sAttributes{
			AttributeValue: `{"disk":{"volume_name":"disk","image_url":"registry.example.invalid/team/source:v1","source_type":"registry","format":"vmdk"}}`,
		}},
	}
	_, err := createVolumes(as, &dbmodel.VersionInfo{ServiceID: "service-1"}, nil, nil, manager)
	if !errors.Is(err, volume.ErrInvalidVMRegistryImport) {
		t.Fatalf("unconverted input must abort the complete VM definition: %v", err)
	}
	if len(as.GetClaims()) != 0 || len(as.GetClaimsManually()) != 0 {
		t.Fatal("failed VMDK import must not materialize a blank disk")
	}
}

// capability_id: rainbond.vm-runtime.vmdk-persistent-root
func TestVMDKSourceUsesConvertedPersistentRoot(t *testing.T) {
	image := "docker://registry.example.invalid/team/converted:build-a"
	templates := []kv.DataVolumeTemplateSpec{{ObjectMeta: metav1.ObjectMeta{Name: "manual-root",
		Annotations: map[string]string{"volume_name": "disk"}},
		Spec: cdi.DataVolumeSpec{Source: &cdi.DataVolumeSource{Registry: &cdi.DataVolumeSourceRegistry{URL: &image}}}}}
	volumes := []kv.Volume{{Name: "manual-root", VolumeSource: kv.VolumeSource{
		DataVolume: &kv.DataVolumeSource{Name: "manual-root"}}}}
	attrs := map[string]string{vmBootSourceFormatKey: "vmdk", "vm_boot_mode": "uefi"}
	path := resolveVMBootPath(attrs, templates)
	if path != vmBootPathImportedRootDisk || validateVMBootPath(attrs, path) != nil {
		t.Fatalf("VMDK source requires its converted imported root, got %q", path)
	}
	prepared, preparedTemplates, blank := prepareVMImageBootVolumes(path, "source.vmdk", volumes, templates)
	if len(prepared) != 1 || prepared[0].ContainerDisk != nil || prepared[0].DataVolume == nil ||
		prepared[0].DataVolume.Name != "manual-root" || len(preparedTemplates) != 1 || blank != "" {
		t.Fatal("VMDK must boot from its imported persistent disk, without an installer/containerDisk fallback")
	}
	domain := &kv.DomainSpec{}
	applyVMBootModeForPath(domain, attrs, path)
	device := vmImageDiskDevice(attrs)
	if domain.Firmware == nil || domain.Firmware.Bootloader == nil || domain.Firmware.Bootloader.EFI == nil ||
		device.Disk == nil || device.Disk.Bus != kv.DiskBusSATA || device.CDRom != nil {
		t.Fatal("Windows VMDK metadata must retain UEFI and the SATA first-boot disk path")
	}
	if missing := resolveVMBootPath(attrs, nil); missing != vmBootPathMissingRootImport || validateVMBootPath(attrs, missing) == nil {
		t.Fatal("missing VMDK import must not silently boot another disk")
	}
}
