package vmrestore

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	dbmodel "github.com/goodrain/rainbond/db/model"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
	kv "kubevirt.io/api/core/v1"
	native "kubevirt.io/api/snapshot/v1beta1"
)

type managedNativeBackendFixture struct {
	*nativeRestoreBackendFixture
	dynamic           dynamic.Interface
	scope             Scope
	createErrorBefore error
	createErrorAfter  error
	beforeCreate      func()
}

func (b *managedNativeBackendFixture) CreateRestore(ctx context.Context, scope Scope, restore *native.VirtualMachineRestore) (*native.VirtualMachineRestore, error) {
	if b.createErrorBefore != nil {
		return nil, b.createErrorBefore
	}
	if b.beforeCreate != nil {
		b.beforeCreate()
	}
	actual, err := b.nativeRestoreBackendFixture.CreateRestore(ctx, scope, restore)
	if b.createErrorAfter != nil {
		return nil, b.createErrorAfter
	}
	return actual, err
}

func (b *managedNativeBackendFixture) VirtualMachine(ctx context.Context, scope Scope) (*kv.VirtualMachine, *kv.VirtualMachineInstance, error) {
	if err := b.errors["vm"]; err != nil {
		return nil, nil, err
	}
	o, err := b.dynamic.Resource(restoreVMGVR).Namespace(scope.Namespace).Get(ctx, b.vm.Name, metav1.GetOptions{})
	if err != nil {
		return nil, nil, err
	}
	var vm kv.VirtualMachine
	if err = runtime.DefaultUnstructuredConverter.FromUnstructured(o.Object, &vm); err != nil {
		return nil, nil, err
	}
	return &vm, b.vmi, nil
}

type managedNativeFixture struct {
	engine    *NativeRestoreEngine
	managed   *ManagedNativeRestore
	backend   *managedNativeBackendFixture
	snapshots *fakeSnapshotBackend
	scope     Scope
	core      *kubefake.Clientset
	dynamic   dynamic.Interface
}

func newManagedNativeFixture(t *testing.T, readOnly ...bool) managedNativeFixture {
	t.Helper()
	snapshots, b, scope := snapshotEngineFixture(t)
	startup := testStartupPolicy("platform")
	startup.NoCloudVolumeName = "cloud-init"
	startup.NoCloudDiskName = "cloud-init"
	b.fixture.content.Spec.Source.VirtualMachine.Spec.Template.Spec.Domain.Devices.Disks = append(b.fixture.content.Spec.Source.VirtualMachine.Spec.Template.Spec.Domain.Devices.Disks, kv.Disk{Name: "cloud-init"})
	b.fixture.content.Spec.Source.VirtualMachine.Spec.Template.Spec.Domain.Firmware = &kv.Firmware{UUID: types.UID(startup.FirmwareUUID)}
	b.fixture.content.Spec.VolumeBackups[0].PersistentVolumeClaim.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
	ro := len(readOnly) > 0 && readOnly[0]
	if ro {
		vm := b.fixture.content.Spec.Source.VirtualMachine
		vm.Spec.Template.Spec.Domain.Devices.Disks = append(vm.Spec.Template.Spec.Domain.Devices.Disks, kv.Disk{Name: "manual8", DiskDevice: kv.DiskDevice{Disk: &kv.DiskTarget{Bus: kv.DiskBusVirtio, ReadOnly: true}}})
		vm.Spec.Template.Spec.Volumes = append(vm.Spec.Template.Spec.Volumes, kv.Volume{Name: "manual8", VolumeSource: kv.VolumeSource{PersistentVolumeClaim: &kv.PersistentVolumeClaimVolumeSource{PersistentVolumeClaimVolumeSource: corev1.PersistentVolumeClaimVolumeSource{ClaimName: "ro-pvc", ReadOnly: true}}}})
		b.fixture.intent.Disks = append(b.fixture.intent.Disks, SnapshotDisk{VolumeID: 8, DiskKey: "data-ro", CanonicalName: "manual8", Role: "data", ReadOnly: true, SourcePVC: ObjectRef{Kind: "PersistentVolumeClaim", Namespace: scope.Namespace, Name: "ro-pvc", UID: "ro-pvc-uid"}, StorageClass: "cephfs-vm-snapshot", RequestedBytes: 1 << 30})
		config, err := SnapshotSafeConfig(nativeAsVM(vm), b.fixture.intent.Disks)
		if err != nil {
			t.Fatal(err)
		}
		b.fixture.intent.ConfigHash, _ = snapshotDigest(config)
		b.fixture.core, _ = SnapshotCoreHash(scope, b.fixture.intent)
		b.fixture.snapshot.Annotations[SnapshotCoreAnnotation] = b.fixture.core
	}
	d := enginePrepare(t, snapshots, scope, b)
	_, err := snapshots.mutate(scope, d.OperationID, "fixture-native-startup", strings.Repeat("c", 64), func(c Current) (Mutation, error) {
		c.Receipt.SnapshotIdentity = &SnapshotFirmwareIdentity{VM: c.Receipt.Snapshot.SourceVM, VMResourceVersion: "1", FirmwareUUID: startup.FirmwareUUID, Pinned: true, Startup: &startup}
		return Mutation{State: c.State, Receipt: c.Receipt}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = snapshots.Halt(context.Background(), scope, engineAction("halt", d)); err != nil {
		t.Fatal(err)
	}
	ready, err := snapshots.Create(context.Background(), scope, engineAction("create", d))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = snapshots.Resume(context.Background(), scope, readyACK("resume", ready)); err != nil {
		t.Fatal(err)
	}
	if _, err = snapshots.Finalize(context.Background(), scope, readyACK("finalize", ready)); err != nil {
		t.Fatal(err)
	}
	database := snapshots.store.db
	if err = database.AutoMigrate(&dbmodel.TenantServiceVolume{}, &dbmodel.TenantServicesPort{}).Error; err != nil {
		t.Fatal(err)
	}
	if err = database.Model(&dbmodel.TenantServices{}).Where("service_id = ?", scope.ServiceID).Updates(map[string]interface{}{"container_cpu": 2000, "container_memory": 2048}).Error; err != nil {
		t.Fatal(err)
	}
	if err = database.Create(&dbmodel.TenantServiceVolume{Model: dbmodel.Model{ID: 7}, ServiceID: scope.ServiceID, VolumeName: "disk", VolumePath: "/disk", VolumeType: "cephfs-vm-snapshot", VolumeCapacity: 20, AccessMode: "ReadWriteOnce"}).Error; err != nil {
		t.Fatal(err)
	}
	for _, row := range []dbmodel.ComponentK8sAttributes{{TenantID: scope.TenantID, ComponentID: scope.ServiceID, Name: "vm_boot_mode", AttributeValue: "bios"}, {TenantID: scope.TenantID, ComponentID: scope.ServiceID, Name: "vm_disk_layout", AttributeValue: `[{"disk_key":"disk","disk_name":"manual7","disk_role":"root","device_type":"disk","source_kind":"volume","order_index":0,"boot":true,"bus":"virtio","boot_order":1}]`}} {
		v := row
		if err = database.Create(&v).Error; err != nil {
			t.Fatal(err)
		}
	}
	if ro {
		if err = database.Create(&dbmodel.TenantServiceVolume{Model: dbmodel.Model{ID: 8}, ServiceID: scope.ServiceID, VolumeName: "data-ro", VolumePath: "/disk-ro", VolumeType: "cephfs-vm-snapshot", VolumeCapacity: 1, AccessMode: "ReadOnlyMany", IsReadOnly: true}).Error; err != nil {
			t.Fatal(err)
		}
		layout := `[{"disk_key":"disk","disk_name":"manual7","disk_role":"root","device_type":"disk","source_kind":"volume","order_index":0,"boot":true,"bus":"virtio","boot_order":1},{"disk_key":"data-ro","disk_name":"manual8","disk_role":"data","device_type":"disk","source_kind":"volume","order_index":1,"boot":false,"bus":"virtio","boot_order":0}]`
		if err = database.Model(&dbmodel.ComponentK8sAttributes{}).Where("component_id = ? AND name = ?", scope.ServiceID, "vm_disk_layout").Update("attribute_value", layout).Error; err != nil {
			t.Fatal(err)
		}
	}
	current, _ := snapshots.store.ReadOnly(scope, "")
	_, err = snapshots.store.Mutate(scope, Expectation{ControlRevision: current.State.ControlRevision, BindingGeneration: current.State.BindingGeneration}, func(c Current) (Mutation, error) {
		c.State.Bindings["7"] = Binding{VolumeID: 7, DiskKey: "disk", CanonicalName: "manual7", Role: "root", Attached: true, ActivePVC: ObjectRef{Kind: "PersistentVolumeClaim", Namespace: scope.Namespace, Name: "active-pvc", UID: "pvc-uid"}}
		if ro {
			c.State.Bindings["8"] = Binding{VolumeID: 8, DiskKey: "data-ro", CanonicalName: "manual8", Role: "data", Attached: true, ActivePVC: ObjectRef{Kind: "PersistentVolumeClaim", Namespace: scope.Namespace, Name: "ro-pvc", UID: "ro-pvc-uid"}}
		}
		c.State.BindingGeneration++
		return Mutation{State: c.State}, nil
	})
	if err != nil {
		t.Fatal("materialized B fixture", err)
	}
	class := "cephfs-vm-snapshot"
	claim := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "active-pvc", Namespace: scope.Namespace, UID: "pvc-uid", Labels: map[string]string{"service_id": scope.ServiceID, "tenant_id": scope.TenantID}}, Spec: corev1.PersistentVolumeClaimSpec{StorageClassName: &class, AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}, Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("20Gi")}}}, Status: corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound}}
	core := kubefake.NewSimpleClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: scope.Namespace, UID: "namespace-uid"}}, claim, &storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: class}, Provisioner: "cephfs.csi.ceph.com"})
	if ro {
		_, err = core.CoreV1().PersistentVolumeClaims(scope.Namespace).Create(context.Background(), &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "ro-pvc", Namespace: scope.Namespace, UID: "ro-pvc-uid", Labels: map[string]string{"service_id": scope.ServiceID, "tenant_id": scope.TenantID}}, Spec: corev1.PersistentVolumeClaimSpec{StorageClassName: &class, AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany}, Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")}}}, Status: corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound}}, metav1.CreateOptions{})
		if err != nil {
			t.Fatal(err)
		}
	}
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{dataVolumeResource: "DataVolumeList", virtualMachineResource: "VirtualMachineList", virtualMachineInstanceResource: "VirtualMachineInstanceList", vmSnapshotResource: "VirtualMachineSnapshotList", vmSnapshotContentResource: "VirtualMachineSnapshotContentList", csiSnapshotResource: "VolumeSnapshotList", csiSnapshotContentResource: "VolumeSnapshotContentList", dataSourceResource: "DataSourceList", nativeLonghornVolumeResource: "VolumeList"})
	vm := nativeAsVM(b.fixture.content.Spec.Source.VirtualMachine).DeepCopy()
	vm.ResourceVersion = "1"
	strategy := kv.RunStrategyHalted
	vm.Spec.RunStrategy = &strategy
	o, _ := EncodeRuntimeVM(vm)
	if _, err = dyn.Resource(restoreVMGVR).Namespace(scope.Namespace).Create(context.Background(), o, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	backend := &managedNativeBackendFixture{nativeRestoreBackendFixture: &nativeRestoreBackendFixture{vm: vm, snap: b.fixture.snapshot.DeepCopy(), errors: map[string]error{}}, dynamic: dyn, scope: scope}
	m := &ManagedNativeRestore{Store: snapshots.store, Resolver: NewResolver(snapshots.store, core, dyn), Snapshots: snapshots, Backend: backend, Core: core, Dynamic: dyn, CurrentStartup: func(context.Context, Scope, *kv.VirtualMachine, string) (RestoreStartupPolicy, error) {
		return startup, nil
	}}
	engine := NewNativeRestoreEngine(backend)
	m.BuildPlatform = func(context.Context, Scope) (*kv.VirtualMachine, error) {
		return nativeAsVM(b.fixture.content.Spec.Source.VirtualMachine).DeepCopy(), nil
	}
	engine.Managed = m
	return managedNativeFixture{engine, m, backend, b, scope, core, dyn}
}

func (f managedNativeFixture) completeNative(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	complete := true
	f.backend.restore.Status = &native.VirtualMachineRestoreStatus{Complete: &complete, Restores: []native.VolumeRestore{{VolumeName: "manual7", PersistentVolumeClaimName: "restored-root", VolumeSnapshotName: "volume-snapshot-a"}}}
	class := "cephfs-vm-snapshot"
	group := "snapshot.storage.k8s.io"
	pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "restored-root", Namespace: f.scope.Namespace, UID: "restored-pvc-uid", Labels: map[string]string{"service_id": f.scope.ServiceID, "tenant_id": f.scope.TenantID}}, Spec: corev1.PersistentVolumeClaimSpec{VolumeName: "restored-pv", StorageClassName: &class, AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}, DataSource: &corev1.TypedLocalObjectReference{APIGroup: &group, Kind: "VolumeSnapshot", Name: "volume-snapshot-a"}, Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("20Gi")}}}, Status: corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound}}
	if _, err := f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Create(ctx, pvc, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	pv := &corev1.PersistentVolume{ObjectMeta: metav1.ObjectMeta{Name: "restored-pv", UID: "restored-pv-uid"}, Spec: corev1.PersistentVolumeSpec{PersistentVolumeSource: corev1.PersistentVolumeSource{CSI: &corev1.CSIPersistentVolumeSource{Driver: "cephfs.csi.ceph.com", VolumeHandle: "restored-volume"}}, ClaimRef: &corev1.ObjectReference{Name: pvc.Name, Namespace: pvc.Namespace, UID: pvc.UID}}}
	if _, err := f.core.CoreV1().PersistentVolumes().Create(ctx, pv, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	vm, _, _ := f.backend.VirtualMachine(ctx, f.scope)
	vm.Spec.Template.Spec.Volumes[0].PersistentVolumeClaim.ClaimName = pvc.Name
	o, _ := EncodeRuntimeVM(vm)
	if _, err := f.dynamic.Resource(restoreVMGVR).Namespace(f.scope.Namespace).Update(ctx, o, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
}

// capability_id: rainbond.vm-restore.native-platform-convergence
func TestManagedNativeRestoreRequiresPlatformCommitBeforeSuccess(t *testing.T) {
	f := newManagedNativeFixture(t)
	ctx := context.Background()
	first, err := f.engine.Start(ctx, f.scope, "snapshot-a", "restore-a")
	if err != nil || first.Complete || !first.RequiresAdvance || f.backend.creates != 1 {
		t.Fatal("first native command", first, err)
	}
	op, _ := nativeOperation(f.scope, "restore-a")
	before, _ := f.managed.Store.ReadOnly(f.scope, op)
	read, err := f.engine.Status(ctx, f.scope, "snapshot-a", "restore-a")
	after, _ := f.managed.Store.ReadOnly(f.scope, op)
	if err != nil || read.Complete || !reflect.DeepEqual(before, after) {
		t.Fatal("status mutated or claimed pending native work", read, err)
	}
	f.completeNative(t)
	read, err = f.engine.Status(ctx, f.scope, "snapshot-a", "restore-a")
	if err != nil || read.Complete || !read.RequiresAdvance {
		t.Fatal("native complete alone was platform success", read, err)
	}
	var result NativeRestoreDTO
	for i := 0; i < 3; i++ {
		result, err = f.engine.Start(ctx, f.scope, "snapshot-a", "restore-a")
		if err != nil {
			t.Fatal("advance", i, err)
		}
		if result.Complete {
			break
		}
	}
	if !result.Complete || result.RequiresAdvance || f.backend.creates != 1 {
		t.Fatal("did not converge exactly one restore", result, f.backend.creates)
	}
	state, err := f.managed.Store.ReadOnly(f.scope, op)
	if err != nil || state.State.Fence != nil || state.State.Bindings["7"].ActivePVC.Name != "restored-root" {
		t.Fatal("native bindings not adopted", state, err)
	}
	selection, err := f.managed.Store.ReadRestoredRuntimeSelection(ctx, f.managed.Resolver, f.scope)
	if err != nil || selection == nil || selection.Bindings["7"].ActivePVC.UID != "restored-pvc-uid" {
		t.Fatal("normal power renderer lacks restored selection", err)
	}
	vm, _, _ := f.backend.VirtualMachine(ctx, f.scope)
	vm.Spec.Template.Spec.Volumes[0].PersistentVolumeClaim.ClaimName = "active-pvc"
	projected, err := ProjectRestoredRuntime(ctx, f.core, vm, *selection)
	if err != nil || projected.Spec.Template.Spec.Volumes[1].PersistentVolumeClaim == nil && projected.Spec.Template.Spec.Volumes[0].PersistentVolumeClaim == nil {
		t.Fatal(err)
	}
	found := false
	for _, v := range projected.Spec.Template.Spec.Volumes {
		if v.Name == "manual7" && v.PersistentVolumeClaim != nil {
			found = v.PersistentVolumeClaim.ClaimName == "restored-root"
		}
	}
	if !found {
		t.Fatal("later ordinary render selected old data")
	}
	// A complete result remains a read-only durable result after ordinary start.
	vm, _, _ = f.backend.VirtualMachine(ctx, f.scope)
	running := kv.RunStrategyAlways
	vm.Spec.RunStrategy = &running
	o, _ := EncodeRuntimeVM(vm)
	_, _ = f.dynamic.Resource(restoreVMGVR).Namespace(f.scope.Namespace).Update(ctx, o, metav1.UpdateOptions{})
	f.backend.vmi = &kv.VirtualMachineInstance{ObjectMeta: metav1.ObjectMeta{Name: vm.Name, Namespace: vm.Namespace, UID: "new-vmi"}}
	if result, err = f.engine.Status(ctx, f.scope, "snapshot-a", "restore-a"); err != nil || !result.Complete {
		t.Fatal("complete read rejected ordinary start", result, err)
	}
	if result, err = f.engine.Start(ctx, f.scope, "snapshot-a", "restore-a"); err != nil || !result.Complete || f.backend.creates != 1 {
		t.Fatal("replay re-ran completed restore", result, err)
	}
}

// capability_id: rainbond.vm-restore.native-platform-convergence
func TestManagedNativeRestoreRejectsUnprovenNativeAndDiskResults(t *testing.T) {
	for _, mode := range []string{"wrong snapshot datasource", "PV claim UID", "foreign PVC", "missing restore disk", "duplicate restore disk", "pending PVC", "wrong driver", "VM mount mismatch"} {
		t.Run(mode, func(t *testing.T) {
			f := newManagedNativeFixture(t)
			ctx := context.Background()
			if _, err := f.engine.Start(ctx, f.scope, "snapshot-a", "restore-a"); err != nil {
				t.Fatal(err)
			}
			f.completeNative(t)
			pvc, _ := f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Get(ctx, "restored-root", metav1.GetOptions{})
			pv, _ := f.core.CoreV1().PersistentVolumes().Get(ctx, "restored-pv", metav1.GetOptions{})
			switch mode {
			case "wrong snapshot datasource":
				pvc.Spec.DataSource.Name = "other"
			case "PV claim UID":
				pv.Spec.ClaimRef.UID = "other"
			case "foreign PVC":
				pvc.Labels["tenant_id"] = "foreign"
				pvc.Labels["service_id"] = "foreign"
			case "missing restore disk":
				f.backend.restore.Status.Restores = nil
			case "duplicate restore disk":
				f.backend.restore.Status.Restores = append(f.backend.restore.Status.Restores, f.backend.restore.Status.Restores[0])
			case "pending PVC":
				pvc.Status.Phase = corev1.ClaimPending
			case "wrong driver":
				pv.Spec.CSI.Driver = "other-driver"
			case "VM mount mismatch":
				vm, _, _ := f.backend.VirtualMachine(ctx, f.scope)
				vm.Spec.Template.Spec.Volumes[0].PersistentVolumeClaim.ClaimName = "other-claim"
				o, _ := EncodeRuntimeVM(vm)
				_, _ = f.dynamic.Resource(restoreVMGVR).Namespace(f.scope.Namespace).Update(ctx, o, metav1.UpdateOptions{})
			}
			_, _ = f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Update(ctx, pvc, metav1.UpdateOptions{})
			_, _ = f.core.CoreV1().PersistentVolumes().Update(ctx, pv, metav1.UpdateOptions{})
			result, err := f.engine.Start(ctx, f.scope, "snapshot-a", "restore-a")
			if err == nil || result.Complete {
				t.Fatal("unproven restored data accepted", mode, result, err)
			}
			op, _ := nativeOperation(f.scope, "restore-a")
			state, _ := f.managed.Store.ReadOnly(f.scope, op)
			if state.State.Fence == nil || state.Receipt.Steps[NativeRestoreCommitStep].Applied {
				t.Fatal("bad native result committed/released protection")
			}
		})
	}
}

// capability_id: rainbond.vm-restore.native-platform-convergence
func TestManagedNativeRestorePreflightAndCapacityAreFailClosed(t *testing.T) {
	for _, mode := range []string{"running VM", "unready snapshot", "missing startup", "unacked snapshot", "unqualified provider", "enlarged logical quota"} {
		t.Run(mode, func(t *testing.T) {
			f := newManagedNativeFixture(t)
			ctx := context.Background()
			switch mode {
			case "running VM":
				f.backend.vmi = &kv.VirtualMachineInstance{}
			case "unready snapshot":
				ready := false
				f.snapshots.fixture.snapshot.Status.ReadyToUse = &ready
			case "missing startup":
				f.managed.CurrentStartup = nil
			case "unacked snapshot":
				r, _ := f.managed.Store.ReadOnly(f.scope, "operation-a")
				key, _ := ReceiptKey("operation-a")
				r.Receipt.ConsoleAckHash = ""
				value, _ := EncodeRecord(r.Receipt)
				f.managed.Store.db.Model(&dbmodel.ComponentK8sAttributes{}).Where("component_id = ? AND name = ?", f.scope.ServiceID, key).Update("attribute_value", value)
			case "unqualified provider":
				f.managed.Core = nil
			case "enlarged logical quota":
				f.managed.Store.db.Model(&dbmodel.TenantServiceVolume{}).Where("ID = ?", 7).Update("volume_capacity", 80)
			}
			if _, err := f.engine.Start(ctx, f.scope, "snapshot-a", "restore-a"); err == nil || f.backend.creates != 0 {
				t.Fatal("unqualified native preflight mutated", mode, err)
			}
		})
	}
	t.Run("CDI overhead accepted with logical quota sealed", func(t *testing.T) {
		f := newManagedNativeFixture(t)
		dto, err := f.managed.Snapshots.ByName(context.Background(), f.scope, "snapshot-a")
		if err != nil {
			t.Fatal(err)
		}
		before, err := f.managed.Store.selectedRuntimeConfig(f.scope)
		if err != nil {
			t.Fatal(err)
		}
		content := f.snapshots.fixture.content.DeepCopy()
		content.Spec.VolumeBackups[0].PersistentVolumeClaim.Spec.Resources.Requests[corev1.ResourceStorage] = *resource.NewQuantity((20<<30)+98765, resource.BinarySI)
		config, err := nativeConfig(f.scope, dto, content, before)
		if err != nil || config.Volumes[0].CapacityGiB != 20 {
			t.Fatal("CDI byte overhead rejected or rounded logical capacity", config, err)
		}
	})
	t.Run("no operation is not found", func(t *testing.T) {
		f := newManagedNativeFixture(t)
		if _, err := f.engine.Status(context.Background(), f.scope, "snapshot-a", "none"); !errors.Is(err, ErrNotFound) {
			t.Fatal(err)
		}
	})
}

// capability_id: rainbond.vm-restore.native-platform-convergence
func TestManagedNativeRestorePinsNativeUIDAndUnknownOutcomes(t *testing.T) {
	ctx := context.Background()
	for _, when := range []string{"before submission", "after submission"} {
		t.Run("uncertain create/"+when, func(t *testing.T) {
			f := newManagedNativeFixture(t)
			if when == "before submission" {
				f.backend.createErrorBefore = errors.New("transport failure")
			} else {
				f.backend.createErrorAfter = errors.New("response lost")
			}
			d, err := f.engine.Start(ctx, f.scope, "snapshot-a", "restore-a")
			if err != nil || !d.RequiresAdvance || d.Complete {
				t.Fatal("uncertain result not retained", d, err)
			}
			f.backend.createErrorBefore = nil
			f.backend.createErrorAfter = nil
			d, err = f.engine.Start(ctx, f.scope, "snapshot-a", "restore-a")
			if err != nil || d.Complete {
				t.Fatal("unknown create replay", d, err)
			}
			want := 0
			if when == "after submission" {
				want = 1
			}
			if f.backend.creates != want {
				t.Fatal("uncertain create was blindly repeated", f.backend.creates, want)
			}
			op, _ := nativeOperation(f.scope, "restore-a")
			state, _ := f.managed.Store.ReadOnly(f.scope, op)
			if state.State.Fence == nil {
				t.Fatal("unknown outcome lost ownership")
			}
		})
	}
	for _, mode := range []string{"native UID", "native nonce", "VM UID", "native target", "snapshot UID", "snapshot read failure", "VM read failure", "restore read failure"} {
		t.Run(mode, func(t *testing.T) {
			f := newManagedNativeFixture(t)
			if _, err := f.engine.Start(ctx, f.scope, "snapshot-a", "restore-a"); err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "native UID":
				f.backend.restore.UID = "replacement-restore"
			case "native nonce":
				f.backend.restore.Annotations["rainbond.io/vm-native-restore-attempt"] = "other"
			case "VM UID":
				vm, _, _ := f.backend.VirtualMachine(ctx, f.scope)
				vm.UID = "replacement-vm"
				o, _ := EncodeRuntimeVM(vm)
				_, _ = f.dynamic.Resource(restoreVMGVR).Namespace(f.scope.Namespace).Update(ctx, o, metav1.UpdateOptions{})
			case "native target":
				f.backend.restore.Spec.Target.Name = "other"
			case "snapshot UID":
				f.backend.snap.UID = "replacement-snapshot"
			case "snapshot read failure":
				f.backend.errors["snapshot"] = errors.New("failure")
			case "VM read failure":
				f.backend.errors["vm"] = errors.New("failure")
			case "restore read failure":
				f.backend.errors["restore"] = errors.New("failure")
			}
			if _, err := f.engine.Status(ctx, f.scope, "snapshot-a", "restore-a"); err == nil {
				t.Fatal("read accepted replacement/unavailable native identity", mode)
			}
			if _, err := f.engine.Start(ctx, f.scope, "snapshot-a", "restore-a"); err == nil || f.backend.creates != 1 {
				t.Fatal("replay accepted replacement/unavailable identity", mode, err)
			}
		})
	}
	t.Run("claimed but absent never repeats create", func(t *testing.T) {
		f := newManagedNativeFixture(t)
		if _, err := f.engine.Start(ctx, f.scope, "snapshot-a", "restore-a"); err != nil {
			t.Fatal(err)
		}
		f.backend.restore = nil
		if _, err := f.engine.Start(ctx, f.scope, "snapshot-a", "restore-a"); err == nil || f.backend.creates != 1 {
			t.Fatal("known native UID disappearance was ignored", err)
		}
	})
	t.Run("native failure retains platform fence", func(t *testing.T) {
		f := newManagedNativeFixture(t)
		if _, err := f.engine.Start(ctx, f.scope, "snapshot-a", "restore-a"); err != nil {
			t.Fatal(err)
		}
		f.backend.restore.Status = &native.VirtualMachineRestoreStatus{Conditions: []native.Condition{{Type: native.ConditionFailure, Status: corev1.ConditionTrue}}}
		d, err := f.engine.Status(ctx, f.scope, "snapshot-a", "restore-a")
		if err != nil || d.Phase != "failed" || d.Complete || d.RequiresAdvance {
			t.Fatal("failed native projection", d, err)
		}
		op, _ := nativeOperation(f.scope, "restore-a")
		c, _ := f.managed.Store.ReadOnly(f.scope, op)
		if c.State.Fence == nil {
			t.Fatal("failed native operation released unverified state")
		}
	})
	for _, when := range []string{"before native", "before commit"} {
		t.Run("active consumer/"+when, func(t *testing.T) {
			f := newManagedNativeFixture(t)
			claim := "active-pvc"
			if when == "before commit" {
				if _, err := f.engine.Start(ctx, f.scope, "snapshot-a", "restore-a"); err != nil {
					t.Fatal(err)
				}
				f.completeNative(t)
				claim = "restored-root"
			}
			_, err := f.core.CoreV1().Pods(f.scope.Namespace).Create(ctx, &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "consumer", Namespace: f.scope.Namespace}, Spec: corev1.PodSpec{Volumes: []corev1.Volume{{Name: "disk", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: claim}}}}}, Status: corev1.PodStatus{Phase: corev1.PodRunning}}, metav1.CreateOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = f.engine.Start(ctx, f.scope, "snapshot-a", "restore-a"); !errors.Is(err, ErrBusy) {
				t.Fatal("active writer not blocked", err)
			}
		})
	}
	t.Run("restored CDI owner remains authoritative", func(t *testing.T) {
		f := newManagedNativeFixture(t)
		if _, err := f.engine.Start(ctx, f.scope, "snapshot-a", "restore-a"); err != nil {
			t.Fatal(err)
		}
		f.completeNative(t)
		dv := &unstructured.Unstructured{Object: map[string]interface{}{"apiVersion": "cdi.kubevirt.io/v1beta1", "kind": "DataVolume", "metadata": map[string]interface{}{"name": "restored-dv", "namespace": f.scope.Namespace, "uid": "restored-dv-uid", "labels": map[string]interface{}{"service_id": f.scope.ServiceID, "tenant_id": f.scope.TenantID}}, "status": map[string]interface{}{"claimName": "restored-root", "phase": "Succeeded"}}}
		_, err := f.dynamic.Resource(dataVolumeResource).Namespace(f.scope.Namespace).Create(ctx, dv, metav1.CreateOptions{})
		if err != nil {
			t.Fatal(err)
		}
		pvc, _ := f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Get(ctx, "restored-root", metav1.GetOptions{})
		pvc.OwnerReferences = []metav1.OwnerReference{{Kind: "DataVolume", Name: dv.GetName(), UID: dv.GetUID()}}
		_, _ = f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Update(ctx, pvc, metav1.UpdateOptions{})
		vm, _, _ := f.backend.VirtualMachine(ctx, f.scope)
		vm.Spec.Template.Spec.Volumes[0].PersistentVolumeClaim = nil
		vm.Spec.Template.Spec.Volumes[0].DataVolume = &kv.DataVolumeSource{Name: dv.GetName()}
		vm.Spec.DataVolumeTemplates = []kv.DataVolumeTemplateSpec{{ObjectMeta: metav1.ObjectMeta{Name: dv.GetName()}}}
		o, _ := EncodeRuntimeVM(vm)
		_, _ = f.dynamic.Resource(restoreVMGVR).Namespace(f.scope.Namespace).Update(ctx, o, metav1.UpdateOptions{})
		var d NativeRestoreDTO
		for i := 0; i < 3; i++ {
			d, err = f.engine.Start(ctx, f.scope, "snapshot-a", "restore-a")
			if err != nil {
				t.Fatal(err)
			}
			if d.Complete {
				break
			}
		}
		if !d.Complete {
			t.Fatal("CDI restore did not converge", d)
		}
		view, err := f.managed.Resolver.ReadBindingView(ctx, f.scope)
		if err != nil || len(view.Volumes) != 1 || view.Volumes[0].ActiveDV == nil || view.Volumes[0].ActiveDV.UID != "restored-dv-uid" {
			t.Fatal("restored CDI identity not pinned", view, err)
		}
	})
	t.Run("invalid and conflicting public identity", func(t *testing.T) {
		f := newManagedNativeFixture(t)
		if _, err := f.engine.Start(ctx, f.scope, "snapshot-a", "../other"); err == nil {
			t.Fatal("invalid request accepted")
		}
		if _, err := f.engine.Start(ctx, f.scope, "snapshot-a", "restore-a"); err != nil {
			t.Fatal(err)
		}
		if _, err := f.engine.Status(ctx, f.scope, "other-snapshot", "restore-a"); !errors.Is(err, ErrConflict) {
			t.Fatal("same key changed source", err)
		}
	})
}

// capability_id: rainbond.vm-restore.native-platform-convergence
func TestManagedNativeRestorePreservesUnknownFieldsAndRejectsUnknownDiskReplacement(t *testing.T) {
	for _, diskField := range []bool{false, true} {
		t.Run(map[bool]string{false: "unrelated future field survives", true: "unknown disk subtree rejected"}[diskField], func(t *testing.T) {
			f := newManagedNativeFixture(t)
			ctx := context.Background()
			if _, err := f.engine.Start(ctx, f.scope, "snapshot-a", "restore-a"); err != nil {
				t.Fatal(err)
			}
			f.completeNative(t)
			raw, _ := f.dynamic.Resource(restoreVMGVR).Namespace(f.scope.Namespace).Get(ctx, f.backend.vm.Name, metav1.GetOptions{})
			if diskField {
				disks, _, _ := unstructured.NestedSlice(raw.Object, "spec", "template", "spec", "domain", "devices", "disks")
				disks[0].(map[string]interface{})["futureDiskFeature"] = true
				_ = unstructured.SetNestedSlice(raw.Object, disks, "spec", "template", "spec", "domain", "devices", "disks")
			} else {
				_ = unstructured.SetNestedField(raw.Object, true, "spec", "template", "spec", "domain", "devices", "futureDeviceFeature")
			}
			annotations := raw.GetAnnotations()
			if annotations == nil {
				annotations = map[string]string{}
			}
			annotations["user.example/keep"] = "keep-me"
			raw.SetAnnotations(annotations)
			_, err := f.dynamic.Resource(restoreVMGVR).Namespace(f.scope.Namespace).Update(ctx, raw, metav1.UpdateOptions{})
			if err != nil {
				t.Fatal(err)
			}
			var d NativeRestoreDTO
			for i := 0; i < 3; i++ {
				d, err = f.engine.Start(ctx, f.scope, "snapshot-a", "restore-a")
				if err != nil || d.Complete {
					break
				}
			}
			if diskField {
				if !errors.Is(err, ErrRestoreCapability) || d.Complete {
					t.Fatal("unrepresentable disk rewrite accepted", d, err)
				}
			} else {
				if err != nil || !d.Complete {
					t.Fatal("preserving update did not converge", d, err)
				}
			}
			after, _ := f.dynamic.Resource(restoreVMGVR).Namespace(f.scope.Namespace).Get(ctx, f.backend.vm.Name, metav1.GetOptions{})
			if after.GetAnnotations()["user.example/keep"] != "keep-me" {
				t.Fatal("user metadata lost")
			}
			if diskField {
				beforeSpec, _, _ := unstructured.NestedMap(raw.Object, "spec")
				afterSpec, _, _ := unstructured.NestedMap(after.Object, "spec")
				if !reflect.DeepEqual(beforeSpec, afterSpec) {
					t.Fatal("unsupported rewrite changed spec")
				}
			} else {
				value, found, _ := unstructured.NestedBool(after.Object, "spec", "template", "spec", "domain", "devices", "futureDeviceFeature")
				if !found || !value {
					t.Fatal("future API field silently deleted")
				}
			}
		})
	}
}

// capability_id: rainbond.vm-restore.native-platform-convergence
func TestManagedNativeRestoreKeepsReadonlyIdentityAndRefusesExtraDisks(t *testing.T) {
	ctx := context.Background()
	t.Run("changed hardware requires separate qualified rollback", func(t *testing.T) {
		f := newManagedNativeFixture(t)
		if err := f.managed.Store.db.Model(&dbmodel.TenantServices{}).Where("service_id = ?", f.scope.ServiceID).Update("container_cpu", 4000).Error; err != nil {
			t.Fatal(err)
		}
		vm, _, _ := f.backend.VirtualMachine(ctx, f.scope)
		vm.Spec.Template.Spec.Domain.CPU = &kv.CPU{Sockets: 1, Cores: 4, Threads: 1}
		object, _ := EncodeRuntimeVM(vm)
		_, _ = f.dynamic.Resource(restoreVMGVR).Namespace(f.scope.Namespace).Update(ctx, object, metav1.UpdateOptions{})
		if _, err := f.engine.Start(ctx, f.scope, "snapshot-a", "restore-a"); !errors.Is(err, ErrNativeRestoreConfigurationChanged) || f.backend.creates != 0 {
			t.Fatal("cross-configuration native restore bypassed Console ACK boundary", err)
		}
	})
	t.Run("root and readonly data", func(t *testing.T) {
		f := newManagedNativeFixture(t, true)
		if _, err := f.engine.Start(ctx, f.scope, "snapshot-a", "restore-a"); err != nil {
			t.Fatal(err)
		}
		f.completeNative(t)
		var d NativeRestoreDTO
		var err error
		for i := 0; i < 3; i++ {
			d, err = f.engine.Start(ctx, f.scope, "snapshot-a", "restore-a")
			if err != nil {
				t.Fatal(err)
			}
			if d.Complete {
				break
			}
		}
		if !d.Complete {
			t.Fatal("readonly volume without backup blocked restore", d)
		}
		op, _ := nativeOperation(f.scope, "restore-a")
		c, _ := f.managed.Store.ReadOnly(f.scope, op)
		if c.State.Bindings["8"].ActivePVC.UID != "ro-pvc-uid" || !c.State.Bindings["8"].Attached {
			t.Fatal("readonly backing identity was replaced")
		}
	})
	t.Run("readonly backing mismatch", func(t *testing.T) {
		f := newManagedNativeFixture(t, true)
		if _, err := f.engine.Start(ctx, f.scope, "snapshot-a", "restore-a"); err != nil {
			t.Fatal(err)
		}
		f.completeNative(t)
		vm, _, _ := f.backend.VirtualMachine(ctx, f.scope)
		for i := range vm.Spec.Template.Spec.Volumes {
			if vm.Spec.Template.Spec.Volumes[i].Name == "manual8" {
				vm.Spec.Template.Spec.Volumes[i].PersistentVolumeClaim.ClaimName = "other-readonly"
			}
		}
		o, _ := EncodeRuntimeVM(vm)
		_, _ = f.dynamic.Resource(restoreVMGVR).Namespace(f.scope.Namespace).Update(ctx, o, metav1.UpdateOptions{})
		if d, err := f.engine.Start(ctx, f.scope, "snapshot-a", "restore-a"); err == nil || d.Complete {
			t.Fatal("readonly identity mismatch accepted", d, err)
		}
	})
	t.Run("B only writable disk rejected before native RPC", func(t *testing.T) {
		f := newManagedNativeFixture(t)
		db := f.managed.Store.db
		if err := db.Create(&dbmodel.TenantServiceVolume{Model: dbmodel.Model{ID: 8}, ServiceID: f.scope.ServiceID, VolumeName: "data-b", VolumePath: "/disk-b", VolumeType: "cephfs-vm-snapshot", VolumeCapacity: 1, AccessMode: "ReadWriteOnce"}).Error; err != nil {
			t.Fatal(err)
		}
		class := "cephfs-vm-snapshot"
		_, err := f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Create(ctx, &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "manual8", Namespace: f.scope.Namespace, UID: "b-only-pvc", Labels: map[string]string{"service_id": f.scope.ServiceID}}, Spec: corev1.PersistentVolumeClaimSpec{StorageClassName: &class, Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")}}}, Status: corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound}}, metav1.CreateOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if _, err = f.engine.Start(ctx, f.scope, "snapshot-a", "restore-a"); !errors.Is(err, ErrRestoreCapability) || f.backend.creates != 0 {
			t.Fatal("native operation risked B-only disk", err)
		}
	})
	t.Run("contradictory complete and failure is never success", func(t *testing.T) {
		f := newManagedNativeFixture(t)
		if _, err := f.engine.Start(ctx, f.scope, "snapshot-a", "restore-a"); err != nil {
			t.Fatal(err)
		}
		f.completeNative(t)
		f.backend.restore.Status.Conditions = []native.Condition{{Type: native.ConditionFailure, Status: corev1.ConditionTrue}}
		for _, command := range []bool{false, true} {
			var d NativeRestoreDTO
			var err error
			if command {
				d, err = f.engine.Start(ctx, f.scope, "snapshot-a", "restore-a")
			} else {
				d, err = f.engine.Status(ctx, f.scope, "snapshot-a", "restore-a")
			}
			if err != nil || d.Phase != "failed" || d.Complete {
				t.Fatal("contradictory native result became success", d, err)
			}
		}
		op, _ := nativeOperation(f.scope, "restore-a")
		c, _ := f.managed.Store.ReadOnly(f.scope, op)
		if c.State.Fence == nil || c.Receipt.Steps[NativeRestoreCommitStep].Applied {
			t.Fatal("failed native result committed/released fence")
		}
	})
}

// capability_id: rainbond.vm-restore.native-platform-convergence
func TestManagedNativeRestoreRejectsDesiredAndLiveAdvancedDrift(t *testing.T) {
	ctx := context.Background()
	for _, mode := range []string{"node selector applied", "node selector DB only", "affinity DB only", "DNS DB only", "network applied", "unknown network field"} {
		t.Run(mode, func(t *testing.T) {
			f := newManagedNativeFixture(t)
			f.managed.BuildPlatform = func(context.Context, Scope) (*kv.VirtualMachine, error) {
				desired := nativeAsVM(f.snapshots.fixture.content.Spec.Source.VirtualMachine).DeepCopy()
				var row dbmodel.ComponentK8sAttributes
				err := f.managed.Store.db.Where("component_id = ? AND name = ?", f.scope.ServiceID, "nodeSelector").First(&row).Error
				if err == nil {
					if err = json.Unmarshal([]byte(row.AttributeValue), &desired.Spec.Template.Spec.NodeSelector); err != nil {
						return nil, err
					}
				}
				if mode == "affinity DB only" {
					desired.Spec.Template.Spec.Affinity = &corev1.Affinity{NodeAffinity: &corev1.NodeAffinity{RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{{MatchExpressions: []corev1.NodeSelectorRequirement{{Key: "pool", Operator: corev1.NodeSelectorOpIn, Values: []string{"b"}}}}}}}}
				}
				if mode == "DNS DB only" {
					desired.Spec.Template.Spec.DNSPolicy = corev1.DNSNone
					desired.Spec.Template.Spec.DNSConfig = &corev1.PodDNSConfig{Nameservers: []string{"192.0.2.53"}}
				}
				return desired, nil
			}
			if strings.HasPrefix(mode, "node selector") {
				if err := f.managed.Store.db.Create(&dbmodel.ComponentK8sAttributes{TenantID: f.scope.TenantID, ComponentID: f.scope.ServiceID, Name: "nodeSelector", AttributeValue: `{"pool":"b"}`}).Error; err != nil {
					t.Fatal(err)
				}
			}
			if mode == "node selector applied" || mode == "network applied" || mode == "unknown network field" {
				vm, _, _ := f.backend.VirtualMachine(ctx, f.scope)
				if mode == "node selector applied" {
					vm.Spec.Template.Spec.NodeSelector = map[string]string{"pool": "b"}
				} else {
					vm.Spec.Template.Spec.Networks = []kv.Network{{Name: "default", NetworkSource: kv.NetworkSource{Pod: &kv.PodNetwork{}}}}
					vm.Spec.Template.Spec.Domain.Devices.Interfaces = []kv.Interface{{Name: "default", Model: "virtio", MacAddress: "02:00:00:00:00:22"}}
				}
				o, _ := EncodeRuntimeVM(vm)
				if mode == "unknown network field" {
					f.snapshots.fixture.content.Spec.Source.VirtualMachine.Spec.Template.Spec.Networks = vm.Spec.Template.Spec.Networks
					f.snapshots.fixture.content.Spec.Source.VirtualMachine.Spec.Template.Spec.Domain.Devices.Interfaces = vm.Spec.Template.Spec.Domain.Devices.Interfaces
					interfaces, _, _ := unstructured.NestedSlice(o.Object, "spec", "template", "spec", "domain", "devices", "interfaces")
					interfaces[0].(map[string]interface{})["futureNetworkFeature"] = true
					_ = unstructured.SetNestedSlice(o.Object, interfaces, "spec", "template", "spec", "domain", "devices", "interfaces")
				}
				_, _ = f.dynamic.Resource(restoreVMGVR).Namespace(f.scope.Namespace).Update(ctx, o, metav1.UpdateOptions{})
			}
			if _, err := f.engine.Start(ctx, f.scope, "snapshot-a", "restore-a"); !errors.Is(err, ErrNativeRestoreConfigurationChanged) || f.backend.creates != 0 {
				t.Fatal("advanced desired/live configuration drift reached native restore", mode, err)
			}
		})
	}
}

// capability_id: rainbond.vm-restore.native-platform-convergence
func TestManagedNativeAdvancedDefaultsDoNotHideRealEdits(t *testing.T) {
	ctx := context.Background()
	for _, mode := range []string{"producer label differs", "probe defaults differ", "stable label changes", "probe target changes"} {
		t.Run(mode, func(t *testing.T) {
			f := newManagedNativeFixture(t)
			source := f.snapshots.fixture.content.Spec.Source.VirtualMachine
			source.Spec.Template.ObjectMeta.Labels = map[string]string{"creater_id": "producer-t1", "user.example/stable": "keep"}
			probe := &kv.Probe{Handler: kv.Handler{TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromInt(80)}}}
			kv.SetDefaults_Probe(probe)
			source.Spec.Template.Spec.ReadinessProbe = probe.DeepCopy()
			live, _, _ := f.backend.VirtualMachine(ctx, f.scope)
			live.Spec.Template.ObjectMeta.Labels = map[string]string{"creater_id": "producer-t1", "user.example/stable": "keep"}
			live.Spec.Template.Spec.ReadinessProbe = probe.DeepCopy()
			object, _ := EncodeRuntimeVM(live)
			_, _ = f.dynamic.Resource(restoreVMGVR).Namespace(f.scope.Namespace).Update(ctx, object, metav1.UpdateOptions{})
			desired := nativeRestoreSourceVM(source)
			desired.Spec.Template.ObjectMeta.Labels["creater_id"] = "producer-t2"
			if mode != "producer label differs" {
				desired.Spec.Template.Spec.ReadinessProbe = &kv.Probe{Handler: kv.Handler{TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromInt(80)}}}
			}
			if mode == "stable label changes" {
				desired.Spec.Template.ObjectMeta.Labels["user.example/stable"] = "changed"
			}
			if mode == "probe target changes" {
				desired.Spec.Template.Spec.ReadinessProbe.TCPSocket.Port = intstr.FromInt(81)
			}
			f.managed.BuildPlatform = func(context.Context, Scope) (*kv.VirtualMachine, error) { return desired, nil }
			_, err := f.engine.Start(ctx, f.scope, "snapshot-a", "restore-a")
			if mode == "stable label changes" || mode == "probe target changes" {
				if !errors.Is(err, ErrNativeRestoreConfigurationChanged) || f.backend.creates != 0 {
					t.Fatal("real advanced edit was hidden", mode, err)
				}
			} else {
				if err != nil || f.backend.creates != 1 {
					t.Fatal("normal producer/defaulted probe blocked same configuration", mode, err)
				}
			}
			if source.Spec.Template.ObjectMeta.Labels["creater_id"] != "producer-t1" || desired.Spec.Template.ObjectMeta.Labels["creater_id"] != "producer-t2" {
				t.Fatal("comparison mutated raw labels")
			}
			if mode != "producer label differs" && desired.Spec.Template.Spec.ReadinessProbe.TimeoutSeconds != 0 {
				t.Fatal("comparison defaulted caller probe in place")
			}
		})
	}
}
