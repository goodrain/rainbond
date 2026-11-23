package vmstorage

import (
	"context"
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
			native := Capability{Supported: test.nativeStatus == "supported", Status: test.nativeStatus, ReasonCode: test.nativeReason, Driver: test.driver, StorageClass: "sc", VolumeSnapshotClass: "snapshot-class", deletionPolicy: "Delete"}
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

// capability_id: rainbond.vm-storage.managed-snapshot-admission
func TestManagedSnapshotAdmissionUsesSelectedClassPolicy(t *testing.T) {
	for _, tt := range []struct {
		name, policy, backing, status, reason string
	}{
		{"Delete", "Delete", "", "supported", "ready"},
		{"Retain remains native only", "Retain", "", "unsupported", "managed_snapshot_class_policy_unsupported"},
		{"missing policy", "", "", "unknown", "managed_snapshot_class_policy_unverified"},
		{"backing snapshot", "Delete", "TrUe", "unsupported", "managed_snapshot_backing_unsupported"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			core, dyn := fixture()
			sc, _ := core.StorageV1().StorageClasses().Get(ctx, "fast", metav1.GetOptions{})
			sc.Provisioner = LonghornSnapshotDriver
			sc.Parameters = map[string]string{"backingSnapshot": tt.backing}
			_, _ = core.StorageV1().StorageClasses().Update(ctx, sc, metav1.UpdateOptions{})
			driver, _ := core.StorageV1().CSIDrivers().Get(ctx, "example.csi.io", metav1.GetOptions{})
			driver.Name = LonghornSnapshotDriver
			driver.ResourceVersion = ""
			_, _ = core.StorageV1().CSIDrivers().Create(ctx, driver, metav1.CreateOptions{})
			pod, _ := core.CoreV1().Pods("infra").Get(ctx, "provider", metav1.GetOptions{})
			pod.Spec.Containers[0].Env[0].Value = LonghornSnapshotDriver
			_, _ = core.CoreV1().Pods("infra").Update(ctx, pod, metav1.UpdateOptions{})
			class := snapshotClass("class-one", LonghornSnapshotDriver, false)
			_ = unstructured.SetNestedField(class.Object, tt.policy, "deletionPolicy")
			_, _ = dyn.Resource(snapshotClassGVR).Update(ctx, class, metav1.UpdateOptions{})
			native := NewInspector(core, dyn).StorageClass(ctx, "fast")
			if !native.Supported || native.VolumeSnapshotClass != "class-one" {
				t.Fatalf("native CSI evidence unexpectedly lost: %+v", native)
			}
			managed := ManagedSnapshotCapability(native)
			if managed.Status != tt.status || managed.ReasonCode != tt.reason || managed.Supported != (tt.status == "supported") {
				t.Fatalf("managed admission: %+v; want %s/%s", managed, tt.status, tt.reason)
			}
		})
	}
}
