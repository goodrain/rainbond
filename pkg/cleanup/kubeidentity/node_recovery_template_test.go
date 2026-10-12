package kubeidentity

import (
	"context"
	"strings"
	"testing"
	"time"

	coordination "github.com/goodrain/rainbond/pkg/cleanup"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// capability_id: rainbond.cleanup.node-recovery-template
func TestNodeRecoveryRequiresOriginalExitAndOmitsCacheVolume(t *testing.T) {
	_, job, pod, pvc, pv, storage := gcExecutorObjects()
	storage.RootPath = "/cache/build"
	zero, one := int32(0), int32(1)
	job.Spec.BackoffLimit = &zero
	job.Spec.Completions = &one
	job.Spec.Parallelism = &one
	c := &job.Spec.Template.Spec.Containers[0]
	c.Command = []string{"/app/node-cleanup"}
	c.VolumeMounts[0].MountPath = "/cache/build"
	c.VolumeMounts = append(c.VolumeMounts, corev1.VolumeMount{Name: "node-state", MountPath: "/node-state"}, corev1.VolumeMount{Name: "node-control", MountPath: "/node-control", ReadOnly: true})
	job.Spec.Template.Spec.Volumes = append(job.Spec.Template.Spec.Volumes, corev1.Volume{Name: "node-state", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "state"}}}, corev1.Volume{Name: "node-control", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: "control"}}})
	job.Spec.Template.Annotations = map[string]string{nodeJournalPVCUID: "state-uid", nodeJournalPVUID: "state-pv-uid"}
	pod.Spec = *job.Spec.Template.Spec.DeepCopy()
	intent := coordination.NodeJobIntent{Namespace: job.Namespace, Name: job.Name, NodeName: "node", NodeUID: "node-uid", Entry: "entry", Fingerprint: strings.Repeat("a", 64)}
	hash, err := coordination.NodeJobSpecHash(job, intent)
	if err != nil {
		t.Fatal(err)
	}
	intent.SpecHash = hash
	binding := coordination.NodeJobBinding{Protocol: 1, NodeJobIntent: intent, JobUID: string(job.UID), PodName: pod.Name, PodUID: string(pod.UID), ContainerID: pod.Status.ContainerStatuses[0].ContainerID, ImageID: c.Image}
	state := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "state", Namespace: "system", UID: "state-uid"}, Spec: corev1.PersistentVolumeClaimSpec{VolumeName: "state-pv"}, Status: corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound}}
	statePV := &corev1.PersistentVolume{ObjectMeta: metav1.ObjectMeta{Name: "state-pv", UID: "state-pv-uid"}, Spec: corev1.PersistentVolumeSpec{ClaimRef: &corev1.ObjectReference{Name: state.Name, Namespace: state.Namespace, UID: state.UID}}}
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node", UID: "node-uid"}}
	client := fake.NewSimpleClientset(job, pod, pvc, pv, state, statePV, node)
	if _, err := BuildNodeRecoveryJob(context.Background(), client, job, storage, binding); err == nil {
		t.Fatal("live native executor accepted")
	}
	pod.Status.Phase = corev1.PodFailed
	pod.Status.ContainerStatuses[0].State = corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{FinishedAt: metav1.NewTime(time.Now()), ExitCode: 1}}
	client.CoreV1().Pods("system").Update(context.Background(), pod, metav1.UpdateOptions{})
	recovery, err := BuildNodeRecoveryJob(context.Background(), client, job, storage, binding)
	if err != nil {
		t.Fatal(err)
	}
	if recovery.UID != "" || !*recovery.Spec.Suspend || len(recovery.Spec.Template.Spec.Volumes) != 2 {
		t.Fatal("unsafe recovery template")
	}
	rc := recovery.Spec.Template.Spec.Containers[0]
	if !strings.Contains(strings.Join(rc.Args, " "), "--recover --original-pod="+pod.Name+" --original-pod-uid="+string(pod.UID)) {
		t.Fatal("original identity missing")
	}
	for _, mount := range rc.VolumeMounts {
		if mount.MountPath != "/node-state" && mount.MountPath != "/node-control" {
			t.Fatal("recovery can access business cache")
		}
	}
	if len(job.Spec.Template.Spec.Volumes) != 3 {
		t.Fatal("original mutated")
	}
	state.UID = "replacement"
	client.CoreV1().PersistentVolumeClaims("system").Update(context.Background(), state, metav1.UpdateOptions{})
	if _, err := BuildNodeRecoveryJob(context.Background(), client, job, storage, binding); err == nil {
		t.Fatal("replacement journal accepted")
	}
}
