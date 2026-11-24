package vmrestore

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/goodrain/rainbond/pkg/vmrestore/copyio"
	"github.com/goodrain/rainbond/pkg/vmrestore/restoreplan"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
)

func resourceFixture(t *testing.T, configure ...func(restoreCommitFixture, *RestoreExecution)) (restoreCommitFixture, RestoreResourcePlan, *RestoreResourceExecutor) {
	t.Helper()
	f := newRestoreCommitFixture(t)
	f.prepare(t)
	frame := RestoreResourcePlan{Scope: f.scope, OperationID: f.expect.OperationID, CoreHash: f.expect.ManifestHash, WorkerImage: "registry.invalid/copy@sha256:" + strings.Repeat("a", 64), RuntimeImageID: "containerd://sha256:" + strings.Repeat("a", 64), DeadlineSeconds: 60, MaxDisks: 8, TargetUID: 107, TargetGID: 107, Disks: []RestoreDiskResource{}}
	for _, id := range []uint{7, 8} {
		frame.Disks = append(frame.Disks, RestoreDiskResource{VolumeID: id, SnapshotContent: ObjectRef{Kind: "VolumeSnapshotContent", Name: "content", UID: "content-uid"}, Snapshot: ObjectRef{Kind: "VolumeSnapshot", Namespace: f.scope.Namespace, Name: "snapshot-" + canonicalVolumeName(id), UID: "snapshot-uid-" + canonicalVolumeName(id)}, PreviousPVC: f.old[map[uint]string{7: "7", 8: "8"}[id]].ActivePVC, ROXClass: "rox", ROXClassUID: "class-rox", TargetClass: "writable", TargetClassUID: "class-target", TargetAccessMode: "ReadWriteMany", ROXRequestBytes: 1 << 30, TargetRequestBytes: 2 << 30, PreviousMaxBytes: 4 << 30})
	}
	frame.FrameHash, _ = RestoreResourceFrameHash(frame)
	cap := CertifiedRestoreStorage{Capability: restoreplan.Capability{Protocol: copyio.Protocol, StorageClassName: "writable", StorageClassUID: "class-target", Driver: "cephfs.csi.ceph.com", DriverVersion: "certified-test-version", Mounter: "kernel", ROXVerified: true, SparsePersistenceVerified: true, WorkerImage: frame.WorkerImage}, ROXClassName: "rox", ROXClassUID: "class-rox", TargetClassName: "writable", TargetClassUID: "class-target", RuntimeImageID: frame.RuntimeImageID, DeleteReclaimVerified: true}
	identityHash, _ := snapshotDigest(testStartupPolicy("restored"))
	execution := RestoreExecution{Frame: frame, Intents: map[string]RestoreObjectIntent{}, Evidence: map[string]restoreplan.CopyEvidence{}, NetworkHash: strings.Repeat("e", 64), IdentityHash: identityHash, BeforeStartup: testStartupPolicy("platform"), SourceStartup: testStartupPolicy("restored")}
	for _, apply := range configure {
		apply(f, &execution)
	}
	_, err := f.store.InitializeRestoreExecution(f.scope, f.expect.OperationID, execution)
	if err != nil {
		t.Fatal(err)
	}
	dyn := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
	e := &RestoreResourceExecutor{Store: f.store, Core: kubefake.NewSimpleClientset(), Dynamic: dyn, Certification: cap, BeforeMutation: func(context.Context, Scope, string, string) error { return nil }}
	dyn.PrependReactor("create", "*", func(a ktesting.Action) (bool, runtime.Object, error) {
		obj := a.(ktesting.CreateAction).GetObject().(*unstructured.Unstructured)
		if obj.GetUID() == "" {
			obj.SetUID(types.UID("created-" + obj.GetName()))
		}
		return false, nil, nil
	})
	return f, execution.Frame, e
}
func inspectPlan(frame RestoreResourcePlan) copyio.Plan {
	p := copyio.Plan{Protocol: copyio.Protocol, OperationID: frame.OperationID, ManifestHash: frame.CoreHash, FrameHash: frame.FrameHash, Mode: "inspect", DeadlineSeconds: frame.DeadlineSeconds, TargetUID: frame.TargetUID, TargetGID: frame.TargetGID}
	for _, d := range frame.Disks {
		p.Disks = append(p.Disks, copyio.DiskPlan{VolumeID: d.VolumeID, SourcePVCUID: "rox-" + canonicalVolumeName(d.VolumeID), PreviousPVCUID: d.PreviousPVC.UID, SourceMaxBytes: d.ROXRequestBytes, PreviousMaxBytes: d.TargetRequestBytes})
	}
	p, _ = copyio.SealPlan(p)
	return p
}

// capability_id: rainbond.vm-restore.resource-execution
func TestRestoreClaimsSeparateROXAndIndependentBlankTarget(t *testing.T) {
	_, frame, _ := resourceFixture(t)
	ro, target, err := RestoreClaims(frame, 7)
	if err != nil {
		t.Fatal(err)
	}
	if ro.Spec.AccessModes[0] != corev1.ReadOnlyMany || ro.Spec.DataSource.Kind != "VolumeSnapshot" || *ro.Spec.StorageClassName != "rox" || target.Spec.DataSource != nil || target.Spec.DataSourceRef != nil || *target.Spec.StorageClassName != "writable" || len(target.OwnerReferences) != 0 {
		t.Fatal("unsafe clone/class/owner")
	}
	plan := inspectPlan(frame)
	mounts := []RestoreMounts{}
	for i, d := range frame.Disks {
		mounts = append(mounts, RestoreMounts{VolumeID: d.VolumeID, ROX: ObjectRef{Kind: "PersistentVolumeClaim", Namespace: frame.Scope.Namespace, Name: frame.name(d.VolumeID, "rox"), UID: plan.Disks[i].SourcePVCUID}, Previous: d.PreviousPVC})
	}
	job, cm, err := RestoreCopyJob(frame, plan, mounts)
	if err != nil {
		t.Fatal(err)
	}
	if job.Spec.TTLSecondsAfterFinished != nil || *job.Spec.BackoffLimit != 0 || len(job.OwnerReferences) != 0 || !*cm.Immutable {
		t.Fatal("job can disappear/retry before evidence")
	}
	for _, volume := range job.Spec.Template.Spec.Volumes {
		if volume.PersistentVolumeClaim != nil && !volume.PersistentVolumeClaim.ReadOnly {
			t.Fatal("source or B writer")
		}
	}
	frame.Disks[0].TargetClass = "rox"
	frame.FrameHash, _ = RestoreResourceFrameHash(frame)
	if _, _, err = RestoreClaims(frame, 7); err == nil {
		t.Fatal("same class accepted")
	}
}

// capability_id: rainbond.vm-restore.resource-execution
func TestCreateUnknownReconcilesOnlyDurableNonceAndPinnedUID(t *testing.T) {
	f, frame, e := resourceFixture(t)
	ro, _, _ := RestoreClaims(frame, 7)
	dyn := e.Dynamic.(*dynamicfake.FakeDynamicClient)
	once := true
	dyn.PrependReactor("create", "persistentvolumeclaims", func(a ktesting.Action) (bool, runtime.Object, error) {
		if !once {
			return false, nil, nil
		}
		once = false
		obj := a.(ktesting.CreateAction).GetObject().(*unstructured.Unstructured)
		obj.SetUID("created-timeout")
		if err := dyn.Tracker().Create(restoreGVR["PersistentVolumeClaim"], obj, f.scope.Namespace); err != nil {
			t.Fatal(err)
		}
		return true, nil, errors.New("synthetic network timeout")
	})
	if _, err := e.Ensure(context.Background(), f.scope, f.expect.OperationID, "rox7", "restore-rox", "PersistentVolumeClaim", ro); !errors.Is(err, ErrRestoreUnknown) {
		t.Fatal(err)
	}
	saved, _ := f.store.Read(f.scope, f.expect.OperationID)
	intent := saved.Receipt.Execution.Intents["rox7"]
	if intent.Nonce == "" || intent.Object.UID != "" {
		t.Fatal("unknown create lost intent")
	}
	ref, err := e.Ensure(context.Background(), f.scope, f.expect.OperationID, "rox7", "restore-rox", "PersistentVolumeClaim", ro)
	if err != nil || ref.UID != "created-timeout" {
		t.Fatal(ref, err)
	}
	got, _ := dyn.Resource(restoreGVR["PersistentVolumeClaim"]).Namespace(f.scope.Namespace).Get(context.Background(), ro.Name, metav1.GetOptions{})
	got.SetUID("replacement")
	_, _ = dyn.Resource(restoreGVR["PersistentVolumeClaim"]).Namespace(f.scope.Namespace).Update(context.Background(), got, metav1.UpdateOptions{})
	if _, err = e.Ensure(context.Background(), f.scope, f.expect.OperationID, "rox7", "restore-rox", "PersistentVolumeClaim", ro); !errors.Is(err, ErrConflict) {
		t.Fatal("adopted same-name replacement", err)
	}
}

// capability_id: rainbond.vm-restore.resource-execution
func TestCreateCollisionAndUncertifiedProviderHaveNoMutation(t *testing.T) {
	f, frame, e := resourceFixture(t)
	ro, _, _ := RestoreClaims(frame, 7)
	e.Certification.Capability.SparsePersistenceVerified = false
	if _, err := e.Ensure(context.Background(), f.scope, f.expect.OperationID, "rox7", "restore-rox", "PersistentVolumeClaim", ro); !errors.Is(err, ErrRestoreCapability) {
		t.Fatal(err)
	}
	if len(e.Dynamic.(*dynamicfake.FakeDynamicClient).Actions()) != 0 {
		t.Fatal("uncertified provider used")
	}
	e.Certification.Capability.SparsePersistenceVerified = true
	u, _ := desiredObject(ro, "PersistentVolumeClaim")
	u.SetUID("foreign")
	_, _ = e.Dynamic.Resource(restoreGVR["PersistentVolumeClaim"]).Namespace(f.scope.Namespace).Create(context.Background(), u, metav1.CreateOptions{})
	if _, err := e.Ensure(context.Background(), f.scope, f.expect.OperationID, "rox7", "restore-rox", "PersistentVolumeClaim", ro); !errors.Is(err, ErrConflict) {
		t.Fatal("adopted pre-intent collision", err)
	}
}

// capability_id: rainbond.vm-restore.resource-execution
func TestROXPVC404DoesNotReleaseSnapshotWhilePinnedPVRemains(t *testing.T) {
	f, frame, e := resourceFixture(t)
	ro, _, _ := RestoreClaims(frame, 7)
	ref, err := e.Ensure(context.Background(), f.scope, f.expect.OperationID, "rox7", "restore-rox", "PersistentVolumeClaim", ro)
	if err != nil {
		t.Fatal(err)
	}
	pvref := ObjectRef{Kind: "PersistentVolume", Name: "retained-pv", UID: "pv-uid"}
	_, err = f.store.executionUpdate(f.scope, f.expect.OperationID, "pv-rox7", pvref, func(c Current) (Mutation, error) {
		v := c.Receipt.Execution.Intents["rox7"]
		v.BoundPV = &pvref
		c.Receipt.Execution.Intents["rox7"] = v
		return Mutation{State: c.State, Receipt: c.Receipt}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	_, _ = e.Core.CoreV1().PersistentVolumes().Create(context.Background(), &corev1.PersistentVolume{ObjectMeta: metav1.ObjectMeta{Name: pvref.Name, UID: types.UID(pvref.UID)}}, metav1.CreateOptions{})
	if err = e.Delete(context.Background(), f.scope, f.expect.OperationID, "rox7"); !errors.Is(err, ErrRestorePending) {
		t.Fatal(err)
	}
	if err = e.Delete(context.Background(), f.scope, f.expect.OperationID, "rox7"); !errors.Is(err, ErrRestorePending) {
		t.Fatal("PVC404 incorrectly released source", err)
	}
	state, _ := f.store.Read(f.scope, f.expect.OperationID)
	if state.Receipt.Execution.Intents["rox7"].Deleted {
		t.Fatal("sealed early release")
	}
	_ = e.Core.CoreV1().PersistentVolumes().Delete(context.Background(), pvref.Name, metav1.DeleteOptions{})
	if err = e.Delete(context.Background(), f.scope, f.expect.OperationID, "rox7"); err != nil {
		t.Fatal(err)
	}
	if ref.UID == "" {
		t.Fatal("fixture")
	}
}

// capability_id: rainbond.vm-restore.resource-execution
func TestDesiredSpecMutationRejectedDespiteCopiedAnnotations(t *testing.T) {
	f, frame, e := resourceFixture(t)
	ro, _, _ := RestoreClaims(frame, 7)
	if _, err := e.Ensure(context.Background(), f.scope, f.expect.OperationID, "rox7", "restore-rox", "PersistentVolumeClaim", ro); err != nil {
		t.Fatal(err)
	}
	ri := e.Dynamic.Resource(restoreGVR["PersistentVolumeClaim"]).Namespace(f.scope.Namespace)
	got, _ := ri.Get(context.Background(), ro.Name, metav1.GetOptions{})
	_ = unstructured.SetNestedSlice(got.Object, []interface{}{"ReadWriteMany"}, "spec", "accessModes")
	_, _ = ri.Update(context.Background(), got, metav1.UpdateOptions{})
	if _, err := e.Ensure(context.Background(), f.scope, f.expect.OperationID, "rox7", "restore-rox", "PersistentVolumeClaim", ro); !errors.Is(err, ErrConflict) {
		t.Fatal("trusted annotations instead of spec", err)
	}
}

// capability_id: rainbond.vm-restore.resource-execution
func TestSourceRequiresExactBaseClassParametersAndNativeBinding(t *testing.T) {
	for _, tc := range []string{"valid", "class-uid", "cluster-id", "snapshot-uid", "content-ref", "deep-clone-class"} {
		t.Run(tc, func(t *testing.T) {
			f, frame, e := resourceFixture(t)
			policy := corev1.PersistentVolumeReclaimDelete
			target := &storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: "writable", UID: "class-target"}, Provisioner: e.Certification.Capability.Driver, ReclaimPolicy: &policy, Parameters: map[string]string{"mounter": "kernel", "clusterID": "TEST-CLUSTER", "fsName": "TEST-FS", "pool": "TEST-POOL", "csi.storage.k8s.io/provisioner-secret-name": "TEST-REFERENCE"}}
			rox := target.DeepCopy()
			rox.Name = "rox"
			rox.UID = "class-rox"
			delete(rox.Parameters, "pool")
			rox.Parameters["backingSnapshot"] = "true"
			if tc == "class-uid" {
				rox.UID = "replacement"
			}
			if tc == "cluster-id" {
				rox.Parameters["clusterID"] = "OTHER-TEST-CLUSTER"
			}
			if tc == "deep-clone-class" {
				delete(rox.Parameters, "backingSnapshot")
			}
			_, _ = e.Core.StorageV1().StorageClasses().Create(context.Background(), target, metav1.CreateOptions{})
			_, _ = e.Core.StorageV1().StorageClasses().Create(context.Background(), rox, metav1.CreateOptions{})
			snapshot := &unstructured.Unstructured{Object: map[string]interface{}{"apiVersion": "snapshot.storage.k8s.io/v1", "kind": "VolumeSnapshot", "metadata": map[string]interface{}{"name": frame.Disks[0].Snapshot.Name, "namespace": f.scope.Namespace, "uid": frame.Disks[0].Snapshot.UID}, "status": map[string]interface{}{"readyToUse": true, "boundVolumeSnapshotContentName": "content"}}}
			content := &unstructured.Unstructured{Object: map[string]interface{}{"apiVersion": "snapshot.storage.k8s.io/v1", "kind": "VolumeSnapshotContent", "metadata": map[string]interface{}{"name": "content", "uid": "content-uid"}, "spec": map[string]interface{}{"driver": e.Certification.Capability.Driver, "volumeSnapshotRef": map[string]interface{}{"name": snapshot.GetName(), "namespace": f.scope.Namespace, "uid": string(snapshot.GetUID())}}}}
			if tc == "snapshot-uid" {
				snapshot.SetUID("replacement")
			}
			if tc == "content-ref" {
				_ = unstructured.SetNestedField(content.Object, "other-uid", "spec", "volumeSnapshotRef", "uid")
			}
			_, _ = e.Dynamic.Resource(schema.GroupVersionResource{Group: "snapshot.storage.k8s.io", Version: "v1", Resource: "volumesnapshots"}).Namespace(f.scope.Namespace).Create(context.Background(), snapshot, metav1.CreateOptions{})
			_, _ = e.Dynamic.Resource(schema.GroupVersionResource{Group: "snapshot.storage.k8s.io", Version: "v1", Resource: "volumesnapshotcontents"}).Create(context.Background(), content, metav1.CreateOptions{})
			err := e.VerifySource(context.Background(), f.scope, f.expect.OperationID, 7)
			if tc == "valid" && err != nil {
				t.Fatal(err)
			}
			if tc != "valid" && err == nil {
				t.Fatal("accepted drift", tc)
			}
		})
	}
}

// capability_id: rainbond.vm-restore.resource-execution
func TestPinBoundClaimRejectsForeignClaimRefAndRetain(t *testing.T) {
	for _, tc := range []string{"valid", "claim-uid", "retain", "driver"} {
		t.Run(tc, func(t *testing.T) {
			f, frame, e := resourceFixture(t)
			ro, _, _ := RestoreClaims(frame, 7)
			ref, err := e.Ensure(context.Background(), f.scope, f.expect.OperationID, "rox7", "restore-rox", "PersistentVolumeClaim", ro)
			if err != nil {
				t.Fatal(err)
			}
			ro.UID = types.UID(ref.UID)
			ro.Spec.VolumeName = "pinned-pv"
			ro.Status.Phase = corev1.ClaimBound
			pv := &corev1.PersistentVolume{ObjectMeta: metav1.ObjectMeta{Name: "pinned-pv", UID: "bound-pv-uid"}, Spec: corev1.PersistentVolumeSpec{ClaimRef: &corev1.ObjectReference{Namespace: f.scope.Namespace, Name: ro.Name, UID: ro.UID}, PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimDelete, PersistentVolumeSource: corev1.PersistentVolumeSource{CSI: &corev1.CSIPersistentVolumeSource{Driver: e.Certification.Capability.Driver}}}}
			if tc == "claim-uid" {
				pv.Spec.ClaimRef.UID = "replacement"
			}
			if tc == "retain" {
				pv.Spec.PersistentVolumeReclaimPolicy = corev1.PersistentVolumeReclaimRetain
			}
			if tc == "driver" {
				pv.Spec.CSI.Driver = "other.csi.invalid"
			}
			_, _ = e.Core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Create(context.Background(), ro, metav1.CreateOptions{})
			_, _ = e.Core.CoreV1().PersistentVolumes().Create(context.Background(), pv, metav1.CreateOptions{})
			got, err := e.PinBoundClaim(context.Background(), f.scope, f.expect.OperationID, "rox7")
			if tc == "valid" {
				if err != nil || got.UID != "bound-pv-uid" {
					t.Fatal(got, err)
				}
			} else if err == nil {
				t.Fatal("accepted foreign reclaim chain", tc)
			}
		})
	}
}

func testStartupPolicy(mode string) RestoreStartupPolicy {
	return RestoreStartupPolicy{FirmwareUUID: "11111111-1111-4111-8111-111111111111", BootstrapMode: mode, NoCloudVolumeName: "platform-cloudinit", NoCloudDiskName: "platform-cloudinit", DataSourceKind: "CloudInitNoCloud", ProducerUID: "test-producer-uid", GeneratedMedia: []RestoreGeneratedMedia{}}
}

// capability_id: rainbond.vm-restore.resource-execution
func TestCompactResourceStepsPreserveReplayOwnershipAndCommitSeal(t *testing.T) {
	f, _, _ := resourceFixture(t)
	for index := 0; index < 64; index++ {
		key := fmt.Sprintf("resource-%03d", index)
		ref := ObjectRef{Kind: "ConfigMap", Namespace: f.scope.Namespace, Name: key, UID: strings.Repeat("u", 110) + fmt.Sprintf("-%03d", index)}
		intent := RestoreObjectIntent{Key: key, Nonce: "nonce-" + key, DesiredHash: strings.Repeat("a", 64), Object: ref, Purpose: "restore-plan"}
		if _, err := f.store.resourceIntent(f.scope, f.expect.OperationID, intent); err != nil {
			t.Fatal("intent receipt grew beyond bound", index, err)
		}
		first, err := f.store.resourceObserved(f.scope, f.expect.OperationID, key, ref, nil)
		if err != nil {
			t.Fatal("observed receipt grew beyond bound", index, err)
		}
		replay, err := f.store.resourceObserved(f.scope, f.expect.OperationID, key, ref, nil)
		if err != nil || !replay.Replayed || replay.State.ControlRevision != first.State.ControlRevision {
			t.Fatal("resource step did not replay", index, err)
		}
	}
	c, _ := f.store.Read(f.scope, f.expect.OperationID)
	if len(c.Receipt.Resources) != 64 {
		t.Fatal("lost exact owned resources")
	}
	if len(c.Receipt.Steps["observed-resource-063"].Resources) != 0 {
		t.Fatal("quadratic resource snapshots retained")
	}
	encoded, err := EncodeRecord(c.Receipt)
	if err != nil || len(encoded) >= MaxRecordBytes {
		t.Fatal("bounded receipt no longer fits", len(encoded), err)
	}
	// Preserve the original shared producer's full atomic commit footprint.
	f.sealCopy(t)
	request := f.request(t)
	committed, err := f.store.CommitRestoreDB(f.scope, f.expectStep(t, RestoreCommitStep), request)
	if err != nil {
		t.Fatal(err)
	}
	if len(committed.Receipt.Steps[RestoreCommitStep].Resources) != len(committed.Receipt.Resources) || committed.Receipt.ConfigCommitHash != committed.Receipt.Steps[RestoreCommitStep].RequestHash || !reflect.DeepEqual(committed.Receipt.OldBindings, f.old) {
		t.Fatal("compact policy changed commit seal/retained B")
	}
	for _, resource := range committed.Receipt.Resources {
		if strings.HasPrefix(resource.Name, "resource-") && resource.UID == "" {
			t.Fatal("lost exact cleanup identity")
		}
	}
}

// capability_id: rainbond.vm-restore.resource-execution
func TestExecutionTransitionCannotRewriteFrameUIDOrEraseCleanupHistory(t *testing.T) {
	f, frame, e := resourceFixture(t)
	ro, _, _ := RestoreClaims(frame, 7)
	if _, err := e.Ensure(context.Background(), f.scope, f.expect.OperationID, "rox7", "restore-rox", "PersistentVolumeClaim", ro); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []string{"frame", "uid", "erase", "startup"} {
		t.Run(tc, func(t *testing.T) {
			_, err := f.store.executionUpdate(f.scope, f.expect.OperationID, "tamper-"+tc, tc, func(c Current) (Mutation, error) {
				switch tc {
				case "frame":
					c.Receipt.Execution.Frame.Attempt++
					c.Receipt.Execution.Frame.FrameHash, _ = RestoreResourceFrameHash(c.Receipt.Execution.Frame)
				case "uid":
					v := c.Receipt.Execution.Intents["rox7"]
					v.Object.UID = "replacement"
					c.Receipt.Execution.Intents["rox7"] = v
				case "erase":
					delete(c.Receipt.Execution.Intents, "rox7")
				case "startup":
					c.Receipt.Execution.BeforeStartup.FirmwareUUID = "22222222-2222-4222-8222-222222222222"
				}
				return Mutation{State: c.State, Receipt: c.Receipt}, nil
			})
			if !errors.Is(err, ErrConflict) {
				t.Fatal("accepted journal rewrite", tc, err)
			}
		})
	}
}
