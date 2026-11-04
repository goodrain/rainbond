package job

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/eapache/channels"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// capability_id: rainbond.builder.build-pod-resources
func TestApplyBuildPodResourcesFromEnv(t *testing.T) {
	for _, tc := range []struct {
		name     string
		cpu      string
		memory   string
		existing bool
	}{
		{name: "unset"},
		{name: "whitespace", cpu: " \t", memory: " "},
		{name: "cpu cores", cpu: "2"},
		{name: "cpu millicores", cpu: "500m"},
		{name: "memory Gi", memory: "4Gi"},
		{name: "memory Mi", memory: "512Mi"},
		{name: "both", cpu: "2", memory: "4Gi"},
		{name: "trim whitespace", cpu: " 250m ", memory: " 1Gi\t"},
		{name: "preserve existing", existing: true},
		{name: "override cpu only", cpu: "1500m", existing: true},
		{name: "override memory only", memory: "2Gi", existing: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("BUILD_POD_CPU", tc.cpu)
			t.Setenv("BUILD_POD_MEMORY", tc.memory)
			pod := &corev1.Pod{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "build"}, {Name: "another-build-container"}}}}
			if tc.existing {
				for i := range pod.Spec.Containers {
					pod.Spec.Containers[i].Resources = corev1.ResourceRequirements{
						Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m"), corev1.ResourceMemory: resource.MustParse("128Mi")},
						Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1"), corev1.ResourceMemory: resource.MustParse("256Mi"), "example.com/gpu": resource.MustParse("1")},
					}
				}
			}
			want := pod.DeepCopy()
			for i := range want.Spec.Containers {
				for name, value := range map[corev1.ResourceName]string{corev1.ResourceCPU: tc.cpu, corev1.ResourceMemory: tc.memory} {
					if strings.TrimSpace(value) == "" {
						continue
					}
					r := &want.Spec.Containers[i].Resources
					if r.Requests == nil {
						r.Requests = corev1.ResourceList{}
					}
					if r.Limits == nil {
						r.Limits = corev1.ResourceList{}
					}
					r.Requests[name] = resource.MustParse(strings.TrimSpace(value))
					r.Limits[name] = resource.MustParse(strings.TrimSpace(value))
				}
			}
			if err := applyBuildPodResources(pod); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(pod, want) {
				t.Fatalf("pod resources = %#v, want %#v", pod.Spec.Containers, want.Spec.Containers)
			}
		})
	}
}

// capability_id: rainbond.builder.build-pod-resources
func TestApplyBuildPodResourcesRejectsInvalidConfig(t *testing.T) {
	for _, tc := range []struct {
		name, cpu, memory, env string
	}{
		{name: "invalid cpu", cpu: "invalid", env: "BUILD_POD_CPU"},
		{name: "zero cpu", cpu: "0", env: "BUILD_POD_CPU"},
		{name: "negative cpu", cpu: "-1", env: "BUILD_POD_CPU"},
		{name: "sub millicore cpu", cpu: "0.1m", env: "BUILD_POD_CPU"},
		{name: "invalid memory", memory: "invalid", env: "BUILD_POD_MEMORY"},
		{name: "zero memory", memory: "0Mi", env: "BUILD_POD_MEMORY"},
		{name: "negative memory", memory: "-1Gi", env: "BUILD_POD_MEMORY"},
		{name: "atomic validation", cpu: "2", memory: "invalid", env: "BUILD_POD_MEMORY"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("BUILD_POD_CPU", tc.cpu)
			t.Setenv("BUILD_POD_MEMORY", tc.memory)
			pod := &corev1.Pod{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "build"}}}}
			before := pod.DeepCopy()
			err := applyBuildPodResources(pod)
			if err == nil || !strings.Contains(err.Error(), tc.env) {
				t.Fatalf("error = %v, want error naming %s", err, tc.env)
			}
			if !reflect.DeepEqual(pod, before) {
				t.Fatal("invalid configuration partially mutated the pod")
			}
		})
	}
}

// capability_id: rainbond.builder.build-pod-resources
func TestExecJobAppliesResourcesBeforeCreate(t *testing.T) {
	for _, invalid := range []bool{false, true} {
		name := "configured"
		if invalid {
			name = "invalid"
		}
		t.Run(name, func(t *testing.T) {
			t.Setenv("BUILD_POD_CPU", "2")
			t.Setenv("BUILD_POD_MEMORY", "4Gi")
			if invalid {
				t.Setenv("BUILD_POD_MEMORY", "invalid")
			}
			client := fake.NewSimpleClientset()
			factory := informers.NewSharedInformerFactory(client, 0)
			c := &controller{KubeClient: client, namespace: "rbd-system", jobInformer: factory.Core().V1().Pods()}
			pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "build", Namespace: c.namespace}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "build"}}}}
			createErr := errors.New("creation intercepted")
			var submitted *corev1.Pod
			client.PrependReactor("create", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
				submitted = action.(k8stesting.CreateAction).GetObject().(*corev1.Pod).DeepCopy()
				return true, nil, createErr
			})
			err := c.ExecJob(context.Background(), pod, nil, nil)
			if invalid {
				if err == nil || !strings.Contains(err.Error(), "BUILD_POD_MEMORY") || len(client.Actions()) != 0 {
					t.Fatalf("invalid configuration: error = %v, Kubernetes actions = %v", err, client.Actions())
				}
				if _, ok := c.jobContainerStatus.Load(pod.Name); ok {
					t.Fatal("invalid configuration registered a job status channel")
				}
				return
			}
			if !errors.Is(err, createErr) || submitted == nil {
				t.Fatalf("error = %v, submitted pod = %v", err, submitted)
			}
			r := submitted.Spec.Containers[0].Resources
			if r.Requests.Cpu().String() != "2" || r.Limits.Cpu().String() != "2" || r.Requests.Memory().String() != "4Gi" || r.Limits.Memory().String() != "4Gi" {
				t.Fatalf("submitted pod has incorrect requests/limits: %#v", r)
			}
		})
	}
}

// capability_id: rainbond.builder.build-pod-resources
func TestExecJobReusesExistingPodWithoutApplyingResources(t *testing.T) {
	t.Setenv("BUILD_POD_CPU", "invalid")
	t.Setenv("BUILD_POD_MEMORY", "invalid")
	client := fake.NewSimpleClientset()
	factory := informers.NewSharedInformerFactory(client, 0)
	c := &controller{KubeClient: client, namespace: "rbd-system", jobInformer: factory.Core().V1().Pods()}
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "existing", Namespace: c.namespace}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "build"}}}}
	before := pod.DeepCopy()
	if err := c.jobInformer.Informer().GetIndexer().Add(pod.DeepCopy()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result := channels.NewRingChannel(1)
	defer result.Close()
	if err := c.ExecJob(ctx, pod, nil, result); err != nil {
		t.Fatal(err)
	}
	select {
	case <-result.Out():
	case <-time.After(time.Second):
		t.Fatal("existing job log goroutine did not exit")
	}
	if len(client.Actions()) != 0 || !reflect.DeepEqual(pod, before) {
		t.Fatal("reusing an existing job must not create or change its resources")
	}
}
