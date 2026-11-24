package vmrestore

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	csi "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ktesting "k8s.io/client-go/testing"
	kv "kubevirt.io/api/core/v1"
	native "kubevirt.io/api/snapshot/v1beta1"
	csifake "kubevirt.io/client-go/externalsnapshotter/fake"
	"kubevirt.io/client-go/kubecli"
	kvfake "kubevirt.io/client-go/kubevirt/fake"
	snapshotclient "kubevirt.io/client-go/kubevirt/typed/snapshot/v1beta1"
)

type nativeSnapshotStub struct {
	snapshotclient.VirtualMachineSnapshotInterface
	object    *native.VirtualMachineSnapshot
	createErr error
	deleted   []metav1.DeleteOptions
}

func (s *nativeSnapshotStub) Create(_ context.Context, obj *native.VirtualMachineSnapshot, _ metav1.CreateOptions) (*native.VirtualMachineSnapshot, error) {
	if s.createErr != nil {
		return nil, s.createErr
	}
	s.object = obj.DeepCopy()
	s.object.UID = "new-native-uid"
	return s.object, nil
}
func (s *nativeSnapshotStub) Get(context.Context, string, metav1.GetOptions) (*native.VirtualMachineSnapshot, error) {
	return s.object, nil
}
func (s *nativeSnapshotStub) Delete(_ context.Context, _ string, options metav1.DeleteOptions) error {
	s.deleted = append(s.deleted, options)
	return nil
}

// capability_id: rainbond.vm-snapshot.native-ownership
func TestNativeSnapshotCreateReconcilesOnlyExactOperationAndCore(t *testing.T) {
	f := snapshotProjectionFixture(t)
	control := gomock.NewController(t)
	defer control.Finish()
	client := kubecli.NewMockKubevirtClient(control)
	stub := &nativeSnapshotStub{object: f.snapshot, createErr: apierrors.NewAlreadyExists(schema.GroupResource{Group: "snapshot.kubevirt.io", Resource: "virtualmachinesnapshots"}, f.intent.Name)}
	client.EXPECT().VirtualMachineSnapshot(f.scope.Namespace).Return(stub).AnyTimes()
	backend := NewKubeSnapshotBackend(nil, client)
	ref, err := backend.Create(context.Background(), f.scope, f.intent, f.op, f.core)
	if err != nil || ref.UID != string(f.snapshot.UID) {
		t.Fatal("exact idempotent native snapshot was not reconciled")
	}
	stub.object = stub.object.DeepCopy()
	stub.object.Labels[SnapshotOperationLabel] = "different-operation"
	if _, err := backend.Create(context.Background(), f.scope, f.intent, f.op, f.core); !errors.Is(err, ErrConflict) {
		t.Fatal("same-name foreign operation was adopted")
	}
}

func TestNativeMetadataKeepsHashOutOfLabels(t *testing.T) {
	f := snapshotProjectionFixture(t)
	object := expectedSnapshot(f.scope, f.intent, f.op, f.core)
	if object.Annotations[SnapshotCoreAnnotation] != f.core || object.Labels[SnapshotCoreAnnotation] != "" || len(object.Labels[SnapshotOperationLabel]) > 63 {
		t.Fatal("full hash was put in invalid Kubernetes label")
	}
}

func TestNativeDeleteUsesUIDAndNeverDeletesSourceObjects(t *testing.T) {
	f := snapshotProjectionFixture(t)
	control := gomock.NewController(t)
	defer control.Finish()
	client := kubecli.NewMockKubevirtClient(control)
	stub := &nativeSnapshotStub{object: f.snapshot}
	client.EXPECT().VirtualMachineSnapshot(f.scope.Namespace).Return(stub).Times(1)
	backend := NewKubeSnapshotBackend(nil, client)
	refs := []ResourceRef{{ObjectRef: f.intent.SourceVM, Purpose: "snapshot-source-vm"}, {ObjectRef: f.intent.Disks[0].SourcePVC, Purpose: "snapshot-source-pvc"}, {ObjectRef: refOf("VirtualMachineSnapshot", f.snapshot), Purpose: "snapshot"}}
	if err := backend.Delete(context.Background(), f.scope, refs); err != nil {
		t.Fatal(err)
	}
	if len(stub.deleted) != 1 || stub.deleted[0].Preconditions == nil || stub.deleted[0].Preconditions.UID == nil || string(*stub.deleted[0].Preconditions.UID) != string(f.snapshot.UID) {
		t.Fatal("native deletion lacked exact UID precondition")
	}
}

type kubeSnapshotFixture struct {
	snapshotFixture
	resolver resolverFixture
	backend  *KubeSnapshotBackend
	native   *kvfake.Clientset
	csi      *csifake.Clientset
}

func newKubeSnapshotFixture(t *testing.T) kubeSnapshotFixture {
	t.Helper()
	f := snapshotProjectionFixture(t)
	r := newResolverFixture(t)
	vm := nativeAsVM(f.content.Spec.Source.VirtualMachine).DeepCopy()
	no := false
	vm.Spec.Running = &no
	vm.Spec.Template.Spec.Volumes[0].PersistentVolumeClaim.ClaimName = "manual7"
	nativeClient := kvfake.NewSimpleClientset(vm, f.snapshot.DeepCopy(), f.content.DeepCopy())
	csiClient := csifake.NewSimpleClientset(f.volumes["volume-snapshot-a"].DeepCopy(), &csi.VolumeSnapshotClass{ObjectMeta: metav1.ObjectMeta{Name: "cephfs-snapshot"}, Driver: "cephfs.csi.ceph.com", DeletionPolicy: csi.VolumeSnapshotContentDelete})
	if _, err := r.core.StorageV1().StorageClasses().Create(context.Background(), &storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: "cephfs-vm-snapshot"}, Provisioner: "cephfs.csi.ceph.com"}, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	client := kubecli.NewMockKubevirtClient(gomock.NewController(t))
	client.EXPECT().VirtualMachine(f.scope.Namespace).Return(nativeClient.KubevirtV1().VirtualMachines(f.scope.Namespace)).AnyTimes()
	client.EXPECT().VirtualMachineInstance(f.scope.Namespace).Return(nativeClient.KubevirtV1().VirtualMachineInstances(f.scope.Namespace)).AnyTimes()
	client.EXPECT().VirtualMachineSnapshot(f.scope.Namespace).Return(nativeClient.SnapshotV1beta1().VirtualMachineSnapshots(f.scope.Namespace)).AnyTimes()
	client.EXPECT().VirtualMachineSnapshotContent(f.scope.Namespace).Return(nativeClient.SnapshotV1beta1().VirtualMachineSnapshotContents(f.scope.Namespace)).AnyTimes()
	client.EXPECT().CoreV1().Return(r.core.CoreV1()).AnyTimes()
	client.EXPECT().StorageV1().Return(r.core.StorageV1()).AnyTimes()
	client.EXPECT().KubernetesSnapshotClient().Return(csiClient).AnyTimes()
	client.EXPECT().DynamicClient().Return(r.dynamic).AnyTimes()
	return kubeSnapshotFixture{f, r, NewKubeSnapshotBackend(r.resolver, client), nativeClient, csiClient}
}

// capability_id: rainbond.vm-snapshot.selected-class-admission
func TestNativeSnapshotAdmissionUsesActualProfileSelection(t *testing.T) {
	for _, tt := range []struct {
		name, preferred, policy, otherPolicy string
		deleting, denied, supported          bool
	}{
		{name: "profile resolves two Delete classes", preferred: "selected", policy: "Delete", otherPolicy: "Delete", supported: true},
		{name: "Retain preference cannot borrow unrelated Delete", preferred: "selected", policy: "Retain", otherPolicy: "Delete"},
		{name: "dangling profile never falls back", preferred: "missing", policy: "Delete"},
		{name: "deleting selected class", policy: "Delete", deleting: true},
		{name: "profile denied", policy: "Delete", denied: true},
		{name: "unique Retain", policy: "Retain"},
		{name: "unique Delete", policy: "Delete", supported: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newKubeSnapshotFixture(t)
			ctx := context.Background()
			_ = f.csi.SnapshotV1().VolumeSnapshotClasses().Delete(ctx, "cephfs-snapshot", metav1.DeleteOptions{})
			class := &csi.VolumeSnapshotClass{ObjectMeta: metav1.ObjectMeta{Name: "selected"}, Driver: "cephfs.csi.ceph.com", DeletionPolicy: csi.DeletionPolicy(tt.policy)}
			if tt.deleting {
				now := metav1.Now()
				class.DeletionTimestamp = &now
			}
			if _, err := f.csi.SnapshotV1().VolumeSnapshotClasses().Create(ctx, class, metav1.CreateOptions{}); err != nil {
				t.Fatal(err)
			}
			if tt.otherPolicy != "" {
				other := class.DeepCopy()
				other.Name, other.DeletionPolicy = "other", csi.DeletionPolicy(tt.otherPolicy)
				_, _ = f.csi.SnapshotV1().VolumeSnapshotClasses().Create(ctx, other, metav1.CreateOptions{})
			}
			if tt.preferred != "" {
				_, err := f.resolver.dynamic.Resource(schema.GroupVersionResource{Group: "cdi.kubevirt.io", Version: "v1beta1", Resource: "storageprofiles"}).Create(ctx, &unstructured.Unstructured{Object: map[string]interface{}{"apiVersion": "cdi.kubevirt.io/v1beta1", "kind": "StorageProfile", "metadata": map[string]interface{}{"name": "cephfs-vm-snapshot"}, "status": map[string]interface{}{"snapshotClass": tt.preferred}}}, metav1.CreateOptions{})
				if err != nil {
					t.Fatal(err)
				}
			}
			if tt.denied {
				f.resolver.dynamic.PrependReactor("get", "storageprofiles", func(ktesting.Action) (bool, runtime.Object, error) { return true, nil, errors.New("denied") })
			}
			err := f.backend.checkStorage(ctx, f.scope, SnapshotDisk{SourcePVC: f.resolver.currentBinding().ActivePVC})
			if (err == nil) != tt.supported {
				t.Fatalf("selected class admission: %v; want supported=%t", err, tt.supported)
			}
		})
	}
}

// capability_id: rainbond.vm-snapshot.longhorn-storage
func TestNativeSnapshotAcceptsQualifiedLonghornStorage(t *testing.T) {
	f := newKubeSnapshotFixture(t)
	ctx := context.Background()
	binding := f.resolver.currentBinding()
	pvc, err := f.resolver.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Get(ctx, binding.ActivePVC.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	class, err := f.resolver.core.StorageV1().StorageClasses().Get(ctx, *pvc.Spec.StorageClassName, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	class.Provisioner = "driver.longhorn.io"
	class.Parameters = map[string]string{"numberOfReplicas": "1", "dataEngine": "v1"}
	if _, err = f.resolver.core.StorageV1().StorageClasses().Update(ctx, class, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	pv, err := f.resolver.core.CoreV1().PersistentVolumes().Get(ctx, pvc.Spec.VolumeName, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	pv.Spec.CSI.Driver = "driver.longhorn.io"
	if _, err = f.resolver.core.CoreV1().PersistentVolumes().Update(ctx, pv, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	if err = f.csi.SnapshotV1().VolumeSnapshotClasses().Delete(ctx, "cephfs-snapshot", metav1.DeleteOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err = f.csi.SnapshotV1().VolumeSnapshotClasses().Create(ctx, &csi.VolumeSnapshotClass{ObjectMeta: metav1.ObjectMeta{Name: "longhorn-vm-snapshot"}, Driver: "driver.longhorn.io", DeletionPolicy: csi.VolumeSnapshotContentDelete}, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	disk := SnapshotDisk{SourcePVC: binding.ActivePVC}
	if err := f.backend.checkStorage(ctx, f.scope, disk); err != nil {
		t.Fatalf("qualified Longhorn storage was rejected: %v", err)
	}
}

func TestNativePreflightAndPowerRequireObservedShutdownAndUnlock(t *testing.T) {
	f := newKubeSnapshotFixture(t)
	ctx := context.Background()
	metadata, err := f.backend.Context(ctx, f.scope)
	if err != nil || metadata.WasRunning || !metadata.Stopped || len(metadata.Disks) != 1 || metadata.Disks[0].SourcePVC.Name != "manual7" {
		t.Fatalf("resolver-backed stopped preflight failed: %v", err)
	}
	vm, err := f.native.KubevirtV1().VirtualMachines(f.scope.Namespace).Get(ctx, f.intent.SourceVM.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	yes := true
	vm.Spec.Running = &yes
	if _, err := f.native.KubevirtV1().VirtualMachines(f.scope.Namespace).Update(ctx, vm, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	instance := &kv.VirtualMachineInstance{ObjectMeta: metav1.ObjectMeta{Name: vm.Name, Namespace: f.scope.Namespace, UID: "instance-uid"}, Status: kv.VirtualMachineInstanceStatus{Phase: kv.Running}}
	if _, err := f.native.KubevirtV1().VirtualMachineInstances(f.scope.Namespace).Create(ctx, instance, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	metadata, err = f.backend.Context(ctx, f.scope)
	if err != nil || !metadata.WasRunning || metadata.Stopped {
		t.Fatalf("running source was rejected by preflight: %v", err)
	}
	if err := f.backend.Halt(ctx, f.scope, f.intent); err != nil {
		t.Fatal(err)
	}
	stopped, running, _, err := f.backend.power(ctx, f.scope, f.intent)
	if err != nil || stopped || !running {
		t.Fatalf("accepted stop request was mistaken for observed shutdown: %v", err)
	}
	no := false
	vm.Spec.Running = &no
	vm.Status.SnapshotInProgress = &f.intent.Name
	if _, err := f.native.KubevirtV1().VirtualMachines(f.scope.Namespace).Update(ctx, vm, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := f.native.KubevirtV1().VirtualMachineInstances(f.scope.Namespace).Delete(ctx, vm.Name, metav1.DeleteOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := f.backend.Resume(ctx, f.scope, f.intent); !errors.Is(err, ErrConflict) {
		t.Fatal("locked source accepted a start request")
	}
	vm.Status.SnapshotInProgress = nil
	if _, err := f.native.KubevirtV1().VirtualMachines(f.scope.Namespace).Update(ctx, vm, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := f.backend.Resume(ctx, f.scope, f.intent); err != nil {
		t.Fatal(err)
	}
	starts, stops := 0, 0
	for _, action := range f.native.Actions() {
		if action.GetSubresource() == "start" {
			starts++
		}
		if action.GetSubresource() == "stop" {
			stops++
		}
	}
	if starts != 1 || stops != 1 {
		t.Fatal("bounded power commands repeated or used a forced path")
	}
}

func TestNativeObserveWaitsForCSIResidualAfterParentDisappearance(t *testing.T) {
	f := newKubeSnapshotFixture(t)
	ctx := context.Background()
	volume := f.volumes["volume-snapshot-a"]
	content := &csi.VolumeSnapshotContent{ObjectMeta: metav1.ObjectMeta{Name: "csi-content", UID: "csi-content-uid"}, Spec: csi.VolumeSnapshotContentSpec{VolumeSnapshotRef: corev1.ObjectReference{Namespace: f.scope.Namespace, Name: volume.Name, UID: volume.UID}}}
	if _, err := f.csi.SnapshotV1().VolumeSnapshotContents().Create(ctx, content, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	observed, err := f.backend.Observe(ctx, f.scope, f.intent, nil)
	if err != nil || observed.CleanupComplete || len(observed.Resources) != 4 {
		t.Fatalf("native/CSI ownership chain was not observed: %v", err)
	}
	known := observed.Resources
	if err := f.backend.Delete(ctx, f.scope, known); err != nil {
		t.Fatal(err)
	}
	observed, err = f.backend.Observe(ctx, f.scope, f.intent, known)
	if err != nil || observed.CleanupComplete || observed.Snapshot != nil || observed.Content != nil || len(observed.Resources) != 1 || observed.Resources[0].Kind != "VolumeSnapshotContent" {
		t.Fatalf("surviving CSI backend state was mistaken for cleanup: %v", err)
	}
	if err := f.csi.SnapshotV1().VolumeSnapshotContents().Delete(ctx, content.Name, metav1.DeleteOptions{}); err != nil {
		t.Fatal(err)
	}
	observed, err = f.backend.Observe(ctx, f.scope, f.intent, known)
	if err != nil || !observed.CleanupComplete || !observed.SourceLockClear {
		t.Fatalf("fully reclaimed descendants were not recognized: %v", err)
	}
	if _, err := f.resolver.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Get(ctx, "manual7", metav1.GetOptions{}); err != nil {
		t.Fatal("cleanup deleted the source claim")
	}
}

func TestNativeObserveRejectsMismatchedAPISourceAndOwner(t *testing.T) {
	for _, mismatch := range []string{"source-group", "source-name", "owner-api", "owner-kind", "owner-name", "owner-uid", "replaced-csi"} {
		t.Run(mismatch, func(t *testing.T) {
			f := newKubeSnapshotFixture(t)
			ctx := context.Background()
			known := []ResourceRef{{ObjectRef: refOf("VolumeSnapshot", f.volumes["volume-snapshot-a"]), Purpose: "volume-snapshot"}}
			if mismatch == "source-group" || mismatch == "source-name" {
				object := f.snapshot.DeepCopy()
				if mismatch == "source-group" {
					wrong := "other.example"
					object.Spec.Source.APIGroup = &wrong
				} else {
					object.Spec.Source.Name = "other-vm"
				}
				if _, err := f.native.SnapshotV1beta1().VirtualMachineSnapshots(f.scope.Namespace).Update(ctx, object, metav1.UpdateOptions{}); err != nil {
					t.Fatal(err)
				}
			} else {
				object := f.volumes["volume-snapshot-a"].DeepCopy()
				switch mismatch {
				case "owner-api":
					object.OwnerReferences[0].APIVersion = "other.example/v1"
				case "owner-kind":
					object.OwnerReferences[0].Kind = "OtherContent"
				case "owner-name":
					object.OwnerReferences[0].Name = "other-content"
				case "owner-uid":
					object.OwnerReferences[0].UID = "other-content-uid"
				default:
					object.UID = "replacement-csi"
				}
				if _, err := f.csi.SnapshotV1().VolumeSnapshots(f.scope.Namespace).Update(ctx, object, metav1.UpdateOptions{}); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := f.backend.Observe(ctx, f.scope, f.intent, known); !errors.Is(err, ErrConflict) {
				t.Fatalf("mismatched native/CSI chain was adopted: %v", err)
			}
		})
	}
}

func TestNativeObserveRejectsCSIContentBoundToAnotherVolumeUID(t *testing.T) {
	f := newKubeSnapshotFixture(t)
	ctx := context.Background()
	volume := f.volumes["volume-snapshot-a"]
	content := &csi.VolumeSnapshotContent{ObjectMeta: metav1.ObjectMeta{Name: "wrong-csi-content", UID: "wrong-csi-content-uid"}, Spec: csi.VolumeSnapshotContentSpec{VolumeSnapshotRef: corev1.ObjectReference{Namespace: f.scope.Namespace, Name: volume.Name, UID: "other-volume-uid"}}}
	if _, err := f.csi.SnapshotV1().VolumeSnapshotContents().Create(ctx, content, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	known := []ResourceRef{
		{ObjectRef: refOf("VolumeSnapshot", volume), Purpose: "volume-snapshot"},
		{ObjectRef: ObjectRef{Kind: "VolumeSnapshot", Namespace: f.scope.Namespace, Name: "other-volume", UID: "other-volume-uid"}, Purpose: "volume-snapshot"},
	}
	if _, err := f.backend.Observe(ctx, f.scope, f.intent, known); !errors.Is(err, ErrConflict) {
		t.Fatalf("CSI content inherited a different volume's allowed UID: %v", err)
	}
}

func independentCloneFixture(t *testing.T) (kubeSnapshotFixture, *corev1.PersistentVolumeClaim, *corev1.PersistentVolume) {
	t.Helper()
	f := newKubeSnapshotFixture(t)
	ctx := context.Background()
	className := "review-independent-writable"
	class := &storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: className}, Provisioner: "cephfs.csi.ceph.com"}
	if _, err := f.resolver.core.StorageV1().StorageClasses().Create(ctx, class, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	group := "snapshot.storage.k8s.io"
	claim := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "review-independent", Namespace: f.scope.Namespace, UID: "review-independent-uid"}, Spec: corev1.PersistentVolumeClaimSpec{StorageClassName: &className, VolumeName: "review-independent-pv", AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany}, DataSource: &corev1.TypedLocalObjectReference{APIGroup: &group, Kind: "VolumeSnapshot", Name: "volume-snapshot-a"}}, Status: corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound}}
	if _, err := f.resolver.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Create(ctx, claim, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	pv := &corev1.PersistentVolume{ObjectMeta: metav1.ObjectMeta{Name: claim.Spec.VolumeName, UID: "review-independent-pv-uid"}, Spec: corev1.PersistentVolumeSpec{StorageClassName: className, AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany}, ClaimRef: &corev1.ObjectReference{Namespace: claim.Namespace, Name: claim.Name, UID: claim.UID}, PersistentVolumeSource: corev1.PersistentVolumeSource{CSI: &corev1.CSIPersistentVolumeSource{Driver: "cephfs.csi.ceph.com", VolumeHandle: "review-independent-target-handle"}}}, Status: corev1.PersistentVolumeStatus{Phase: corev1.VolumeBound}}
	if _, err := f.resolver.core.CoreV1().PersistentVolumes().Create(ctx, pv, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	return f, claim, pv
}

func TestNativeIndependentWritableCloneDoesNotKeepSnapshotReference(t *testing.T) {
	f, _, _ := independentCloneFixture(t)
	ctx := context.Background()
	resource := ResourceRef{ObjectRef: refOf("VolumeSnapshot", f.volumes["volume-snapshot-a"]), Purpose: "volume-snapshot"}
	if err := f.backend.DeleteReferences(ctx, f.scope, []ResourceRef{resource}); err != nil {
		t.Fatalf("bound independent writable PVC provenance blocked snapshot deletion: %v", err)
	}
}

func TestNativeCloneReferenceGateRequiresIndependentWritableBinding(t *testing.T) {
	for _, invalid := range []string{"pending", "rox-retain", "backing-snapshot", "pv-readonly", "claim-uid", "driver", "released-pv", "missing-pv", "missing-class", "block", "pv-backing", "mount-ro", "empty-handle", "deleting-claim"} {
		t.Run(invalid, func(t *testing.T) {
			f, claim, pv := independentCloneFixture(t)
			ctx := context.Background()
			switch invalid {
			case "pending":
				claim.Status.Phase = corev1.ClaimPending
			case "rox-retain":
				claim.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany}
				pv.Spec.AccessModes = claim.Spec.AccessModes
				pv.Spec.PersistentVolumeReclaimPolicy = corev1.PersistentVolumeReclaimRetain
			case "backing-snapshot":
				class, err := f.resolver.core.StorageV1().StorageClasses().Get(ctx, *claim.Spec.StorageClassName, metav1.GetOptions{})
				if err != nil {
					t.Fatal(err)
				}
				class.Parameters = map[string]string{"backingSnapshot": "true"}
				if _, err := f.resolver.core.StorageV1().StorageClasses().Update(ctx, class, metav1.UpdateOptions{}); err != nil {
					t.Fatal(err)
				}
			case "pv-readonly":
				pv.Spec.CSI.ReadOnly = true
			case "claim-uid":
				pv.Spec.ClaimRef.UID = "replaced-claim"
			case "driver":
				pv.Spec.CSI.Driver = "other.csi.example"
			case "released-pv":
				pv.Status.Phase = corev1.VolumeReleased
			case "block":
				mode := corev1.PersistentVolumeBlock
				claim.Spec.VolumeMode = &mode
			case "pv-backing":
				pv.Spec.CSI.VolumeAttributes = map[string]string{"backingSnapshot": "true"}
			case "mount-ro":
				pv.Spec.MountOptions = []string{"ro"}
			case "empty-handle":
				pv.Spec.CSI.VolumeHandle = ""
			case "deleting-claim":
				now := metav1.Now()
				claim.DeletionTimestamp = &now
			}
			if _, err := f.resolver.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Update(ctx, claim, metav1.UpdateOptions{}); err != nil {
				t.Fatal(err)
			}
			if _, err := f.resolver.core.CoreV1().PersistentVolumes().Update(ctx, pv, metav1.UpdateOptions{}); err != nil {
				t.Fatal(err)
			}
			if invalid == "missing-pv" {
				if err := f.resolver.core.CoreV1().PersistentVolumes().Delete(ctx, pv.Name, metav1.DeleteOptions{}); err != nil {
					t.Fatal(err)
				}
			}
			if invalid == "missing-class" {
				if err := f.resolver.core.StorageV1().StorageClasses().Delete(ctx, *claim.Spec.StorageClassName, metav1.DeleteOptions{}); err != nil {
					t.Fatal(err)
				}
			}
			resource := ResourceRef{ObjectRef: refOf("VolumeSnapshot", f.volumes["volume-snapshot-a"]), Purpose: "volume-snapshot"}
			if err := f.backend.DeleteReferences(ctx, f.scope, []ResourceRef{resource}); !errors.Is(err, ErrBusy) {
				t.Fatalf("unproven clone dependency allowed deletion: %v", err)
			}
		})
	}
}

func TestNativeCloneDataSourceRefAndUnavailableProof(t *testing.T) {
	for _, scenario := range []string{"independent-ref", "foreign-namespace", "pv-unavailable", "class-unavailable", "list-unavailable"} {
		t.Run(scenario, func(t *testing.T) {
			f, claim, _ := independentCloneFixture(t)
			ctx := context.Background()
			group := csi.GroupName
			claim.Spec.DataSource = nil
			claim.Spec.DataSourceRef = &corev1.TypedObjectReference{APIGroup: &group, Kind: "VolumeSnapshot", Name: "volume-snapshot-a"}
			if scenario == "foreign-namespace" {
				ns := "other-team"
				claim.Spec.DataSourceRef.Namespace = &ns
				claim.Status.Phase = corev1.ClaimPending
			}
			if _, err := f.resolver.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Update(ctx, claim, metav1.UpdateOptions{}); err != nil {
				t.Fatal(err)
			}
			reject := func(ktesting.Action) (bool, runtime.Object, error) { return true, nil, ErrReferenceUnavailable }
			switch scenario {
			case "pv-unavailable":
				f.resolver.core.PrependReactor("get", "persistentvolumes", reject)
			case "class-unavailable":
				f.resolver.core.PrependReactor("get", "storageclasses", reject)
			case "list-unavailable":
				f.resolver.core.PrependReactor("list", "persistentvolumeclaims", reject)
			}
			resource := ResourceRef{ObjectRef: refOf("VolumeSnapshot", f.volumes["volume-snapshot-a"]), Purpose: "volume-snapshot"}
			err := f.backend.DeleteReferences(ctx, f.scope, []ResourceRef{resource})
			if scenario == "independent-ref" || scenario == "foreign-namespace" {
				if err != nil {
					t.Fatal(err)
				}
			} else if !errors.Is(err, ErrReferenceUnavailable) {
				t.Fatalf("unavailable independence proof was ignored: %v", err)
			}
		})
	}
}

func TestNativeReadAndPowerFailuresRemainUnavailable(t *testing.T) {
	for _, failure := range []string{"vm-list", "vmi-get", "pod-list", "storage-get", "pv-get", "snapshot-classes", "snapshot-get", "content-get", "csi-list", "csi-content-list", "start", "stop", "exists"} {
		t.Run(failure, func(t *testing.T) {
			f := newKubeSnapshotFixture(t)
			ctx := context.Background()
			reject := func(ktesting.Action) (bool, runtime.Object, error) { return true, nil, ErrReferenceUnavailable }
			var err error
			switch failure {
			case "vm-list":
				f.native.PrependReactor("list", "virtualmachines", reject)
				_, err = f.backend.Context(ctx, f.scope)
			case "vmi-get":
				f.native.PrependReactor("get", "virtualmachineinstances", reject)
				_, err = f.backend.Context(ctx, f.scope)
			case "pod-list":
				f.resolver.core.PrependReactor("list", "pods", reject)
				_, _, _, err = f.backend.power(ctx, f.scope, f.intent)
			case "storage-get", "pv-get", "snapshot-classes":
				disk := SnapshotDisk{SourcePVC: f.resolver.currentBinding().ActivePVC}
				if failure == "storage-get" {
					f.resolver.core.PrependReactor("get", "storageclasses", reject)
				}
				if failure == "pv-get" {
					f.resolver.core.PrependReactor("get", "persistentvolumes", reject)
				}
				if failure == "snapshot-classes" {
					f.csi.PrependReactor("list", "volumesnapshotclasses", reject)
				}
				err = f.backend.checkStorage(ctx, f.scope, disk)
			case "snapshot-get":
				f.native.PrependReactor("get", "virtualmachinesnapshots", reject)
				_, err = f.backend.Observe(ctx, f.scope, f.intent, nil)
			case "content-get":
				f.native.PrependReactor("get", "virtualmachinesnapshotcontents", reject)
				_, err = f.backend.Observe(ctx, f.scope, f.intent, nil)
			case "csi-list":
				f.csi.PrependReactor("list", "volumesnapshots", reject)
				_, err = f.backend.Observe(ctx, f.scope, f.intent, nil)
			case "csi-content-list":
				f.csi.PrependReactor("list", "volumesnapshotcontents", reject)
				_, err = f.backend.Observe(ctx, f.scope, f.intent, nil)
			case "start":
				f.native.PrependReactor("put", "virtualmachines", reject)
				err = f.backend.Resume(ctx, f.scope, f.intent)
			case "stop":
				vm, e := f.native.KubevirtV1().VirtualMachines(f.scope.Namespace).Get(ctx, f.intent.SourceVM.Name, metav1.GetOptions{})
				if e != nil {
					t.Fatal(e)
				}
				yes := true
				vm.Spec.Running = &yes
				if _, e := f.native.KubevirtV1().VirtualMachines(f.scope.Namespace).Update(ctx, vm, metav1.UpdateOptions{}); e != nil {
					t.Fatal(e)
				}
				f.native.PrependReactor("put", "virtualmachines", reject)
				err = f.backend.Halt(ctx, f.scope, f.intent)
			case "exists":
				f.native.PrependReactor("get", "virtualmachinesnapshots", reject)
				_, err = f.backend.Exists(ctx, f.scope, f.intent.Name)
			}
			if !errors.Is(err, ErrReferenceUnavailable) {
				t.Fatalf("unavailable backend was treated as successful observation/action: %v", err)
			}
		})
	}
}

func TestNativeOccupancyAndMissingSourceRemainDistinct(t *testing.T) {
	f := newKubeSnapshotFixture(t)
	ctx := context.Background()
	if exists, err := f.backend.Exists(ctx, f.scope, f.intent.Name); err != nil || !exists {
		t.Fatal("occupied name was considered available")
	}
	if err := f.native.SnapshotV1beta1().VirtualMachineSnapshots(f.scope.Namespace).Delete(ctx, f.intent.Name, metav1.DeleteOptions{}); err != nil {
		t.Fatal(err)
	}
	if exists, err := f.backend.Exists(ctx, f.scope, f.intent.Name); err != nil || exists {
		t.Fatal("missing name was not available")
	}
	if err := f.native.KubevirtV1().VirtualMachines(f.scope.Namespace).Delete(ctx, f.intent.SourceVM.Name, metav1.DeleteOptions{}); err != nil {
		t.Fatal(err)
	}
	stopped, running, _, err := f.backend.power(ctx, f.scope, f.intent)
	if err != nil || stopped || running {
		t.Fatal("missing source VM was falsely observed halted or running")
	}
}

func TestNativeSourceUIDWriterAndRuntimeGuards(t *testing.T) {
	for _, scenario := range []string{"foreign-tenant", "no-template", "halt-replaced-uid", "resume-replaced-uid", "writer-pod", "pending-volume", "wrong-claim", "unmanaged-disk", "protection-finalizer", "always-intent", "halted-intent", "missing-intent"} {
		t.Run(scenario, func(t *testing.T) {
			f := newKubeSnapshotFixture(t)
			ctx := context.Background()
			vm, err := f.native.KubevirtV1().VirtualMachines(f.scope.Namespace).Get(ctx, f.intent.SourceVM.Name, metav1.GetOptions{})
			if err != nil {
				t.Fatal(err)
			}
			switch scenario {
			case "foreign-tenant":
				vm.Labels["tenant_id"] = "other-tenant"
			case "no-template":
				vm.Spec.Template = nil
			case "halt-replaced-uid", "resume-replaced-uid":
				vm.UID = "replacement-vm"
			case "wrong-claim":
				vm.Spec.Template.Spec.Volumes[0].PersistentVolumeClaim.ClaimName = "wrong-claim"
			case "unmanaged-disk":
				vm.Spec.Template.Spec.Volumes = append(vm.Spec.Template.Spec.Volumes, kv.Volume{Name: "unmanaged", VolumeSource: kv.VolumeSource{EmptyDisk: &kv.EmptyDiskSource{}}})
			case "protection-finalizer":
				vm.Finalizers = []string{"snapshot.kubevirt.io/snapshot-source-protection"}
			case "always-intent":
				strategy := kv.RunStrategyAlways
				vm.Spec.Running = nil
				vm.Spec.RunStrategy = &strategy
			case "halted-intent":
				strategy := kv.RunStrategyHalted
				vm.Spec.Running = nil
				vm.Spec.RunStrategy = &strategy
			case "missing-intent":
				vm.Spec.Running = nil
				vm.Spec.RunStrategy = nil
			}
			if _, err := f.native.KubevirtV1().VirtualMachines(f.scope.Namespace).Update(ctx, vm, metav1.UpdateOptions{}); err != nil {
				t.Fatal(err)
			}
			switch scenario {
			case "halt-replaced-uid":
				if err := f.backend.Halt(ctx, f.scope, f.intent); !errors.Is(err, ErrConflict) {
					t.Fatal("Stop targeted replacement VM")
				}
				return
			case "resume-replaced-uid", "protection-finalizer":
				if err := f.backend.Resume(ctx, f.scope, f.intent); !errors.Is(err, ErrConflict) {
					t.Fatal("Start targeted replacement/protected VM")
				}
				return
			case "writer-pod":
				for _, phase := range []corev1.PodPhase{corev1.PodSucceeded, corev1.PodRunning} {
					name := "writer-" + strings.ToLower(string(phase))
					pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: f.scope.Namespace}, Spec: corev1.PodSpec{Volumes: []corev1.Volume{{Name: "source", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: f.intent.Disks[0].SourcePVC.Name}}}}}, Status: corev1.PodStatus{Phase: phase}}
					if _, err := f.resolver.core.CoreV1().Pods(f.scope.Namespace).Create(ctx, pod, metav1.CreateOptions{}); err != nil {
						t.Fatal(err)
					}
				}
				stopped, _, _, err := f.backend.power(ctx, f.scope, f.intent)
				if err != nil || stopped {
					t.Fatal("live PVC writer was called stopped")
				}
				return
			case "pending-volume":
				claim, err := f.resolver.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Get(ctx, "manual7", metav1.GetOptions{})
				if err != nil {
					t.Fatal(err)
				}
				claim.Status.Phase = corev1.ClaimPending
				if _, err := f.resolver.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Update(ctx, claim, metav1.UpdateOptions{}); err != nil {
					t.Fatal(err)
				}
			}
			metadata, err := f.backend.Context(ctx, f.scope)
			if scenario == "always-intent" || scenario == "halted-intent" || scenario == "missing-intent" {
				wantRunning := scenario == "always-intent"
				if err != nil || metadata.WasRunning != wantRunning || metadata.Stopped == wantRunning {
					t.Fatalf("run intent observation mismatch: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("unsafe source/storage configuration passed preflight")
			}
		})
	}
}

func TestNativeStorageIdentityCannotBeReplacedOrBacked(t *testing.T) {
	for _, scenario := range []string{"claim-uid", "backing-class", "block-mode"} {
		t.Run(scenario, func(t *testing.T) {
			f := newKubeSnapshotFixture(t)
			ctx := context.Background()
			disk := SnapshotDisk{SourcePVC: f.resolver.currentBinding().ActivePVC}
			claim, err := f.resolver.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Get(ctx, "manual7", metav1.GetOptions{})
			if err != nil {
				t.Fatal(err)
			}
			switch scenario {
			case "claim-uid":
				claim.UID = "replaced-claim"
			case "block-mode":
				mode := corev1.PersistentVolumeBlock
				claim.Spec.VolumeMode = &mode
			case "backing-class":
				class, err := f.resolver.core.StorageV1().StorageClasses().Get(ctx, *claim.Spec.StorageClassName, metav1.GetOptions{})
				if err != nil {
					t.Fatal(err)
				}
				class.Parameters = map[string]string{"backingSnapshot": "true"}
				if _, err := f.resolver.core.StorageV1().StorageClasses().Update(ctx, class, metav1.UpdateOptions{}); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := f.resolver.core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Update(ctx, claim, metav1.UpdateOptions{}); err != nil {
				t.Fatal(err)
			}
			if err := f.backend.checkStorage(ctx, f.scope, disk); err == nil {
				t.Fatal("replaced/backed/block source passed snapshot preflight")
			}
		})
	}
}
