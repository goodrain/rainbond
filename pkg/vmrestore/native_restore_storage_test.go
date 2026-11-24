package vmrestore

import (
	"context"
	"errors"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kv "kubevirt.io/api/core/v1"
)

// capability_id: rainbond.vm-restore.native-source-retention
func TestNativeRestoreRetainsSourceDataVolumeBeforeControllerCleanup(t *testing.T) {
	f := newManagedNativeFixture(t)
	ctx := context.Background()
	dv := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "cdi.kubevirt.io/v1beta1", "kind": "DataVolume",
		"metadata": map[string]interface{}{"name": "source-dv", "namespace": f.scope.Namespace, "uid": "source-dv-uid", "labels": map[string]interface{}{"service_id": f.scope.ServiceID, "tenant_id": f.scope.TenantID}},
		"status":   map[string]interface{}{"claimName": "active-pvc", "phase": "Succeeded"},
	}}
	if _, err := f.dynamic.Resource(dataVolumeResource).Namespace(f.scope.Namespace).Create(ctx, dv, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	state, _ := f.managed.Store.ReadOnly(f.scope, "")
	if _, err := f.managed.Store.Mutate(f.scope, Expectation{ControlRevision: state.State.ControlRevision, BindingGeneration: state.State.BindingGeneration}, func(current Current) (Mutation, error) {
		binding := current.State.Bindings["7"]
		ref := refOf("DataVolume", dv)
		binding.ActiveDV = &ref
		current.State.Bindings["7"] = binding
		current.State.BindingGeneration++
		return Mutation{State: current.State}, nil
	}); err != nil {
		t.Fatal(err)
	}
	vm, _, _ := f.backend.VirtualMachine(ctx, f.scope)
	vm.Spec.Template.Spec.Volumes[0].PersistentVolumeClaim = nil
	vm.Spec.Template.Spec.Volumes[0].DataVolume = &kv.DataVolumeSource{Name: dv.GetName()}
	vm.Spec.DataVolumeTemplates = []kv.DataVolumeTemplateSpec{{ObjectMeta: metav1.ObjectMeta{Name: dv.GetName()}}}
	raw, _ := EncodeRuntimeVM(vm)
	if _, err := f.dynamic.Resource(restoreVMGVR).Namespace(f.scope.Namespace).Update(ctx, raw, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	f.backend.beforeCreate = func() {
		live, _, _ := f.backend.VirtualMachine(ctx, f.scope)
		for _, template := range live.Spec.DataVolumeTemplates {
			if template.Name == dv.GetName() {
				_ = f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Delete(ctx, "active-pvc", metav1.DeleteOptions{})
				_ = f.dynamic.Resource(dataVolumeResource).Namespace(f.scope.Namespace).Delete(ctx, dv.GetName(), metav1.DeleteOptions{})
			}
		}
	}
	for attempt := 0; attempt < 3 && f.backend.creates == 0; attempt++ {
		if _, err := f.engine.Start(ctx, f.scope, "snapshot-a", "restore-a"); err != nil {
			t.Fatal(err)
		}
	}
	if f.backend.creates != 1 {
		t.Fatal("native restore was not submitted exactly once")
	}
	if _, err := f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Get(ctx, "active-pvc", metav1.GetOptions{}); err != nil {
		t.Fatal("native cleanup deleted the source disk needed by CSI snapshot clones", err)
	}
	if _, err := f.dynamic.Resource(dataVolumeResource).Namespace(f.scope.Namespace).Get(ctx, dv.GetName(), metav1.GetOptions{}); err != nil {
		t.Fatal("the original DataVolume was not retained", err)
	}
	f.completeNative(t)
	var result NativeRestoreDTO
	for attempt := 0; attempt < 4 && !result.Complete; attempt++ {
		var err error
		result, err = f.engine.Start(ctx, f.scope, "snapshot-a", "restore-a")
		if err != nil {
			t.Fatal(err)
		}
	}
	if !result.Complete {
		t.Fatal("retaining the source prevented native restore completion")
	}
}

// capability_id: rainbond.vm-restore.native-storage-readiness
func TestNativeRestoreWaitsForLonghornCloneAndPersistsFailure(t *testing.T) {
	for _, outcome := range []string{"completed", "failed"} {
		t.Run(outcome, func(t *testing.T) {
			f := newManagedNativeFixture(t)
			ctx := context.Background()
			handle := "snap://source-volume/snapshot-a"
			for _, content := range f.snapshots.contents {
				content.Spec.Driver = "driver.longhorn.io"
				content.Status.SnapshotHandle = &handle
			}
			class, _ := f.core.StorageV1().StorageClasses().Get(ctx, "cephfs-vm-snapshot", metav1.GetOptions{})
			class.Provisioner = "driver.longhorn.io"
			if _, err := f.core.StorageV1().StorageClasses().Update(ctx, class, metav1.UpdateOptions{}); err != nil {
				t.Fatal(err)
			}
			if _, err := f.engine.Start(ctx, f.scope, "snapshot-a", "restore-a"); err != nil {
				t.Fatal(err)
			}
			f.completeNative(t)
			pv, _ := f.core.CoreV1().PersistentVolumes().Get(ctx, "restored-pv", metav1.GetOptions{})
			pv.Spec.CSI.Driver = "driver.longhorn.io"
			if _, err := f.core.CoreV1().PersistentVolumes().Update(ctx, pv, metav1.UpdateOptions{}); err != nil {
				t.Fatal(err)
			}
			gvr := schema.GroupVersionResource{Group: "longhorn.io", Version: "v1beta2", Resource: "volumes"}
			volume := &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": "longhorn.io/v1beta2", "kind": "Volume",
				"metadata": map[string]interface{}{"name": "restored-volume", "namespace": "storage-system", "uid": "longhorn-volume-uid"},
				"spec":     map[string]interface{}{"dataSource": handle},
				"status":   map[string]interface{}{"cloneStatus": map[string]interface{}{"state": "initiated"}},
			}}
			if _, err := f.dynamic.Resource(gvr).Namespace(volume.GetNamespace()).Create(ctx, volume, metav1.CreateOptions{}); err != nil {
				t.Fatal(err)
			}
			for attempt := 0; attempt < 3; attempt++ {
				result, err := f.engine.Start(ctx, f.scope, "snapshot-a", "restore-a")
				if err != nil || result.Complete || result.Phase != "pending" {
					t.Fatal("PVC Bound was treated as a completed backend clone", result, err)
				}
			}
			_ = unstructured.SetNestedField(volume.Object, "snap://another-volume/snapshot-a", "spec", "dataSource")
			if _, err := f.dynamic.Resource(gvr).Namespace(volume.GetNamespace()).Update(ctx, volume, metav1.UpdateOptions{}); err != nil {
				t.Fatal(err)
			}
			if result, err := f.engine.Start(ctx, f.scope, "snapshot-a", "restore-a"); !errors.Is(err, ErrConflict) || result.Complete {
				t.Fatal("a clone from another source was adopted", result, err)
			}
			_ = unstructured.SetNestedField(volume.Object, handle, "spec", "dataSource")
			_ = unstructured.SetNestedField(volume.Object, "completed", "status", "cloneStatus", "state")
			if _, err := f.dynamic.Resource(gvr).Namespace(volume.GetNamespace()).Update(ctx, volume, metav1.UpdateOptions{}); err != nil {
				t.Fatal(err)
			}
			if result, err := f.engine.Start(ctx, f.scope, "snapshot-a", "restore-a"); !errors.Is(err, ErrConflict) || result.Complete {
				t.Fatal("completion without the matching source identity was accepted", result, err)
			}
			_ = unstructured.SetNestedField(volume.Object, "source-volume", "status", "cloneStatus", "sourceVolume")
			_ = unstructured.SetNestedField(volume.Object, "snapshot-a", "status", "cloneStatus", "snapshot")
			_ = unstructured.SetNestedField(volume.Object, outcome, "status", "cloneStatus", "state")
			if _, err := f.dynamic.Resource(gvr).Namespace(volume.GetNamespace()).Update(ctx, volume, metav1.UpdateOptions{}); err != nil {
				t.Fatal(err)
			}
			var result NativeRestoreDTO
			for attempt := 0; attempt < 4; attempt++ {
				var err error
				result, err = f.engine.Start(ctx, f.scope, "snapshot-a", "restore-a")
				if err != nil {
					t.Fatal(err)
				}
				if result.Complete || result.Phase == "failed" {
					break
				}
			}
			if outcome == "completed" && !result.Complete || outcome == "failed" && (result.Phase != "failed" || result.Complete || result.RequiresAdvance) {
				t.Fatal("backend clone outcome was not reflected in restore status", result)
			}
			readback, err := f.engine.Status(ctx, f.scope, "snapshot-a", "restore-a")
			if err != nil || readback.Phase != result.Phase {
				t.Fatal("readback lost the backend clone outcome", readback, err)
			}
			if outcome == "failed" {
				control, err := f.managed.Store.ReadOnly(f.scope, "")
				if err != nil || control.State.Fence == nil || control.State.Bindings["7"].ActivePVC.Name != "active-pvc" {
					t.Fatal("failed clone committed the target disk or released the fence", err)
				}
			}
		})
	}
}
