package v1

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
