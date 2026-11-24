package vmrestore

import (
	"context"
	"errors"
	"testing"

	dbmodel "github.com/goodrain/rainbond/db/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func ordinaryFixture(t *testing.T, action string) (resolverFixture, string) {
	t.Helper()
	f := newResolverFixture(t)
	f.seed(t, f.currentBinding())
	strategy := "Always"
	if action == "stop" {
		strategy = "Halted"
	}
	vm := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "kubevirt.io/v1", "kind": "VirtualMachine",
		"metadata": map[string]interface{}{"name": "vm", "namespace": f.scope.Namespace, "uid": "ordinary-uid"},
		"spec":     map[string]interface{}{"runStrategy": strategy, "template": map[string]interface{}{"spec": map[string]interface{}{"volumes": []interface{}{map[string]interface{}{"name": "manual7", "persistentVolumeClaim": map[string]interface{}{"claimName": "manual7"}}}}}},
	}}
	f.dynamicObject(t, virtualMachineResource, vm)
	_, op, err := f.store.ClaimOrdinaryVM(f.scope, action, refOf("VirtualMachine", vm))
	if err != nil {
		t.Fatal(err)
	}
	return f, op
}

func ordinaryInstance(t *testing.T, f resolverFixture, claim string) {
	t.Helper()
	f.dynamicObject(t, virtualMachineInstanceResource, &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "kubevirt.io/v1", "kind": "VirtualMachineInstance",
		"metadata": map[string]interface{}{"name": "vm", "namespace": f.scope.Namespace, "uid": "instance-uid", "ownerReferences": []interface{}{map[string]interface{}{"apiVersion": "kubevirt.io/v1", "kind": "VirtualMachine", "name": "vm", "uid": "ordinary-uid"}}},
		"spec":     map[string]interface{}{"volumes": []interface{}{map[string]interface{}{"name": "manual7", "persistentVolumeClaim": map[string]interface{}{"claimName": claim}}}},
		"status":   map[string]interface{}{"phase": "Running"},
	}})
}

// capability_id: rainbond.vm-restore.ordinary-power-admission
func TestOrdinaryStartCannotCompleteOnOldDiskOrWrongDesiredPower(t *testing.T) {
	for _, mismatch := range []string{"instance-disk", "vm-disk", "vm-power", "extra-instance"} {
		t.Run(mismatch, func(t *testing.T) {
			f, op := ordinaryFixture(t, "start")
			claim := "manual7"
			if mismatch == "instance-disk" {
				claim = "old-b-disk"
			}
			ordinaryInstance(t, f, claim)
			vm, _ := f.dynamic.Resource(virtualMachineResource).Namespace(f.scope.Namespace).Get(context.Background(), "vm", metav1.GetOptions{})
			if mismatch == "vm-power" {
				_ = unstructured.SetNestedField(vm.Object, "Halted", "spec", "runStrategy")
			}
			if mismatch == "vm-disk" {
				_ = unstructured.SetNestedSlice(vm.Object, []interface{}{map[string]interface{}{"name": "manual7", "persistentVolumeClaim": map[string]interface{}{"claimName": "old-b-disk"}}}, "spec", "template", "spec", "volumes")
			}
			_, _ = f.dynamic.Resource(virtualMachineResource).Namespace(f.scope.Namespace).Update(context.Background(), vm, metav1.UpdateOptions{})
			if mismatch == "extra-instance" {
				vmi, _ := f.dynamic.Resource(virtualMachineInstanceResource).Namespace(f.scope.Namespace).Get(context.Background(), "vm", metav1.GetOptions{})
				vmi.SetName("extra")
				vmi.SetUID("extra-instance")
				_, _ = f.dynamic.Resource(virtualMachineInstanceResource).Namespace(f.scope.Namespace).Create(context.Background(), vmi, metav1.CreateOptions{})
			}
			if err := f.store.ObserveOrdinaryVMCompletion(context.Background(), f.resolver, f.dynamic, f.scope); err == nil {
				t.Fatal("inaccurate runtime observation released admission")
			}
			c, _ := f.store.Read(f.scope, op)
			if c.State.Admission == nil {
				t.Fatal("admission lost")
			}
		})
	}
}

// capability_id: rainbond.vm-restore.ordinary-power-admission
func TestOrdinaryPowerCompletionUsesPinnedUIDAndCurrentBindingGeneration(t *testing.T) {
	for _, action := range []string{"start", "stop"} {
		t.Run(action, func(t *testing.T) {
			f, op := ordinaryFixture(t, action)
			if action == "start" {
				ordinaryInstance(t, f, "manual7")
			}
			if err := f.store.ObserveOrdinaryVMCompletion(context.Background(), f.resolver, f.dynamic, f.scope); err != nil {
				t.Fatal(err)
			}
			c, _ := f.store.Read(f.scope, op)
			if c.State.Admission != nil || !c.Receipt.Steps["ordinary-complete"].Applied {
				t.Fatal("exact observed completion not recorded")
			}
			if err := f.store.ObserveOrdinaryVMCompletion(context.Background(), f.resolver, f.dynamic, f.scope); err != nil {
				t.Fatal(err)
			}
		})
	}
	f, _ := ordinaryFixture(t, "stop")
	vm, _ := f.dynamic.Resource(virtualMachineResource).Namespace(f.scope.Namespace).Get(context.Background(), "vm", metav1.GetOptions{})
	vm.SetUID("replacement")
	_, _ = f.dynamic.Resource(virtualMachineResource).Namespace(f.scope.Namespace).Update(context.Background(), vm, metav1.UpdateOptions{})
	if err := f.store.ObserveOrdinaryVMCompletion(context.Background(), f.resolver, f.dynamic, f.scope); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
}

// capability_id: rainbond.vm-restore.ordinary-power-admission
func TestOrdinaryStopWaitsForDesiredHaltAndAllInstancesGone(t *testing.T) {
	f, _ := ordinaryFixture(t, "stop")
	vm, _ := f.dynamic.Resource(virtualMachineResource).Namespace(f.scope.Namespace).Get(context.Background(), "vm", metav1.GetOptions{})
	_ = unstructured.SetNestedField(vm.Object, "Always", "spec", "runStrategy")
	_, _ = f.dynamic.Resource(virtualMachineResource).Namespace(f.scope.Namespace).Update(context.Background(), vm, metav1.UpdateOptions{})
	if err := f.store.ObserveOrdinaryVMCompletion(context.Background(), f.resolver, f.dynamic, f.scope); !errors.Is(err, ErrRestorePending) {
		t.Fatal("stop completed without halted desired state", err)
	}
}

// capability_id: rainbond.vm-restore.ordinary-power-admission
func TestOrdinaryUnknownWriteKeepsAdmissionAndKnownPreWriteFailureReleases(t *testing.T) {
	for _, known := range []bool{false, true} {
		f, op := ordinaryFixture(t, "start")
		if err := f.store.FinishOrdinaryVMAttempt(f.scope, "another-owner", known); !errors.Is(err, ErrConflict) {
			t.Fatal(err)
		}
		if err := f.store.FinishOrdinaryVMAttempt(f.scope, op, known); err != nil {
			t.Fatal(err)
		}
		c, _ := f.store.Read(f.scope, op)
		if known && c.State.Admission != nil {
			t.Fatal("known pre-write abort kept admission")
		}
		if !known {
			if c.State.Admission == nil || !c.State.Admission.OutcomeUnknown {
				t.Fatal("unknown update lost admission")
			}
			ordinaryInstance(t, f, "manual7")
			if err := f.store.ObserveOrdinaryVMCompletion(context.Background(), f.resolver, f.dynamic, f.scope); err != nil {
				t.Fatal(err)
			}
		}
	}
}

// capability_id: rainbond.vm-restore.ordinary-power-admission
func TestOrdinaryAdmissionRejectsUnownedStoreMutation(t *testing.T) {
	f, _ := ordinaryFixture(t, "start")
	c, _ := f.store.Read(f.scope, "")
	_, err := f.store.Mutate(f.scope, Expectation{ControlRevision: c.State.ControlRevision, BindingGeneration: c.State.BindingGeneration}, func(c Current) (Mutation, error) { c.State.BindingGeneration++; return Mutation{State: c.State}, nil })
	if !errors.Is(err, ErrBusy) {
		t.Fatal("another writer bypassed ordinary admission", err)
	}
}

// capability_id: rainbond.vm-restore.ordinary-power-admission
func TestOrdinaryRestartWaitsForNewInstanceUID(t *testing.T) {
	f, op := ordinaryFixture(t, "start")
	if err := f.store.FinishOrdinaryVMAttempt(f.scope, op, true); err != nil {
		t.Fatal(err)
	}
	ordinaryInstance(t, f, "manual7")
	vm, _ := f.dynamic.Resource(virtualMachineResource).Namespace(f.scope.Namespace).Get(context.Background(), "vm", metav1.GetOptions{})
	_, op, err := f.store.ClaimOrdinaryVM(f.scope, "restart", refOf("VirtualMachine", vm), ObjectRef{Kind: "VirtualMachineInstance", Namespace: f.scope.Namespace, Name: "vm", UID: "instance-uid"})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.store.ObserveOrdinaryVMCompletion(context.Background(), f.resolver, f.dynamic, f.scope); !errors.Is(err, ErrRestorePending) {
		t.Fatal("old running VMI counted as completed restart", err)
	}
	instance, _ := f.dynamic.Resource(virtualMachineInstanceResource).Namespace(f.scope.Namespace).Get(context.Background(), "vm", metav1.GetOptions{})
	instance.SetUID("replacement-instance")
	_, _ = f.dynamic.Resource(virtualMachineInstanceResource).Namespace(f.scope.Namespace).Update(context.Background(), instance, metav1.UpdateOptions{})
	if err := f.store.ObserveOrdinaryVMCompletion(context.Background(), f.resolver, f.dynamic, f.scope); err != nil {
		t.Fatal(err)
	}
	c, _ := f.store.Read(f.scope, op)
	if c.State.Admission != nil || !c.Receipt.Steps["ordinary-complete"].Applied {
		t.Fatal("restart not completed after new instance was observed")
	}
}

// capability_id: rainbond.vm-restore.ordinary-power-admission
func TestPowerEventCompletionIsAtomicWithObservedAdmissionRelease(t *testing.T) {
	f, op := ordinaryFixture(t, "start")
	if err := f.store.FinishOrdinaryVMAttempt(f.scope, op, true); err != nil {
		t.Fatal(err)
	}
	if err := f.store.db.AutoMigrate(&dbmodel.ServiceEvent{}).Error; err != nil {
		t.Fatal(err)
	}
	event := dbmodel.ServiceEvent{EventID: "power-event", TenantID: f.scope.TenantID, ServiceID: f.scope.ServiceID, Status: "checking", FinalStatus: "running"}
	if err := f.store.db.Create(&event).Error; err != nil {
		t.Fatal(err)
	}
	ref := ObjectRef{Kind: "VirtualMachine", Namespace: f.scope.Namespace, Name: "vm", UID: "ordinary-uid"}
	_, op, err := f.store.ClaimOrdinaryVMWithEvent(f.scope, "start", ref, event.EventID)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.store.ObserveOrdinaryVMCompletion(context.Background(), f.resolver, f.dynamic, f.scope); !errors.Is(err, ErrRestorePending) {
		t.Fatal(err)
	}
	var observed dbmodel.ServiceEvent
	if err := f.store.db.Where("event_id = ?", event.EventID).Take(&observed).Error; err != nil {
		t.Fatal(err)
	}
	if observed.Status != "checking" {
		t.Fatal("pending power reported event success")
	}
	ordinaryInstance(t, f, "manual7")
	if err := f.store.db.Model(&dbmodel.ServiceEvent{}).Where("event_id = ?", event.EventID).UpdateColumn("tenant_id", "foreign").Error; err != nil {
		t.Fatal(err)
	}
	if err := f.store.ObserveOrdinaryVMCompletion(context.Background(), f.resolver, f.dynamic, f.scope); !errors.Is(err, ErrConflict) {
		t.Fatal("foreign event accepted", err)
	}
	c, _ := f.store.Read(f.scope, op)
	if c.State.Admission == nil {
		t.Fatal("failed event transaction released admission")
	}
	if err := f.store.db.Model(&dbmodel.ServiceEvent{}).Where("event_id = ?", event.EventID).UpdateColumn("tenant_id", f.scope.TenantID).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.store.ObserveOrdinaryVMCompletion(context.Background(), f.resolver, f.dynamic, f.scope); err != nil {
		t.Fatal(err)
	}
	if err := f.store.db.Where("event_id = ?", event.EventID).Take(&observed).Error; err != nil {
		t.Fatal(err)
	}
	c, _ = f.store.Read(f.scope, op)
	if c.State.Admission != nil || observed.Status != "success" || observed.FinalStatus != "complete" {
		t.Fatal("event and admission did not complete together")
	}
}
