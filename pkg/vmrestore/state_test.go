package vmrestore

import (
	"errors"
	"strings"
	"testing"
)

// capability_id: rainbond.vm-restore.state-scope-and-resolver
func TestStateScopeAndResolver(t *testing.T) {
	scope := Scope{TenantID: "tenant", ServiceID: "service", Namespace: "team"}
	state := NewState(scope)
	if _, ok, err := state.ResolveClaim(123); err != nil || ok {
		t.Fatal("unrestored volume did not use canonical fallback")
	}
	original := ObjectRef{Kind: "PersistentVolumeClaim", Namespace: "team", Name: "manual123", UID: "source-uid"}
	active := ObjectRef{Kind: "PersistentVolumeClaim", Namespace: "team", Name: "restored-123", UID: "target-uid"}
	state.Bindings["123"] = Binding{Role: "root", Attached: true, VolumeID: 123, DiskKey: "disk", CanonicalName: "manual123", ActivePVC: active, PreviousPVC: &original, RestoreID: "operation-a", SnapshotUID: "snapshot-uid"}
	ref, found, err := state.ResolveClaim(123)
	if err != nil || !found || ref != active {
		t.Fatal("restored active claim was not selected")
	}
	if err := CheckLivePVC(ref, "team", "restored-123", "replacement-uid"); !errors.Is(err, ErrConflict) {
		t.Fatal("same-name replacement was accepted")
	}
	if err := state.Validate(Scope{TenantID: "other", ServiceID: "service", Namespace: "team"}); !errors.Is(err, ErrCorrupt) {
		t.Fatal("cross-tenant state accepted")
	}
	state.Bindings["123"] = Binding{}
	if _, _, err := state.ResolveClaim(123); !errors.Is(err, ErrCorrupt) {
		t.Fatal("corrupt binding fell back to original source")
	}
}

// capability_id: rainbond.vm-restore.private-attributes
func TestPrivateAttributeNames(t *testing.T) {
	for _, name := range []string{StateKey, LegacyBindingsKey, ReceiptPrefix + "operation-a", "VM_RESTORE_STATE", "vm_volume_bindings "} {
		if !errors.Is(CheckPublicAttribute(name), ErrInternalAttribute) {
			t.Fatal("internal key exposed")
		}
	}
	for _, name := range []string{"vm_disk_layout", "vm_boot_mode", "nodeSelector"} {
		if CheckPublicAttribute(name) != nil {
			t.Fatal("public config was incorrectly hidden")
		}
	}
}

// capability_id: rainbond.vm-restore.durable-admission
func TestAdmissionDoesNotExpireOrReleaseOtherOwner(t *testing.T) {
	state := NewState(Scope{TenantID: "tenant", ServiceID: "service", Namespace: "team"})
	if err := state.AdmitMutation("action-a", "start"); err != nil {
		t.Fatal(err)
	}
	if err := state.FinishMutation("action-b", true); !errors.Is(err, ErrConflict) {
		t.Fatal("wrong owner released admission")
	}
	if err := state.FinishMutation("action-a", false); err != nil {
		t.Fatal(err)
	}
	if state.Admission == nil || !state.Admission.OutcomeUnknown {
		t.Fatal("unknown outcome lost durable admission")
	}
	if err := state.AdmitMutation("action-b", "start"); !errors.Is(err, ErrBusy) {
		t.Fatal("unknown outcome automatically expired")
	}
	if err := state.FinishMutation("action-a", true); err != nil {
		t.Fatal(err)
	}
	state.Fence = &Fence{Kind: "restore", Phase: "prepared", OperationID: "operation-a", ManifestHash: strings.Repeat("a", 64), TargetVMUID: "vm-uid"}
	if err := state.AdmitMutation("action-b", "start"); !errors.Is(err, ErrBusy) {
		t.Fatal("restore fence did not reject mutation")
	}
}

func TestRecordRejectsUnknownSchemaFieldsAndTrailingJSON(t *testing.T) {
	for _, input := range []string{`{"schema_version":1,"unknown":true}`, `{} {}`, strings.Repeat("x", MaxRecordBytes+1)} {
		var state State
		if !errors.Is(DecodeRecord(input, &state), ErrCorrupt) {
			t.Fatal("invalid persisted state was accepted")
		}
	}
}

func TestDetachedBindingNeverFallsBackAndCrossNamespaceRejected(t *testing.T) {
	scope := Scope{TenantID: "tenant", ServiceID: "service", Namespace: "team"}
	state := NewState(scope)
	state.Bindings["7"] = Binding{VolumeID: 7, DiskKey: "data", CanonicalName: "manual7", Role: "data", Attached: false, ActivePVC: ObjectRef{Kind: "PersistentVolumeClaim", Namespace: "team", Name: "retained-b", UID: "b-uid"}, RestoreID: "operation-a", SnapshotUID: "snap-uid"}
	_, found, err := state.ResolveClaim(7)
	if !found || !errors.Is(err, ErrDetached) {
		t.Fatal("detached disk fell back to canonical source")
	}
	resolved, err := state.ResolveVolume(7)
	if err != nil || !resolved.Mapped || resolved.Attached || resolved.DV != nil {
		t.Fatal("plain detached PVC acquired a fabricated DV")
	}
	binding := state.Bindings["7"]
	binding.ActivePVC.Namespace = "another-team"
	state.Bindings["7"] = binding
	if _, _, err := state.ResolveClaim(7); !errors.Is(err, ErrCorrupt) {
		t.Fatal("cross-namespace PVC was accepted")
	}
}

func stateValidationBinding() Binding {
	return Binding{VolumeID: 123, DiskKey: "root", CanonicalName: "manual123", Role: "root", Attached: true,
		ActivePVC: ObjectRef{Kind: "PersistentVolumeClaim", Namespace: "team", Name: "restored-123", UID: "pvc-uid"},
		RestoreID: "operation-a", SnapshotUID: "snapshot-uid"}
}

func TestStateRejectsNonCanonicalBindingKeys(t *testing.T) {
	for _, duplicate := range []bool{false, true} {
		state := NewState(Scope{TenantID: "tenant", ServiceID: "service", Namespace: "team"})
		state.Bindings["00123"] = stateValidationBinding()
		if duplicate {
			state.Bindings["123"] = stateValidationBinding()
		}
		if !errors.Is(state.Validate(state.Scope), ErrCorrupt) {
			t.Error("numeric alias was accepted as a canonical binding key")
		}
		if _, _, err := state.ResolveClaim(123); !errors.Is(err, ErrCorrupt) {
			t.Error("numeric alias allowed claim fallback or selection")
		}
		if _, err := state.ResolveVolume(123); !errors.Is(err, ErrCorrupt) {
			t.Error("numeric alias allowed volume fallback or selection")
		}
	}
}

func TestStateReceiptResourceIdentities(t *testing.T) {
	scope := Scope{TenantID: "tenant", ServiceID: "service", Namespace: "team"}
	valid := ResourceRef{ObjectRef: ObjectRef{Kind: "PersistentVolumeClaim", Namespace: "team", Name: "target", UID: "target-uid"}, Purpose: "target"}
	invalid := map[string]ResourceRef{}
	for name, change := range map[string]func(*ResourceRef){
		"foreign_namespace": func(r *ResourceRef) { r.Namespace = "other-team" },
		"missing_namespace": func(r *ResourceRef) { r.Namespace = "" },
		"missing_uid":       func(r *ResourceRef) { r.UID = "" },
		"missing_name":      func(r *ResourceRef) { r.Name = "" },
		"invalid_name":      func(r *ResourceRef) { r.Name = "bad/name" },
		"empty_dns_label":   func(r *ResourceRef) { r.Name = "bad..name" },
		"too_long_name":     func(r *ResourceRef) { r.Name = strings.Repeat("a", 254) },
		"missing_purpose":   func(r *ResourceRef) { r.Purpose = "" },
		"invalid_purpose":   func(r *ResourceRef) { r.Purpose = "bad/purpose" },
		"unknown_kind":      func(r *ResourceRef) { r.Kind = "Secret" },
		"namespace_delete":  func(r *ResourceRef) { r.Kind = "Namespace"; r.Name = scope.Namespace; r.Namespace = "" },
		"cluster_with_ns":   func(r *ResourceRef) { r.Kind = "PersistentVolume" },
	} {
		r := valid
		change(&r)
		invalid[name] = r
	}
	for _, location := range []string{"receipt", "step"} {
		for name, ref := range invalid {
			t.Run(location+"/"+name, func(t *testing.T) {
				r := NewReceipt(scope, "operation-a", strings.Repeat("a", 64))
				if location == "receipt" {
					r.Resources = []ResourceRef{ref}
				} else {
					r.Steps["copy"] = StepResult{RequestHash: strings.Repeat("b", 64), Resources: []ResourceRef{ref}, Applied: true}
				}
				if !errors.Is(r.Validate(scope, r.OperationID), ErrCorrupt) {
					t.Fatal("unsafe resource identity was accepted")
				}
			})
		}
	}
	for _, kind := range []string{"PersistentVolumeClaim", "DataVolume", "VirtualMachine", "VirtualMachineInstance", "Pod", "Job", "ConfigMap", "VirtualMachineSnapshot", "VirtualMachineSnapshotContent", "VolumeSnapshot", "PersistentVolume", "StorageClass", "VolumeSnapshotContent"} {
		t.Run("valid/"+kind, func(t *testing.T) {
			ref := valid
			ref.Kind = kind
			if kind == "PersistentVolume" || kind == "StorageClass" || kind == "VolumeSnapshotContent" {
				ref.Namespace = ""
			}
			r := NewReceipt(scope, "operation-a", strings.Repeat("a", 64))
			r.Resources = []ResourceRef{ref}
			r.Steps["copy"] = StepResult{RequestHash: strings.Repeat("b", 64), Resources: []ResourceRef{ref}, Applied: true}
			if err := r.Validate(scope, r.OperationID); err != nil {
				t.Fatal("supported scoped resource was rejected")
			}
		})
	}
}

func TestStateReceiptRejectsInvalidScope(t *testing.T) {
	for _, scope := range []Scope{{ServiceID: "service", Namespace: "team"}, {TenantID: "tenant", ServiceID: "service", Namespace: "team-"}} {
		r := NewReceipt(scope, "operation-a", strings.Repeat("a", 64))
		if !errors.Is(r.Validate(scope, r.OperationID), ErrCorrupt) {
			t.Fatal("invalid receipt scope was accepted")
		}
	}
}

func TestStateReceiptAcceptsKubernetesSubdomainNames(t *testing.T) {
	scope := Scope{TenantID: "tenant", ServiceID: "service", Namespace: "team"}
	for _, length := range []int{64, 253} {
		r := NewReceipt(scope, "operation-a", strings.Repeat("a", 64))
		ref := ResourceRef{ObjectRef: ObjectRef{Kind: "PersistentVolumeClaim", Namespace: scope.Namespace, Name: strings.Repeat("a", length), UID: "pvc-uid"}, Purpose: "target"}
		r.Resources = []ResourceRef{ref}
		r.Steps["copy"] = StepResult{RequestHash: strings.Repeat("b", 64), Resources: []ResourceRef{ref}, Applied: true}
		if err := r.Validate(scope, r.OperationID); err != nil {
			t.Error("legal Kubernetes subdomain resource name was rejected")
		}
	}
}

func TestStateResolutionDoesNotExposeMutableIdentity(t *testing.T) {
	state := NewState(Scope{TenantID: "tenant", ServiceID: "service", Namespace: "team"})
	binding := stateValidationBinding()
	binding.ActiveDV = &ObjectRef{Kind: "DataVolume", Namespace: "team", Name: "active-dv", UID: "dv-uid"}
	state.Bindings["123"] = binding
	resolved, err := state.ResolveVolume(123)
	if err != nil || resolved.PVC == nil || resolved.DV == nil {
		t.Fatal("valid binding did not resolve")
	}
	resolved.PVC.Name = "reader-change"
	resolved.DV.Name = "reader-change"
	if state.Bindings["123"].ActivePVC.Name != "restored-123" || state.Bindings["123"].ActiveDV.Name != "active-dv" {
		t.Fatal("reader modified authoritative identities through returned references")
	}
}
