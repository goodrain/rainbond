package kubeidentity

import (
	"context"
	"errors"
	"strings"
	"testing"

	coordination "github.com/goodrain/rainbond/pkg/cleanup"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func gcExecutorObjects() (*corev1.Service, *batchv1.Job, *corev1.Pod, *corev1.PersistentVolumeClaim, *corev1.PersistentVolume, coordination.StorageRegistration) {
	service, _, pvc, pv := bindingObjects()
	yes, no := true, false
	image := "example.test/gc@sha256:" + strings.Repeat("a", 64)
	container := corev1.Container{Name: "gc", Image: image, Command: []string{"/registry-gc"}, VolumeMounts: []corev1.VolumeMount{{Name: "data", MountPath: "/var/lib/registry", SubPath: "warehouse"}}}
	spec := corev1.PodSpec{NodeName: "node", RestartPolicy: corev1.RestartPolicyNever, AutomountServiceAccountToken: &no, Containers: []corev1.Container{container}, Volumes: []corev1.Volume{{Name: "data", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: pvc.Name}}}}}
	job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: "owned-gc", Namespace: "system", UID: "job-uid", ResourceVersion: "1"}, Spec: batchv1.JobSpec{Suspend: &no, Template: corev1.PodTemplateSpec{Spec: spec}}}
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "owned-gc-pod", Namespace: "system", UID: "gc-pod-uid", ResourceVersion: "1", Labels: map[string]string{"job-name": job.Name}, OwnerReferences: []metav1.OwnerReference{{APIVersion: "batch/v1", Kind: "Job", Name: job.Name, UID: job.UID, Controller: &yes}}}, Spec: *spec.DeepCopy(), Status: corev1.PodStatus{Phase: corev1.PodRunning, ContainerStatuses: []corev1.ContainerStatus{{Name: "gc", ContainerID: "containerd://gc", ImageID: image, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}}}}}
	binding := coordination.StorageRegistration{StorageID: "store", Generation: "generation", RootPath: "/var/lib/registry", VolumeUID: volumeIdentity("pvc", "system", string(pvc.UID), string(pv.UID), "warehouse")}
	return service, job, pod, pvc, pv, binding
}

// capability_id: rainbond.cleanup.gc-executor-identity
func TestGCExecutorIdentityRejectsUntrustedPodAndStorage(t *testing.T) {
	for _, kind := range []string{"valid", "wrong-owner", "replaced-pod", "image", "command", "extra-container", "restarted", "not-running", "service-member", "replaced-pv", "subpath", "writable-token", "suspended-job", "extra-volume"} {
		t.Run(kind, func(t *testing.T) {
			service, job, pod, pvc, pv, binding := gcExecutorObjects()
			switch kind {
			case "wrong-owner":
				pod.OwnerReferences[0].UID = "other-job"
			case "replaced-pod":
				pod.UID = "replacement"
			case "image":
				pod.Status.ContainerStatuses[0].ImageID = "example.test/gc@sha256:" + strings.Repeat("b", 64)
			case "command":
				pod.Spec.Containers[0].Command = []string{"/bin/sh"}
			case "extra-container":
				pod.Spec.Containers = append(pod.Spec.Containers, corev1.Container{Name: "other"})
			case "restarted":
				pod.Status.ContainerStatuses[0].RestartCount = 1
			case "not-running":
				pod.Status.ContainerStatuses[0].State = corev1.ContainerState{}
			case "service-member":
				pod.Labels["app"] = "registry"
			case "replaced-pv":
				pv.UID = "replacement"
			case "subpath":
				pod.Spec.Containers[0].VolumeMounts[0].SubPath = "other"
			case "writable-token":
				yes := true
				pod.Spec.AutomountServiceAccountToken = &yes
			case "suspended-job":
				yes := true
				job.Spec.Suspend = &yes
			case "extra-volume":
				pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{Name: "other"})
			}
			client := fake.NewSimpleClientset(service, job, pod, pvc, pv)
			observed, err := InspectGCExecutor(context.Background(), client, service.Name, job, pod.Name, "gc-pod-uid", binding)
			if kind == "valid" {
				if err != nil || observed.PodUID != "gc-pod-uid" || observed.VolumeUID != binding.VolumeUID {
					t.Fatal(observed, err)
				}
			} else if err == nil {
				t.Fatal("unsafe GC executor accepted", kind)
			}
		})
	}
}

// capability_id: rainbond.cleanup.gc-executor-termination
func TestTerminatedGCExecutorRequiresActualOriginalContainerExit(t *testing.T) {
	service, job, pod, pvc, pv, binding := gcExecutorObjects()
	client := fake.NewSimpleClientset(service, job, pod, pvc, pv)
	if _, err := InspectTerminatedGCExecutor(context.Background(), client, service.Name, job, pod.Name, string(pod.UID), binding); !errors.Is(err, ErrExecutorRunning) {
		t.Fatal("running executor not reported as pending", err)
	}
	pod.Status.Phase = corev1.PodSucceeded
	if _, err := client.CoreV1().Pods(pod.Namespace).Update(context.Background(), pod, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := InspectTerminatedGCExecutor(context.Background(), client, service.Name, job, pod.Name, string(pod.UID), binding); err == nil {
		t.Fatal("Pod phase alone accepted")
	}
	pod.Status.ContainerStatuses[0].State = corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 0, FinishedAt: metav1.Now()}}
	if _, err := client.CoreV1().Pods(pod.Namespace).Update(context.Background(), pod, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := InspectTerminatedGCExecutor(context.Background(), client, service.Name, job, pod.Name, string(pod.UID), binding); err != nil {
		t.Fatal(err)
	}
	if _, err := InspectTerminatedGCExecutor(context.Background(), client, service.Name, job, pod.Name, "other-pod", binding); err == nil {
		t.Fatal("wrong Pod accepted")
	}
}
