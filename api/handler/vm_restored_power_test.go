package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	apimodel "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/api/util/bcode"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/pkg/vmrestore"
	"github.com/goodrain/rainbond/pkg/vmrestore/restoreplan"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
	kv "kubevirt.io/api/core/v1"
	"kubevirt.io/client-go/kubecli"
)

func powerDigest(t *testing.T, value interface{}) string {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func restoredPowerFixture(t *testing.T) (*ServiceAction, *vmrestore.Store, *vmrestore.Resolver, vmrestore.Scope, *dynamicfake.FakeDynamicClient) {
	t.Helper()
	a, _, database, scope := snapshotFacadeFixture(t)
	if err := database.AutoMigrate(&dbmodel.TenantServiceVolume{}, &dbmodel.TenantServicesPort{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Model(&dbmodel.TenantServices{}).Where("service_id = ?", scope.ServiceID).Updates(map[string]interface{}{"container_cpu": 1000, "container_memory": 1024}).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Create(&dbmodel.TenantServiceVolume{Model: dbmodel.Model{ID: 7}, ServiceID: scope.ServiceID, VolumeName: "disk", VolumePath: "/disk", VolumeCapacity: 1, VolumeType: "cephfs-vm-snapshot", AccessMode: "ReadWriteMany"}).Error; err != nil {
		t.Fatal(err)
	}
	before := vmrestore.RestoreConfig{Hardware: vmrestore.SnapshotConfig{CPUMilli: 1000, MemoryMiB: 1024, BootMode: "bios", DiskLayout: []vmrestore.SnapshotLayout{{VolumeID: 7, DiskKey: "disk", CanonicalName: "manual7", Role: "root", Device: "disk", Bus: "virtio", BootOrder: 1}}}, Volumes: []vmrestore.RestoreVolumeConfig{{VolumeID: 7, DiskKey: "disk", CapacityGiB: 1, StorageClass: "cephfs-vm-snapshot", AccessMode: "ReadWriteMany"}}}
	after := before
	after.Hardware = before.Hardware
	after.Hardware.CPUMilli = 2000
	after.Hardware.MemoryMiB = 2048
	for _, attr := range []dbmodel.ComponentK8sAttributes{{TenantID: scope.TenantID, ComponentID: scope.ServiceID, Name: "vm_boot_mode", AttributeValue: "bios"}, {TenantID: scope.TenantID, ComponentID: scope.ServiceID, Name: "vm_disk_layout", AttributeValue: `[{"disk_key":"disk","disk_name":"manual7","disk_role":"root","device_type":"disk","source_kind":"volume","order_index":0,"boot":true,"bus":"virtio","boot_order":1}]`}} {
		row := attr
		if err := database.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	store := vmrestore.NewStore(database)
	manifest := strings.Repeat("a", 64)
	configHash, _ := vmrestore.RestoreConfigHash(after)
	oldRef := vmrestore.ObjectRef{Kind: "PersistentVolumeClaim", Namespace: scope.Namespace, Name: "manual7", UID: "b-root-uid"}
	old := map[string]vmrestore.Binding{"7": {VolumeID: 7, DiskKey: "disk", CanonicalName: "manual7", Role: "root", Attached: true, ActivePVC: oldRef}}
	intent := vmrestore.RestoreIntent{CurrentVM: vmrestore.ObjectRef{Kind: "VirtualMachine", Namespace: scope.Namespace, Name: "vm", UID: "vm-uid"}, SourceSnapshot: vmrestore.ObjectRef{Kind: "VirtualMachineSnapshot", Namespace: scope.Namespace, Name: "snapshot", UID: "snapshot-uid"}, SourceContent: vmrestore.ObjectRef{Kind: "VirtualMachineSnapshotContent", Namespace: scope.Namespace, Name: "content", UID: "content-uid"}, SourceVMUID: "a-source-vm", SourceCoreHash: manifest, SourceDocumentHash: manifest, Before: before, ConfigAfterHash: configHash, PowerAfter: "stopped"}
	expect := vmrestore.Expectation{OperationID: "power-restore", ManifestHash: manifest, Step: "restore-prepare"}
	if _, err := store.PrepareRestoreDB(scope, expect, intent, old); err != nil {
		t.Fatal(err)
	}
	startup := vmrestore.RestoreStartupPolicy{FirmwareUUID: "11111111-1111-4111-8111-111111111111", BootstrapMode: "restored", NoCloudVolumeName: "platform-cloudinit", NoCloudDiskName: "platform-cloudinit", DataSourceKind: "CloudInitNoCloud", ProducerUID: "fixture-producer", GeneratedMedia: []vmrestore.RestoreGeneratedMedia{}}
	prior := startup
	prior.BootstrapMode = "platform"
	frame := vmrestore.RestoreResourcePlan{Scope: scope, OperationID: expect.OperationID, CoreHash: manifest, WorkerImage: "registry.invalid/copy@sha256:" + strings.Repeat("b", 64), RuntimeImageID: "containerd://sha256:" + strings.Repeat("b", 64), DeadlineSeconds: 60, MaxDisks: 8, TargetUID: 107, TargetGID: 107, Disks: []vmrestore.RestoreDiskResource{{VolumeID: 7, SnapshotContent: vmrestore.ObjectRef{Kind: "VolumeSnapshotContent", Name: "csi-content", UID: "csi-content-uid"}, Snapshot: vmrestore.ObjectRef{Kind: "VolumeSnapshot", Namespace: scope.Namespace, Name: "csi-snapshot", UID: "csi-snapshot-uid"}, PreviousPVC: oldRef, ROXClass: "cephfs-rox", ROXClassUID: "rox-class", TargetClass: "cephfs-vm-snapshot", TargetClassUID: "target-class", TargetAccessMode: "ReadWriteMany", ROXRequestBytes: 1 << 30, TargetRequestBytes: 1 << 30, PreviousMaxBytes: 1 << 30}}}
	frame.FrameHash, _ = vmrestore.RestoreResourceFrameHash(frame)
	network, err := store.NetworkIdentityHash(scope)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.InitializeRestoreExecution(scope, expect.OperationID, vmrestore.RestoreExecution{ConfigAfter: &after, Frame: frame, Intents: map[string]vmrestore.RestoreObjectIntent{}, Evidence: map[string]restoreplan.CopyEvidence{}, NetworkHash: network, IdentityHash: powerDigest(t, startup), BeforeStartup: prior, SourceStartup: startup}); err != nil {
		t.Fatal(err)
	}
	target := vmrestore.ObjectRef{Kind: "PersistentVolumeClaim", Namespace: scope.Namespace, Name: "restored-root", UID: "a-root-uid"}
	for _, step := range []string{"restore-writers-clear", "restore-copy-verified"} {
		c, _ := store.Read(scope, expect.OperationID)
		ex := expect
		ex.ControlRevision = c.State.ControlRevision
		ex.BindingGeneration = c.State.BindingGeneration
		ex.Step = step
		ex.RequestHash = manifest
		if _, err := store.Mutate(scope, ex, func(c vmrestore.Current) (vmrestore.Mutation, error) {
			c.State.Fence.Phase = "copy-verified"
			if step == "restore-copy-verified" {
				c.Receipt.Resources = append(c.Receipt.Resources, vmrestore.ResourceRef{ObjectRef: target, Purpose: "restore-target"})
			}
			return vmrestore.Mutation{State: c.State, Receipt: c.Receipt}, nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	c, _ := store.Read(scope, expect.OperationID)
	ex := expect
	ex.ControlRevision = c.State.ControlRevision
	ex.BindingGeneration = c.State.BindingGeneration
	ex.Step = vmrestore.RestoreCommitStep
	bindings, err := vmrestore.BuildRestoreBindings(scope, expect.OperationID, intent.SourceSnapshot.UID, old, after, map[uint]vmrestore.ObjectRef{7: target})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CommitRestoreDB(scope, ex, vmrestore.RestoreCommitRequest{Config: after, Bindings: bindings}); err != nil {
		t.Fatal(err)
	}
	// The lifecycle fixture begins after successful restoration; full engine
	// commit/ACK/runtime/finalize sequencing is separately exercised in vmrestore.
	c, _ = store.Read(scope, expect.OperationID)
	ex.ControlRevision = c.State.ControlRevision
	ex.BindingGeneration = c.State.BindingGeneration
	ex.Step = "fixture-finished"
	ex.RequestHash = manifest
	if _, err := store.Mutate(scope, ex, func(c vmrestore.Current) (vmrestore.Mutation, error) {
		c.State.Fence = nil
		return vmrestore.Mutation{State: c.State, Receipt: c.Receipt}, nil
	}); err != nil {
		t.Fatal(err)
	}
	core := kubefake.NewSimpleClientset(&corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: target.Name, Namespace: scope.Namespace, UID: types.UID(target.UID), Labels: map[string]string{"service_id": scope.ServiceID, "tenant_id": scope.TenantID}}, Status: corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound}})
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{{Group: "kubevirt.io", Version: "v1", Resource: "virtualmachines"}: "VirtualMachineList", {Group: "kubevirt.io", Version: "v1", Resource: "virtualmachineinstances"}: "VirtualMachineInstanceList"})
	strategy := kv.RunStrategyHalted
	vm := &kv.VirtualMachine{ObjectMeta: metav1.ObjectMeta{Name: "vm", Namespace: scope.Namespace, UID: "vm-uid", ResourceVersion: "1", Labels: map[string]string{"service_id": scope.ServiceID, "tenant_id": scope.TenantID}}, Spec: kv.VirtualMachineSpec{RunStrategy: &strategy, Template: &kv.VirtualMachineInstanceTemplateSpec{Spec: kv.VirtualMachineInstanceSpec{Domain: kv.DomainSpec{CPU: &kv.CPU{Cores: 2, Sockets: 1, Threads: 1}, Memory: &kv.Memory{Guest: resource.NewQuantity(2<<30, resource.BinarySI)}, Firmware: &kv.Firmware{UUID: types.UID(startup.FirmwareUUID)}}, Volumes: []kv.Volume{{Name: "manual7", VolumeSource: kv.VolumeSource{PersistentVolumeClaim: &kv.PersistentVolumeClaimVolumeSource{PersistentVolumeClaimVolumeSource: corev1.PersistentVolumeClaimVolumeSource{ClaimName: target.Name}}}}}}}}}
	object, err := vmrestore.EncodeRuntimeVM(vm)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := dyn.Resource(schema.GroupVersionResource{Group: "kubevirt.io", Version: "v1", Resource: "virtualmachines"}).Namespace(scope.Namespace).Create(context.Background(), object, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	ctrl := gomock.NewController(t)
	client := kubecli.NewMockKubevirtClient(ctrl)
	client.EXPECT().DynamicClient().Return(dyn).AnyTimes()
	a.kubeClient = core
	a.kubevirtClient = client
	return a, store, vmrestore.NewResolver(store, core, dyn), scope, dyn
}

// capability_id: rainbond.vm-restore.ordinary-power-handler
func TestRestoredPowerHandlerUsesAThroughStartAndStopAndFinishesActualEvents(t *testing.T) {
	a, store, resolver, scope, dyn := restoredPowerFixture(t)
	vmGVR := schema.GroupVersionResource{Group: "kubevirt.io", Version: "v1", Resource: "virtualmachines"}
	vmiGVR := schema.GroupVersionResource{Group: "kubevirt.io", Version: "v1", Resource: "virtualmachineinstances"}
	for _, action := range []string{"start", "stop"} {
		eventID := "power-" + action
		if err := a.getDBManager().DB().Create(&dbmodel.ServiceEvent{EventID: eventID, TenantID: scope.TenantID, ServiceID: scope.ServiceID, Status: "checking", FinalStatus: "running"}).Error; err != nil {
			t.Fatal(err)
		}
		ctx := newVMOperationEventContext(eventID)
		var err error
		if action == "start" {
			err = a.StartOrCreateVM(ctx, &apimodel.StartStopStruct{TenantID: scope.TenantID, ServiceID: scope.ServiceID, EventID: eventID, TaskType: action}, "")
		} else {
			err = a.StopVM(ctx, scope.ServiceID)
		}
		var pending *bcode.Code
		if !errors.As(err, &pending) || pending.Status != 202 {
			t.Fatal("real handler did not preserve pending runtime", action, err)
		}
		vm, err := dyn.Resource(vmGVR).Namespace(scope.Namespace).Get(context.Background(), "vm", metav1.GetOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if action == "start" {
			var typed kv.VirtualMachine
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(vm.Object, &typed); err != nil {
				t.Fatal(err)
			}
			instance := &kv.VirtualMachineInstance{TypeMeta: metav1.TypeMeta{APIVersion: "kubevirt.io/v1", Kind: "VirtualMachineInstance"}, ObjectMeta: metav1.ObjectMeta{Name: "vm", Namespace: scope.Namespace, UID: "running-instance", OwnerReferences: []metav1.OwnerReference{{Kind: "VirtualMachine", Name: "vm", UID: "vm-uid"}}}, Spec: typed.Spec.Template.Spec, Status: kv.VirtualMachineInstanceStatus{Phase: kv.Running}}
			raw, _ := json.Marshal(instance)
			object := &unstructured.Unstructured{}
			if err := object.UnmarshalJSON(raw); err != nil {
				t.Fatal(err)
			}
			if _, err := dyn.Resource(vmiGVR).Namespace(scope.Namespace).Create(context.Background(), object, metav1.CreateOptions{}); err != nil {
				t.Fatal(err)
			}
		} else {
			if err := dyn.Resource(vmiGVR).Namespace(scope.Namespace).Delete(context.Background(), "vm", metav1.DeleteOptions{}); err != nil {
				t.Fatal(err)
			}
		}
		if err := store.ObserveOrdinaryVMCompletion(context.Background(), resolver, dyn, scope); err != nil {
			t.Fatal(err)
		}
		var event dbmodel.ServiceEvent
		if err := a.getDBManager().DB().Where("event_id = ?", eventID).Take(&event).Error; err != nil {
			t.Fatal(err)
		}
		if event.Status != "success" || event.FinalStatus != "complete" {
			t.Fatal("runtime completion did not finish event")
		}
		volumes, _, _ := unstructured.NestedSlice(vm.Object, "spec", "template", "spec", "volumes")
		found := false
		for _, raw := range volumes {
			v := raw.(map[string]interface{})
			if v["name"] == "manual7" {
				p := v["persistentVolumeClaim"].(map[string]interface{})
				found = p["claimName"] == "restored-root"
			}
		}
		firmware, _, _ := unstructured.NestedString(vm.Object, "spec", "template", "spec", "domain", "firmware", "uuid")
		if !found || firmware != "11111111-1111-4111-8111-111111111111" {
			t.Fatal("handler fell back to B or changed A identity")
		}
	}
}

// capability_id: rainbond.vm-power.cancel-pending-handler
func TestRestoredPowerHandlerCanStopAnUnfinishedStartup(t *testing.T) {
	a, store, resolver, scope, dyn := restoredPowerFixture(t)
	ctx := context.Background()
	for _, id := range []string{"unfinished-start", "cancel-start"} {
		if err := a.getDBManager().DB().Create(&dbmodel.ServiceEvent{EventID: id, TenantID: scope.TenantID, ServiceID: scope.ServiceID, Status: "checking"}).Error; err != nil {
			t.Fatal(err)
		}
	}
	err := a.StartOrCreateVM(newVMOperationEventContext("unfinished-start"), &apimodel.StartStopStruct{TenantID: scope.TenantID, ServiceID: scope.ServiceID, TaskType: "start", EventID: "unfinished-start"}, "")
	var accepted *bcode.Code
	if !errors.As(err, &accepted) || accepted.Status != 202 {
		t.Fatal("startup was not accepted as pending", err)
	}
	vmGVR := schema.GroupVersionResource{Group: "kubevirt.io", Version: "v1", Resource: "virtualmachines"}
	vmiGVR := schema.GroupVersionResource{Group: "kubevirt.io", Version: "v1", Resource: "virtualmachineinstances"}
	instance := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "kubevirt.io/v1", "kind": "VirtualMachineInstance",
		"metadata": map[string]interface{}{"name": "vm", "namespace": scope.Namespace, "uid": "pending-instance", "ownerReferences": []interface{}{map[string]interface{}{"kind": "VirtualMachine", "name": "vm", "uid": "vm-uid"}}},
		"status":   map[string]interface{}{"phase": "Scheduling"},
	}}
	if _, err := dyn.Resource(vmiGVR).Namespace(scope.Namespace).Create(ctx, instance, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	before, _ := store.ReadOnly(scope, "")
	err = a.StopVM(newVMOperationEventContext("cancel-start"), scope.ServiceID)
	if !errors.As(err, &accepted) || accepted.Status != 202 {
		t.Fatal("stop of pending startup was rejected", err)
	}
	vm, _ := dyn.Resource(vmGVR).Namespace(scope.Namespace).Get(ctx, "vm", metav1.GetOptions{})
	strategy, _, _ := unstructured.NestedString(vm.Object, "spec", "runStrategy")
	if strategy != "Halted" {
		t.Fatal("stop did not change the power intent")
	}
	if err := dyn.Resource(vmiGVR).Namespace(scope.Namespace).Delete(ctx, "vm", metav1.DeleteOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := store.ObserveOrdinaryVMCompletion(ctx, resolver, dyn, scope); err != nil {
		t.Fatal(err)
	}
	after, _ := store.ReadOnly(scope, "")
	if after.State.Admission != nil || after.State.BindingGeneration != before.State.BindingGeneration || after.State.Bindings["7"].ActivePVC.UID != before.State.Bindings["7"].ActivePVC.UID {
		t.Fatal("stop did not release admission while retaining the restored disk")
	}
}

// capability_id: rainbond.vm-restore.ordinary-power-handler
func TestOrdinarySpecSyncPreservesPublishedFirmwareAndDoesNotMutateDesired(t *testing.T) {
	existing := &kv.VirtualMachine{Spec: kv.VirtualMachineSpec{Template: &kv.VirtualMachineInstanceTemplateSpec{Spec: kv.VirtualMachineInstanceSpec{Domain: kv.DomainSpec{Firmware: &kv.Firmware{UUID: "11111111-1111-4111-8111-111111111111"}, CPU: &kv.CPU{Cores: 1}}}}}}
	desired := existing.DeepCopy()
	desired.Spec.Template.Spec.Domain.Firmware = nil
	desired.Spec.Template.Spec.Domain.CPU.Cores = 2
	updated, changed := applyVirtualMachineSpec(existing, desired)
	if !changed || updated.Spec.Template.Spec.Domain.Firmware.UUID != existing.Spec.Template.Spec.Domain.Firmware.UUID || desired.Spec.Template.Spec.Domain.Firmware != nil {
		t.Fatal("ordinary sync reset identity or mutated the render input")
	}
	if _, changed := applyVirtualMachineSpec(updated, desired); changed {
		t.Fatal("identity preservation caused perpetual spec updates")
	}
}
