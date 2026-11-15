package vmstorage

import (
	"reflect"
	"testing"
)

// capability_id: rainbond.vm-storage.managed-snapshot-policy
func TestManagedSnapshotStoragePolicyPreservesNativeFactsAndUnknown(t *testing.T) {
	for _, test := range []struct {
		name, driver, nativeStatus, nativeReason, status, reason string
		supported                                                bool
	}{
		{"cephfs", ManagedSnapshotDriver, "supported", "ready", "supported", "ready", true},
		{"longhorn", LonghornSnapshotDriver, "supported", "ready", "supported", "ready", true},
		{"rbd", "rbd.csi.ceph.com", "supported", "ready", "unsupported", "managed_snapshot_driver_unsupported", false},
		{"nfs", "example.nfs", "unsupported", "csi_driver_not_registered", "unsupported", "managed_snapshot_driver_unsupported", false},
		{"unknown driver", "", "unknown", "storage_class_read_failed", "unknown", "storage_class_read_failed", false},
		{"read failed after driver read", "rbd.csi.ceph.com", "unknown", "snapshot_class_read_failed", "unknown", "snapshot_class_read_failed", false},
		{"cephfs missing class", ManagedSnapshotDriver, "unsupported", "snapshot_class_missing", "unsupported", "snapshot_class_missing", false},
		{"driver missing despite positive flag", "", "supported", "ready", "unknown", "managed_snapshot_driver_unverified", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			native := Capability{Supported: test.nativeStatus == "supported", Status: test.nativeStatus, ReasonCode: test.nativeReason, Driver: test.driver, StorageClass: "sc", VolumeSnapshotClass: "snapshot-class"}
			before := native
			managed := ManagedSnapshotCapability(native)
			if managed.Supported != test.supported || managed.Status != test.status || managed.ReasonCode != test.reason {
				t.Fatal("managed policy disagreed with known/unknown driver evidence", managed)
			}
			if !reflect.DeepEqual(before, native) || managed.Driver != native.Driver || managed.StorageClass != native.StorageClass || managed.VolumeSnapshotClass != native.VolumeSnapshotClass {
				t.Fatal("policy changed original CSI evidence or physical identity")
			}
		})
	}
}
