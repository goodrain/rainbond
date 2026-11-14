package vmrestore

import (
	"context"
	"errors"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kt "k8s.io/client-go/testing"
	kv "kubevirt.io/api/core/v1"
)

func snapshotFirmwareFixture(t *testing.T) (*SnapshotFirmwareMaterializer, *SnapshotEngine, Scope, SnapshotActionRequest) {
	t.Helper()
	e, b, scope := snapshotEngineFixture(t)
	d := enginePrepare(t, e, scope, b)
	strategy := kv.RunStrategyAlways
	vm := &kv.VirtualMachine{ObjectMeta: metav1.ObjectMeta{Name: b.fixture.intent.SourceVM.Name, Namespace: scope.Namespace, UID: types.UID(b.fixture.intent.SourceVM.UID), ResourceVersion: "1", Labels: map[string]string{"service_id": scope.ServiceID, "tenant_id": scope.TenantID}}, Spec: kv.VirtualMachineSpec{RunStrategy: &strategy, Template: &kv.VirtualMachineInstanceTemplateSpec{}}}
	instance := &kv.VirtualMachineInstance{ObjectMeta: metav1.ObjectMeta{Name: vm.Name, Namespace: vm.Namespace, UID: "source-instance", ResourceVersion: "1", OwnerReferences: []metav1.OwnerReference{{Kind: "VirtualMachine", Name: vm.Name, UID: vm.UID}}}, Spec: kv.VirtualMachineInstanceSpec{Domain: kv.DomainSpec{Firmware: &kv.Firmware{UUID: "11111111-1111-4111-8111-111111111111"}}}, Status: kv.VirtualMachineInstanceStatus{Phase: kv.Running}}
	u, err := EncodeRuntimeVM(vm)
	if err != nil {
		t.Fatal(err)
	}
	v := runtimeInstanceObject(t, instance)
	dyn := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), u, v)
	return &SnapshotFirmwareMaterializer{Store: e.store, Dynamic: dyn}, e, scope, engineAction("halt", d)
}

// capability_id: rainbond.vm-snapshot.fenced-firmware-identity
func TestSnapshotFirmwarePinSealsActualVMIIdentityBeforeNativeSnapshot(t *testing.T) {
	m, e, scope, request := snapshotFirmwareFixture(t)
	if err := m.Pin(context.Background(), scope, request.OperationID); !errors.Is(err, ErrRestorePending) {
		t.Fatal(err)
	}
	c, _ := e.store.Read(scope, request.OperationID)
	if c.Receipt.SnapshotIdentity == nil || c.Receipt.SnapshotIdentity.Pinned || !c.Receipt.Steps["snapshot-identity-issued"].Applied {
		t.Fatal("update response counted as observed identity")
	}
	if err := m.Pin(context.Background(), scope, request.OperationID); err != nil {
		t.Fatal(err)
	}
	c, _ = e.store.Read(scope, request.OperationID)
	if !c.Receipt.SnapshotIdentity.Pinned || c.Receipt.SnapshotIdentity.FirmwareUUID != "11111111-1111-4111-8111-111111111111" || c.State.Fence == nil {
		t.Fatal("actual source identity not sealed under fence")
	}
	updates := 0
	for _, action := range m.Dynamic.(*dynamicfake.FakeDynamicClient).Actions() {
		if action.GetVerb() == "update" {
			updates++
			if action.GetResource().Resource != "virtualmachines" {
				t.Fatal("live instance was mutated")
			}
		}
	}
	if updates != 1 {
		t.Fatal(updates)
	}
}

// capability_id: rainbond.vm-snapshot.fenced-firmware-identity
func TestSnapshotFirmwareUnknownWriteIsNeverRepeated(t *testing.T) {
	m, e, scope, request := snapshotFirmwareFixture(t)
	updates := 0
	m.Dynamic.(*dynamicfake.FakeDynamicClient).PrependReactor("update", "virtualmachines", func(kt.Action) (bool, runtime.Object, error) {
		updates++
		return true, nil, errors.New("fixture timeout")
	})
	for i := 0; i < 2; i++ {
		if err := m.Pin(context.Background(), scope, request.OperationID); !errors.Is(err, ErrRestoreUnknown) {
			t.Fatal(err)
		}
	}
	c, _ := e.store.Read(scope, request.OperationID)
	if updates != 1 || c.State.Fence == nil || c.Receipt.SnapshotIdentity.Pinned {
		t.Fatal("unknown source identity became ready or was repeated")
	}
}

// capability_id: rainbond.vm-snapshot.fenced-firmware-identity
func TestSnapshotHaltWaitsForPinnedIdentityAndNativeContentMustPreserveIt(t *testing.T) {
	m, e, scope, request := snapshotFirmwareFixture(t)
	e.Identity = m
	b := e.backend.(*fakeSnapshotBackend)
	d, err := e.Halt(context.Background(), scope, request)
	if err != nil || d.ErrorCode != "vm_snapshot_identity_pending" || b.halts != 0 {
		t.Fatal("snapshot halted before identity was observed", d.ErrorCode, err, b.halts)
	}
	d, err = e.Halt(context.Background(), scope, request)
	if err != nil || b.halts != 1 {
		t.Fatal("halt did not follow observed identity", err, b.halts)
	}
	request.RequestID = "snapshot-create"
	if _, err = e.Create(context.Background(), scope, request); !errors.Is(err, ErrConflict) {
		t.Fatal("native content lost firmware identity but became ready", err)
	}
	vm := b.fixture.content.Spec.Source.VirtualMachine
	if vm.Spec.Template.Spec.Domain.Firmware == nil {
		vm.Spec.Template.Spec.Domain.Firmware = &kv.Firmware{}
	}
	vm.Spec.Template.Spec.Domain.Firmware.UUID = "11111111-1111-4111-8111-111111111111"
	d, err = e.Get(context.Background(), scope, request.OperationID)
	if err != nil || !d.ReadyToUse {
		t.Fatal("matching native identity did not become verifiable", err)
	}
}

// capability_id: rainbond.vm-snapshot.fenced-firmware-identity
func TestSnapshotFirmwareDoesNotAdoptReplacementInstanceAfterIssuedUpdate(t *testing.T){
	m,e,scope,request:=snapshotFirmwareFixture(t)
	if err:=m.Pin(context.Background(),scope,request.OperationID);!errors.Is(err,ErrRestorePending){t.Fatal(err)}
	c,_:=e.store.Read(scope,request.OperationID)
	ref:=c.Receipt.SnapshotIdentity.VMI
	instance,err:=m.Dynamic.Resource(restoreVMIGVR).Namespace(scope.Namespace).Get(context.Background(),ref.Name,metav1.GetOptions{});if err!=nil{t.Fatal(err)}
	instance.SetUID("replacement-instance")
	if _,err:=m.Dynamic.Resource(restoreVMIGVR).Namespace(scope.Namespace).Update(context.Background(),instance,metav1.UpdateOptions{});err!=nil{t.Fatal(err)}
	if err:=m.Pin(context.Background(),scope,request.OperationID);!errors.Is(err,ErrConflict){t.Fatal("replacement VMI satisfied old identity intent",err)}
	c,_=e.store.Read(scope,request.OperationID);if c.Receipt.SnapshotIdentity.Pinned || c.State.Fence==nil{t.Fatal("replacement source became pinned")}
}
