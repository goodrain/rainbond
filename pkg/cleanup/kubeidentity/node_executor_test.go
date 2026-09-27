package kubeidentity

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	coordination "github.com/goodrain/rainbond/pkg/cleanup"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// capability_id: rainbond.cleanup.node-executor-admission
func TestNodeExecutorRequiresOriginalRuntimeNodeAndMount(t *testing.T) {
	for _, mutation := range []string{"valid", "node", "owner", "mount", "runtime", "restarted", "volume", "command"} {
		t.Run(mutation, func(t *testing.T) {
			_, job, pod, pvc, pv, storage := gcExecutorObjects()
			storage.RootPath = "/cache/build"
			job.Spec.Template.Spec.Containers[0].Command = []string{"/app/node-cleanup"}
			job.Spec.Template.Spec.Containers[0].VolumeMounts[0].MountPath = storage.RootPath
			zero, one := int32(0), int32(1)
			job.Spec.BackoffLimit = &zero
			job.Spec.Completions = &one
			job.Spec.Parallelism = &one
			pod.Spec = *job.Spec.Template.Spec.DeepCopy()
			intent := coordination.NodeJobIntent{Namespace: job.Namespace, Name: job.Name, NodeName: "node", NodeUID: "node-uid", Entry: "entry", Fingerprint: strings.Repeat("b", 64)}
			hash, err := coordination.NodeJobSpecHash(job, intent)
			if err != nil {
				t.Fatal(err)
			}
			intent.SpecHash = hash
			binding := coordination.NodeJobBinding{Protocol: 1, NodeJobIntent: intent, JobUID: string(job.UID)}
			node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node", UID: "node-uid"}}
			switch mutation {
			case "node":
				node.UID = "replacement"
			case "owner":
				pod.OwnerReferences[0].UID = "other"
			case "mount":
				pod.Spec.Containers[0].VolumeMounts[0].SubPath = "other"
			case "runtime":
				pod.Status.ContainerStatuses[0].ImageID = "other"
			case "restarted":
				pod.Status.ContainerStatuses[0].RestartCount = 1
			case "volume":
				pv.UID = "replacement"
			case "command":
				pod.Spec.Containers[0].Command = []string{"/bin/sh"}
			}
			client := fake.NewSimpleClientset(job, pod, pvc, pv, node)
			observed, err := InspectNodeExecutor(context.Background(), client, job, pod.Name, string(pod.UID), storage, binding)
			if mutation == "valid" {
				if err != nil || observed.NodeUID != "node-uid" || observed.VolumeUID != storage.VolumeUID {
					t.Fatal(observed, err)
				}

				binding.PodName = observed.PodName
				binding.PodUID = observed.PodUID
				binding.ContainerID = observed.ContainerID
				binding.ImageID = observed.ImageID
				if _, err := InspectTerminatedNodeExecutor(context.Background(), client, job, pod.Name, string(pod.UID), storage, binding); !errors.Is(err, ErrExecutorRunning) {
					t.Fatal("running executor accepted as terminated", err)
				}
				pod.Status.Phase = corev1.PodSucceeded
				pod.Status.ContainerStatuses[0].State = corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{FinishedAt: metav1.NewTime(time.Now()), ExitCode: 0}}
				if _, err := client.CoreV1().Pods(pod.Namespace).Update(context.Background(), pod, metav1.UpdateOptions{}); err != nil {
					t.Fatal(err)
				}
				if _, err := InspectTerminatedNodeExecutor(context.Background(), client, job, pod.Name, string(pod.UID), storage, binding); err != nil {
					t.Fatal("original terminated executor rejected", err)
				}
				pod.Status.ContainerStatuses[0].ContainerID = "containerd://replacement"
				client.CoreV1().Pods(pod.Namespace).Update(context.Background(), pod, metav1.UpdateOptions{})
				if _, err := InspectTerminatedNodeExecutor(context.Background(), client, job, pod.Name, string(pod.UID), storage, binding); err == nil {
					t.Fatal("replacement runtime accepted on completion")
				}
			} else if err == nil {
				t.Fatal("untrusted executor accepted")
			}
		})
	}
}
