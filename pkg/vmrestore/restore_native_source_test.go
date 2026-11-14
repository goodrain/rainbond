package vmrestore

import (
	"context"
	"errors"
	"strings"
	"testing"

	dbmodel "github.com/goodrain/rainbond/db/model"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
	kv "kubevirt.io/api/core/v1"
)

// capability_id: rainbond.vm-restore.native-source-config
func TestNativeRestoreConfigReadsOriginalAccessModeAndRejectsMissingOrAmbiguousMetadata(t *testing.T) {
	for _, mode := range []string{"ReadWriteOnce", "ReadWriteMany", "ReadOnlyMany", "missing", "ambiguous"} {
		t.Run(mode, func(t *testing.T) {
			f := snapshotProjectionFixture(t)
			switch mode {
			case "missing":
			case "ambiguous":
				f.content.Spec.VolumeBackups[0].PersistentVolumeClaim.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce, corev1.ReadWriteMany}
			default:
				f.content.Spec.VolumeBackups[0].PersistentVolumeClaim.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.PersistentVolumeAccessMode(mode)}
			}
			dto, err := f.project(SnapshotProof{Sealed: true, SnapshotUID: string(f.snapshot.UID), ContentUID: string(f.content.UID), VolumeSnapshotUIDs: map[string]string{"volume-snapshot-a": "volume-snapshot-uid"}})
			if err != nil {
				t.Fatal(err)
			}
			config, err := NativeRestoreConfig(dto, f.content)
			if mode == "missing" || mode == "ambiguous" {
				if !errors.Is(err, ErrRestoreCapability) {
					t.Fatal("source access mode was guessed", mode, err)
				}
				return
			}
			if err != nil || len(config.Volumes) != 1 || config.Volumes[0].AccessMode != mode || config.Volumes[0].CapacityGiB != 20 || config.Volumes[0].StorageClass != "cephfs-vm-snapshot" {
				t.Fatal(config, err)
			}
		})
	}
}

// capability_id: rainbond.vm-restore.native-source-config
func TestNativeRestoreSourceZeroQualificationNeverReadsOrAllocates(t *testing.T) {
	f := newRestoreCommitFixture(t)
	p := &NativeRestorePreparation{Store: f.store}
	if _, err := p.Read(context.Background(), f.scope, "operation", "snapshot", "previous"); !errors.Is(err, ErrRestoreCapability) {
		t.Fatal(err)
	}
	c, _ := f.store.Read(f.scope, "")
	if c.State.Fence != nil || c.State.Admission != nil || c.State.ControlRevision != 0 {
		t.Fatal("unqualified source changed state")
	}
}

// capability_id: rainbond.vm-restore.native-source-config
func TestNativeSourceReadsFreshManagedAAndKeepsSealedBOnReplay(t *testing.T) {
	snapshots, b, scope := snapshotEngineFixture(t)
	startup := testStartupPolicy("platform")
	startup.NoCloudVolumeName = "cloud-init"
	startup.NoCloudDiskName = "cloud-init"
	b.fixture.content.Spec.Source.VirtualMachine.Spec.Template.Spec.Domain.Devices.Disks = append(b.fixture.content.Spec.Source.VirtualMachine.Spec.Template.Spec.Domain.Devices.Disks, kv.Disk{Name: "cloud-init"})
	if b.fixture.content.Spec.Source.VirtualMachine.Spec.Template.Spec.Domain.Firmware == nil {
		b.fixture.content.Spec.Source.VirtualMachine.Spec.Template.Spec.Domain.Firmware = &kv.Firmware{}
	}
	b.fixture.content.Spec.Source.VirtualMachine.Spec.Template.Spec.Domain.Firmware.UUID = "11111111-1111-4111-8111-111111111111"
	b.fixture.content.Spec.VolumeBackups[0].PersistentVolumeClaim.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
	d := enginePrepare(t, snapshots, scope, b)
	_, err := snapshots.mutate(scope, d.OperationID, "fixture-trusted-startup", strings.Repeat("c", 64), func(c Current) (Mutation, error) {
		c.Receipt.SnapshotIdentity = &SnapshotFirmwareIdentity{VM: c.Receipt.Snapshot.SourceVM, VMResourceVersion: "1", FirmwareUUID: startup.FirmwareUUID, Pinned: true, Startup: &startup}
		return Mutation{State: c.State, Receipt: c.Receipt}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := snapshots.Halt(context.Background(), scope, engineAction("halt", d)); err != nil {
		t.Fatal(err)
	}
	ready, err := snapshots.Create(context.Background(), scope, engineAction("create", d))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := snapshots.Finalize(context.Background(), scope, readyACK("ack", ready)); !errors.Is(err, ErrSnapshotPending) {
		t.Fatal(err)
	}
	if _, err := snapshots.Resume(context.Background(), scope, readyACK("resume", ready)); err != nil {
		t.Fatal(err)
	}
	if _, err := snapshots.Finalize(context.Background(), scope, readyACK("finalize", ready)); err != nil {
		t.Fatal(err)
	}
	database := snapshots.store.db
	if err := database.AutoMigrate(&dbmodel.TenantServiceVolume{}, &dbmodel.TenantServicesPort{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Model(&dbmodel.TenantServices{}).Where("service_id = ?", scope.ServiceID).Updates(map[string]interface{}{"container_cpu": 2000, "container_memory": 2048}).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Create(&dbmodel.TenantServiceVolume{Model: dbmodel.Model{ID: 7}, ServiceID: scope.ServiceID, VolumeName: "disk", VolumePath: "/disk", VolumeType: "cephfs-vm-snapshot", VolumeCapacity: 20, AccessMode: "ReadWriteMany"}).Error; err != nil {
		t.Fatal(err)
	}
	for _, row := range []dbmodel.ComponentK8sAttributes{{TenantID: scope.TenantID, ComponentID: scope.ServiceID, Name: "vm_boot_mode", AttributeValue: "bios"}, {TenantID: scope.TenantID, ComponentID: scope.ServiceID, Name: "vm_disk_layout", AttributeValue: `[{"disk_key":"disk","disk_name":"manual7","disk_role":"root","device_type":"disk","source_kind":"volume","order_index":0,"boot":true,"bus":"virtio","boot_order":1}]`}} {
		value := row
		if err := database.Create(&value).Error; err != nil {
			t.Fatal(err)
		}
	}
	class := "cephfs-vm-snapshot"
	core := kubefake.NewSimpleClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: scope.Namespace, UID: "namespace-uid"}}, &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "manual7", Namespace: scope.Namespace, UID: "b-pvc", Labels: map[string]string{"service_id": scope.ServiceID, "tenant_id": scope.TenantID}}, Spec: corev1.PersistentVolumeClaimSpec{StorageClassName: &class, Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("20Gi")}}}, Status: corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound}})
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{dataVolumeResource: "DataVolumeList", virtualMachineResource: "VirtualMachineList", virtualMachineInstanceResource: "VirtualMachineInstanceList", vmSnapshotResource: "VirtualMachineSnapshotList", vmSnapshotContentResource: "VirtualMachineSnapshotContentList", csiSnapshotResource: "VolumeSnapshotList", csiSnapshotContentResource: "VolumeSnapshotContentList", dataSourceResource: "DataSourceList"})
	vm := nativeAsVM(b.fixture.content.Spec.Source.VirtualMachine).DeepCopy()
	vm.UID = "b-vm"
	vm.Name = "current-vm"
	vm.Spec.Template.Spec.Volumes[0].PersistentVolumeClaim.ClaimName = "manual7"
	strategy := kv.RunStrategyAlways
	vm.Spec.RunStrategy = &strategy
	object, err := EncodeRuntimeVM(vm)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := dyn.Resource(restoreVMGVR).Namespace(scope.Namespace).Create(context.Background(), object, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	_, _, model := resourceFixture(t)
	cert := model.Certification
	cert.TargetClassName = class
	cert.TargetClassUID = "target-class"
	cert.Capability.StorageClassName = class
	cert.Capability.StorageClassUID = "target-class"
	cert.ROXClassName = "cephfs-rox"
	cert.Capability.TargetUID = 107
	cert.Capability.TargetGID = 107
	resolver := NewResolver(snapshots.store, core, dyn)
	source := &NativeRestorePreparation{Store: snapshots.store, Resolver: resolver, Snapshots: snapshots, Dynamic: dyn, Certification: cert, CurrentStartup: func(context.Context, Scope, *kv.VirtualMachine, string) (RestoreStartupPolicy, error) {
		return startup, nil
	}, DeadlineSeconds: 60, MaxDisks: 8}
	engine := &RestoreEngine{Store: snapshots.store, Resolver: resolver, Resources: &RestoreResourceExecutor{Store: snapshots.store, Core: core, Dynamic: dyn, Certification: cert}, Source: source}
	c, err := engine.Prepare(context.Background(), scope, "native-restore", d.SnapshotName, "previous")
	if err != nil {
		t.Fatal(err)
	}
	if c.Receipt.Execution.ConfigAfter.Volumes[0].AccessMode != "ReadWriteOnce" || c.Receipt.Restore.Before.Volumes[0].AccessMode != "ReadWriteMany" || !c.Receipt.Restore.WasRunning {
		t.Fatal("native A or original B/power was guessed")
	}
	vm.Spec.RunStrategy = nil
	running := false
	vm.Spec.Running = &running
	changed, _ := EncodeRuntimeVM(vm)
	_, _ = dyn.Resource(restoreVMGVR).Namespace(scope.Namespace).Update(context.Background(), changed, metav1.UpdateOptions{})
	replay, err := source.Read(context.Background(), scope, "native-restore", d.SnapshotName, "previous")
	if err != nil || !replay.Intent.WasRunning {
		t.Fatal("replay replaced original power intent with stopped current value", err)
	}
}
