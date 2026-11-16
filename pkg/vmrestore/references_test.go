package vmrestore

import (
	"context"
	"errors"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ktesting "k8s.io/client-go/testing"
)

type untrustedAdmissionContextKey string

func (f resolverFixture) addClaim(t *testing.T, name, uid, pvName string) {
	t.Helper()
	ctx := context.Background()
	source, err := f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Get(ctx, "manual7", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	target := source.DeepCopy()
	target.Name = name
	target.UID = types.UID(uid)
	target.Spec.VolumeName = pvName
	if _, err := f.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Create(ctx, target, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	pv, err := f.core.CoreV1().PersistentVolumes().Get(ctx, "pv-original", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	pv = pv.DeepCopy()
	pv.Name = pvName
	pv.UID = types.UID(pvName + "-uid")
	pv.Spec.ClaimRef = &corev1.ObjectReference{Namespace: f.scope.Namespace, Name: name, UID: target.UID}
	pv.Spec.CSI.VolumeHandle = "NON_SECRET_HANDLE_" + name
	if _, err := f.core.CoreV1().PersistentVolumes().Create(ctx, pv, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
}
func oldSnapshotContent(f resolverFixture) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{"apiVersion": "snapshot.kubevirt.io/v1beta1", "kind": "VirtualMachineSnapshotContent", "metadata": map[string]interface{}{"name": "old-content", "namespace": f.scope.Namespace, "uid": "old-content-uid"}, "spec": map[string]interface{}{"source": map[string]interface{}{"virtualMachine": map[string]interface{}{"metadata": map[string]interface{}{"labels": map[string]interface{}{"service_id": f.scope.ServiceID}}, "spec": map[string]interface{}{"template": map[string]interface{}{"spec": map[string]interface{}{"volumes": []interface{}{map[string]interface{}{"name": "manual7"}}}}}}}, "volumeBackups": []interface{}{map[string]interface{}{"volumeName": "manual7", "persistentVolumeClaim": map[string]interface{}{"metadata": map[string]interface{}{"name": "manual7", "namespace": f.scope.Namespace, "uid": "pvc-original"}}}}}}}
}

// capability_id: rainbond.vm-restore.volume-reference-matrix
func TestOldBLogicalReferencesDoNotBlockIndependentAExpansion(t *testing.T) {
	f := newResolverFixture(t)
	f.addClaim(t, "restored-a", "pvc-restored", "pv-restored")
	binding := f.currentBinding()
	previous := binding.ActivePVC
	binding.ActivePVC.Name = "restored-a"
	binding.ActivePVC.UID = "pvc-restored"
	binding.PreviousPVC = &previous
	binding.RestoreID = "operation-a"
	binding.SnapshotUID = "snapshot-origin"
	f.seed(t, binding)
	f.dynamicObject(t, vmSnapshotContentResource, oldSnapshotContent(f))
	refs, err := f.resolver.VolumeReferences(context.Background(), f.scope, 7)
	if err != nil {
		t.Fatal(err)
	}
	if !refs.CanExpand || refs.CanDelete || len(refs.SnapshotRefs) == 0 || len(refs.RollbackRefs) == 0 {
		t.Fatal("logical old B refs were confused with physical current A")
	}
	for _, ref := range refs.SnapshotRefs {
		if ref.Physical {
			t.Fatal("old B snapshot was marked as current A physical dependency")
		}
	}
}

func TestCurrentPhysicalSnapshotBlocksExpansion(t *testing.T) {
	f := newResolverFixture(t)
	f.dynamicObject(t, vmSnapshotContentResource, oldSnapshotContent(f))
	refs, err := f.resolver.VolumeReferences(context.Background(), f.scope, 7)
	if err != nil {
		t.Fatal(err)
	}
	if refs.CanExpand || refs.CanDelete || len(refs.SnapshotRefs) == 0 || !refs.SnapshotRefs[0].Physical {
		t.Fatal("current PVC snapshot dependency was ignored")
	}
}

func TestActorAdmissionContextOnlyIgnoresItsOwnBusyFlag(t *testing.T) {
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
	for _, test := range []struct {
		name    string
		ctx     context.Context
		allowed bool
	}{{"public", context.Background(), false}, {"wrong", WithAdmissionContext(context.Background(), "other-slot"), false}, {"forged-key", context.WithValue(context.Background(), untrustedAdmissionContextKey("vm_restore_admission"), "owner-slot"), false}, {"owner", WithAdmissionContext(context.Background(), "owner-slot"), true}} {
		t.Run(test.name, func(t *testing.T) {
			refs, err := f.resolver.VolumeReferences(test.ctx, f.scope, 7)
			if err != nil {
				t.Fatal(err)
			}
			if refs.CanExpand != test.allowed {
				t.Fatal("operation admission context was not scoped to its owner")
			}
		})
	}
	// An internal context is not permission to ignore a physical snapshot.
	f.dynamicObject(t, vmSnapshotContentResource, oldSnapshotContent(f))
	refs, err := f.resolver.VolumeReferences(WithAdmissionContext(context.Background(), "owner-slot"), f.scope, 7)
	if err != nil || refs.CanExpand {
		t.Fatal("owner context bypassed physical snapshot protection")
	}
}

func TestOwnContextCannotBypassRestoreFence(t *testing.T) {
	f := newResolverFixture(t)
	e := Expectation{OperationID: "operation-a", ManifestHash: strings.Repeat("a", 64), Step: "prepare", RequestHash: strings.Repeat("b", 64)}
	_, err := f.store.Mutate(f.scope, e, func(c Current) (Mutation, error) {
		c.State.Fence = &Fence{Kind: "restore", Phase: "copying", OperationID: e.OperationID, ManifestHash: e.ManifestHash, TargetVMUID: "vm-uid"}
		receipt := NewReceipt(f.scope, e.OperationID, e.ManifestHash)
		return Mutation{State: c.State, Receipt: &receipt}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	refs, err := f.resolver.VolumeReferences(WithAdmissionContext(context.Background(), "operation-a"), f.scope, 7)
	if err != nil || refs.CanExpand || refs.CanDelete {
		t.Fatal("normal actor context bypassed restore fence")
	}
}

func TestIndirectDataSourceAndPendingVolumeRequestAreProtected(t *testing.T) {
	f := newResolverFixture(t)
	ds := &unstructured.Unstructured{Object: map[string]interface{}{"apiVersion": "cdi.kubevirt.io/v1beta1", "kind": "DataSource", "metadata": map[string]interface{}{"name": "shared-source", "namespace": f.scope.Namespace, "uid": "data-source-uid"}, "spec": map[string]interface{}{"source": map[string]interface{}{"pvc": map[string]interface{}{"name": "manual7", "namespace": f.scope.Namespace}}}}}
	f.dynamicObject(t, dataSourceResource, ds)
	clone := &unstructured.Unstructured{Object: map[string]interface{}{"apiVersion": "cdi.kubevirt.io/v1beta1", "kind": "DataVolume", "metadata": map[string]interface{}{"name": "clone", "namespace": "other", "uid": "clone-uid"}, "spec": map[string]interface{}{"sourceRef": map[string]interface{}{"kind": "DataSource", "name": "shared-source", "namespace": f.scope.Namespace}}}}
	f.dynamicObject(t, dataVolumeResource, clone)
	vm := &unstructured.Unstructured{Object: map[string]interface{}{"apiVersion": "kubevirt.io/v1", "kind": "VirtualMachine", "metadata": map[string]interface{}{"name": "own-vm", "namespace": f.scope.Namespace, "uid": "vm-uid", "labels": map[string]interface{}{"service_id": f.scope.ServiceID}}, "status": map[string]interface{}{"volumeRequests": []interface{}{map[string]interface{}{"addVolumeOptions": map[string]interface{}{"volumeSource": map[string]interface{}{"persistentVolumeClaim": map[string]interface{}{"claimName": "manual7"}}}}}}}}
	f.dynamicObject(t, virtualMachineResource, vm)
	refs, err := f.resolver.VolumeReferences(context.Background(), f.scope, 7)
	if err != nil {
		t.Fatal(err)
	}
	if refs.CanExpand || refs.CanDelete || len(refs.ExternalRefs) < 2 || len(refs.InflightRefs) == 0 {
		t.Fatal("indirect source or pending attachment was not protected")
	}
}

func TestUnreadableReferencesFailClosed(t *testing.T) {
	f := newResolverFixture(t)
	f.dynamic.PrependReactor("list", "virtualmachinesnapshots", func(ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(vmSnapshotResource.GroupResource(), "", errors.New("read unavailable"))
	})
	if _, err := f.resolver.VolumeReferences(context.Background(), f.scope, 7); !errors.Is(err, ErrReferenceUnavailable) {
		t.Fatal("unreadable references were treated as empty")
	}
}

func TestRestoredAliasWithNoRealReferencesCanBeDeletedAndExpanded(t *testing.T) {
	f := newResolverFixture(t)
	f.addClaim(t, "restored-a", "pvc-restored", "pv-restored")
	binding := f.currentBinding()
	binding.ActivePVC.Name = "restored-a"
	binding.ActivePVC.UID = "pvc-restored"
	binding.RestoreID = "operation-a"
	binding.SnapshotUID = "provenance-only"
	f.seed(t, binding)
	refs, err := f.resolver.VolumeReferences(context.Background(), f.scope, 7)
	if err != nil {
		t.Fatal(err)
	}
	if !refs.CanDelete || !refs.CanExpand || len(refs.SnapshotRefs) != 0 {
		t.Fatal("alias or provenance label became a permanent unsupported restriction")
	}
}

func TestCSIVolumeSnapshotContentUsesBackendIdentityWithoutExposingHandle(t *testing.T) {
	f := newResolverFixture(t)
	content := &unstructured.Unstructured{Object: map[string]interface{}{"apiVersion": "snapshot.storage.k8s.io/v1", "kind": "VolumeSnapshotContent", "metadata": map[string]interface{}{"name": "csi-content", "uid": "csi-content-uid"}, "spec": map[string]interface{}{"driver": "cephfs.csi.ceph.com", "source": map[string]interface{}{"volumeHandle": "NON_SECRET_BACKEND_HANDLE"}}}}
	f.dynamicObject(t, csiSnapshotContentResource, content)
	refs, err := f.resolver.VolumeReferences(context.Background(), f.scope, 7)
	if err != nil {
		t.Fatal(err)
	}
	if refs.CanExpand || len(refs.SnapshotRefs) == 0 {
		t.Fatal("CSI content retained after snapshot deletion was ignored")
	}
}

func TestOwnVMLauncherDoesNotBlockExpansionButForeignPodDoes(t *testing.T) {
	f := newResolverFixture(t)
	volume := map[string]interface{}{"name": "manual7", "persistentVolumeClaim": map[string]interface{}{"claimName": "manual7"}}
	vm := &unstructured.Unstructured{Object: map[string]interface{}{"apiVersion": "kubevirt.io/v1", "kind": "VirtualMachine", "metadata": map[string]interface{}{"name": "own-vm", "namespace": f.scope.Namespace, "uid": "vm-own", "labels": map[string]interface{}{"service_id": f.scope.ServiceID}}, "spec": map[string]interface{}{"template": map[string]interface{}{"spec": map[string]interface{}{"volumes": []interface{}{volume}}}}}}
	f.dynamicObject(t, virtualMachineResource, vm)
	vmi := &unstructured.Unstructured{Object: map[string]interface{}{"apiVersion": "kubevirt.io/v1", "kind": "VirtualMachineInstance", "metadata": map[string]interface{}{"name": "own-vm", "namespace": f.scope.Namespace, "uid": "vmi-own", "labels": map[string]interface{}{"service_id": f.scope.ServiceID}, "ownerReferences": []interface{}{map[string]interface{}{"apiVersion": "kubevirt.io/v1", "kind": "VirtualMachine", "name": "own-vm", "uid": "vm-own"}}}, "spec": map[string]interface{}{"volumes": []interface{}{volume}}}}
	f.dynamicObject(t, virtualMachineInstanceResource, vmi)
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "virt-launcher-own", Namespace: f.scope.Namespace, UID: "pod-own", OwnerReferences: []metav1.OwnerReference{{Kind: "VirtualMachineInstance", Name: "own-vm", UID: "vmi-own"}}}, Spec: corev1.PodSpec{Volumes: []corev1.Volume{{Name: "disk", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "manual7"}}}}}, Status: corev1.PodStatus{Phase: corev1.PodRunning}}
	if _, err := f.core.CoreV1().Pods(f.scope.Namespace).Create(context.Background(), pod, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	refs, err := f.resolver.VolumeReferences(context.Background(), f.scope, 7)
	if err != nil || !refs.CanExpand || len(refs.OwnAttachments) != 3 || refs.CanDelete {
		t.Fatal("own runtime references were confused with external consumers")
	}
	// KubeVirt owns a hotplug attachment Pod through virt-launcher rather than
	// directly through the VMI. Its observed UID still belongs to this VM.
	if err := unstructured.SetNestedSlice(vmi.Object, []interface{}{map[string]interface{}{
		"name": "manual7", "persistentVolumeClaimInfo": map[string]interface{}{"claimName": "manual7"},
		"hotplugVolume": map[string]interface{}{"attachPodName": "hp-volume-own", "attachPodUID": "hotplug-own"},
	}}, "status", "volumeStatus"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.dynamic.Resource(virtualMachineInstanceResource).Namespace(f.scope.Namespace).Update(context.Background(), vmi, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	hotplug := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "hp-volume-own", Namespace: f.scope.Namespace, UID: "hotplug-own", Labels: map[string]string{"kubevirt.io": "hotplug-disk"}, OwnerReferences: []metav1.OwnerReference{{APIVersion: "v1", Kind: "Pod", Name: pod.Name, UID: pod.UID}}}, Spec: pod.Spec, Status: pod.Status}
	if _, err := f.core.CoreV1().Pods(f.scope.Namespace).Create(context.Background(), hotplug, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	refs, err = f.resolver.VolumeReferences(context.Background(), f.scope, 7)
	if err != nil || !refs.CanExpand || len(refs.OwnAttachments) != 4 || len(refs.ExternalRefs) != 0 || refs.CanDelete {
		t.Fatal("own observed hotplug attachment was mistaken for an external disk consumer", refs, err)
	}
	for _, mode := range []string{"replacement-uid", "unobserved-name", "wrong-launcher"} {
		foreign := hotplug.DeepCopy()
		switch mode {
		case "replacement-uid":
			foreign.UID = "replacement-hotplug"
		case "unobserved-name":
			foreign.Name = "hp-volume-other"
		case "wrong-launcher":
			foreign.OwnerReferences[0].UID = "unrelated-launcher"
		}
		if err := f.core.CoreV1().Pods(f.scope.Namespace).Delete(context.Background(), hotplug.Name, metav1.DeleteOptions{}); err != nil {
			t.Fatal(err)
		}
		if _, err := f.core.CoreV1().Pods(f.scope.Namespace).Create(context.Background(), foreign, metav1.CreateOptions{}); err != nil {
			t.Fatal(err)
		}
		refs, err = f.resolver.VolumeReferences(context.Background(), f.scope, 7)
		if err != nil || refs.CanExpand || len(refs.ExternalRefs) == 0 {
			t.Fatal("unverified hotplug attachment was accepted", mode, refs, err)
		}
		if err := f.core.CoreV1().Pods(f.scope.Namespace).Delete(context.Background(), foreign.Name, metav1.DeleteOptions{}); err != nil {
			t.Fatal(err)
		}
		if _, err := f.core.CoreV1().Pods(f.scope.Namespace).Create(context.Background(), hotplug, metav1.CreateOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	external := pod.DeepCopy()
	external.Name = "foreign-consumer"
	external.UID = "external-pod"
	external.OwnerReferences = nil
	if _, err := f.core.CoreV1().Pods(f.scope.Namespace).Create(context.Background(), external, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	refs, err = f.resolver.VolumeReferences(context.Background(), f.scope, 7)
	if err != nil || refs.CanExpand || len(refs.ExternalRefs) == 0 {
		t.Fatal("foreign Pod mount was ignored")
	}
}

func TestOptionalReferenceAPINotFoundDoesNotMeanEmptyWhenDiscovered(t *testing.T) {
	f := newResolverFixture(t)
	f.core.Resources = []*metav1.APIResourceList{{GroupVersion: vmSnapshotResource.GroupVersion().String(), APIResources: []metav1.APIResource{{Name: vmSnapshotResource.Resource, Kind: "VirtualMachineSnapshot", Namespaced: true}}}}
	f.dynamic.PrependReactor("list", vmSnapshotResource.Resource, func(ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewNotFound(vmSnapshotResource.GroupResource(), "")
	})
	if _, err := f.resolver.VolumeReferences(context.Background(), f.scope, 7); !errors.Is(err, ErrReferenceUnavailable) {
		t.Fatal("discovered but unreadable API was treated as no references")
	}
}

func TestOlderReceiptKeepsExactBIdentityWithoutBlockingAExpansion(t *testing.T) {
	f := newResolverFixture(t)
	f.addClaim(t, "restored-a", "pvc-restored", "pv-restored")
	old := f.currentBinding()
	active := old
	active.ActivePVC.Name = "restored-a"
	active.ActivePVC.UID = "pvc-restored"
	active.RestoreID = "operation-a"
	active.SnapshotUID = "snapshot-origin"
	f.seed(t, active)
	current, err := f.store.Read(f.scope, "")
	if err != nil {
		t.Fatal(err)
	}
	e := Expectation{ControlRevision: current.State.ControlRevision, BindingGeneration: current.State.BindingGeneration, OperationID: "prior-restore", ManifestHash: strings.Repeat("a", 64), Step: "retain", RequestHash: strings.Repeat("b", 64)}
	_, err = f.store.Mutate(f.scope, e, func(c Current) (Mutation, error) {
		receipt := NewReceipt(f.scope, e.OperationID, e.ManifestHash)
		receipt.OldBindings["7"] = old
		receipt.NewBindings["7"] = active
		receipt.RetainOldPVCs = true
		return Mutation{State: c.State, Receipt: &receipt}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := f.resolver.ResolveVolume(context.Background(), f.scope, 7)
	if err != nil || len(resolved.Retained) != 1 || resolved.Retained[0].UID != "pvc-original" {
		t.Fatal("old operation lost exact retained B identity")
	}
	refs, err := f.resolver.VolumeReferences(context.Background(), f.scope, 7)
	if err != nil || !refs.CanExpand || refs.CanDelete || len(refs.RollbackRefs) != 1 || refs.RollbackRefs[0].Physical {
		t.Fatal("retained B identity was confused with active A")
	}
}

func TestOptionalSnapshotAPIAbsenceAndVersionMismatch(t *testing.T) {
	for _, mode := range []string{"absent", "different-version"} {
		t.Run(mode, func(t *testing.T) {
			f := newResolverFixture(t)
			f.dynamic.PrependReactor("list", vmSnapshotResource.Resource, func(ktesting.Action) (bool, runtime.Object, error) {
				return true, nil, apierrors.NewNotFound(vmSnapshotResource.GroupResource(), "")
			})
			if mode == "different-version" {
				f.core.Resources = []*metav1.APIResourceList{{GroupVersion: vmSnapshotResource.Group + "/v1alpha1", APIResources: []metav1.APIResource{{Name: vmSnapshotResource.Resource, Kind: "VirtualMachineSnapshot", Namespaced: true}}}}
			}
			refs, err := f.resolver.VolumeReferences(context.Background(), f.scope, 7)
			if mode == "absent" {
				if err != nil || !refs.CanExpand {
					t.Fatalf("absent optional API disabled an otherwise independent disk: %v", err)
				}
			} else if !errors.Is(err, ErrReferenceUnavailable) {
				t.Fatalf("existing unsupported API version was treated as no references: %v", err)
			}
		})
	}
}

func TestSnapshotLayoutAndPendingRequestsProtectDifferentResources(t *testing.T) {
	for _, mode := range []string{"layout-only", "pending-vm-snapshot", "pending-csi-snapshot", "bound-csi-snapshot"} {
		t.Run(mode, func(t *testing.T) {
			f := newResolverFixture(t)
			switch mode {
			case "layout-only":
				content := oldSnapshotContent(f)
				unstructured.RemoveNestedField(content.Object, "spec", "volumeBackups")
				f.dynamicObject(t, vmSnapshotContentResource, content)
			case "pending-vm-snapshot":
				vm := &unstructured.Unstructured{Object: map[string]interface{}{"apiVersion": "kubevirt.io/v1", "kind": "VirtualMachine", "metadata": map[string]interface{}{"name": "own-vm", "namespace": f.scope.Namespace, "uid": "vm-uid", "labels": map[string]interface{}{"service_id": f.scope.ServiceID}}}}
				f.dynamicObject(t, virtualMachineResource, vm)
				snapshot := &unstructured.Unstructured{Object: map[string]interface{}{"apiVersion": "snapshot.kubevirt.io/v1beta1", "kind": "VirtualMachineSnapshot", "metadata": map[string]interface{}{"name": "pending", "namespace": f.scope.Namespace, "uid": "snapshot-uid"}, "spec": map[string]interface{}{"source": map[string]interface{}{"name": "own-vm"}}, "status": map[string]interface{}{"phase": "InProgress"}}}
				f.dynamicObject(t, vmSnapshotResource, snapshot)
			default:
				snapshot := &unstructured.Unstructured{Object: map[string]interface{}{"apiVersion": "snapshot.storage.k8s.io/v1", "kind": "VolumeSnapshot", "metadata": map[string]interface{}{"name": "csi-snapshot", "namespace": f.scope.Namespace, "uid": "snapshot-uid"}, "spec": map[string]interface{}{"source": map[string]interface{}{"persistentVolumeClaimName": "manual7"}}}}
				if mode == "bound-csi-snapshot" {
					content := &unstructured.Unstructured{Object: map[string]interface{}{"apiVersion": "snapshot.storage.k8s.io/v1", "kind": "VolumeSnapshotContent", "metadata": map[string]interface{}{"name": "csi-content", "uid": "content-uid"}, "spec": map[string]interface{}{"driver": "cephfs.csi.ceph.com", "source": map[string]interface{}{"volumeHandle": "NON_SECRET_BACKEND_HANDLE"}}}}
					f.dynamicObject(t, csiSnapshotContentResource, content)
					snapshot.Object["status"] = map[string]interface{}{"boundVolumeSnapshotContentName": "csi-content"}
				}
				f.dynamicObject(t, csiSnapshotResource, snapshot)
			}
			refs, err := f.resolver.VolumeReferences(context.Background(), f.scope, 7)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "layout-only" {
				if refs.CanDelete || !refs.CanExpand || len(refs.SnapshotRefs) != 1 || !refs.SnapshotRefs[0].Logical || refs.SnapshotRefs[0].Physical {
					t.Fatal("logical snapshot layout was confused with physical disk dependency")
				}
			} else if refs.CanDelete || refs.CanExpand || len(refs.SnapshotRefs)+len(refs.InflightRefs) == 0 {
				t.Fatal("pending or bound snapshot source was not protected")
			}
		})
	}
}

func TestPVCCloneReferencesIncludeCrossNamespaceDataSources(t *testing.T) {
	for _, mode := range []string{"local-source", "cross-namespace-source-ref", "different-source"} {
		t.Run(mode, func(t *testing.T) {
			f := newResolverFixture(t)
			claim := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "clone", Namespace: f.scope.Namespace, UID: "clone-uid"}}
			if mode == "cross-namespace-source-ref" {
				claim.Namespace = "other"
				claim.Spec.DataSourceRef = &corev1.TypedObjectReference{Kind: "PersistentVolumeClaim", Name: "manual7", Namespace: &f.scope.Namespace}
			} else {
				claim.Spec.DataSource = &corev1.TypedLocalObjectReference{Kind: "PersistentVolumeClaim", Name: "manual7"}
				if mode == "different-source" {
					claim.Spec.DataSource.Name = "unrelated"
				}
			}
			if _, err := f.core.CoreV1().PersistentVolumeClaims(claim.Namespace).Create(context.Background(), claim, metav1.CreateOptions{}); err != nil {
				t.Fatal(err)
			}
			refs, err := f.resolver.VolumeReferences(context.Background(), f.scope, 7)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "different-source" {
				if !refs.CanExpand || len(refs.ExternalRefs) != 0 {
					t.Fatal("unrelated clone falsely blocked current disk")
				}
			} else if refs.CanDelete || refs.CanExpand || len(refs.ExternalRefs) != 1 || !refs.ExternalRefs[0].Physical || refs.ExternalRefs[0].Reason != "pvc_clone_source" {
				t.Fatal("declared PVC clone source was not protected")
			}
		})
	}
}
