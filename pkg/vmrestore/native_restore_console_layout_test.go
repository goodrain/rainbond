package vmrestore

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	dbmodel "github.com/goodrain/rainbond/db/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kv "kubevirt.io/api/core/v1"
)

func consoleLayoutFixture(t *testing.T, bus string) (managedNativeFixture, string) {
	t.Helper()
	f := newManagedNativeFixture(t)
	image := "registry.example.invalid/virtio@sha256:" + strings.Repeat("a", 64)
	source := f.snapshots.fixture.content.Spec.Source.VirtualMachine
	source.Spec.Template.Spec.Volumes = append(source.Spec.Template.Spec.Volumes, kv.Volume{Name: "virtio", VolumeSource: kv.VolumeSource{ContainerDisk: &kv.ContainerDiskSource{Image: image}}})
	source.Spec.Template.Spec.Domain.Devices.Disks = append(source.Spec.Template.Spec.Domain.Devices.Disks, kv.Disk{Name: "virtio", DiskDevice: kv.DiskDevice{CDRom: &kv.CDRomTarget{Bus: kv.DiskBusSATA}}})
	live, _, err := f.backend.VirtualMachine(context.Background(), f.scope)
	if err != nil {
		t.Fatal(err)
	}
	live.Spec.Template.Spec.Volumes = source.Spec.Template.Spec.Volumes
	live.Spec.Template.Spec.Domain.Devices.Disks = source.Spec.Template.Spec.Domain.Devices.Disks
	object, err := EncodeRuntimeVM(live)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.dynamic.Resource(restoreVMGVR).Namespace(f.scope.Namespace).Update(context.Background(), object, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	layout := `[{"disk_key":"disk","disk_name":"system-disk","disk_role":"root","device_type":"disk","source_kind":"volume","order_index":0,"boot":true` + bus + `},{"disk_key":"virtio","disk_name":"virtio","disk_role":"data","device_type":"cdrom","source_kind":"container_disk","order_index":1,"boot":false,"image":"` + image + `"}]`
	if err := f.managed.Store.db.Model(&dbmodel.ComponentK8sAttributes{}).Where("component_id = ? AND name = ?", f.scope.ServiceID, "vm_disk_layout").Update("attribute_value", layout).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.managed.Store.db.Model(&dbmodel.TenantServiceVolume{}).Where("service_id = ?", f.scope.ServiceID).Update("access_mode", "RWO").Error; err != nil {
		t.Fatal(err)
	}
	return f, layout
}

// capability_id: rainbond.vm-restore.native-console-layout
func TestSelectedRuntimeConfigNormalizesConsoleDiskLayout(t *testing.T) {
	f, _ := consoleLayoutFixture(t, "")
	config, err := f.managed.Store.selectedRuntimeConfig(f.scope)
	if err != nil {
		t.Fatalf("ordinary Console layout was rejected: %v", err)
	}
	if len(config.Hardware.DiskLayout) != 1 || config.Hardware.DiskLayout[0].Bus != "sata" || config.Hardware.DiskLayout[0].BootOrder != 1 || config.Hardware.DiskLayout[0].CanonicalName != "manual7" || len(config.Volumes) != 1 || config.Volumes[0].AccessMode != "ReadWriteOnce" {
		t.Fatalf("incorrect logical disk selection: %#v", config)
	}
}

// capability_id: rainbond.vm-restore.native-console-layout
func TestManagedNativeRestorePreservesConsoleDiskMetadata(t *testing.T) {
	ctx := context.Background()
	f, raw := consoleLayoutFixture(t, `,"bus":"virtio"`)
	first, err := f.engine.Start(ctx, f.scope, "snapshot-a", "restore-console-layout")
	if err != nil || first.Complete || f.backend.creates != 1 {
		t.Fatalf("Console layout did not enter native restore: %#v %v", first, err)
	}
	f.completeNative(t)
	var result NativeRestoreDTO
	for i := 0; i < 4; i++ {
		result, err = f.engine.Start(ctx, f.scope, "snapshot-a", "restore-console-layout")
		if err != nil {
			t.Fatal(err)
		}
		if result.Complete {
			break
		}
	}
	if !result.Complete {
		t.Fatal("native restore did not finish")
	}
	var row dbmodel.ComponentK8sAttributes
	if err := f.managed.Store.db.Where("component_id = ? AND name = ?", f.scope.ServiceID, "vm_disk_layout").Take(&row).Error; err != nil || row.AttributeValue != raw {
		t.Fatal("data-only restore rewrote Console display/installation-media metadata", err, row.AttributeValue)
	}
	var volume dbmodel.TenantServiceVolume
	if err := f.managed.Store.db.Where("service_id = ?", f.scope.ServiceID).Take(&volume).Error; err != nil || volume.AccessMode != "RWO" {
		t.Fatal("data-only restore rewrote the original volume declaration", err, volume.AccessMode)
	}
	selection, err := f.managed.Store.ReadRestoredRuntimeSelection(ctx, f.managed.Resolver, f.scope)
	if err != nil || selection == nil {
		t.Fatal("restored runtime cannot read unchanged Console layout", err)
	}
	live, _, _ := f.backend.VirtualMachine(ctx, f.scope)
	before := live.DeepCopy()
	projected, err := ProjectRestoredRuntime(ctx, f.core, live, *selection)
	if err != nil {
		t.Fatal(err)
	}
	var media *kv.ContainerDiskSource
	for _, v := range projected.Spec.Template.Spec.Volumes {
		if v.Name == "virtio" {
			media = v.ContainerDisk
		}
	}
	var original *kv.ContainerDiskSource
	for _, v := range before.Spec.Template.Spec.Volumes {
		if v.Name == "virtio" {
			original = v.ContainerDisk
		}
	}
	if media == nil || !reflect.DeepEqual(media, original) {
		t.Fatal("restored runtime lost or replaced the read-only installation media")
	}
}

// capability_id: rainbond.vm-restore.native-console-layout
func TestManagedNativeRestoreRejectsChangedInstallationMedia(t *testing.T) {
	for _, field := range []string{"image", "boot", "writable"} {
		t.Run(field, func(t *testing.T) {
			f, _ := consoleLayoutFixture(t, `,"bus":"virtio"`)
			desired := nativeAsVM(f.snapshots.fixture.content.Spec.Source.VirtualMachine).DeepCopy()
			for i := range desired.Spec.Template.Spec.Volumes {
				if desired.Spec.Template.Spec.Volumes[i].Name == "virtio" && field == "image" {
					desired.Spec.Template.Spec.Volumes[i].ContainerDisk.Image = "registry.example.invalid/changed:latest"
				}
			}
			for i := range desired.Spec.Template.Spec.Domain.Devices.Disks {
				d := &desired.Spec.Template.Spec.Domain.Devices.Disks[i]
				if d.Name != "virtio" {
					continue
				}
				if field == "boot" {
					order := uint(3)
					d.BootOrder = &order
				}
				if field == "writable" {
					writable := false
					d.CDRom.ReadOnly = &writable
				}
			}
			f.managed.BuildPlatform = func(context.Context, Scope) (*kv.VirtualMachine, error) { return desired, nil }
			_, err := f.engine.Start(context.Background(), f.scope, "snapshot-a", "restore-changed-media")
			if !errors.Is(err, ErrNativeRestoreConfigurationChanged) || f.backend.creates != 0 {
				t.Fatal("changed installation media entered native restore", err)
			}
		})
	}
}

// capability_id: rainbond.vm-restore.native-console-layout
func TestNativeConsoleSelectionRejectsMalformedOrChangedRows(t *testing.T) {
	for _, mode := range []string{"unknown field", "image on PVC", "duplicate key", "invalid access mode", "capacity changes"} {
		t.Run(mode, func(t *testing.T) {
			f, raw := consoleLayoutFixture(t, `,"bus":"virtio"`)
			selected, err := f.managed.Store.selectedRuntimeConfig(f.scope)
			if err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "unknown field":
				raw = strings.Replace(raw, `"boot":true`, `"boot":true,"unknown":1`, 1)
			case "image on PVC":
				raw = strings.Replace(raw, `"boot":true`, `"boot":true,"image":"bad"`, 1)
			case "duplicate key":
				raw = strings.Replace(raw, `"disk_key":"virtio"`, `"disk_key":"disk"`, 1)
			case "invalid access mode":
				if err := f.managed.Store.db.Model(&dbmodel.TenantServiceVolume{}).Where("service_id = ?", f.scope.ServiceID).Update("access_mode", "RWO,RWX").Error; err != nil {
					t.Fatal(err)
				}
			case "capacity changes":
				if err := f.managed.Store.db.Model(&dbmodel.TenantServiceVolume{}).Where("service_id = ?", f.scope.ServiceID).Update("volume_capacity", 21).Error; err != nil {
					t.Fatal(err)
				}
			}
			if err := f.managed.Store.db.Model(&dbmodel.ComponentK8sAttributes{}).Where("component_id = ? AND name = ?", f.scope.ServiceID, "vm_disk_layout").Update("attribute_value", raw).Error; err != nil {
				t.Fatal(err)
			}
			tx := f.managed.Store.db.Begin()
			defer tx.Rollback()
			if err := requireBeforeNativeConfig(tx, f.scope, selected); err == nil {
				t.Fatal("malformed or changed configuration passed the native commit comparison")
			}
		})
	}
}

// capability_id: rainbond.vm-restore.native-console-layout
func TestNativeControlRejectsCrossConfigurationBeforeFence(t *testing.T) {
	f := newRestoreCommitFixture(t)
	f.seedBeforeAttributes(t)
	c := NativeRestoreControl{Config: f.after, BeforeStartup: testStartupPolicy("platform"), SourceStartup: testStartupPolicy("restored"), SnapshotOperationID: "snapshot-source", NativeProofHash: strings.Repeat("a", 64), RestoreName: "vmr-012345678901234567890123", RequestID: "request-a", Nonce: "attempt-a"}
	if _, err := f.store.PrepareNativeRestoreDB(f.scope, f.expect, f.intent, c, f.old); err == nil {
		t.Fatal("cross-configuration native restore acquired a fence")
	}
	current, err := f.store.ReadOnly(f.scope, "")
	if err != nil || current.State.Fence != nil {
		t.Fatal("rejected restore changed the component fence", err)
	}
}
