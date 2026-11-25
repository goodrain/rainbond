package vmrestore

import (
	"context"
	"errors"
	"reflect"
	"testing"

	dbmodel "github.com/goodrain/rainbond/db/model"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func pendingPowerFixture(t *testing.T) (resolverFixture, string) {
	t.Helper()
	f, previous := ordinaryFixture(t, "start")
	if err := f.store.FinishOrdinaryVMAttempt(f.scope, previous, true); err != nil {
		t.Fatal(err)
	}
	if err := f.store.db.AutoMigrate(&dbmodel.ServiceEvent{}).Error; err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"pending-start", "requested-stop"} {
		if err := f.store.db.Create(&dbmodel.ServiceEvent{EventID: id, TenantID: f.scope.TenantID, ServiceID: f.scope.ServiceID, Status: "checking"}).Error; err != nil {
			t.Fatal(err)
		}
	}
	vm, _ := f.dynamic.Resource(restoreVMGVR).Namespace(f.scope.Namespace).Get(context.Background(), "vm", metav1.GetOptions{})
	vm.SetResourceVersion("1")
	vm.SetLabels(map[string]string{"service_id": f.scope.ServiceID, "tenant_id": f.scope.TenantID})
	if _, err := f.dynamic.Resource(restoreVMGVR).Namespace(f.scope.Namespace).Update(context.Background(), vm, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	_, op, err := f.store.ClaimOrdinaryVMWithEvent(f.scope, "start", refOf("VirtualMachine", vm), "pending-start")
	if err != nil {
		t.Fatal(err)
	}
	ordinaryInstance(t, f, "manual7")
	instance, _ := f.dynamic.Resource(restoreVMIGVR).Namespace(f.scope.Namespace).Get(context.Background(), "vm", metav1.GetOptions{})
	_ = unstructured.SetNestedField(instance.Object, "Scheduling", "status", "phase")
	if _, err := f.dynamic.Resource(restoreVMIGVR).Namespace(f.scope.Namespace).Update(context.Background(), instance, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	return f, op
}

// capability_id: rainbond.vm-power.cancel-pending-start
func TestPendingStartStopsBeforeReleasingAdmissionAndCompletesBothEvents(t *testing.T) {
	for _, unknown := range []bool{false, true} {
		t.Run(map[bool]string{false: "known write", true: "uncertain write"}[unknown], func(t *testing.T) {
			f, op := pendingPowerFixture(t)
			ctx := context.Background()
			before, _ := f.store.ReadOnly(f.scope, op)
			if unknown {
				if err := f.store.FinishOrdinaryVMAttempt(f.scope, op, false); err != nil {
					t.Fatal(err)
				}
			}
			if err := f.store.StopPendingOrdinaryVM(ctx, f.resolver, f.dynamic, f.scope, "requested-stop"); !errors.Is(err, ErrRestorePending) {
				t.Fatal("stop must wait for the existing instance to disappear", err)
			}
			vm, _ := f.dynamic.Resource(restoreVMGVR).Namespace(f.scope.Namespace).Get(ctx, "vm", metav1.GetOptions{})
			strategy, _, _ := unstructured.NestedString(vm.Object, "spec", "runStrategy")
			if strategy != "Halted" {
				t.Fatal("pending start could not be stopped")
			}
			current, _ := f.store.ReadOnly(f.scope, op)
			if current.State.Admission == nil || current.State.Admission.Token != op {
				t.Fatal("the operation was unlocked before runtime stopped")
			}
			if err := f.dynamic.Resource(restoreVMIGVR).Namespace(f.scope.Namespace).Delete(ctx, "vm", metav1.DeleteOptions{}); err != nil {
				t.Fatal(err)
			}
			writer := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "old-writer", Namespace: f.scope.Namespace}, Spec: corev1.PodSpec{Volumes: []corev1.Volume{{Name: "root", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "manual7"}}}}}, Status: corev1.PodStatus{Phase: corev1.PodRunning}}
			if _, err := f.core.CoreV1().Pods(f.scope.Namespace).Create(ctx, writer, metav1.CreateOptions{}); err != nil {
				t.Fatal(err)
			}
			if err := f.store.ObserveOrdinaryVMCompletion(ctx, f.resolver, f.dynamic, f.scope); !errors.Is(err, ErrRestorePending) {
				t.Fatal("an active writer was ignored", err)
			}
			if err := f.core.CoreV1().Pods(f.scope.Namespace).Delete(ctx, writer.Name, metav1.DeleteOptions{}); err != nil {
				t.Fatal(err)
			}
			if err := f.store.db.Model(&dbmodel.ServiceEvent{}).Where("event_id = ?", "requested-stop").UpdateColumn("tenant_id", "foreign").Error; err != nil {
				t.Fatal(err)
			}
			if err := f.store.ObserveOrdinaryVMCompletion(ctx, f.resolver, f.dynamic, f.scope); !errors.Is(err, ErrConflict) {
				t.Fatal("a foreign stop event was completed", err)
			}
			current, _ = f.store.ReadOnly(f.scope, op)
			if current.State.Admission == nil {
				t.Fatal("event completion failure released admission")
			}
			if err := f.store.db.Model(&dbmodel.ServiceEvent{}).Where("event_id = ?", "requested-stop").UpdateColumn("tenant_id", f.scope.TenantID).Error; err != nil {
				t.Fatal(err)
			}
			if err := f.store.ObserveOrdinaryVMCompletion(ctx, f.resolver, f.dynamic, f.scope); err != nil {
				t.Fatal(err)
			}
			current, _ = f.store.ReadOnly(f.scope, op)
			if current.State.Admission != nil || current.State.BindingGeneration != before.State.BindingGeneration || !reflect.DeepEqual(current.State.Bindings, before.State.Bindings) {
				t.Fatal("stop changed disk bindings or failed to release admission")
			}
			for id, expected := range map[string]string{"pending-start": "failure", "requested-stop": "success"} {
				var event dbmodel.ServiceEvent
				if err := f.store.db.Where("event_id = ?", id).Take(&event).Error; err != nil || event.Status != expected || event.FinalStatus != "complete" || event.EndTime == "" {
					t.Fatal("power event was not completed correctly", id, event.Status, event.FinalStatus, err)
				}
			}
		})
	}
}

// capability_id: rainbond.vm-power.cancel-pending-start
func TestPendingStartRefusesToStopReplacementVM(t *testing.T) {
	f, op := pendingPowerFixture(t)
	ctx := context.Background()
	vm, _ := f.dynamic.Resource(restoreVMGVR).Namespace(f.scope.Namespace).Get(ctx, "vm", metav1.GetOptions{})
	vm.SetUID("replacement-uid")
	_, _ = f.dynamic.Resource(restoreVMGVR).Namespace(f.scope.Namespace).Update(ctx, vm, metav1.UpdateOptions{})
	if err := f.store.StopPendingOrdinaryVM(ctx, f.resolver, f.dynamic, f.scope, "requested-stop"); !errors.Is(err, ErrConflict) {
		t.Fatal("a different VM identity was accepted", err)
	}
	current, _ := f.store.ReadOnly(f.scope, op)
	if current.State.Admission == nil {
		t.Fatal("failed identity verification released admission")
	}
}

// capability_id: rainbond.vm-power.terminal-start-failure
func TestTerminalInstanceFailureStopsVMAndClosesFailedStart(t *testing.T) {
	f, op := pendingPowerFixture(t)
	ctx := context.Background()
	instance, _ := f.dynamic.Resource(restoreVMIGVR).Namespace(f.scope.Namespace).Get(ctx, "vm", metav1.GetOptions{})
	_ = unstructured.SetNestedField(instance.Object, "Failed", "status", "phase")
	_, _ = f.dynamic.Resource(restoreVMIGVR).Namespace(f.scope.Namespace).Update(ctx, instance, metav1.UpdateOptions{})
	_ = f.store.ObserveOrdinaryVMCompletion(ctx, f.resolver, f.dynamic, f.scope)
	vm, _ := f.dynamic.Resource(restoreVMGVR).Namespace(f.scope.Namespace).Get(ctx, "vm", metav1.GetOptions{})
	strategy, _, _ := unstructured.NestedString(vm.Object, "spec", "runStrategy")
	if strategy != "Halted" {
		t.Fatal("failed startup retained a running power intent")
	}
	_ = f.dynamic.Resource(restoreVMIGVR).Namespace(f.scope.Namespace).Delete(ctx, "vm", metav1.DeleteOptions{})
	if err := f.store.ObserveOrdinaryVMCompletion(ctx, f.resolver, f.dynamic, f.scope); !errors.Is(err, ErrOrdinaryVMFailed) {
		t.Fatal("failed startup was not reported as failed", err)
	}
	current, _ := f.store.ReadOnly(f.scope, op)
	var event dbmodel.ServiceEvent
	_ = f.store.db.Where("event_id = ?", "pending-start").Take(&event).Error
	if current.State.Admission != nil || event.Status != "failure" || event.FinalStatus != "complete" {
		t.Fatal("terminal startup failure did not close its event and admission")
	}
	t.Run("restart waits for replacement after the retired instance fails", func(t *testing.T) {
		previous, previousOp := pendingPowerFixture(t)
		if err := previous.store.FinishOrdinaryVMAttempt(previous.scope, previousOp, true); err != nil {
			t.Fatal(err)
		}
		old, _ := previous.dynamic.Resource(restoreVMIGVR).Namespace(previous.scope.Namespace).Get(ctx, "vm", metav1.GetOptions{})
		_ = unstructured.SetNestedField(old.Object, "Failed", "status", "phase")
		_, _ = previous.dynamic.Resource(restoreVMIGVR).Namespace(previous.scope.Namespace).Update(ctx, old, metav1.UpdateOptions{})
		vm, _ := previous.dynamic.Resource(restoreVMGVR).Namespace(previous.scope.Namespace).Get(ctx, "vm", metav1.GetOptions{})
		_, restartOp, err := previous.store.ClaimOrdinaryVM(previous.scope, "restart", refOf("VirtualMachine", vm), refOf("VirtualMachineInstance", old))
		if err != nil {
			t.Fatal(err)
		}
		if err := previous.store.ObserveOrdinaryVMCompletion(ctx, previous.resolver, previous.dynamic, previous.scope); !errors.Is(err, ErrRestorePending) {
			t.Fatal("retired restart instance was treated as a failed new startup", err)
		}
		vm, _ = previous.dynamic.Resource(restoreVMGVR).Namespace(previous.scope.Namespace).Get(ctx, "vm", metav1.GetOptions{})
		power, _, _ := unstructured.NestedString(vm.Object, "spec", "runStrategy")
		pending, _ := previous.store.ReadOnly(previous.scope, restartOp)
		if power != "Always" || pending.State.Admission == nil || pending.Receipt.Steps[ordinaryFailureStopStep].Applied {
			t.Fatal("normal restart was cancelled during instance replacement")
		}
	})
}

// capability_id: rainbond.vm-power.terminal-start-failure
func TestFailedCloneStopsPendingStartupInsteadOfKeepingOperationBusy(t *testing.T) {
	f, op := pendingPowerFixture(t)
	ctx := context.Background()
	pvc, _ := f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Get(ctx, "manual7", metav1.GetOptions{})
	group := "snapshot.storage.k8s.io"
	pvc.Spec.DataSource = &corev1.TypedLocalObjectReference{APIGroup: &group, Kind: "VolumeSnapshot", Name: "source-snapshot"}
	_, _ = f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Update(ctx, pvc, metav1.UpdateOptions{})
	pv, _ := f.core.CoreV1().PersistentVolumes().Get(ctx, "pv-original", metav1.GetOptions{})
	pv.Spec.CSI.Driver = "driver.longhorn.io"
	_, _ = f.core.CoreV1().PersistentVolumes().Update(ctx, pv, metav1.UpdateOptions{})
	f.dynamicObject(t, nativeLonghornVolumeResource, &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "longhorn.io/v1beta2", "kind": "Volume",
		"metadata": map[string]interface{}{"name": pv.Spec.CSI.VolumeHandle, "namespace": "storage-system", "uid": "volume-uid"},
		"spec":     map[string]interface{}{"dataSource": "snap://source-volume/source-snapshot"},
		"status":   map[string]interface{}{"cloneStatus": map[string]interface{}{"state": "failed"}},
	}})
	if err := f.store.ObserveOrdinaryVMCompletion(ctx, f.resolver, f.dynamic, f.scope); !errors.Is(err, ErrRestorePending) {
		t.Fatal("failed clone must stop the existing runtime before completing", err)
	}
	vm, _ := f.dynamic.Resource(restoreVMGVR).Namespace(f.scope.Namespace).Get(ctx, "vm", metav1.GetOptions{})
	strategy, _, _ := unstructured.NestedString(vm.Object, "spec", "runStrategy")
	if strategy != "Halted" {
		t.Fatal("failed backend clone left startup running")
	}
	_ = f.dynamic.Resource(restoreVMIGVR).Namespace(f.scope.Namespace).Delete(ctx, "vm", metav1.DeleteOptions{})
	if err := f.store.ObserveOrdinaryVMCompletion(ctx, f.resolver, f.dynamic, f.scope); !errors.Is(err, ErrOrdinaryVMFailed) {
		t.Fatal("failed backend clone did not close startup as failed", err)
	}
	current, _ := f.store.ReadOnly(f.scope, op)
	if current.State.Admission != nil {
		t.Fatal("failed clone kept the component operation permanently busy")
	}
}
