package v1

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubevirtv1 "kubevirt.io/api/core/v1"
)

func TestGetServiceStatusReturnsAbnormalForUnschedulablePod(t *testing.T) {
	service := &AppService{
		AppServiceBase: AppServiceBase{
			ServiceType: TypeDeployment,
			Replicas:    1,
		},
		deployment: &appsv1.Deployment{
			Status: appsv1.DeploymentStatus{
				ReadyReplicas: 0,
			},
		},
		pods: []*corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pending-pod",
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodPending,
					Conditions: []corev1.PodCondition{
						{
							Type:    corev1.PodScheduled,
							Status:  corev1.ConditionFalse,
							Reason:  "Unschedulable",
							Message: "0/1 nodes are available: 1 Insufficient cpu.",
						},
					},
				},
			},
		},
	}

	if got := service.GetServiceStatus(); got != ABNORMAL {
		t.Fatalf("expected unschedulable deployment to be %q, got %q", ABNORMAL, got)
	}
}

func TestGetServiceStatusReturnsWaitingWhenCurrentVersionPodIsScheduling(t *testing.T) {
	service := &AppService{
		AppServiceBase: AppServiceBase{
			ServiceType:   TypeDeployment,
			Replicas:      1,
			DeployVersion: "v1",
		},
		deployment: &appsv1.Deployment{
			Status: appsv1.DeploymentStatus{
				ReadyReplicas: 1,
			},
		},
		pods: []*corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "running-pod",
					Labels: map[string]string{"version": "v1"},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.PodReady,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "scheduling-pod",
					Labels: map[string]string{"version": "v1"},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodPending,
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.PodScheduled,
							Status: corev1.ConditionFalse,
							Reason: "Scheduling",
						},
					},
				},
			},
		},
	}

	if got := service.GetServiceStatus(); got != WAITING {
		t.Fatalf("expected scheduling deployment to be %q, got %q", WAITING, got)
	}
}

func TestGetServiceStatusReturnsClosedForStoppedVirtualMachine(t *testing.T) {
	service := &AppService{
		AppServiceBase: AppServiceBase{
			ServiceType: TypeVirtualMachine,
			Replicas:    1,
		},
		virtualmachine: &kubevirtv1.VirtualMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "demo-vm",
				Namespace:       "demo-ns",
				ResourceVersion: "1",
			},
			Status: kubevirtv1.VirtualMachineStatus{
				PrintableStatus: kubevirtv1.VirtualMachineStatusStopped,
			},
		},
	}

	if got := service.GetServiceStatus(); got != CLOSED {
		t.Fatalf("expected stopped vm to be %q, got %q", CLOSED, got)
	}
}

// capability_id: rainbond.worker.status.daemonset
func TestGetServiceStatusReturnsRunningForReadyDaemonSet(t *testing.T) {
	service := &AppService{
		AppServiceBase: AppServiceBase{
			ServiceType:   TypeDaemonSet,
			DeployVersion: "v1",
		},
		daemonset: &appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{
				ResourceVersion: "1",
				Labels:          map[string]string{"version": "v1"},
			},
			Status: appsv1.DaemonSetStatus{
				DesiredNumberScheduled: 2,
				NumberReady:            2,
			},
		},
		pods: []*corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"version": "v1"}},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					Conditions: []corev1.PodCondition{{
						Type:   corev1.PodReady,
						Status: corev1.ConditionTrue,
					}},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"version": "v1"}},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					Conditions: []corev1.PodCondition{{
						Type:   corev1.PodReady,
						Status: corev1.ConditionTrue,
					}},
				},
			},
		},
	}

	if got := service.GetServiceStatus(); got != RUNNING {
		t.Fatalf("expected ready daemonset to be %q, got %q", RUNNING, got)
	}
}

func TestGetServiceStatusReturnsAbnormalForUnschedulableDaemonSetPod(t *testing.T) {
	service := &AppService{
		AppServiceBase: AppServiceBase{
			ServiceType:   TypeDaemonSet,
			DeployVersion: "v1",
		},
		daemonset: &appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{ResourceVersion: "1"},
			Status: appsv1.DaemonSetStatus{
				DesiredNumberScheduled: 1,
				NumberReady:            0,
			},
		},
		pods: []*corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"version": "v1"}},
				Status: corev1.PodStatus{
					Phase: corev1.PodPending,
					Conditions: []corev1.PodCondition{{
						Type:   corev1.PodScheduled,
						Status: corev1.ConditionFalse,
						Reason: "Unschedulable",
					}},
				},
			},
		},
	}

	if got := service.GetServiceStatus(); got != ABNORMAL {
		t.Fatalf("expected unschedulable daemonset to be %q, got %q", ABNORMAL, got)
	}
}
