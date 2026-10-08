package kubeidentity

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
)

func writerCoverageFixture() *fake.Clientset {
	objects := []runtime.Object{}
	yes, one := true, int32(1)
	for _, name := range []string{"rbd-api", "rbd-worker", "rbd-chaos", "rbd-app-ui"} {
		meta := metav1.ObjectMeta{Name: name, Namespace: "system", UID: types.UID(name), ResourceVersion: "1", Generation: 1}
		template := corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"name": name}}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: name, Image: "example/" + name + ":new"}}}}
		owner := metav1.OwnerReference{APIVersion: "apps/v1", Kind: "DaemonSet", Name: name, UID: meta.UID, Controller: &yes}
		selector := &metav1.LabelSelector{MatchLabels: map[string]string{"name": name}}
		if name == "rbd-chaos" {
			objects = append(objects, &appsv1.DaemonSet{ObjectMeta: meta, Spec: appsv1.DaemonSetSpec{Selector: selector, Template: template}, Status: appsv1.DaemonSetStatus{ObservedGeneration: 1, DesiredNumberScheduled: 1, CurrentNumberScheduled: 1, UpdatedNumberScheduled: 1, NumberReady: 1, NumberAvailable: 1}})
		} else {
			deployment := &appsv1.Deployment{ObjectMeta: meta, Spec: appsv1.DeploymentSpec{Replicas: &one, Selector: selector, Template: template}, Status: appsv1.DeploymentStatus{ObservedGeneration: 1, Replicas: 1, UpdatedReplicas: 1, ReadyReplicas: 1, AvailableReplicas: 1}}
			owner.Kind = "Deployment"
			rsmeta := metav1.ObjectMeta{Name: name + "-rs", Namespace: "system", UID: types.UID(name + "-rs"), ResourceVersion: "1", OwnerReferences: []metav1.OwnerReference{owner}}
			objects = append(objects, deployment, &appsv1.ReplicaSet{ObjectMeta: rsmeta, Spec: appsv1.ReplicaSetSpec{Replicas: &one, Selector: selector, Template: template}})
			owner.Kind = "ReplicaSet"
			owner.Name = rsmeta.Name
			owner.UID = rsmeta.UID
		}
		podmeta := metav1.ObjectMeta{Name: name + "-pod", Namespace: "system", UID: types.UID(name + "-pod"), ResourceVersion: "1", Labels: template.Labels, OwnerReferences: []metav1.OwnerReference{owner}}
		objects = append(objects, &corev1.Pod{ObjectMeta: podmeta, Spec: template.Spec, Status: corev1.PodStatus{Phase: corev1.PodRunning, ContainerStatuses: []corev1.ContainerStatus{{Name: name, Ready: true, ContainerID: "containerd://" + name, ImageID: "sha256:" + name, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}}}}})
	}
	return fake.NewSimpleClientset(objects...)
}

// capability_id: rainbond.cleanup.reference-writer-controller-coverage
func TestReferenceWriterCoverageRejectsIncompleteRollout(t *testing.T) {
	ctx := context.Background()
	for _, scenario := range []string{"complete", "old-image", "missing-role", "unready", "desired-replica-missing", "controller-changed", "new-pod"} {
		t.Run(scenario, func(t *testing.T) {
			client := writerCoverageFixture()
			pod, _ := client.CoreV1().Pods("system").Get(ctx, "rbd-api-pod", metav1.GetOptions{})
			switch scenario {
			case "old-image":
				pod.Spec.Containers[0].Image = "example/old"
				client.CoreV1().Pods("system").Update(ctx, pod, metav1.UpdateOptions{})
			case "missing-role":
				client.CoreV1().Pods("system").Delete(ctx, pod.Name, metav1.DeleteOptions{})
			case "unready":
				pod.Status.ContainerStatuses[0].Ready = false
				client.CoreV1().Pods("system").Update(ctx, pod, metav1.UpdateOptions{})
			case "desired-replica-missing":
				d, _ := client.AppsV1().Deployments("system").Get(ctx, "rbd-api", metav1.GetOptions{})
				*d.Spec.Replicas = 2
				client.AppsV1().Deployments("system").Update(ctx, d, metav1.UpdateOptions{})
			case "controller-changed":
				calls := 0
				client.PrependReactor("get", "deployments", func(a ktesting.Action) (bool, runtime.Object, error) {
					if a.(ktesting.GetAction).GetName() != "rbd-api" {
						return false, nil, nil
					}
					calls++
					if calls == 2 {
						d, _ := client.Tracker().Get(appsv1.SchemeGroupVersion.WithResource("deployments"), "system", "rbd-api")
						copy := d.(*appsv1.Deployment).DeepCopy()
						copy.ResourceVersion = "2"
						return true, copy, nil
					}
					return false, nil, nil
				})
			case "new-pod":
				calls := 0
				client.PrependReactor("list", "pods", func(a ktesting.Action) (bool, runtime.Object, error) {
					calls++
					if calls == 2 {
						extra := pod.DeepCopy()
						extra.Name = "new-api"
						extra.UID = "new-api"
						client.Tracker().Create(corev1.SchemeGroupVersion.WithResource("pods"), extra, "system")
					}
					return false, nil, nil
				})
			}
			writers, err := InspectReferenceWriterCoverage(ctx, client, "system")
			if scenario == "complete" {
				if err != nil || len(writers) != 4 {
					t.Fatal(writers, err)
				}
			} else if err == nil {
				t.Fatal("incomplete runtime accepted", writers)
			}
		})
	}
}
