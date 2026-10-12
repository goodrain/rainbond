package kubeidentity

import (
	"context"
	"errors"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	ktesting "k8s.io/client-go/testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
)

func bindingObjects() (*corev1.Service, *corev1.Pod, *corev1.PersistentVolumeClaim, *corev1.PersistentVolume) {
	service := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "rbd-hub", Namespace: "system", UID: "service-uid", ResourceVersion: "1"}, Spec: corev1.ServiceSpec{Selector: map[string]string{"app": "registry"}}}
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "hub", Namespace: "system", UID: "pod-uid", ResourceVersion: "1", Labels: map[string]string{"app": "registry"}}, Spec: corev1.PodSpec{NodeName: "node", Volumes: []corev1.Volume{{Name: "data", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "registry"}}}}, Containers: []corev1.Container{{Name: "registry", VolumeMounts: []corev1.VolumeMount{{Name: "data", MountPath: "/var/lib/registry", SubPath: "warehouse"}}}, {Name: "coordinator", VolumeMounts: []corev1.VolumeMount{{Name: "data", MountPath: "/registry", SubPath: "warehouse", ReadOnly: true}}}}}}
	pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "registry", Namespace: "system", UID: "claim-uid"}, Spec: corev1.PersistentVolumeClaimSpec{VolumeName: "pv"}, Status: corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound}}
	pv := &corev1.PersistentVolume{ObjectMeta: metav1.ObjectMeta{Name: "pv", UID: "volume-uid"}, Spec: corev1.PersistentVolumeSpec{ClaimRef: &corev1.ObjectReference{Name: "registry", Namespace: "system", UID: "claim-uid"}}, Status: corev1.PersistentVolumeStatus{Phase: corev1.VolumeBound}}
	return service, pod, pvc, pv
}

// capability_id: rainbond.cleanup.registry-kubernetes-binding
func TestRegistryMountBindingUsesRealVolumeAndPodIdentity(t *testing.T) {
	service, pod, pvc, pv := bindingObjects()
	client := fake.NewSimpleClientset(service, pod, pvc, pv)
	request := RegistryMountRequest{Namespace: "system", Service: "rbd-hub", Pod: "hub", PodUID: "pod-uid", RegistryContainer: "registry", CoordinatorContainer: "coordinator", RegistryRoot: "/var/lib/registry", CoordinatorRoot: "/registry"}
	first, err := InspectRegistryMount(context.Background(), client, request)
	if err != nil || first.VolumeUID == "" || first.RelativeRoot != "warehouse" {
		t.Fatal(first, err)
	}
	// Recreating a PV under the same name must produce a different identity.
	pv.UID = types.UID("recreated-volume")
	if _, err := client.CoreV1().PersistentVolumes().Update(context.Background(), pv, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	second, err := InspectRegistryMount(context.Background(), client, request)
	if err != nil || second.VolumeUID == first.VolumeUID {
		t.Fatal("PV replacement not detected", err)
	}
}
func TestRegistryMountBindingRejectsMismatches(t *testing.T) {
	for _, kind := range []string{"pod-uid", "selector", "subpath", "claim-uid", "writable-sidecar", "subpath-expression", "block-volume", "nested-mount"} {
		t.Run(kind, func(t *testing.T) {
			service, pod, pvc, pv := bindingObjects()
			request := RegistryMountRequest{Namespace: "system", Service: "rbd-hub", Pod: "hub", PodUID: "pod-uid", RegistryContainer: "registry", CoordinatorContainer: "coordinator", RegistryRoot: "/var/lib/registry", CoordinatorRoot: "/registry"}
			switch kind {
			case "pod-uid":
				request.PodUID = "old-pod"
			case "selector":
				pod.Labels["app"] = "another"
			case "subpath":
				pod.Spec.Containers[1].VolumeMounts[0].SubPath = "another"
			case "claim-uid":
				pv.Spec.ClaimRef.UID = "other-claim"
			case "writable-sidecar":
				pod.Spec.Containers[1].VolumeMounts[0].ReadOnly = false
			case "subpath-expression":
				pod.Spec.Containers[1].VolumeMounts[0].SubPathExpr = "$(POD_NAME)"
			case "block-volume":
				mode := corev1.PersistentVolumeBlock
				pvc.Spec.VolumeMode = &mode
			case "nested-mount":
				pod.Spec.Containers[1].VolumeMounts = append(pod.Spec.Containers[1].VolumeMounts, corev1.VolumeMount{Name: "other", MountPath: "/registry/docker", ReadOnly: true})
			}
			if _, err := InspectRegistryMount(context.Background(), fake.NewSimpleClientset(service, pod, pvc, pv), request); err == nil {
				t.Fatal("invalid mount accepted")
			}
		})
	}
}

func TestRegistryHostPathIdentityIncludesNodeUID(t *testing.T) {
	service, pod, _, _ := bindingObjects()
	pod.Spec.Volumes[0].VolumeSource = corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/grdata"}}
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node", UID: "first-node"}}
	client := fake.NewSimpleClientset(service, pod, node)
	request := RegistryMountRequest{Namespace: "system", Service: "rbd-hub", Pod: "hub", PodUID: "pod-uid", RegistryContainer: "registry", CoordinatorContainer: "coordinator", RegistryRoot: "/var/lib/registry", CoordinatorRoot: "/registry"}
	first, err := InspectRegistryMount(context.Background(), client, request)
	if err != nil {
		t.Fatal(err)
	}
	node.UID = "replacement-node"
	if _, err := client.CoreV1().Nodes().Update(context.Background(), node, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	second, err := InspectRegistryMount(context.Background(), client, request)
	if err != nil || first.VolumeUID == second.VolumeUID {
		t.Fatal("hostPath identity survived node replacement", err)
	}
}
func TestRegistryMountReadFailureNeverBecomesEmptyProof(t *testing.T) {
	service, pod, pvc, pv := bindingObjects()
	client := fake.NewSimpleClientset(service, pod, pvc, pv)
	client.PrependReactor("get", "persistentvolumes", func(ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("untrusted upstream detail")
	})
	request := RegistryMountRequest{Namespace: "system", Service: "rbd-hub", Pod: "hub", PodUID: "pod-uid", RegistryContainer: "registry", CoordinatorContainer: "coordinator", RegistryRoot: "/var/lib/registry", CoordinatorRoot: "/registry"}
	result, err := InspectRegistryMount(context.Background(), client, request)
	if err != ErrBinding || result.VolumeUID != "" {
		t.Fatal("failed lookup granted identity", result, err)
	}
}
