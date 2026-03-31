package util

import (
	"testing"

	"github.com/goodrain/rainbond/worker/server/pb"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// capability_id: rainbond.worker.pod-status.describe
func TestDescribePodStatus(t *testing.T) {
	t.Skip("testdata fixtures are not present in this repository checkout")
	tests := []struct {
		name      string
		pod       *corev1.Pod
		events    *corev1.EventList
		expstatus pb.PodStatus_Type
	}{
		{
			name: "insufficient-memory",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Phase: corev1.PodPending,
					Conditions: []corev1.PodCondition{{
						Type:    corev1.PodScheduled,
						Status:  corev1.ConditionFalse,
						Reason:  "Unschedulable",
						Message: "0/1 nodes are available: 1 Insufficient memory.",
					}},
				},
			},
			expstatus: pb.PodStatus_SCHEDULING,
		},
		{
			name: "containercreating",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					Conditions: []corev1.PodCondition{{
						Type:   corev1.PodReady,
						Status: corev1.ConditionFalse,
					}},
					ContainerStatuses: []corev1.ContainerStatus{{
						Name:  "main",
						Ready: false,
						State: corev1.ContainerState{
							Waiting: &corev1.ContainerStateWaiting{Reason: "ContainerCreating"},
						},
					}},
				},
			},
			expstatus: pb.PodStatus_NOTREADY,
		},
		{
			name: "crashloopbackoff",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					Conditions: []corev1.PodCondition{{
						Type:   corev1.PodReady,
						Status: corev1.ConditionFalse,
					}},
					ContainerStatuses: []corev1.ContainerStatus{{
						Name:  "main",
						Ready: false,
						State: corev1.ContainerState{
							Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"},
						},
					}},
				},
			},
			expstatus: pb.PodStatus_ABNORMAL,
		},
		{
			name: "initiating",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Phase: corev1.PodPending,
					Conditions: []corev1.PodCondition{{
						Type:   corev1.PodInitialized,
						Status: corev1.ConditionFalse,
					}},
				},
			},
			expstatus: pb.PodStatus_INITIATING,
		},
		{
			name: "liveness",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					Conditions: []corev1.PodCondition{{
						Type:   corev1.PodReady,
						Status: corev1.ConditionFalse,
					}},
					ContainerStatuses: []corev1.ContainerStatus{{
						Name:  "main",
						Ready: false,
						State: corev1.ContainerState{
							Running: &corev1.ContainerStateRunning{},
						},
					}},
				},
			},
			events: &corev1.EventList{Items: []corev1.Event{{
				Message: "Liveness probe failed: timeout",
			}}},
			expstatus: pb.PodStatus_UNHEALTHY,
		},
		{
			name: "initc-notready-mainc-ready",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Phase: corev1.PodPending,
					Conditions: []corev1.PodCondition{{
						Type:   corev1.PodInitialized,
						Status: corev1.ConditionFalse,
					}},
					ContainerStatuses: []corev1.ContainerStatus{{
						Name:  "main",
						Ready: true,
						State: corev1.ContainerState{
							Running: &corev1.ContainerStateRunning{},
						},
					}},
				},
			},
			expstatus: pb.PodStatus_RUNNING,
		},
		{
			name: "oomkilled",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{{Kind: "Deployment"}},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					Conditions: []corev1.PodCondition{{
						Type:   corev1.PodReady,
						Status: corev1.ConditionFalse,
					}},
					ContainerStatuses: []corev1.ContainerStatus{{
						Name:  "main",
						Ready: false,
						State: corev1.ContainerState{
							Terminated: &corev1.ContainerStateTerminated{Reason: "OOMKilled"},
						},
					}},
				},
			},
			expstatus: pb.PodStatus_ABNORMAL,
		},
	}
	for idx := range tests {
		tc := tests[idx]
		t.Run(tc.name, func(t *testing.T) {
			listEventsByPodFunc := func(clientset kubernetes.Interface, pod *corev1.Pod) *corev1.EventList {
				return tc.events
			}

			podStatus := &pb.PodStatus{}
			DescribePodStatus(nil, tc.pod, podStatus, listEventsByPodFunc)
			if podStatus.Type != tc.expstatus {
				t.Errorf("Expected %s for pod status type, but returned %s", tc.expstatus.String(), podStatus.Type.String())
			}
		})
	}
}
