package kubeidentity

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	coordination "github.com/goodrain/rainbond/pkg/cleanup"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
)

func gcTemplateFixture(t *testing.T) (*fake.Clientset, coordination.StorageRegistration) {
	t.Helper()
	svc, pod, pvc, pv := bindingObjects()
	svc.Spec.Ports = []corev1.ServicePort{{Port: 5000, TargetPort: intstr.FromInt(5001)}}
	pod.Annotations = map[string]string{"rainbond.io/registry-gc-executor": "v1"}
	pod.Spec.Containers[0].Env = []corev1.EnvVar{{Name: "REGISTRY_HTTP_ADDR", Value: "127.0.0.1:5000"}, {Name: "REGISTRY_STORAGE_FILESYSTEM_ROOTDIRECTORY", Value: "/var/lib/registry"}, {Name: "REGISTRY_STORAGE_MAINTENANCE_UPLOADPURGING_ENABLED", Value: "false"}}
	binding := coordination.StorageRegistration{StorageID: "store", Generation: "one", RootPath: "/var/lib/registry", VolumeUID: volumeIdentity("pvc", "system", string(pvc.UID), string(pv.UID), "warehouse")}
	sidecar := &pod.Spec.Containers[1]
	sidecar.Image = "example.test/coordinator@sha256:" + strings.Repeat("a", 64)
	sidecar.Command = []string{"/registry-coordinator"}
	sidecar.Args = []string{"--listen=:5001", "--upstream=http://127.0.0.1:5000", "--storage-id=store", "--storage-generation=one", "--volume-uid=" + binding.VolumeUID, "--registry-path=/var/lib/registry", "--storage-root=/registry", "--coordination-api=http://rbd-api.system:8443", "--allow-internal-http=true", "--credential-file=/control/token"}
	sidecar.VolumeMounts = append(sidecar.VolumeMounts, corev1.VolumeMount{Name: "control", MountPath: "/control", ReadOnly: true})
	pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{Name: "control", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: "coordination"}}})
	pod.Status.ContainerStatuses = []corev1.ContainerStatus{{Name: "coordinator", ContainerID: "containerd://coordinator", ImageID: sidecar.Image, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}}, {Name: "registry", ImageID: "example.test/native@sha256:" + strings.Repeat("b", 64), State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}}}
	client := fake.NewSimpleClientset(svc, pod, pvc, pv, gcCleanerFixture())
	client.PrependReactor("list", "pods", func(action ktesting.Action) (bool, runtime.Object, error) {
		value, err := client.Tracker().List(corev1.SchemeGroupVersion.WithResource("pods"), corev1.SchemeGroupVersion.WithKind("Pod"), action.GetNamespace())
		if err == nil {
			list := value.(*corev1.PodList)
			selected := action.(ktesting.ListAction).GetListRestrictions().Labels
			filtered := []corev1.Pod{}
			for _, item := range list.Items {
				if selected.Matches(labels.Set(item.Labels)) {
					filtered = append(filtered, item)
				}
			}
			list.Items = filtered
			list.ResourceVersion = "snapshot"
		}
		return true, value, err
	})
	return client, binding
}

// capability_id: rainbond.cleanup.gc-job-template
func TestGCJobTemplateUsesObservedStorageAndMountedCredentials(t *testing.T) {
	client, binding := gcTemplateFixture(t)
	r := coordination.CoordinationRequest{StorageID: "store", Generation: "one", OperationID: "gc", Owner: "manual", Kind: "gc", Scope: "*", Fingerprint: "selection"}
	job, err := BuildRegistryGCJob(context.Background(), client, "system", "rbd-hub", binding, r)
	if err != nil {
		t.Fatal(err)
	}
	if job.Spec.Suspend == nil || !*job.Spec.Suspend || job.Spec.Template.Spec.NodeName != "node" {
		t.Fatal("job starts without binding or wrong node")
	}
	c := job.Spec.Template.Spec.Containers[0]
	if c.Name != "gc" || c.Command[0] != "/registry-gc" || c.VolumeMounts[0].SubPath != "warehouse" || c.VolumeMounts[0].MountPath != binding.RootPath {
		t.Fatal("wrong data mapping")
	}
	if c.Env[0].Name != "CLEANUP_GC_OPERATION" {
		t.Fatal("missing immutable operation descriptor")
	}
	var descriptor struct {
		Binding coordination.StorageRegistration `json:"binding"`
		Request coordination.CoordinationRequest `json:"request"`
	}
	if json.Unmarshal([]byte(c.Env[0].Value), &descriptor) != nil || descriptor.Binding != binding || descriptor.Request.OperationID != "gc" {
		t.Fatal("wrong task descriptor")
	}
	for _, action := range client.Actions() {
		if action.GetResource().Resource == "secrets" {
			t.Fatal("read credential contents")
		}
	}
}

func TestGCJobTemplateRejectsUnsafeSource(t *testing.T) {
	for _, kind := range []string{"background-writer", "missing-capability", "mutable-image", "wrong-volume", "credential-hostpath", "wrong-binding", "mixed-image"} {
		t.Run(kind, func(t *testing.T) {
			client, binding := gcTemplateFixture(t)
			pod, err := client.CoreV1().Pods("system").Get(context.Background(), "hub", metav1.GetOptions{})
			if err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "background-writer":
				pod.Spec.Containers[0].Env[2].Value = "true"
			case "missing-capability":
				pod.Annotations = nil
			case "mutable-image":
				pod.Spec.Containers[1].Image = "example.test/coordinator:latest"
			case "wrong-volume":
				binding.VolumeUID = "other"
			case "credential-hostpath":
				pod.Spec.Volumes[1].VolumeSource = corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/etc"}}
			case "mixed-image":
				other := pod.DeepCopy()
				other.Name = "hub-other"
				other.UID = "other-pod"
				other.Spec.Containers[1].Image = "example.test/coordinator@sha256:" + strings.Repeat("b", 64)
				other.Status.ContainerStatuses[0].ImageID = other.Spec.Containers[1].Image
				if _, err := client.CoreV1().Pods("system").Create(context.Background(), other, metav1.CreateOptions{}); err != nil {
					t.Fatal(err)
				}
			case "wrong-binding":
				pod.Spec.Containers[1].Args[2] = "--storage-id=other"
			}
			if _, err := client.CoreV1().Pods("system").Update(context.Background(), pod, metav1.UpdateOptions{}); err != nil {
				t.Fatal(err)
			}
			r := coordination.CoordinationRequest{StorageID: "store", Generation: "one", OperationID: "gc", Owner: "manual", Kind: "gc", Scope: "*", Fingerprint: "selection"}
			if _, err := BuildRegistryGCJob(context.Background(), client, "system", "rbd-hub", binding, r); err == nil {
				t.Fatal("unsafe source accepted")
			}
		})
	}
}

// capability_id: rainbond.cleanup.gc-executor-termination
func TestGCSourceFingerprintDetectsRuntimeAndConfigurationChanges(t *testing.T) {
	client, binding := gcTemplateFixture(t)
	r := coordination.CoordinationRequest{StorageID: "store", Generation: "one", OperationID: "gc", Owner: "manual", Kind: "gc", Scope: "*", Fingerprint: "selection"}
	first, err := BuildRegistryGCJob(context.Background(), client, "system", "rbd-hub", binding, r)
	if err != nil {
		t.Fatal(err)
	}
	fingerprint := first.Spec.Template.Annotations["rainbond.io/gc-source-fingerprint"]
	if len(fingerprint) != 64 {
		t.Fatal("source identity not recorded")
	}
	pod, err := client.CoreV1().Pods("system").Get(context.Background(), "hub", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	pod.Spec.Containers[0].Env = append(pod.Spec.Containers[0].Env, corev1.EnvVar{Name: "REGISTRY_STORAGE_MAINTENANCE_READONLY_ENABLED", Value: "true"})
	if _, err := client.CoreV1().Pods("system").Update(context.Background(), pod, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	changed, err := BuildRegistryGCJob(context.Background(), client, "system", "rbd-hub", binding, r)
	if err != nil || changed.Spec.Template.Annotations["rainbond.io/gc-source-fingerprint"] == fingerprint {
		t.Fatal("changed native config not detected", err)
	}
}

func gcCleanerFixture() *corev1.Pod {
	image := "example.test/chaos@sha256:" + strings.Repeat("c", 64)
	return &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "chaos", Namespace: "system", UID: "chaos-uid", ResourceVersion: "1", Labels: map[string]string{"name": "rbd-chaos"}}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "chaos", Image: image, Command: []string{"/run/rainbond-chaos"}, Args: []string{"--clean-up=false"}}}}, Status: corev1.PodStatus{Phase: corev1.PodRunning, ContainerStatuses: []corev1.ContainerStatus{{Name: "chaos", ImageID: image, ContainerID: "containerd://chaos", State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}}}}}
}

// capability_id: rainbond.cleanup.gc-reject-legacy-cleaner
func TestGCJobRejectsLegacyAutomaticCleanup(t *testing.T) {
	for _, args := range [][]string{nil, {"--clean-up=true"}, {"--clean-up=false", "--clean-up=true"}, {"--", "--clean-up=false"}} {
		client, binding := gcTemplateFixture(t)
		pod := gcCleanerFixture()
		pod.Spec.Containers[0].Args = args
		client.CoreV1().Pods("system").Update(context.Background(), pod, metav1.UpdateOptions{})
		r := coordination.CoordinationRequest{StorageID: "store", Generation: "one", OperationID: "gc", Owner: "manual", Kind: "gc", Scope: "*", Fingerprint: "selection"}
		if _, err := BuildRegistryGCJob(context.Background(), client, "system", "rbd-hub", binding, r); err == nil {
			t.Fatal("legacy cleaner allowed", args)
		}
	}
}

func TestGCSourceTracksCleanerProcessButNotStatusHeartbeat(t *testing.T) {
	client, binding := gcTemplateFixture(t)
	r := coordination.CoordinationRequest{StorageID: "store", Generation: "one", OperationID: "gc", Owner: "manual", Kind: "gc", Scope: "*", Fingerprint: "selection"}
	original, err := BuildRegistryGCJob(context.Background(), client, "system", "rbd-hub", binding, r)
	if err != nil {
		t.Fatal(err)
	}
	pod := gcCleanerFixture()
	pod.ResourceVersion = "2"
	client.CoreV1().Pods("system").Update(context.Background(), pod, metav1.UpdateOptions{})
	current, err := BuildRegistryGCJob(context.Background(), client, "system", "rbd-hub", binding, r)
	if err != nil || !SameRegistryGCSource(original, current) {
		t.Fatal("status-only change invalidated source", err)
	}
	pod.Status.ContainerStatuses[0].ContainerID = "containerd://replacement"
	client.CoreV1().Pods("system").Update(context.Background(), pod, metav1.UpdateOptions{})
	current, err = BuildRegistryGCJob(context.Background(), client, "system", "rbd-hub", binding, r)
	if err != nil || SameRegistryGCSource(original, current) {
		t.Fatal("cleaner process replacement ignored", err)
	}
	client.CoreV1().Pods("system").Delete(context.Background(), pod.Name, metav1.DeleteOptions{})
	if _, err := BuildRegistryGCJob(context.Background(), client, "system", "rbd-hub", binding, r); err == nil {
		t.Fatal("missing cleaner treated as disabled")
	}
}
