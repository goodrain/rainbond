package vmrestore

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	dbmodel "github.com/goodrain/rainbond/db/model"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func (f resolverFixture) footprintPVC(t *testing.T, operation string, id uint, generation uint64) {
	t.Helper()
	pvc, err := f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Get(context.Background(), "manual7", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	pvc = pvc.DeepCopy()
	pvc.Name = "restore-target"
	pvc.UID = ""
	if err := MarkRestorePVC(pvc, f.scope, id, generation, operation); err != nil {
		t.Fatal(err)
	}
	pvc.UID = "target-uid"
	if _, err := f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Create(context.Background(), pvc, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
}

// capability_id: rainbond.vm-restore.lost-ledger-footprint
func TestLostWholeLedgerCannotFallbackToOriginalB(t *testing.T) {
	for _, kind := range []string{"vm", "pvc"} {
		t.Run(kind, func(t *testing.T) {
			f := newResolverFixture(t)
			if kind == "pvc" {
				f.footprintPVC(t, "lost-operation", 7, 1)
			} else {
				vm := &unstructured.Unstructured{Object: map[string]interface{}{"apiVersion": "kubevirt.io/v1", "kind": "VirtualMachine", "metadata": map[string]interface{}{"name": "managed-vm", "namespace": f.scope.Namespace, "uid": "vm-uid", "labels": map[string]interface{}{"service_id": f.scope.ServiceID}, "annotations": map[string]interface{}{BindingGenerationAnnotation: "1"}}}}
				f.dynamicObject(t, virtualMachineResource, vm)
			}
			if _, err := f.resolver.ResolveVolume(context.Background(), f.scope, 7); !errors.Is(err, ErrCorrupt) {
				t.Fatal("restoration footprint without ledger selected original B")
			}
		})
	}
}

func TestLegitimateInFlightTargetDoesNotBecomeASecondMapping(t *testing.T) {
	f := newResolverFixture(t)
	e := Expectation{OperationID: "operation-a", ManifestHash: strings.Repeat("a", 64), Step: "prepare", RequestHash: strings.Repeat("b", 64)}
	_, err := f.store.Mutate(f.scope, e, func(c Current) (Mutation, error) {
		c.State.Fence = &Fence{Kind: "restore", Phase: "allocating", OperationID: e.OperationID, ManifestHash: e.ManifestHash, TargetVMUID: "vm-uid"}
		receipt := NewReceipt(f.scope, e.OperationID, e.ManifestHash)
		return Mutation{State: c.State, Receipt: &receipt}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	f.footprintPVC(t, e.OperationID, 7, 1)
	resolved, err := f.resolver.ResolveVolume(context.Background(), f.scope, 7)
	if err != nil || resolved.Mapped || resolved.ActivePVC.Name != "manual7" || resolved.ActivePVC.UID != "pvc-original" {
		t.Fatal("inflight target was either blocked or adopted as a label-derived mapping")
	}
}

func TestHealthyGenerationAllowsNewUnmappedDiskWithoutItsOwnRestoreFootprint(t *testing.T) {
	f := newResolverFixture(t)
	f.seed(t, f.currentBinding())
	if err := f.store.db.Create(&dbmodel.TenantServiceVolume{Model: dbmodel.Model{ID: 8}, ServiceID: f.scope.ServiceID, VolumeName: "later-data", VolumeType: "cephfs-vm-snapshot"}).Error; err != nil {
		t.Fatal(err)
	}
	vm := &unstructured.Unstructured{Object: map[string]interface{}{"apiVersion": "kubevirt.io/v1", "kind": "VirtualMachine", "metadata": map[string]interface{}{"name": "managed-vm", "namespace": f.scope.Namespace, "uid": "vm-uid", "labels": map[string]interface{}{"service_id": f.scope.ServiceID}, "annotations": map[string]interface{}{BindingGenerationAnnotation: "1"}}}}
	f.dynamicObject(t, virtualMachineResource, vm)
	resolved, err := f.resolver.ResolveVolume(context.Background(), f.scope, 8)
	if err != nil || resolved.Mapped || resolved.ActivePVC != nil || resolved.Provisioning != "not_provisioned" {
		t.Fatal("new disk was confused with lost mapping")
	}
}

func TestMarkersDoNotAdoptExistingResourceOrUseTestLabelsAsAuthority(t *testing.T) {
	f := newResolverFixture(t)
	existing, err := f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Get(context.Background(), "manual7", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if err := MarkRestorePVC(existing, f.scope, 7, 1, "operation-a"); !errors.Is(err, ErrCorrupt) {
		t.Fatal("marker helper relabeled an existing resource")
	}
	pod := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "test-only", Namespace: f.scope.Namespace, UID: "test-uid", Labels: map[string]string{"codex.rainbond.io/owner-thread": "synthetic-test"}}}
	if _, err := f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Create(context.Background(), pod, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	resolved, err := f.resolver.ResolveVolume(context.Background(), f.scope, 7)
	if err != nil || resolved.ActivePVC.UID != "pvc-original" {
		t.Fatal("test-only labels affected production binding authority")
	}
}

func TestOrphanBindingWithoutOwnedLogicalRecordIsRejected(t *testing.T) {
	f := newResolverFixture(t)
	f.seed(t, f.currentBinding())
	if err := f.store.db.Where("ID = ?", 7).Delete(&dbmodel.TenantServiceVolume{}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := f.resolver.ReadBindingView(context.Background(), f.scope); !errors.Is(err, ErrCorrupt) {
		t.Fatal("orphan binding was silently dropped from complete-set view")
	}
}

func TestCommittedBindingFootprintSurvivesMissingResources(t *testing.T) {
	for _, rollback := range []bool{false, true} {
		name := "committed"
		if rollback {
			name = "rolled-back-canonical"
		}
		t.Run(name, func(t *testing.T) {
			f := newResolverFixture(t)
			old := f.currentBinding()
			active := old
			active.ActivePVC.Name, active.ActivePVC.UID = "restored-a", "pvc-restored"
			active.RestoreID, active.SnapshotUID = "operation-a", "snapshot-origin"
			e := Expectation{OperationID: "operation-a", ManifestHash: strings.Repeat("a", 64), Step: "restore-commit", RequestHash: strings.Repeat("b", 64)}
			_, err := f.store.Mutate(f.scope, e, func(c Current) (Mutation, error) {
				c.State.Bindings["7"] = active
				c.State.BindingGeneration++
				receipt := NewReceipt(f.scope, e.OperationID, e.ManifestHash)
				receipt.OldBindings["7"], receipt.NewBindings["7"] = old, active
				receipt.ConfigCommitHash = e.RequestHash
				return Mutation{State: c.State, Receipt: &receipt}, nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if rollback {
				e.ControlRevision, e.BindingGeneration = 1, 1
				e.Step, e.RequestHash = "restore-rollback", strings.Repeat("c", 64)
				_, err = f.store.Mutate(f.scope, e, func(c Current) (Mutation, error) {
					c.State.Bindings["7"] = old
					c.State.BindingGeneration++
					return Mutation{State: c.State, Receipt: c.Receipt}, nil
				})
				if err != nil {
					t.Fatal(err)
				}
				resolved, err := f.resolver.ResolveVolume(context.Background(), f.scope, 7)
				if err != nil || !resolved.Mapped || resolved.BindingGeneration != 2 || resolved.ActivePVC == nil || resolved.ActivePVC.UID != old.ActivePVC.UID {
					t.Fatal("healthy rollback lost its pinned canonical identity")
				}
			}
			// A and the VM do not exist. Only the valid commit witness survives.
			var row dbmodel.ComponentK8sAttributes
			if err := f.store.db.Where("component_id = ? AND name = ?", f.scope.ServiceID, StateKey).First(&row).Error; err != nil {
				t.Fatal(err)
			}
			var state State
			if err := DecodeRecord(row.AttributeValue, &state); err != nil {
				t.Fatal(err)
			}
			delete(state.Bindings, "7")
			value, err := EncodeRecord(state)
			if err != nil {
				t.Fatal(err)
			}
			if err := f.store.db.Model(&row).Update("attribute_value", value).Error; err != nil {
				t.Fatal(err)
			}
			if _, err := f.resolver.ResolveVolume(context.Background(), f.scope, 7); !errors.Is(err, ErrCorrupt) {
				t.Fatalf("missing committed mapping selected canonical B: %v", err)
			}
			if _, err := f.resolver.ReadBindingView(context.Background(), f.scope); !errors.Is(err, ErrCorrupt) {
				t.Errorf("binding view accepted missing committed mapping: %v", err)
			}
			if _, err := f.resolver.VolumeReferences(context.Background(), f.scope, 7); !errors.Is(err, ErrCorrupt) {
				t.Errorf("reference view accepted missing committed mapping: %v", err)
			}
			var after dbmodel.ComponentK8sAttributes
			if err := f.store.db.First(&after, row.ID).Error; err != nil {
				t.Fatal(err)
			}
			if after.AttributeValue != value {
				t.Fatal("reader reconstructed the missing mapping from a receipt")
			}
		})
	}
}

func TestPreparedCanonicalBindingsDoNotImplyCommittedMapping(t *testing.T) {
	for _, mode := range []string{"first-old-only", "first-planned", "later-old-only", "later-planned", "cancelled-plan"} {
		t.Run(mode, func(t *testing.T) {
			f := newResolverFixture(t)
			if strings.HasPrefix(mode, "later-") {
				if err := f.store.db.Create(&dbmodel.TenantServiceVolume{Model: dbmodel.Model{ID: 8}, ServiceID: f.scope.ServiceID, VolumeName: "earlier-data", VolumeType: "cephfs-vm-snapshot"}).Error; err != nil {
					t.Fatal(err)
				}
				f.addClaim(t, "manual8", "pvc-8", "pv-8")
				prior := f.currentBinding()
				prior.VolumeID, prior.DiskKey, prior.CanonicalName = 8, "earlier-data", "manual8"
				prior.ActivePVC.Name, prior.ActivePVC.UID = "manual8", "pvc-8"
				f.seed(t, prior)
			}
			current, err := f.store.Read(f.scope, "")
			if err != nil {
				t.Fatal(err)
			}
			e := Expectation{ControlRevision: current.State.ControlRevision, BindingGeneration: current.State.BindingGeneration, OperationID: "operation-a", ManifestHash: strings.Repeat("a", 64), Step: "restore-prepare", RequestHash: strings.Repeat("b", 64)}
			_, err = f.store.Mutate(f.scope, e, func(c Current) (Mutation, error) {
				c.State.Fence = &Fence{Kind: "restore", Phase: "prepared", OperationID: e.OperationID, ManifestHash: e.ManifestHash, TargetVMUID: "vm-uid"}
				receipt := NewReceipt(f.scope, e.OperationID, e.ManifestHash)
				receipt.OldBindings["7"] = f.currentBinding()
				if strings.Contains(mode, "plan") {
					planned := f.currentBinding()
					planned.ActivePVC.Name, planned.ActivePVC.UID = "restored-a", "not-created-yet"
					planned.RestoreID, planned.SnapshotUID = e.OperationID, "snapshot-origin"
					receipt.NewBindings["7"] = planned
				}
				return Mutation{State: c.State, Receipt: &receipt}, nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if mode == "cancelled-plan" {
				e.ControlRevision++
				e.Step, e.RequestHash = "restore-cancelled", strings.Repeat("c", 64)
				_, err = f.store.Mutate(f.scope, e, func(c Current) (Mutation, error) {
					c.State.Fence = nil
					return Mutation{State: c.State, Receipt: c.Receipt}, nil
				})
				if err != nil {
					t.Fatal(err)
				}
			}
			resolved, err := f.resolver.ResolveVolume(context.Background(), f.scope, 7)
			if err != nil || resolved.Mapped || resolved.ActivePVC == nil || resolved.ActivePVC.UID != "pvc-original" || resolved.BindingGeneration != current.State.BindingGeneration {
				t.Fatalf("uncommitted receipt blocked genuine canonical resolution: %v", err)
			}
		})
	}
}

func TestNewDiskAbsentFromCommittedReceiptCanResolveCanonically(t *testing.T) {
	f := newResolverFixture(t)
	old := f.currentBinding()
	e := Expectation{OperationID: "operation-a", ManifestHash: strings.Repeat("a", 64), Step: "restore-commit", RequestHash: strings.Repeat("b", 64)}
	_, err := f.store.Mutate(f.scope, e, func(c Current) (Mutation, error) {
		c.State.Bindings["7"] = old
		c.State.BindingGeneration++
		receipt := NewReceipt(f.scope, e.OperationID, e.ManifestHash)
		receipt.NewBindings["7"] = old
		receipt.ConfigCommitHash = e.RequestHash
		return Mutation{State: c.State, Receipt: &receipt}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.store.db.Create(&dbmodel.TenantServiceVolume{Model: dbmodel.Model{ID: 8}, ServiceID: f.scope.ServiceID, VolumeName: "later-data", VolumeType: "cephfs-vm-snapshot"}).Error; err != nil {
		t.Fatal(err)
	}
	f.addClaim(t, "manual8", "pvc-8", "pv-8")
	resolved, err := f.resolver.ResolveVolume(context.Background(), f.scope, 8)
	if err != nil || resolved.Mapped || resolved.ActivePVC == nil || resolved.ActivePVC.UID != "pvc-8" {
		t.Fatalf("healthy new disk was confused with committed disk 7: %v", err)
	}
}

func TestCommitFootprintRequiresMatchingSealedSuccess(t *testing.T) {
	for _, mode := range []string{"hash-without-step", "step-without-hash", "different-hash", "zero-generation"} {
		t.Run(mode, func(t *testing.T) {
			f := newResolverFixture(t)
			e := Expectation{OperationID: "operation-a", ManifestHash: strings.Repeat("a", 64), Step: "restore-commit", RequestHash: strings.Repeat("b", 64)}
			_, err := f.store.Mutate(f.scope, e, func(c Current) (Mutation, error) {
				c.State.BindingGeneration++
				receipt := NewReceipt(f.scope, e.OperationID, e.ManifestHash)
				receipt.NewBindings["7"] = f.currentBinding()
				receipt.ConfigCommitHash = e.RequestHash
				return Mutation{State: c.State, Receipt: &receipt}, nil
			})
			if err != nil {
				t.Fatal(err)
			}
			var row dbmodel.ComponentK8sAttributes
			if err := f.store.db.Where("component_id = ? AND name = ?", f.scope.ServiceID, ReceiptPrefix+e.OperationID).First(&row).Error; err != nil {
				t.Fatal(err)
			}
			var receipt Receipt
			if err := DecodeRecord(row.AttributeValue, &receipt); err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "hash-without-step":
				receipt.Steps["restore-prepare"] = receipt.Steps[e.Step]
				delete(receipt.Steps, e.Step)
			case "step-without-hash":
				receipt.ConfigCommitHash = ""
			case "different-hash":
				receipt.ConfigCommitHash = strings.Repeat("c", 64)
			case "zero-generation":
				step := receipt.Steps[e.Step]
				step.BindingGeneration = 0
				receipt.Steps[e.Step] = step
			}
			value, err := json.Marshal(receipt)
			if err != nil {
				t.Fatal(err)
			}
			if err := f.store.db.Model(&row).Update("attribute_value", string(value)).Error; err != nil {
				t.Fatal(err)
			}
			if _, err := f.resolver.ResolveVolume(context.Background(), f.scope, 7); !errors.Is(err, ErrCorrupt) {
				t.Fatalf("invalid commit witness allowed fallback: %v", err)
			}
		})
	}
}

func TestVMGenerationMarkerPreservesMetadataAndRejectsWrongScope(t *testing.T) {
	scope := Scope{TenantID: "tenant", ServiceID: "service", Namespace: "team"}
	vm := &unstructured.Unstructured{Object: map[string]interface{}{"metadata": map[string]interface{}{"namespace": scope.Namespace, "annotations": map[string]interface{}{"existing": "preserved"}}}}
	if err := MarkVMGeneration(vm, scope, 3); err != nil || vm.GetAnnotations()[BindingGenerationAnnotation] != "3" || vm.GetAnnotations()["existing"] != "preserved" {
		t.Fatal("generation stamp did not preserve existing metadata")
	}
	bare := &unstructured.Unstructured{Object: map[string]interface{}{"metadata": map[string]interface{}{"namespace": scope.Namespace}}}
	if err := MarkVMGeneration(bare, scope, 0); err != nil || bare.GetAnnotations()[BindingGenerationAnnotation] != "0" {
		t.Fatal("initial desired VM generation could not be stamped")
	}
	wrong := bare.DeepCopy()
	wrong.SetNamespace("other")
	if err := MarkVMGeneration(wrong, scope, 1); !errors.Is(err, ErrCorrupt) {
		t.Fatal("cross-namespace VM was stamped")
	}
	if err := MarkVMGeneration(nil, scope, 1); !errors.Is(err, ErrCorrupt) {
		t.Fatal("nil VM was accepted")
	}
}

func TestUnmappedNewDiskChecksKnownAndMalformedFootprints(t *testing.T) {
	for _, mode := range []string{"known-binding", "known-receipt", "unknown-operation", "malformed-vm-generation", "known-pvc", "malformed-pvc-generation"} {
		t.Run(mode, func(t *testing.T) {
			f := newResolverFixture(t)
			binding := f.currentBinding()
			if mode == "known-binding" || mode == "known-pvc" {
				binding.RestoreID, binding.SnapshotUID = "prior-operation", "snapshot-origin"
			}
			f.seed(t, binding)
			if err := f.store.db.Create(&dbmodel.TenantServiceVolume{Model: dbmodel.Model{ID: 8}, ServiceID: f.scope.ServiceID, VolumeName: "new-data", VolumeType: "cephfs-vm-snapshot"}).Error; err != nil {
				t.Fatal(err)
			}
			if mode == "known-receipt" {
				e := Expectation{ControlRevision: 1, BindingGeneration: 1, OperationID: "prior-operation", ManifestHash: strings.Repeat("a", 64), Step: "retain", RequestHash: strings.Repeat("b", 64)}
				if _, err := f.store.Mutate(f.scope, e, func(c Current) (Mutation, error) {
					receipt := NewReceipt(f.scope, e.OperationID, e.ManifestHash)
					return Mutation{State: c.State, Receipt: &receipt}, nil
				}); err != nil {
					t.Fatal(err)
				}
			}
			if strings.Contains(mode, "pvc") {
				claim, err := f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Get(context.Background(), "manual7", metav1.GetOptions{})
				if err != nil {
					t.Fatal(err)
				}
				claim.Annotations = map[string]string{BindingGenerationAnnotation: "1", RestoreVolumeIDAnnotation: "7"}
				claim.Labels[RestoreOperationLabel] = "prior-operation"
				if mode == "malformed-pvc-generation" {
					claim.Annotations[BindingGenerationAnnotation] = "01"
				}
				if _, err := f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Update(context.Background(), claim, metav1.UpdateOptions{}); err != nil {
					t.Fatal(err)
				}
			} else {
				generation := "1"
				if mode == "malformed-vm-generation" {
					generation = "01"
				}
				vm := &unstructured.Unstructured{Object: map[string]interface{}{"apiVersion": "kubevirt.io/v1", "kind": "VirtualMachine", "metadata": map[string]interface{}{"name": "managed-vm", "namespace": f.scope.Namespace, "uid": "vm-uid", "labels": map[string]interface{}{"service_id": f.scope.ServiceID, RestoreOperationLabel: "prior-operation"}, "annotations": map[string]interface{}{BindingGenerationAnnotation: generation}}}}
				f.dynamicObject(t, virtualMachineResource, vm)
			}
			resolved, err := f.resolver.ResolveVolume(context.Background(), f.scope, 8)
			if strings.HasPrefix(mode, "known-") {
				if err != nil || resolved.Mapped || resolved.ActivePVC != nil || resolved.Provisioning != "not_provisioned" {
					t.Fatalf("known prior footprint blocked genuinely new disk: %v", err)
				}
			} else if !errors.Is(err, ErrCorrupt) {
				t.Fatalf("unknown or malformed footprint permitted fallback: %v", err)
			}
		})
	}
}
