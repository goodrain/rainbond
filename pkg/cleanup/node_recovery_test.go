package cleanup

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/goodrain/rainbond/db/model"
)

// capability_id: rainbond.cleanup.node-recovery-binding
func TestNodeRecoveryIntentIsDurableAndCannotReplaceOriginal(t *testing.T) {
	database, _ := coordinationDB(t)
	storage, err := ProvisionManagedCacheStorage(database, "cache", "/cache/build")
	if err != nil {
		t.Fatal(err)
	}
	database.Model(&model.CleanupStorage{}).Where("storage_id = ?", storage.StorageID).Update("mode", "ready")
	intent := NodeJobIntent{Namespace: "system", NodeName: "node", NodeUID: "uid", Entry: "entry", Fingerprint: strings.Repeat("a", 64), SpecHash: strings.Repeat("b", 64)}
	request, err := ManagedNodeRequest(storage, "owner", "recover-task", "plan", intent)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AcquireOperation(database, request); err != nil {
		t.Fatal(err)
	}
	if _, _, err := PrepareNodeJob(database, request, intent); err != nil {
		t.Fatal(err)
	}
	parent, err := ReadNodeJobBinding(database, request)
	if err != nil {
		t.Fatal(err)
	}
	recovery := NodeRecoveryBinding{Namespace: "system", Name: "recovery", SpecHash: strings.Repeat("c", 64)}
	if _, _, err := PrepareNodeRecovery(database, request, recovery); err == nil {
		t.Fatal("ungranted parent recovered")
	}
	parent.JobUID = "job-uid"
	parent.PodName = "original"
	parent.PodUID = "pod-uid"
	parent.ContainerID = "containerd://original"
	parent.ImageID = "example.test/image@sha256:" + strings.Repeat("d", 64)
	raw, _ := json.Marshal(parent)
	database.Model(&model.CleanupOperation{}).Where("operation_id = ?", request.OperationID).Updates(map[string]interface{}{"node_execution_json": string(raw), "state": "executing"})
	recovery.Name = NodeRecoveryName(parent)
	saved, created, err := PrepareNodeRecovery(database, request, recovery)
	if err != nil || !created || saved.JobUID != "" {
		t.Fatal(saved, created, err)
	}
	if _, created, err := PrepareNodeRecovery(database, request, recovery); err != nil || created {
		t.Fatal("repeat creation granted", err)
	}
	if err := BindNodeRecovery(database, request, recovery, "recovery-uid"); err != nil {
		t.Fatal(err)
	}
	if err := BindNodeRecovery(database, request, recovery, "other-uid"); err == nil {
		t.Fatal("replacement recovery adopted")
	}
	changed := recovery
	changed.SpecHash = strings.Repeat("e", 64)
	if _, _, err := PrepareNodeRecovery(database, request, changed); err == nil {
		t.Fatal("changed recovery accepted")
	}
	final, err := ReadNodeJobBinding(database, request)
	if err != nil || final.PodUID != parent.PodUID || final.Recovery.JobUID != "recovery-uid" {
		t.Fatal(final, err)
	}
	state, err := InspectOperation(database, request)
	if err != nil || state != "executing" {
		t.Fatal("recovery intent released protection", state, err)
	}
}

// capability_id: rainbond.cleanup.node-recovery-submission
func TestRecoverySubmissionNeverRecreatesAfterLostResponse(t *testing.T) {
	database, _ := coordinationDB(t)
	storage, err := ProvisionManagedCacheStorage(database, "cache", "/cache/build")
	if err != nil {
		t.Fatal(err)
	}
	database.Model(&model.CleanupStorage{}).Where("storage_id = ?", storage.StorageID).Update("mode", "ready")
	intent := NodeJobIntent{Namespace: "system", NodeName: "node", NodeUID: "uid", Entry: "entry", Fingerprint: strings.Repeat("a", 64), SpecHash: strings.Repeat("b", 64)}
	request, err := ManagedNodeRequest(storage, "owner", "recover-create", "plan", intent)
	if err != nil {
		t.Fatal(err)
	}
	AcquireOperation(database, request)
	PrepareNodeJob(database, request, intent)
	parent, _ := ReadNodeJobBinding(database, request)
	parent.JobUID = "job-uid"
	parent.PodName = "original"
	parent.PodUID = "pod-uid"
	parent.ContainerID = "containerd://original"
	parent.ImageID = "example.test/image@sha256:" + strings.Repeat("d", 64)
	raw, _ := json.Marshal(parent)
	database.Model(&model.CleanupOperation{}).Where("operation_id = ?", request.OperationID).Updates(map[string]interface{}{"node_execution_json": string(raw), "state": "executing"})
	job := suspendedGCFixture()
	job.Name = NodeRecoveryName(parent)
	job.Spec.Template.Spec.NodeName = "node"
	yes, no := true, false
	zero, one := int32(0), int32(1)
	job.Spec.Suspend = &yes
	job.Spec.BackoffLimit = &zero
	job.Spec.Completions = &one
	job.Spec.Parallelism = &one
	job.Spec.Template.Spec.AutomountServiceAccountToken = &no
	c := &job.Spec.Template.Spec.Containers[0]
	c.Command = []string{"/app/node-cleanup"}
	c.Args = []string{"--recover", "--original-pod=original", "--original-pod-uid=pod-uid"}
	c.VolumeMounts = []corev1.VolumeMount{{Name: "node-state", MountPath: "/node-state"}, {Name: "node-control", MountPath: "/node-control", ReadOnly: true}}
	job.Spec.Template.Spec.Volumes = []corev1.Volume{{Name: "node-state", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "state"}}}, {Name: "node-control", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: "control"}}}}
	altered := job.DeepCopy()
	altered.Spec.Template.Spec.Containers[0].Args = append(altered.Spec.Template.Spec.Containers[0].Args, "--recover=false")
	if _, err := NodeRecoverySpecHash(altered, parent); err == nil {
		t.Fatal("native mode override accepted")
	}
	kube := fake.NewSimpleClientset()
	creates := 0
	jobs := gcTestJobClient{JobInterface: kube.BatchV1().Jobs("system")}
	jobs.create = func(ctx context.Context, input *batchv1.Job, options metav1.CreateOptions) (*batchv1.Job, error) {
		if len(options.DryRun) > 0 {
			return input.DeepCopy(), nil
		}
		creates++
		binding, err := ReadNodeJobBinding(database, request)
		if err != nil || binding.Recovery == nil || binding.Recovery.JobUID != "" {
			t.Fatal("create before durable intent", err)
		}
		saved := input.DeepCopy()
		saved.UID = "recovery-uid"
		saved.ResourceVersion = "1"
		if err := kube.Tracker().Create(batchv1.SchemeGroupVersion.WithResource("jobs"), saved, "system"); err != nil {
			t.Fatal(err)
		}
		return nil, errors.New("lost response")
	}
	if _, err := SubmitSuspendedNodeRecovery(context.Background(), database, jobs, request, job); !errors.Is(err, ErrCoordinationUncertain) {
		t.Fatal(err)
	}
	found, err := SubmitSuspendedNodeRecovery(context.Background(), database, jobs, request, job)
	if err != nil || found.UID != "recovery-uid" || creates != 1 {
		t.Fatal("create replayed", err, creates)
	}
	starter := &gcStartTestClient{JobInterface: kube.BatchV1().Jobs("system"), loseResponse: true}
	if _, err := StartNodeRecovery(context.Background(), database, starter, request); !errors.Is(err, ErrCoordinationUncertain) {
		t.Fatal("lost start response not uncertain", err)
	}
	if _, err := StartNodeRecovery(context.Background(), database, starter, request); err != nil || starter.patches != 1 {
		t.Fatal("start replayed", err, starter.patches)
	}
	kube.Tracker().Delete(batchv1.SchemeGroupVersion.WithResource("jobs"), "system", job.Name)
	if _, err := SubmitSuspendedNodeRecovery(context.Background(), database, jobs, request, job); !errors.Is(err, ErrCoordinationUncertain) || creates != 1 {
		t.Fatal("missing recovery recreated", err)
	}
}
