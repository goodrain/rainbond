package vmrestore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/goodrain/rainbond/pkg/vmrestore/copyio"
	"github.com/goodrain/rainbond/pkg/vmrestore/restoreplan"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
	kt "k8s.io/client-go/testing"
)

// This provider is a test boundary. It supplies explicit synthetic qualification
// and capacity and is never wired by the production factory.
type engineTestCapacity struct {
	capability restoreplan.Capability
	releases   int
}

func (p *engineTestCapacity) Reserve(_ context.Context, _ Scope, op string, _ RestoreResourcePlan, _ copyio.Report) (restoreplan.Capability, restoreplan.SpaceEvidence, error) {
	return p.capability, restoreplan.SpaceEvidence{StorageClassUID: p.capability.StorageClassUID, Source: "ceph-pool-and-host-verified", ObservedAt: time.Now(), ValidFor: time.Minute, PoolUsableBytes: 1 << 30, HostFreeBytes: 10 << 30, HostFloorBytes: 5 << 30, ReservationOperationID: op, ReservationHeld: true}, nil
}
func (*engineTestCapacity) Check(context.Context, Scope, string, restoreplan.Budget) error {
	return nil
}
func (p *engineTestCapacity) Release(context.Context, Scope, string) error { p.releases++; return nil }

type engineScenario struct {
	f       restoreCommitFixture
	e       *RestoreEngine
	frame   RestoreResourcePlan
	runtime *restoreFakeRuntime
	logs    *restoreFakeLogs
}

func newEngineScenario(t *testing.T) engineScenario {
	t.Helper()
	f, frame, resources := resourceFixture(t, func(_ restoreCommitFixture, x *RestoreExecution) {
		for i := range x.Frame.Disks {
			x.Frame.Disks[i].SnapshotContent.Name = fmt.Sprintf("content-%d", i)
			x.Frame.Disks[i].SnapshotContent.UID = fmt.Sprintf("content-uid-%d", i)
			x.Frame.Disks[i].TargetRequestBytes = x.Frame.Disks[i].ROXRequestBytes
		}
		x.Frame.FrameHash, _ = RestoreResourceFrameHash(x.Frame)
	})
	cap := &resources.Certification.Capability
	cap.RawCopies, cap.AllocationNumerator, cap.AllocationDenominator, cap.MetadataReserveBytes, cap.MaxLogicalBytes, cap.TargetUID, cap.TargetGID = 3, 2, 1, 1<<20, 1<<40, 107, 107
	identity, _ := snapshotDigest(testStartupPolicy("restored"))
	r := &restoreFakeRuntime{observation: RestoreRuntimeObservation{VM: f.intent.CurrentVM, NetworkHash: strings.Repeat("e", 64), IdentityHash: identity, Halted: true, WritersClear: true}}
	logs := &restoreFakeLogs{}
	e := &RestoreEngine{Store: f.store, Resources: resources, Runtime: r, Capacity: &engineTestCapacity{capability: *cap}, Capture: &RestoreCapture{Store: f.store, Core: resources.Core, Logs: logs}}
	e.Source = &engineTestPreparation{value: RestorePreparation{Intent: f.intent, Config: f.after, Frame: frame, NetworkHash: r.observation.NetworkHash, IdentityHash: identity, BeforeStartup: testStartupPolicy("platform"), SourceStartup: testStartupPolicy("restored")}}
	e.WireRestoreResourceGuard()
	core := resources.Core.(*kubefake.Clientset)
	dyn := resources.Dynamic.(*dynamicfake.FakeDynamicClient)
	dyn.PrependReactor("create", "*", func(a kt.Action) (bool, runtime.Object, error) {
		obj := a.(kt.CreateAction).GetObject().(*unstructured.Unstructured)
		if obj.GetUID() == "" {
			obj.SetUID(types.UID("created-" + obj.GetName()))
		}
		var typed runtime.Object
		switch a.GetResource().Resource {
		case "persistentvolumeclaims":
			typed = &corev1.PersistentVolumeClaim{}
		case "configmaps":
			typed = &corev1.ConfigMap{}
		case "jobs":
			typed = &batchv1.Job{}
		}
		if typed != nil {
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, typed); err != nil {
				t.Fatal(err)
			}
			if err := core.Tracker().Create(a.GetResource(), typed, a.GetNamespace()); err != nil {
				t.Fatal(err)
			}
		}
		return false, nil, nil
	})
	dyn.PrependReactor("delete", "*", func(a kt.Action) (bool, runtime.Object, error) {
		_ = core.Tracker().Delete(a.GetResource(), a.GetNamespace(), a.(kt.DeleteAction).GetName())
		return false, nil, nil
	})
	policy := corev1.PersistentVolumeReclaimDelete
	target := &storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: "writable", UID: "class-target"}, Provisioner: cap.Driver, ReclaimPolicy: &policy, Parameters: map[string]string{"mounter": "kernel", "clusterID": "TEST-CLUSTER", "fsName": "TEST-FS", "pool": "TEST-POOL"}}
	rox := target.DeepCopy()
	rox.Name = "rox"
	rox.UID = "class-rox"
	delete(rox.Parameters, "pool")
	rox.Parameters["backingSnapshot"] = "true"
	for _, sc := range []*storagev1.StorageClass{target, rox} {
		if _, err := core.StorageV1().StorageClasses().Create(context.Background(), sc, metav1.CreateOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	for _, d := range frame.Disks {
		snap := &unstructured.Unstructured{Object: map[string]interface{}{"apiVersion": "snapshot.storage.k8s.io/v1", "kind": "VolumeSnapshot", "metadata": map[string]interface{}{"name": d.Snapshot.Name, "namespace": f.scope.Namespace, "uid": d.Snapshot.UID}, "status": map[string]interface{}{"readyToUse": true, "boundVolumeSnapshotContentName": d.SnapshotContent.Name}}}
		content := &unstructured.Unstructured{Object: map[string]interface{}{"apiVersion": "snapshot.storage.k8s.io/v1", "kind": "VolumeSnapshotContent", "metadata": map[string]interface{}{"name": d.SnapshotContent.Name, "uid": d.SnapshotContent.UID}, "spec": map[string]interface{}{"driver": cap.Driver, "volumeSnapshotRef": map[string]interface{}{"name": d.Snapshot.Name, "namespace": f.scope.Namespace, "uid": d.Snapshot.UID}}}}
		for _, pair := range []struct {
			gvr schema.GroupVersionResource
			obj *unstructured.Unstructured
		}{{csiSnapshotResource, snap}, {csiSnapshotContentResource, content}} {
			if _, err := dyn.Resource(pair.gvr).Namespace(pair.obj.GetNamespace()).Create(context.Background(), pair.obj, metav1.CreateOptions{}); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := core.CoreV1().PersistentVolumeClaims(f.scope.Namespace).Create(context.Background(), &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: d.PreviousPVC.Name, Namespace: f.scope.Namespace, UID: types.UID(d.PreviousPVC.UID)}}, metav1.CreateOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	return engineScenario{f, e, frame, r, logs}
}

func (s engineScenario) command(t *testing.T, action string, command RestoreCommand) RestoreOperationDTO {
	t.Helper()
	c, err := s.f.store.Read(s.f.scope, s.f.expect.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	command.ExpectedControlRevision = c.State.ControlRevision
	command.ExpectedBindingGeneration = c.State.BindingGeneration
	status, err := s.e.Status(s.f.scope, s.f.expect.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	command.ExpectedOperationEpoch = status.OperationEpoch
	d, err := s.e.Command(context.Background(), s.f.scope, s.f.expect.OperationID, action, command)
	if err != nil {
		t.Fatal(action, err)
	}
	return d
}

func (s engineScenario) bind(t *testing.T, key string) {
	t.Helper()
	c, _ := s.f.store.Read(s.f.scope, s.f.expect.OperationID)
	ref := c.Receipt.Execution.Intents[key].Object
	pvc, err := s.e.Resources.Core.CoreV1().PersistentVolumeClaims(s.f.scope.Namespace).Get(context.Background(), ref.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	pvc.Spec.VolumeName = "pv-" + ref.Name
	pvc.Status.Phase = corev1.ClaimBound
	if _, err = s.e.Resources.Core.CoreV1().PersistentVolumeClaims(s.f.scope.Namespace).Update(context.Background(), pvc, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	pv := &corev1.PersistentVolume{ObjectMeta: metav1.ObjectMeta{Name: pvc.Spec.VolumeName, UID: types.UID("pv-uid-" + ref.Name)}, Spec: corev1.PersistentVolumeSpec{PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimDelete, ClaimRef: &corev1.ObjectReference{Name: ref.Name, Namespace: ref.Namespace, UID: types.UID(ref.UID)}, PersistentVolumeSource: corev1.PersistentVolumeSource{CSI: &corev1.CSIPersistentVolumeSource{Driver: s.e.Resources.Certification.Capability.Driver}}}}
	if _, err = s.e.Resources.Core.CoreV1().PersistentVolumes().Create(context.Background(), pv, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
}

func (s engineScenario) finishStage(t *testing.T, mode string) {
	t.Helper()
	s.command(t, "stage-plan", RestoreCommand{Mode: mode})
	s.command(t, "stage-job", RestoreCommand{Mode: mode})
	c, _ := s.f.store.Read(s.f.scope, s.f.expect.OperationID)
	plan, _, err := s.e.stagePlan(context.Background(), c, mode)
	if err != nil {
		t.Fatal(err)
	}
	jobRef := c.Receipt.Execution.Intents["job-"+mode].Object
	job, err := s.e.Resources.Core.BatchV1().Jobs(s.f.scope.Namespace).Get(context.Background(), jobRef.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	job.Status.Succeeded = 1
	job.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: corev1.ConditionTrue}}
	if _, err = s.e.Resources.Core.BatchV1().Jobs(s.f.scope.Namespace).UpdateStatus(context.Background(), job, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	report := copyio.Report{Protocol: copyio.Protocol, OperationID: plan.OperationID, PlanHash: plan.PlanHash, Mode: mode, OK: true, Phase: "complete", Disks: []copyio.DiskResult{}}
	for _, d := range plan.Disks {
		file := copyio.FileResult{LogicalBytes: 8, ReadBytes: 8, SHA256: strings.Repeat("1", 64), NonzeroChunkBytes: 8}
		result := copyio.DiskResult{VolumeID: d.VolumeID, SourcePVCUID: d.SourcePVCUID, PreviousPVCUID: d.PreviousPVCUID, TargetPVCUID: d.TargetPVCUID, Source: file, PreviousBefore: file}
		if mode != "inspect" {
			result.PreviousAfter = file
			result.Target = file
			result.Target.UID = 107
			result.Target.GID = 107
			result.Target.Mode = 0660
			result.Published = true
			if mode == "copy" {
				result.WrittenBytes = 8
			}
		}
		report.Disks = append(report.Disks, result)
	}
	payload, _ := json.Marshal(report)
	sum := sha256.Sum256(payload)
	summary, _ := json.Marshal(copyio.Summary{Protocol: copyio.Protocol, OperationID: plan.OperationID, PlanHash: plan.PlanHash, OK: true, ReportSHA256: hex.EncodeToString(sum[:]), ReportBytes: len(payload), DiskCount: len(report.Disks)})
	owner := true
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod-" + mode, Namespace: s.f.scope.Namespace, UID: types.UID("pod-uid-" + mode), Annotations: job.Spec.Template.Annotations, Labels: job.Spec.Template.Labels, OwnerReferences: []metav1.OwnerReference{{APIVersion: "batch/v1", Kind: "Job", Name: job.Name, UID: job.UID, Controller: &owner}}}, Spec: *job.Spec.Template.Spec.DeepCopy(), Status: corev1.PodStatus{Phase: corev1.PodSucceeded, ContainerStatuses: []corev1.ContainerStatus{{Name: "copy", ImageID: s.frame.RuntimeImageID, State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 0, Message: string(summary), StartedAt: metav1.NewTime(time.Unix(1700000000, 0)), FinishedAt: metav1.NewTime(time.Unix(1700000010, 0))}}}}}}
	pod.Labels["batch.kubernetes.io/controller-uid"] = string(job.UID)
	if _, err = s.e.Resources.Core.CoreV1().Pods(s.f.scope.Namespace).Create(context.Background(), pod, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	u, err := desiredObject(pod, "Pod")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.e.Resources.Dynamic.Resource(restoreGVR["Pod"]).Namespace(s.f.scope.Namespace).Create(context.Background(), u, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	s.command(t, "stage-pod", RestoreCommand{Mode: mode})
	s.logs.body = string(payload) + "\n"
	s.command(t, "stage-capture", RestoreCommand{Mode: mode})
}

func (s engineScenario) cleanup(t *testing.T, key string) {
	t.Helper()
	if err := s.e.CleanupObject(context.Background(), s.f.scope, s.f.expect.OperationID, key, false); !errors.Is(err, ErrRestorePending) {
		t.Fatal(key, err)
	}
	c, _ := s.f.store.Read(s.f.scope, s.f.expect.OperationID)
	v := c.Receipt.Execution.Intents[key]
	if v.BoundPV != nil {
		_ = s.e.Resources.Core.CoreV1().PersistentVolumes().Delete(context.Background(), v.BoundPV.Name, metav1.DeleteOptions{})
	}
	if err := s.e.CleanupObject(context.Background(), s.f.scope, s.f.expect.OperationID, key, false); err != nil {
		t.Fatal(key, err)
	}
}

// capability_id: rainbond.vm-restore.bounded-execution
func TestBoundedCommandsDriveActualResourcesCaptureAndAtomicCommit(t *testing.T) {
	s := newEngineScenario(t)
	s.command(t, "halt", RestoreCommand{})
	for _, d := range s.frame.Disks {
		s.command(t, "ensure-rox", RestoreCommand{VolumeID: d.VolumeID})
		key := "rox" + strconv.Itoa(int(d.VolumeID))
		s.bind(t, key)
		s.command(t, "pin-rox", RestoreCommand{VolumeID: d.VolumeID})
	}
	s.finishStage(t, "inspect")
	for _, key := range []string{"job-inspect", "pod-inspect", "plan-inspect"} {
		s.cleanup(t, key)
	}
	for _, d := range s.frame.Disks {
		s.command(t, "ensure-target", RestoreCommand{VolumeID: d.VolumeID})
		key := "target" + strconv.Itoa(int(d.VolumeID))
		s.bind(t, key)
		s.command(t, "pin-target", RestoreCommand{VolumeID: d.VolumeID})
	}
	s.finishStage(t, "copy")
	for _, key := range []string{"job-copy", "pod-copy", "plan-copy"} {
		s.cleanup(t, key)
	}
	s.finishStage(t, "verify")
	d := s.command(t, "commit", RestoreCommand{Config: &s.f.after})
	if !d.ConfigCommitted || d.Complete || d.ConfigCommitHash == "" {
		t.Fatal("commit did not remain distinct from completion", d)
	}
	c, _ := s.f.store.Read(s.f.scope, s.f.expect.OperationID)
	if len(c.State.Bindings) != 3 || c.State.Bindings["9"].Attached || c.State.Bindings["7"].ActivePVC.UID == s.f.old["7"].ActivePVC.UID || !c.Receipt.RetainOldPVCs {
		t.Fatal("atomic disk set lost A/B ownership")
	}
	s.command(t, "config-ack", RestoreCommand{CommitHash: d.ConfigCommitHash, DocumentHash: strings.Repeat("a", 64)})
	s.runtime.observation.BindingGeneration = c.State.BindingGeneration
	s.runtime.observation.DesiredMatches = true
	s.runtime.observation.ObservedGeneration = 1
	s.runtime.observation.VMGeneration = 1
	s.command(t, "sync-halted", RestoreCommand{})
	for _, key := range []string{"job-verify", "pod-verify", "plan-verify", "rox7", "rox8"} {
		s.cleanup(t, key)
	}
	s.runtime.observation.Running = true
	s.runtime.observation.Halted = false
	s.command(t, "resume", RestoreCommand{})
	d = s.command(t, "finalize", RestoreCommand{})
	if !d.Complete {
		t.Fatal("full acknowledged observed sequence did not finalize")
	}
	c, _ = s.f.store.Read(s.f.scope, s.f.expect.OperationID)
	if c.State.Fence != nil || !c.Receipt.SnapshotReferenceReleased {
		t.Fatal("complete cleanup did not release fence")
	}
}

// capability_id: rainbond.vm-restore.bounded-execution
func TestStagePodObservationCountsAllOwnerReferencesWithoutLabels(t *testing.T) {
	for _, count := range []int{1, 2} {
		t.Run(strconv.Itoa(count), func(t *testing.T) {
			s := newEngineScenario(t)
			s.command(t, "halt", RestoreCommand{})
			for _, d := range s.frame.Disks {
				s.command(t, "ensure-rox", RestoreCommand{VolumeID: d.VolumeID})
				s.bind(t, "rox"+strconv.Itoa(int(d.VolumeID)))
				s.command(t, "pin-rox", RestoreCommand{VolumeID: d.VolumeID})
			}
			s.command(t, "stage-plan", RestoreCommand{Mode: "inspect"})
			s.command(t, "stage-job", RestoreCommand{Mode: "inspect"})
			c, _ := s.f.store.Read(s.f.scope, s.f.expect.OperationID)
			ref := c.Receipt.Execution.Intents["job-inspect"].Object
			owner := true
			for i := 0; i < count; i++ {
				labels := map[string]string{}
				if count == 2 && i == 0 {
					labels["batch.kubernetes.io/controller-uid"] = ref.UID
				}
				pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("pod-%d", i), Namespace: s.f.scope.Namespace, UID: types.UID(fmt.Sprintf("pod-uid-%d", i)), Labels: labels, Annotations: map[string]string{RestoreFrameAnnotation: s.frame.FrameHash}, OwnerReferences: []metav1.OwnerReference{{Kind: "Job", Name: ref.Name, UID: types.UID(ref.UID), Controller: &owner}}}}
				if _, err := s.e.Resources.Core.CoreV1().Pods(s.f.scope.Namespace).Create(context.Background(), pod, metav1.CreateOptions{}); err != nil {
					t.Fatal(err)
				}
			}
			_, err := s.e.ObserveStagePod(context.Background(), s.f.scope, s.f.expect.OperationID, "inspect")
			if count == 1 && err != nil {
				t.Fatal("single unlabelled owner could not be observed", err)
			}
			if count == 2 && !errors.Is(err, ErrRestorePending) {
				t.Fatal("second unlabelled owned pod was hidden", err)
			}
		})
	}
}
