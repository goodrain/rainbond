package vmrestore

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"

	dbmodel "github.com/goodrain/rainbond/db/model"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	corefake "k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
)

type resolverFixture struct {
	resolver *Resolver
	store    *Store
	core     *corefake.Clientset
	dynamic  *dynamicfake.FakeDynamicClient
	scope    Scope
	volume   dbmodel.TenantServiceVolume
}

func newResolverFixture(t *testing.T) resolverFixture {
	t.Helper()
	store, database, scope := fixtureStore(t)
	if err := database.AutoMigrate(&dbmodel.TenantServiceVolume{}, &dbmodel.TenantServiceMountRelation{}).Error; err != nil {
		t.Fatal(err)
	}
	volume := dbmodel.TenantServiceVolume{Model: dbmodel.Model{ID: 7}, ServiceID: scope.ServiceID, VolumeName: "data", VolumeType: "cephfs-vm-snapshot", VolumeCapacity: 1}
	if err := database.Create(&volume).Error; err != nil {
		t.Fatal(err)
	}
	size := resource.MustParse("1Gi")
	storage := "cephfs-vm-snapshot"
	pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "manual7", Namespace: scope.Namespace, UID: "pvc-original", ResourceVersion: "1", Labels: map[string]string{"service_id": scope.ServiceID, "tenant_id": scope.TenantID}}, Spec: corev1.PersistentVolumeClaimSpec{StorageClassName: &storage, VolumeName: "pv-original", Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: size}}}, Status: corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound, Capacity: corev1.ResourceList{corev1.ResourceStorage: size}}}
	pv := &corev1.PersistentVolume{ObjectMeta: metav1.ObjectMeta{Name: "pv-original", UID: "pv-original-uid"}, Spec: corev1.PersistentVolumeSpec{ClaimRef: &corev1.ObjectReference{Namespace: scope.Namespace, Name: pvc.Name, UID: pvc.UID}, PersistentVolumeSource: corev1.PersistentVolumeSource{CSI: &corev1.CSIPersistentVolumeSource{Driver: "cephfs.csi.ceph.com", VolumeHandle: "NON_SECRET_BACKEND_HANDLE"}}}}
	core := corefake.NewSimpleClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: scope.Namespace, UID: "namespace-uid"}}, pvc, pv)
	kinds := map[schema.GroupVersionResource]string{dataVolumeResource: "DataVolumeList", virtualMachineResource: "VirtualMachineList", virtualMachineInstanceResource: "VirtualMachineInstanceList", vmSnapshotResource: "VirtualMachineSnapshotList", vmSnapshotContentResource: "VirtualMachineSnapshotContentList", csiSnapshotResource: "VolumeSnapshotList", csiSnapshotContentResource: "VolumeSnapshotContentList", dataSourceResource: "DataSourceList", nativeLonghornVolumeResource: "VolumeList"}
	dynamic := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), kinds)
	return resolverFixture{NewResolver(store, core, dynamic), store, core, dynamic, scope, volume}
}
func (f resolverFixture) seed(t *testing.T, binding Binding) {
	t.Helper()
	current, err := f.store.Read(f.scope, "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.store.Mutate(f.scope, Expectation{ControlRevision: current.State.ControlRevision, BindingGeneration: current.State.BindingGeneration}, func(c Current) (Mutation, error) {
		c.State.Bindings[strconv.FormatUint(uint64(binding.VolumeID), 10)] = binding
		c.State.BindingGeneration++
		return Mutation{State: c.State}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
func (f resolverFixture) currentBinding() Binding {
	return Binding{VolumeID: 7, DiskKey: "data", CanonicalName: "manual7", Role: "data", Attached: true, ActivePVC: ObjectRef{Kind: "PersistentVolumeClaim", Namespace: f.scope.Namespace, Name: "manual7", UID: "pvc-original"}}
}
func (f resolverFixture) dynamicObject(t *testing.T, gvr schema.GroupVersionResource, obj *unstructured.Unstructured) {
	t.Helper()
	if _, err := f.dynamic.Resource(gvr).Namespace(obj.GetNamespace()).Create(context.Background(), obj, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
}

// capability_id: rainbond.vm-restore.live-volume-resolver
func TestLiveResolverCanonicalMaterializationAndPlainPVC(t *testing.T) {
	f := newResolverFixture(t)
	resolved, err := f.resolver.ResolveVolume(context.Background(), f.scope, 7)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Mapped || resolved.ActivePVC == nil || resolved.ActivePVC.UID != "pvc-original" || resolved.ActiveDV != nil || resolved.Provisioning != "bound" {
		t.Fatal("unmapped canonical resolution was inaccurate")
	}
	binding, err := resolved.MaterializedBinding()
	if err != nil {
		t.Fatal(err)
	}
	if binding.RestoreID != "" || binding.SnapshotUID != "" {
		t.Fatal("canonical identity fabricated snapshot provenance")
	}
	f.seed(t, binding)
	again, err := f.resolver.ResolveVolume(context.Background(), f.scope, 7)
	if err != nil || !again.Mapped || again.ActivePVC.UID != "pvc-original" || again.ActiveDV != nil {
		t.Fatal("materialized canonical identity did not remain pinned")
	}
}

func TestLiveResolverUsesDVActualClaimAndRejectsLaterRemap(t *testing.T) {
	f := newResolverFixture(t)
	ctx := context.Background()
	original, err := f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Get(ctx, "manual7", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	alias := original.DeepCopy()
	alias.Name = "cdi-alias"
	alias.UID = "pvc-alias"
	alias.Labels = nil
	alias.OwnerReferences = []metav1.OwnerReference{{Kind: "DataVolume", Name: "manual7", UID: "dv-uid"}}
	if _, err := f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Create(ctx, alias, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	dv := &unstructured.Unstructured{Object: map[string]interface{}{"apiVersion": "cdi.kubevirt.io/v1beta1", "kind": "DataVolume", "metadata": map[string]interface{}{"name": "manual7", "namespace": f.scope.Namespace, "uid": "dv-uid", "labels": map[string]interface{}{"service_id": f.scope.ServiceID}}, "status": map[string]interface{}{"claimName": "cdi-alias"}}}
	f.dynamicObject(t, dataVolumeResource, dv)
	resolved, err := f.resolver.ResolveVolume(ctx, f.scope, 7)
	if err != nil || resolved.ActivePVC.Name != "cdi-alias" || resolved.ActiveDV == nil || resolved.ActiveDV.UID != "dv-uid" {
		t.Fatal("DV status.claimName was not used")
	}
	binding, err := resolved.MaterializedBinding()
	if err != nil {
		t.Fatal(err)
	}
	f.seed(t, binding)
	dv.Object["status"] = map[string]interface{}{"claimName": "manual7"}
	if _, err := f.dynamic.Resource(dataVolumeResource).Namespace(f.scope.Namespace).Update(ctx, dv, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.resolver.ResolveVolume(ctx, f.scope, 7); !errors.Is(err, ErrConflict) {
		t.Fatal("persisted mapping was silently repointed by changed DV status")
	}
}

func TestLiveResolverMissingAndReplacedMappedPVCNeverFallback(t *testing.T) {
	for _, mode := range []string{"missing", "replaced", "detached"} {
		t.Run(mode, func(t *testing.T) {
			f := newResolverFixture(t)
			binding := f.currentBinding()
			binding.ActivePVC.Name = "restored-a"
			binding.ActivePVC.UID = "restored-uid"
			binding.RestoreID = "operation-a"
			binding.SnapshotUID = "snapshot-uid"
			binding.Attached = mode != "detached"
			if mode != "missing" {
				pvc, _ := f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Get(context.Background(), "manual7", metav1.GetOptions{})
				pvc.Name = "restored-a"
				pvc.UID = "restored-uid"
				if mode == "replaced" {
					pvc.UID = "unrelated-new-uid"
				}
				if _, err := f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Create(context.Background(), pvc, metav1.CreateOptions{}); err != nil {
					t.Fatal(err)
				}
			}
			f.seed(t, binding)
			resolved, err := f.resolver.ResolveVolume(context.Background(), f.scope, 7)
			if mode == "detached" {
				if err != nil || resolved.Attached || resolved.ActivePVC.Name != "restored-a" {
					t.Fatal("detached mapping fell back or attached itself")
				}
			} else if !errors.Is(err, ErrConflict) {
				t.Fatal("missing or replaced mapped PVC fell back to original")
			}
		})
	}
}

func TestLiveResolverRejectsForeignLogicalVolumeAndDoesNotInventUID(t *testing.T) {
	f := newResolverFixture(t)
	if err := f.store.db.Create(&dbmodel.TenantServiceVolume{Model: dbmodel.Model{ID: 8}, ServiceID: "foreign", VolumeName: "data", VolumeType: "cephfs-vm-snapshot"}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := f.resolver.ResolveVolume(context.Background(), f.scope, 8); !errors.Is(err, ErrNotFound) {
		t.Fatal("foreign logical volume was resolved")
	}
	if err := f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Delete(context.Background(), "manual7", metav1.DeleteOptions{}); err != nil {
		t.Fatal(err)
	}
	resolved, err := f.resolver.ResolveVolume(context.Background(), f.scope, 7)
	if err != nil || resolved.ActivePVC != nil || resolved.Provisioning != "not_provisioned" {
		t.Fatal("unprovisioned volume acquired a fabricated PVC UID")
	}
}

func TestBindingViewDoesNotExposeAdmissionOrBackendHandle(t *testing.T) {
	f := newResolverFixture(t)
	_, err := f.store.Mutate(f.scope, Expectation{}, func(c Current) (Mutation, error) {
		if err := c.State.AdmitMutation("owner-slot", "expand"); err != nil {
			return Mutation{}, err
		}
		return Mutation{State: c.State}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	view, err := f.resolver.ReadBindingView(context.Background(), f.scope)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	if !view.MutationInProgress || len(view.Volumes) != 1 || strings.Contains(string(data), "owner-slot") || strings.Contains(string(data), "NON_SECRET_BACKEND_HANDLE") {
		t.Fatal("binding view exposed control or backend internals")
	}
}

func TestCanonicalMaterializedIdentitySurvivesRollbackAndRejectsNewUID(t *testing.T) {
	f := newResolverFixture(t)
	ctx := context.Background()
	resolved, err := f.resolver.ResolveVolume(ctx, f.scope, 7)
	if err != nil {
		t.Fatal(err)
	}
	original, err := resolved.MaterializedBinding()
	if err != nil {
		t.Fatal(err)
	}
	f.seed(t, original)
	if err := f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Delete(ctx, "manual7", metav1.DeleteOptions{}); err != nil {
		t.Fatal(err)
	}
	size := resource.MustParse("1Gi")
	storage := "cephfs-vm-snapshot"
	replacement := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "manual7", Namespace: f.scope.Namespace, UID: "different-uid", Labels: map[string]string{"service_id": f.scope.ServiceID}}, Spec: corev1.PersistentVolumeClaimSpec{StorageClassName: &storage, Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: size}}}, Status: corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound}}
	if _, err := f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Create(ctx, replacement, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.resolver.ResolveVolume(ctx, f.scope, 7); !errors.Is(err, ErrConflict) {
		t.Fatal("rollback cleared canonical identity and adopted a replacement")
	}
}

func TestResolverRejectsMixedGenerationDuringKubernetesReads(t *testing.T) {
	f := newResolverFixture(t)
	mutated := false
	f.core.PrependReactor("get", "persistentvolumeclaims", func(action ktesting.Action) (bool, runtime.Object, error) {
		if !mutated {
			mutated = true
			_, err := f.store.Mutate(f.scope, Expectation{}, func(c Current) (Mutation, error) {
				if err := c.State.AdmitMutation("other-action", "update"); err != nil {
					return Mutation{}, err
				}
				return Mutation{State: c.State}, nil
			})
			if err != nil {
				t.Fatal(err)
			}
		}
		return false, nil, nil
	})
	if _, err := f.resolver.ReadBindingView(context.Background(), f.scope); !errors.Is(err, ErrConflict) {
		t.Fatal("mixed generation was returned after concurrent control change")
	}
}

func TestLiveResolverRejectsCorruptHistoricalReceipts(t *testing.T) {
	for _, mode := range []string{"missing-retention", "missing-applied", "unapplied", "zero-revision", "future-generation"} {
		t.Run(mode, func(t *testing.T) {
			f := newResolverFixture(t)
			f.seed(t, f.currentBinding())
			e := expectation()
			e.ControlRevision, e.BindingGeneration = 1, 1
			_, err := f.store.Mutate(f.scope, e, func(c Current) (Mutation, error) {
				receipt := NewReceipt(f.scope, e.OperationID, e.ManifestHash)
				receipt.OldBindings["7"] = f.currentBinding()
				receipt.RetainOldPVCs = true
				return Mutation{State: c.State, Receipt: &receipt}, nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := f.resolver.ResolveVolume(context.Background(), f.scope, 7); err != nil {
				t.Fatalf("valid historical receipt was unreadable: %v", err)
			}
			var row dbmodel.ComponentK8sAttributes
			if err := f.store.db.Where("component_id = ? AND name = ?", f.scope.ServiceID, ReceiptPrefix+e.OperationID).First(&row).Error; err != nil {
				t.Fatal(err)
			}
			var fields map[string]interface{}
			if err := json.Unmarshal([]byte(row.AttributeValue), &fields); err != nil {
				t.Fatal(err)
			}
			step := fields["steps"].(map[string]interface{})[e.Step].(map[string]interface{})
			switch mode {
			case "missing-retention":
				delete(fields, "retain_old_pvcs")
			case "missing-applied":
				delete(step, "applied")
			case "unapplied":
				step["applied"] = false
			case "zero-revision":
				step["control_revision"] = 0
			case "future-generation":
				step["binding_generation"] = 2
			}
			value, err := json.Marshal(fields)
			if err != nil {
				t.Fatal(err)
			}
			if err := f.store.db.Model(&row).Update("attribute_value", string(value)).Error; err != nil {
				t.Fatal(err)
			}
			if _, err := f.resolver.ResolveVolume(context.Background(), f.scope, 7); !errors.Is(err, ErrCorrupt) {
				t.Errorf("ResolveVolume accepted corrupt historical receipt: %v", err)
			}
			if _, err := f.resolver.ReadBindingView(context.Background(), f.scope); !errors.Is(err, ErrCorrupt) {
				t.Errorf("ReadBindingView accepted corrupt historical receipt: %v", err)
			}
			if _, err := f.resolver.VolumeReferences(context.Background(), f.scope, 7); !errors.Is(err, ErrCorrupt) {
				t.Errorf("VolumeReferences accepted corrupt historical receipt: %v", err)
			}
		})
	}
}

func TestLiveResolverRejectsConflictingPVCOwnership(t *testing.T) {
	for _, mode := range []string{"foreign-service-label", "wrong-owner-uid", "wrong-owner-name", "missing-dv"} {
		t.Run(mode, func(t *testing.T) {
			f := newResolverFixture(t)
			ctx := context.Background()
			pvc, err := f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Get(ctx, "manual7", metav1.GetOptions{})
			if err != nil {
				t.Fatal(err)
			}
			pvc.OwnerReferences = []metav1.OwnerReference{{Kind: "DataVolume", Name: "manual7", UID: "dv-uid"}}
			switch mode {
			case "foreign-service-label":
				pvc.Labels["service_id"] = "foreign-service"
			case "wrong-owner-uid":
				pvc.OwnerReferences[0].UID = "replaced-dv-uid"
			case "wrong-owner-name":
				pvc.OwnerReferences[0].Name = "other-dv"
			}
			if _, err := f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Update(ctx, pvc, metav1.UpdateOptions{}); err != nil {
				t.Fatal(err)
			}
			if mode != "missing-dv" {
				dv := &unstructured.Unstructured{Object: map[string]interface{}{"apiVersion": "cdi.kubevirt.io/v1beta1", "kind": "DataVolume", "metadata": map[string]interface{}{"name": "manual7", "namespace": f.scope.Namespace, "uid": "dv-uid", "labels": map[string]interface{}{"service_id": f.scope.ServiceID}}, "status": map[string]interface{}{"claimName": "manual7"}}}
				f.dynamicObject(t, dataVolumeResource, dv)
			}
			if _, err := f.resolver.ResolveVolume(ctx, f.scope, 7); !errors.Is(err, ErrConflict) {
				t.Fatalf("conflicting PVC ownership was accepted: %v", err)
			}
		})
	}
}

// capability_id: rainbond.vm-restore.lost-ledger-footprint
func TestPartialMappingLossWithReceiptCannotSelectB(t *testing.T) {
	f := newResolverFixture(t)
	f.addClaim(t, "restored-a", "pvc-restored", "pv-restored")
	old := f.currentBinding()
	active := old
	active.ActivePVC.Name, active.ActivePVC.UID = "restored-a", "pvc-restored"
	active.RestoreID, active.SnapshotUID = "operation-a", "snapshot-origin"
	e := Expectation{OperationID: "operation-a", ManifestHash: strings.Repeat("a", 64), Step: "commit", RequestHash: strings.Repeat("b", 64)}
	_, err := f.store.Mutate(f.scope, e, func(c Current) (Mutation, error) {
		c.State.Bindings["7"] = active
		c.State.BindingGeneration++
		receipt := NewReceipt(f.scope, e.OperationID, e.ManifestHash)
		receipt.OldBindings["7"], receipt.NewBindings["7"] = old, active
		receipt.RetainOldPVCs = true
		return Mutation{State: c.State, Receipt: &receipt}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// Only bindings are damaged: state generation, successful receipt and VM footprint survive.
	var row dbmodel.ComponentK8sAttributes
	if err := f.store.db.Where("component_id = ? AND name = ?", f.scope.ServiceID, StateKey).First(&row).Error; err != nil {
		t.Fatal(err)
	}
	var state State
	if err := DecodeRecord(row.AttributeValue, &state); err != nil {
		t.Fatal(err)
	}
	delete(state.Bindings, "7")
	encoded, err := EncodeRecord(state)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.store.db.Model(&row).Update("attribute_value", encoded).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Delete(context.Background(), "restored-a", metav1.DeleteOptions{}); err != nil {
		t.Fatal(err)
	}
	vm := &unstructured.Unstructured{Object: map[string]interface{}{"apiVersion": "kubevirt.io/v1", "kind": "VirtualMachine", "metadata": map[string]interface{}{"name": "own-vm", "namespace": f.scope.Namespace, "uid": "vm-own", "labels": map[string]interface{}{"service_id": f.scope.ServiceID, RestoreOperationLabel: e.OperationID}, "annotations": map[string]interface{}{BindingGenerationAnnotation: "1"}}, "spec": map[string]interface{}{"template": map[string]interface{}{"spec": map[string]interface{}{"volumes": []interface{}{map[string]interface{}{"name": "manual7", "persistentVolumeClaim": map[string]interface{}{"claimName": "restored-a"}}}}}}}}
	f.dynamicObject(t, virtualMachineResource, vm)
	got, err := f.resolver.ResolveVolume(context.Background(), f.scope, 7)
	if !errors.Is(err, ErrCorrupt) {
		t.Fatalf("partial mapping loss must fail closed; got err=%v mapped=%v pvc=%+v", err, got.Mapped, got.ActivePVC)
	}
}
