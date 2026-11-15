package vmrestore

import (
	"context"
	"errors"
	"strings"
	"testing"

	dbmodel "github.com/goodrain/rainbond/db/model"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ktesting "k8s.io/client-go/testing"
)

type reclaimReferenceFixture struct {
	resolverFixture
	operation string
	hash      string
	ctx       context.Context
	binding   Binding
}

func newReclaimReferenceFixture(t *testing.T) reclaimReferenceFixture {
	t.Helper()
	f := newResolverFixture(t)
	binding := f.currentBinding()
	binding.ActiveDV = &ObjectRef{Kind: "DataVolume", Namespace: f.scope.Namespace, Name: "active-dv", UID: "active-dv-uid"}
	f.dynamicObject(t, dataVolumeResource, &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "cdi.kubevirt.io/v1beta1", "kind": "DataVolume",
		"metadata": map[string]interface{}{"name": binding.ActiveDV.Name, "namespace": f.scope.Namespace,
			"uid": binding.ActiveDV.UID, "labels": map[string]interface{}{"service_id": f.scope.ServiceID, "tenant_id": f.scope.TenantID}},
		"status": map[string]interface{}{"claimName": binding.ActivePVC.Name},
	}})
	f.seed(t, binding)
	operation, hash := "disk-delete-owned", strings.Repeat("a", 64)
	current, err := f.store.Read(f.scope, "")
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := f.store.Mutate(f.scope, Expectation{ControlRevision: current.State.ControlRevision,
		BindingGeneration: current.State.BindingGeneration, OperationID: operation, ManifestHash: hash,
		Step: "prepare", RequestHash: hash}, func(c Current) (Mutation, error) {
		if err := c.State.AdmitMutation(operation, "disk-delete-"+hash); err != nil {
			return Mutation{}, err
		}
		receipt := NewReceipt(f.scope, operation, hash)
		receipt.OldBindings["7"], receipt.NewBindings["7"] = binding, binding
		receipt.Resources = []ResourceRef{{ObjectRef: binding.ActivePVC, Purpose: "mutation-active-pvc"},
			{ObjectRef: *binding.ActiveDV, Purpose: "mutation-active-dv"}}
		return Mutation{State: c.State, Receipt: &receipt}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.store.Mutate(f.scope, Expectation{ControlRevision: prepared.State.ControlRevision,
		BindingGeneration: prepared.State.BindingGeneration, OperationID: operation, ManifestHash: hash,
		Step: "issued", RequestHash: hash}, func(c Current) (Mutation, error) {
		return Mutation{State: c.State, Receipt: c.Receipt}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return reclaimReferenceFixture{f, operation, hash, WithAdmissionContext(context.Background(), operation), binding}
}

// capability_id: rainbond.vm-restore.mutation-volume-references
func TestMutationReferencesRecheckSurvivingPVCWithoutRestoringDeletedDV(t *testing.T) {
	f := newReclaimReferenceFixture(t)
	if err := f.dynamic.Resource(dataVolumeResource).Namespace(f.scope.Namespace).Delete(f.ctx, f.binding.ActiveDV.Name, metav1.DeleteOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.resolver.ResolveVolume(f.ctx, f.scope, 7); !errors.Is(err, ErrConflict) {
		t.Fatal("ordinary resolution must still reject missing active DV")
	}
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "late-reader", Namespace: f.scope.Namespace, UID: "late-reader-uid"},
		Spec: corev1.PodSpec{Volumes: []corev1.Volume{{Name: "data", VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: f.binding.ActivePVC.Name}}}}}}
	if _, err := f.core.CoreV1().Pods(f.scope.Namespace).Create(f.ctx, pod, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	before, err := f.store.Read(f.scope, f.operation)
	if err != nil {
		t.Fatal(err)
	}
	f.core.ClearActions()
	f.dynamic.ClearActions()
	refs, err := f.resolver.VolumeReferencesForMutation(f.ctx, f.scope, 7, f.operation)
	if err != nil || refs.CanDelete || len(refs.ExternalRefs) != 1 || refs.ExternalRefs[0].Reason != "pod_mount" {
		t.Fatalf("surviving PVC reference was not reported: err=%v", err)
	}
	for _, action := range append(f.core.Actions(), f.dynamic.Actions()...) {
		if action.GetVerb() != "get" && action.GetVerb() != "list" {
			t.Fatal("reference query mutated Kubernetes resources")
		}
	}
	if err := f.core.CoreV1().Pods(f.scope.Namespace).Delete(f.ctx, pod.Name, metav1.DeleteOptions{}); err != nil {
		t.Fatal(err)
	}
	refs, err = f.resolver.VolumeReferencesForMutation(f.ctx, f.scope, 7, f.operation)
	if err != nil || !refs.CanDelete || refs.CanExpand || refs.CanDetach {
		t.Fatalf("released reference must permit only owned deletion: err=%v", err)
	}
	after, err := f.store.Read(f.scope, f.operation)
	if err != nil || after.State.ControlRevision != before.State.ControlRevision || after.State.BindingGeneration != before.State.BindingGeneration || after.State.Admission == nil {
		t.Fatal("query changed the durable deletion state")
	}
}

func TestMutationReferencesRejectReplacementUIDAndWrongActor(t *testing.T) {
	for _, kind := range []string{"no-context", "wrong-context", "wrong-operation", "foreign-scope", "other-volume", "new-pvc", "new-dv", "generation", "unknown-admission"} {
		t.Run(kind, func(t *testing.T) {
			f := newReclaimReferenceFixture(t)
			ctx, scope, operation, volumeID := f.ctx, f.scope, f.operation, uint(7)
			switch kind {
			case "no-context":
				ctx = context.Background()
			case "wrong-context":
				ctx = WithAdmissionContext(ctx, "different-owner")
			case "wrong-operation":
				operation = "different-operation"
			case "foreign-scope":
				scope.TenantID = "foreign"
			case "other-volume":
				volumeID = 99
			case "new-pvc":
				pvc, err := f.core.CoreV1().PersistentVolumeClaims(scope.Namespace).Get(ctx, f.binding.ActivePVC.Name, metav1.GetOptions{})
				if err != nil {
					t.Fatal(err)
				}
				pvc.UID = types.UID("replacement-pvc")
				if _, err := f.core.CoreV1().PersistentVolumeClaims(scope.Namespace).Update(ctx, pvc, metav1.UpdateOptions{}); err != nil {
					t.Fatal(err)
				}
			case "new-dv":
				if err := f.dynamic.Resource(dataVolumeResource).Namespace(scope.Namespace).Delete(ctx, f.binding.ActiveDV.Name, metav1.DeleteOptions{}); err != nil {
					t.Fatal(err)
				}
				f.dynamicObject(t, dataVolumeResource, &unstructured.Unstructured{Object: map[string]interface{}{"apiVersion": "cdi.kubevirt.io/v1beta1", "kind": "DataVolume", "metadata": map[string]interface{}{"namespace": scope.Namespace, "name": f.binding.ActiveDV.Name, "uid": "replacement-dv"}}})
			case "generation", "unknown-admission":
				current, err := f.store.Read(scope, operation)
				if err != nil {
					t.Fatal(err)
				}
				_, err = f.store.Mutate(scope, Expectation{ControlRevision: current.State.ControlRevision, BindingGeneration: current.State.BindingGeneration, AdmissionOwner: f.operation}, func(c Current) (Mutation, error) {
					if kind == "generation" {
						c.State.BindingGeneration++
					} else {
						c.State.Admission.OutcomeUnknown = true
					}
					return Mutation{State: c.State}, nil
				})
				if err != nil {
					t.Fatal(err)
				}
			}
			if _, err := f.resolver.VolumeReferencesForMutation(ctx, scope, volumeID, operation); err == nil {
				t.Fatal("untrusted or stale mutation query was accepted")
			}
		})
	}
}

func TestMutationReferencesKeepSharedSnapshotAndCloneProtection(t *testing.T) {
	for _, kind := range []string{"shared", "snapshot", "clone", "unreadable"} {
		t.Run(kind, func(t *testing.T) {
			f := newReclaimReferenceFixture(t)
			if err := f.dynamic.Resource(dataVolumeResource).Namespace(f.scope.Namespace).Delete(f.ctx, f.binding.ActiveDV.Name, metav1.DeleteOptions{}); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "shared":
				if err := f.store.db.Create(&dbmodel.TenantServiceMountRelation{ServiceID: "other", DependServiceID: f.scope.ServiceID, VolumeName: f.binding.DiskKey}).Error; err != nil {
					t.Fatal(err)
				}
			case "snapshot":
				f.dynamicObject(t, vmSnapshotContentResource, oldSnapshotContent(f.resolverFixture))
			case "clone":
				f.dynamicObject(t, dataVolumeResource, &unstructured.Unstructured{Object: map[string]interface{}{"apiVersion": "cdi.kubevirt.io/v1beta1", "kind": "DataVolume", "metadata": map[string]interface{}{"name": "new-clone", "namespace": f.scope.Namespace, "uid": "clone-uid"}, "spec": map[string]interface{}{"source": map[string]interface{}{"pvc": map[string]interface{}{"namespace": f.scope.Namespace, "name": f.binding.ActivePVC.Name}}}}})
			case "unreadable":
				f.core.PrependReactor("list", "pods", func(ktesting.Action) (bool, runtime.Object, error) {
					return true, nil, apierrors.NewForbidden(schema.GroupResource{Resource: "pods"}, "", errors.New("denied"))
				})
			}
			refs, err := f.resolver.VolumeReferencesForMutation(f.ctx, f.scope, 7, f.operation)
			if kind == "unreadable" {
				if !errors.Is(err, ErrReferenceUnavailable) {
					t.Fatal("unreadable references were treated as empty")
				}
				return
			}
			if err != nil || refs.CanDelete || len(refs.BlockingReasons) == 0 {
				t.Fatalf("reference protection was skipped: %v", err)
			}
		})
	}
}

func TestMutationReferencesAllowRecordedOwnerOnDeletingSurvivor(t *testing.T) {
	f := newReclaimReferenceFixture(t)
	if err := f.dynamic.Resource(dataVolumeResource).Namespace(f.scope.Namespace).Delete(f.ctx, f.binding.ActiveDV.Name, metav1.DeleteOptions{}); err != nil {
		t.Fatal(err)
	}
	pvc, err := f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Get(f.ctx, f.binding.ActivePVC.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	pvc.Labels = nil
	pvc.OwnerReferences = []metav1.OwnerReference{{APIVersion: "cdi.kubevirt.io/v1beta1", Kind: "DataVolume", Name: f.binding.ActiveDV.Name, UID: types.UID(f.binding.ActiveDV.UID)}}
	now := metav1.Now()
	pvc.DeletionTimestamp = &now
	if _, err = f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Update(f.ctx, pvc, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	refs, err := f.resolver.VolumeReferencesForMutation(f.ctx, f.scope, 7, f.operation)
	if err != nil || !refs.CanDelete {
		t.Fatalf("recorded owner on deleting PVC could not be rechecked: %v", err)
	}
	if _, err = f.resolver.ResolveVolume(f.ctx, f.scope, 7); !errors.Is(err, ErrConflict) {
		t.Fatal("ordinary resolution was weakened")
	}
}

func TestMutationReferencesRejectStateOrUIDChangesDuringScan(t *testing.T) {
	for _, kind := range []string{"revision", "replacement"} {
		t.Run(kind, func(t *testing.T) {
			f := newReclaimReferenceFixture(t)
			pvc, err := f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Get(f.ctx, f.binding.ActivePVC.Name, metav1.GetOptions{})
			if err != nil {
				t.Fatal(err)
			}
			f.core.PrependReactor("list", "pods", func(ktesting.Action) (bool, runtime.Object, error) {
				if kind == "replacement" {
					pvc.UID = "late-replacement"
					if err := f.core.Tracker().Update(corev1.SchemeGroupVersion.WithResource("persistentvolumeclaims"), pvc, f.scope.Namespace); err != nil {
						t.Fatal(err)
					}
				} else {
					c, err := f.store.Read(f.scope, f.operation)
					if err != nil {
						t.Fatal(err)
					}
					_, err = f.store.Mutate(f.scope, Expectation{ControlRevision: c.State.ControlRevision, BindingGeneration: c.State.BindingGeneration, AdmissionOwner: f.operation}, func(c Current) (Mutation, error) { return Mutation{State: c.State}, nil })
					if err != nil {
						t.Fatal(err)
					}
				}
				return false, nil, nil
			})
			refs, queryErr := f.resolver.VolumeReferencesForMutation(f.ctx, f.scope, 7, f.operation)
			if !errors.Is(queryErr, ErrConflict) || refs.CanDelete {
				t.Fatalf("changed scan retained delete eligibility: %v", queryErr)
			}
		})
	}
}

func TestMutationReferencesRequireExactIssuedReceipt(t *testing.T) {
	for _, kind := range []string{"missing-receipt", "unissued", "wrong-hash", "wrong-resources", "wrong-binding", "wrong-action", "complete"} {
		t.Run(kind, func(t *testing.T) {
			f := newReclaimReferenceFixture(t)
			if refs, err := f.resolver.VolumeReferencesForMutation(f.ctx, f.scope, 7, f.operation); err != nil || !refs.CanDelete {
				t.Fatal("valid issued control failed")
			}
			if kind == "wrong-action" {
				c, err := f.store.Read(f.scope, f.operation)
				if err != nil {
					t.Fatal(err)
				}
				_, err = f.store.Mutate(f.scope, Expectation{ControlRevision: c.State.ControlRevision, BindingGeneration: c.State.BindingGeneration, AdmissionOwner: f.operation}, func(c Current) (Mutation, error) {
					c.State.Admission.Action = "disk-expand-" + f.hash
					return Mutation{State: c.State}, nil
				})
				if err != nil {
					t.Fatal(err)
				}
			} else {
				key, err := ReceiptKey(f.operation)
				if err != nil {
					t.Fatal(err)
				}
				if kind == "missing-receipt" {
					if err := f.store.db.Where("component_id = ? AND name = ?", f.scope.ServiceID, key).Delete(&dbmodel.ComponentK8sAttributes{}).Error; err != nil {
						t.Fatal(err)
					}
				} else {
					c, err := f.store.Read(f.scope, f.operation)
					if err != nil {
						t.Fatal(err)
					}
					switch kind {
					case "unissued":
						delete(c.Receipt.Steps, "issued")
					case "wrong-hash":
						s := c.Receipt.Steps["issued"]
						s.RequestHash = strings.Repeat("b", 64)
						c.Receipt.Steps["issued"] = s
					case "wrong-resources":
						c.Receipt.Resources[0].UID = "other-pvc"
					case "wrong-binding":
						b := c.Receipt.OldBindings["7"]
						b.ActivePVC.UID = "other-pvc"
						c.Receipt.OldBindings["7"] = b
					case "complete":
						c.Receipt.Steps["complete"] = c.Receipt.Steps["issued"]
					}
					encoded, err := EncodeRecord(c.Receipt)
					if err != nil {
						t.Fatal(err)
					}
					if err := f.store.db.Model(&dbmodel.ComponentK8sAttributes{}).Where("component_id = ? AND name = ?", f.scope.ServiceID, key).Update("attribute_value", encoded).Error; err != nil {
						t.Fatal(err)
					}
				}
			}
			f.core.ClearActions()
			f.dynamic.ClearActions()
			if _, err := f.resolver.VolumeReferencesForMutation(f.ctx, f.scope, 7, f.operation); err == nil {
				t.Fatal("invalid reclaim proof was accepted")
			}
			if len(f.core.Actions())+len(f.dynamic.Actions()) != 0 {
				t.Fatal("invalid proof reached Kubernetes")
			}
		})
	}
}

func TestMutationReferencesAllowMissingPVCButRejectForeignSurvivors(t *testing.T) {
	for _, kind := range []string{"pvc-gone", "both-gone", "foreign-pvc-label", "foreign-dv-label", "dv-remapped", "wrong-owner", "pvc-forbidden", "dv-forbidden"} {
		t.Run(kind, func(t *testing.T) {
			f := newReclaimReferenceFixture(t)
			switch kind {
			case "pvc-gone", "both-gone":
				if err := f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Delete(f.ctx, f.binding.ActivePVC.Name, metav1.DeleteOptions{}); err != nil {
					t.Fatal(err)
				}
				if kind == "both-gone" {
					if err := f.dynamic.Resource(dataVolumeResource).Namespace(f.scope.Namespace).Delete(f.ctx, f.binding.ActiveDV.Name, metav1.DeleteOptions{}); err != nil {
						t.Fatal(err)
					}
				}
			case "foreign-pvc-label", "wrong-owner":
				pvc, err := f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Get(f.ctx, f.binding.ActivePVC.Name, metav1.GetOptions{})
				if err != nil {
					t.Fatal(err)
				}
				if kind == "foreign-pvc-label" {
					pvc.Labels["tenant_id"] = "other"
				} else {
					pvc.OwnerReferences = []metav1.OwnerReference{{Kind: "DataVolume", Name: "other", UID: "other"}}
				}
				if _, err := f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Update(f.ctx, pvc, metav1.UpdateOptions{}); err != nil {
					t.Fatal(err)
				}
			case "foreign-dv-label", "dv-remapped":
				dv, err := f.dynamic.Resource(dataVolumeResource).Namespace(f.scope.Namespace).Get(f.ctx, f.binding.ActiveDV.Name, metav1.GetOptions{})
				if err != nil {
					t.Fatal(err)
				}
				if kind == "foreign-dv-label" {
					dv.SetLabels(map[string]string{"service_id": f.scope.ServiceID, "tenant_id": "other"})
				} else {
					if err := unstructured.SetNestedField(dv.Object, "other-claim", "status", "claimName"); err != nil {
						t.Fatal(err)
					}
				}
				if _, err := f.dynamic.Resource(dataVolumeResource).Namespace(f.scope.Namespace).Update(f.ctx, dv, metav1.UpdateOptions{}); err != nil {
					t.Fatal(err)
				}
			case "pvc-forbidden":
				f.core.PrependReactor("get", "persistentvolumeclaims", func(ktesting.Action) (bool, runtime.Object, error) {
					return true, nil, apierrors.NewForbidden(schema.GroupResource{Resource: "persistentvolumeclaims"}, "", errors.New("denied"))
				})
			case "dv-forbidden":
				f.dynamic.PrependReactor("get", "datavolumes", func(ktesting.Action) (bool, runtime.Object, error) {
					return true, nil, apierrors.NewForbidden(schema.GroupResource{Resource: "datavolumes"}, "", errors.New("denied"))
				})
			}
			refs, err := f.resolver.VolumeReferencesForMutation(f.ctx, f.scope, 7, f.operation)
			if kind == "pvc-gone" || kind == "both-gone" {
				if err != nil || !refs.CanDelete {
					t.Fatalf("exact absent target prevented cleanup facts: %v", err)
				}
			} else if err == nil {
				t.Fatal("foreign or unreadable target was accepted")
			}
		})
	}
}
