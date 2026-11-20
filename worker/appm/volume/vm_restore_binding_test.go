package volume

import (
	"errors"
	"testing"

	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/pkg/vmrestore"
)

// capability_id: rainbond.vm-restore.worker-bound-disk-rendering
func TestRestoredDiskUsesActivePVCWithoutReimportAndDetachedDiskStaysOut(t *testing.T) {
	for _, attached := range []bool{true, false} {
		as := newVMAppServiceForVolumeTest()
		as.VMStorageView = &vmrestore.BindingView{Volumes: []vmrestore.ResolvedVolume{{VolumeID: 7, Mapped: true, Attached: attached, CanonicalName: "manual7", DiskKey: "disk", ActivePVC: &vmrestore.ObjectRef{Kind: "PersistentVolumeClaim", Namespace: as.GetNamespace(), Name: "restored-root", UID: "root-a"}, Provisioning: "bound"}}}
		row := &dbmodel.TenantServiceVolume{Model: dbmodel.Model{ID: 7}, ServiceID: as.ServiceID, VolumeName: "disk", VolumePath: "/disk", VolumeType: "cephfs-vm-snapshot", AccessMode: "ReadWriteMany", VolumeCapacity: 20}
		volume := NewVolumeManager(as, row, nil, nil, nil, nil, nil, false).(*ShareFileVolume)
		define := &Define{as: as}
		if err := volume.CreateVolume(define); err != nil {
			t.Fatal(err)
		}
		if len(define.GetVMDataVolumeTemplates()) != 0 {
			t.Fatal("restored disk scheduled a CDI reimport")
		}
		if attached {
			if len(define.vmVolume) != 1 || define.vmVolume[0].PersistentVolumeClaim == nil || define.vmVolume[0].PersistentVolumeClaim.ClaimName != "restored-root" || len(define.vmDisk) != 1 {
				t.Fatal("active binding not used")
			}
		} else if len(define.vmVolume) != 0 || len(define.vmDisk) != 0 {
			t.Fatal("B-only detached disk was rendered")
		}
	}
}

// capability_id: rainbond.vm-restore.worker-bound-disk-rendering
func TestMappedDiskUnavailableDoesNotFallBackToImport(t *testing.T) {
	as := newVMAppServiceForVolumeTest()
	as.VMStorageView = &vmrestore.BindingView{Volumes: []vmrestore.ResolvedVolume{{VolumeID: 7, Mapped: true, Attached: true, CanonicalName: "manual7", DiskKey: "disk", Provisioning: "pending"}}}
	row := &dbmodel.TenantServiceVolume{Model: dbmodel.Model{ID: 7}, ServiceID: as.ServiceID, VolumeName: "disk", VolumePath: "/disk", VolumeType: "cephfs-vm-snapshot"}
	volume := NewVolumeManager(as, row, nil, nil, nil, nil, nil, false).(*ShareFileVolume)
	define := &Define{as: as}
	if err := volume.CreateVolume(define); !errors.Is(err, vmrestore.ErrConflict) {
		t.Fatal(err)
	}
	if len(define.GetVMDataVolumeTemplates()) != 0 || len(define.vmVolume) != 0 {
		t.Fatal("unavailable mapped disk fell back to canonical import")
	}
}
