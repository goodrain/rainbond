package kubeidentity

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	coordination "github.com/goodrain/rainbond/pkg/cleanup"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
)

// capability_id: rainbond.cleanup.node-inventory-report
func TestInventoryReportRequiresOriginalSuccessfulPod(t *testing.T) {
	_, _, pvc, pv := bindingObjects()
	no, yes := false, true
	image := "example.test/plugin@sha256:" + strings.Repeat("b", 64)
	job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: "inventory", Namespace: "system", UID: "job-uid", ResourceVersion: "1"}, Spec: batchv1.JobSpec{Suspend: &no, Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{NodeName: "node", RestartPolicy: corev1.RestartPolicyNever, AutomountServiceAccountToken: &no, Containers: []corev1.Container{{Name: "node-inventory", Image: image, Command: []string{"/app/node-inventory"}, Args: []string{"--output=-"}, VolumeMounts: []corev1.VolumeMount{{Name: "cache", MountPath: "/cache/build", SubPath: "owned", ReadOnly: true}}}}, Volumes: []corev1.Volume{{Name: "cache", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: pvc.Name, ReadOnly: true}}}}}}}, Status: batchv1.JobStatus{Succeeded: 1}}
	now := time.Now().UTC()
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "collector", Labels: map[string]string{"job-name": job.Name}, Namespace: "system", UID: "pod-uid", ResourceVersion: "1", OwnerReferences: []metav1.OwnerReference{{APIVersion: "batch/v1", Kind: "Job", Name: job.Name, UID: job.UID, Controller: &yes}}}, Spec: *job.Spec.Template.Spec.DeepCopy(), Status: corev1.PodStatus{Phase: corev1.PodSucceeded, ContainerStatuses: []corev1.ContainerStatus{{Name: "node-inventory", ImageID: image, ContainerID: "containerd://owned", State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 0, StartedAt: metav1.NewTime(now.Add(-time.Minute)), FinishedAt: metav1.NewTime(now)}}}}}}
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node", UID: "node-uid"}}
	client := fake.NewSimpleClientset(job, pod, node, pvc, pv)
	client.PrependReactor("list", "pods", func(ktesting.Action) (bool, runtime.Object, error) {
		return true, &corev1.PodList{ListMeta: metav1.ListMeta{ResourceVersion: "1"}, Items: []corev1.Pod{*pod}}, nil
	})
	binding := coordination.StorageRegistration{VolumeUID: volumeIdentity("pvc", "system", string(pvc.UID), string(pv.UID), "owned"), RootPath: "/cache/build"}
	settings := NodeInventorySettings{Region: "rainbond", ScanID: "scan"}
	payload := `{"version":1,"region":"rainbond","node":"node","nodeUid":"node-uid","scanId":"scan","observedAt":"` + now.Format(time.RFC3339Nano) + `","resources":[],"sources":[]}`
	reads := 0
	read := func(context.Context, string, string, string) (io.ReadCloser, error) {
		reads++
		return io.NopCloser(strings.NewReader(payload)), nil
	}
	if data, err := readNodeInventoryReport(context.Background(), client, job, binding, settings, read); err != nil || len(data) == 0 {
		t.Fatal("valid report rejected", err)
	}
	pod.Status.ContainerStatuses[0].ImageID = "containerd://mirror.example/plugin@sha256:" + strings.Repeat("b", 64)
	if data, err := readNodeInventoryReport(context.Background(), client, job, binding, settings, read); err != nil || len(data) == 0 {
		t.Fatal("digest-equivalent image alias rejected", err)
	}
	pod.Status.ContainerStatuses[0].ImageID = "containerd://mirror.example/plugin@sha256:" + strings.Repeat("c", 64)
	before := reads
	if _, err := readNodeInventoryReport(context.Background(), client, job, binding, settings, read); err == nil || reads != before {
		t.Fatal("different image digest accepted")
	}
	pod.Status.ContainerStatuses[0].ImageID = "containerd://mirror.example/plugin@sha256:" + strings.Repeat("b", 64)
	payload = strings.ReplaceAll(payload, `"scan"`, `"old-scan"`)
	if _, err := readNodeInventoryReport(context.Background(), client, job, binding, settings, read); err == nil {
		t.Fatal("stale report accepted")
	}
	pod.OwnerReferences[0].UID = "foreign-job"
	before = reads
	if _, err := readNodeInventoryReport(context.Background(), client, job, binding, settings, read); err == nil || reads != before {
		t.Fatal("foreign logs read")
	}
}
