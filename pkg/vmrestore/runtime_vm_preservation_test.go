package vmrestore

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	ktesting "k8s.io/client-go/testing"
	kv "kubevirt.io/api/core/v1"
)

func preservationVM(t *testing.T) (*kv.VirtualMachine, *unstructured.Unstructured) {
	t.Helper()
	vm := &kv.VirtualMachine{ObjectMeta: metav1.ObjectMeta{Name: "vm", Namespace: "team", UID: "vm-uid", ResourceVersion: "7", Labels: map[string]string{"service_id": "service", "tenant_id": "tenant"}}, Spec: kv.VirtualMachineSpec{Template: &kv.VirtualMachineInstanceTemplateSpec{}}}
	vm.Spec.Template.Spec.Domain.CPU = &kv.CPU{Cores: 1}
	vm.Spec.Template.Spec.Domain.Firmware = &kv.Firmware{UUID: "old-firmware"}
	vm.Spec.Template.Spec.Domain.Devices.Disks = []kv.Disk{{Name: "disk", DiskDevice: kv.DiskDevice{Disk: &kv.DiskTarget{Bus: "virtio"}}}}
	vm.Spec.Template.Spec.Domain.Devices.Interfaces = []kv.Interface{{Name: "default", MacAddress: "02:00:00:00:00:01"}}
	vm.Spec.Template.Spec.Volumes = []kv.Volume{{Name: "disk", VolumeSource: kv.VolumeSource{PersistentVolumeClaim: &kv.PersistentVolumeClaimVolumeSource{}}}}
	vm.Spec.Template.Spec.Volumes[0].PersistentVolumeClaim.ClaimName = "current-a"
	raw, err := EncodeRuntimeVM(vm)
	if err != nil {
		t.Fatal(err)
	}
	return vm, raw
}

// capability_id: rainbond.vm-restore.raw-runtime-preservation
func TestRuntimeDiffPreservesUnknownMapsMetadataAndStatus(t *testing.T) {
	for _, action := range []string{"halt", "firmware", "hardware", "known-list"} {
		t.Run(action, func(t *testing.T) {
			before, raw := preservationVM(t)
			_ = unstructured.SetNestedField(raw.Object, "Terminate", "spec", "template", "spec", "domain", "rebootPolicy")
			_ = unstructured.SetNestedMap(raw.Object, map[string]interface{}{"nested": map[string]interface{}{"preserve": "target-only"}}, "spec", "template", "spec", "domain", "futurePolicy")
			_ = unstructured.SetNestedField(raw.Object, "cpu-extension", "spec", "template", "spec", "domain", "cpu", "futureFeature")
			_ = unstructured.SetNestedMap(raw.Object, map[string]interface{}{"keep": "metadata"}, "metadata", "targetExtension")
			_ = unstructured.SetNestedMap(raw.Object, map[string]interface{}{"keep": "status"}, "status", "futureStatus")
			original := raw.DeepCopy()
			after := before.DeepCopy()
			switch action {
			case "halt":
				strategy := kv.RunStrategyHalted
				after.Spec.RunStrategy, after.Spec.Running = &strategy, nil
			case "firmware":
				after.Spec.Template.Spec.Domain.Firmware.UUID = "new-firmware"
			case "hardware":
				after.Spec.Template.Spec.Domain.CPU.Cores = 2
			case "known-list":
				after.Spec.Template.Spec.Domain.Devices.Interfaces[0].MacAddress = "02:00:00:00:00:02"
			}
			result, err := PreserveRuntimeVM(raw, before, after)
			if err != nil {
				t.Fatal(err)
			}
			for _, path := range [][]string{{"spec", "template", "spec", "domain", "rebootPolicy"}, {"spec", "template", "spec", "domain", "futurePolicy"}, {"spec", "template", "spec", "domain", "cpu", "futureFeature"}, {"metadata"}, {"status"}} {
				old, _, _ := unstructured.NestedFieldNoCopy(original.Object, path...)
				value, _, _ := unstructured.NestedFieldNoCopy(result.Object, path...)
				if !reflect.DeepEqual(old, value) {
					t.Fatal("uncontrolled target data changed", path)
				}
			}
			if !reflect.DeepEqual(original, raw) {
				t.Fatal("merge mutated its observation frame")
			}
			var projected kv.VirtualMachine
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(result.Object, &projected); err != nil {
				t.Fatal(err)
			}
			if !RuntimeVMMatches(&projected, after) {
				t.Fatal("preservation discarded the intended known VM changes")
			}
		})
	}
}

// capability_id: rainbond.vm-restore.raw-runtime-preservation
func TestOwnedRuntimeRawReadCannotAdoptLateForeignFence(t *testing.T) {
	f, engine, backend := kubeRuntimeFixture(t)
	client := backend.Dynamic.(*dynamicfake.FakeDynamicClient)
	client.ClearActions()
	gets := 0
	client.PrependReactor("get", "virtualmachines", func(action ktesting.Action) (bool, runtime.Object, error) {
		gets++
		if gets != 3 {
			return false, nil, nil
		}
		current, err := f.store.Read(f.scope, f.expect.OperationID)
		if err != nil {
			t.Fatal(err)
		}
		_, err = f.store.Mutate(f.scope, Expectation{ControlRevision: current.State.ControlRevision, BindingGeneration: current.State.BindingGeneration, OperationID: f.expect.OperationID, ManifestHash: current.Receipt.ManifestHash, RequestHash: current.Receipt.ManifestHash, Step: "test-late-release"}, func(c Current) (Mutation, error) {
			c.State.Fence = nil
			return Mutation{State: c.State, Receipt: c.Receipt}, nil
		})
		if err != nil {
			t.Fatal(err)
		}
		current, err = f.store.Read(f.scope, "")
		if err != nil {
			t.Fatal(err)
		}
		_, err = f.store.Mutate(f.scope, Expectation{ControlRevision: current.State.ControlRevision, BindingGeneration: current.State.BindingGeneration, OperationID: "foreign-op", ManifestHash: strings.Repeat("f", 64), RequestHash: strings.Repeat("e", 64), Step: "foreign-prepare"}, func(c Current) (Mutation, error) {
			receipt := NewReceipt(f.scope, "foreign-op", strings.Repeat("f", 64))
			c.State.Fence = &Fence{Kind: "restore", Phase: "prepared", OperationID: "foreign-op", ManifestHash: receipt.ManifestHash, TargetVMUID: f.intent.CurrentVM.UID}
			return Mutation{State: c.State, Receipt: &receipt}, nil
		})
		if err != nil {
			t.Fatal(err)
		}
		return false, nil, nil
	})
	_, err := engine.RuntimeStep(context.Background(), f.scope, f.expect.OperationID, "halt")
	if err == nil {
		t.Fatal("late foreign fence was adopted")
	}
	for _, action := range client.Actions() {
		if action.GetVerb() == "update" {
			t.Fatal("late owner change did not stop VM write")
		}
	}
}

// capability_id: rainbond.vm-restore.raw-runtime-preservation
func TestRuntimeDiffRefusesUnknownReplacementBeforeVMWrite(t *testing.T) {
	for _, change := range []string{"list-replacement", "subtree-removal"} {
		t.Run(change, func(t *testing.T) {
			before, raw := preservationVM(t)
			after := before.DeepCopy()
			switch change {
			case "list-replacement":
				volumes, _, _ := unstructured.NestedSlice(raw.Object, "spec", "template", "spec", "volumes")
				volumes[0].(map[string]interface{})["targetOnlySource"] = map[string]interface{}{"keep": true}
				_ = unstructured.SetNestedSlice(raw.Object, volumes, "spec", "template", "spec", "volumes")
				after.Spec.Template.Spec.Volumes[0].PersistentVolumeClaim.ClaimName = "restored-a"
			case "subtree-removal":
				_ = unstructured.SetNestedField(raw.Object, "keep", "spec", "template", "spec", "domain", "cpu", "targetOnlyFeature")
				after.Spec.Template.Spec.Domain.CPU = nil
			}
			client := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), raw)
			if _, err := UpdateObservedRuntimeVM(context.Background(), client, before, after); !errors.Is(err, ErrRestoreCapability) {
				t.Fatal("unrepresented subtree loss was not explicitly refused", err)
			}
			for _, action := range client.Actions() {
				if action.GetVerb() != "get" {
					t.Fatal("unsupported replacement wrote the VM", action.GetVerb())
				}
			}
			actual, _ := client.Resource(restoreVMGVR).Namespace(before.Namespace).Get(context.Background(), before.Name, metav1.GetOptions{})
			if !reflect.DeepEqual(raw, actual) {
				t.Fatal("refusal changed target object")
			}
		})
	}
}

// capability_id: rainbond.vm-restore.raw-runtime-preservation
func TestRuntimeDiffRejectsDifferentObservationFrame(t *testing.T) {
	for _, change := range []string{"uid", "rv", "scope", "known-spec", "owner-label"} {
		t.Run(change, func(t *testing.T) {
			before, raw := preservationVM(t)
			after := before.DeepCopy()
			strategy := kv.RunStrategyHalted
			after.Spec.RunStrategy = &strategy
			switch change {
			case "uid":
				raw.SetUID("replacement")
			case "rv":
				raw.SetResourceVersion("8")
			case "scope":
				raw.SetNamespace("foreign")
			case "known-spec":
				_ = unstructured.SetNestedField(raw.Object, int64(3), "spec", "template", "spec", "domain", "cpu", "cores")
			case "owner-label":
				labels := raw.GetLabels()
				labels["service_id"] = "foreign"
				raw.SetLabels(labels)
			}
			original := raw.DeepCopy()
			if _, err := PreserveRuntimeVM(raw, before, after); !errors.Is(err, ErrConflict) {
				t.Fatal("latest raw was adopted as a new authority", err)
			}
			if !reflect.DeepEqual(original, raw) {
				t.Fatal("rejected observation was mutated")
			}
		})
	}
}
