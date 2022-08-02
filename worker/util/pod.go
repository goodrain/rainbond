package util

import (
	"fmt"
	"sort"
	"strings"

	k8sutil "github.com/goodrain/rainbond/util/k8s"
	"github.com/goodrain/rainbond/worker/server/pb"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

// PodStatusAdvice -
type PodStatusAdvice string

// String converts PodStatusAdvice to string
func (p PodStatusAdvice) String() string {
	return string(p)
}

// PodStatusAdviceOOM -
var PodStatusAdviceOOM PodStatusAdvice = "OutOfMemory"

// PodStatusAdviceUnhealthy -
var PodStatusAdviceUnhealthy PodStatusAdvice = "Unhealthy"

// PodStatusAdviceInitiating -
var PodStatusAdviceInitiating PodStatusAdvice = "Initiating"

var podStatusTbl = map[string]pb.PodStatus_Type{
	string(corev1.PodPending):      pb.PodStatus_INITIATING,
	string(corev1.PodRunning):      pb.PodStatus_RUNNING,
	string(corev1.PodSucceeded):    pb.PodStatus_ABNORMAL,
	string(corev1.PodFailed):       pb.PodStatus_ABNORMAL,
	string(corev1.PodUnknown):      pb.PodStatus_UNKNOWN,
	string(corev1.PodReady):        pb.PodStatus_NOTREADY,
	string(corev1.PodInitialized):  pb.PodStatus_INITIATING,
	string(corev1.PodScheduled):    pb.PodStatus_SCHEDULING,
	string(corev1.ContainersReady): pb.PodStatus_NOTREADY,
}

// DescribePodStatus -
func DescribePodStatus(clientset kubernetes.Interface, pod *corev1.Pod, podStatus *pb.PodStatus, f k8sutil.ListEventsByPod) {
	defer func() {
		podStatus.TypeStr = podStatus.Type.String()
	}()
	if pod.DeletionTimestamp != nil {
		podStatus.Type = pb.PodStatus_TEMINATING
		podStatus.Message = fmt.Sprintf("Termination Grace Period:\t%ds", *pod.DeletionGracePeriodSeconds)
	} else if len(pod.Status.Conditions) == 0 {
		podStatus.Type = podStatusTbl[string(pod.Status.Phase)]
		if len(pod.Status.Reason) > 0 {
			podStatus.Reason = pod.Status.Reason
		}
		if len(pod.Status.Message) > 0 {
			podStatus.Message = pod.Status.Message
		}
	} else {
		// schedule, ready, init
		podStatus.Type = pb.PodStatus_RUNNING
		sort.Sort(SortableConditionType(pod.Status.Conditions))
		for _, condition := range pod.Status.Conditions {
			if condition.Status == corev1.ConditionTrue {
				continue
			}
			podStatus.Type = podStatusTbl[string(condition.Type)] // find the latest not ready condition
			podStatus.Reason = condition.Reason
			podStatus.Message = condition.Message
		}
	}
	if podStatus.Type == pb.PodStatus_INITIATING {
		podStatus.Advice = PodStatusAdviceInitiating.String()
		// if all main container ready
		if len(pod.Status.ContainerStatuses) > 0 {
			allMainCReady := true
			for _, mainC := range pod.Status.ContainerStatuses {
				if !mainC.Ready {
					allMainCReady = false
					break
				}
			}
			if allMainCReady {
				podStatus.Type = pb.PodStatus_RUNNING
				return
			}
		}
		return
	}
	if podStatus.Type == pb.PodStatus_NOTREADY {
		for _, cstatus := range pod.Status.ContainerStatuses {
			if !cstatus.Ready && cstatus.State.Terminated != nil {
				podStatus.Type = pb.PodStatus_ABNORMAL
				if cstatus.State.Terminated.Reason == "OOMKilled" {
					podStatus.Advice = PodStatusAdviceOOM.String()
				}
				for _, OwnerReference := range pod.OwnerReferences{
					if OwnerReference.Kind == "Job"{
						if cstatus.State.Terminated.Reason == "Completed" {
							podStatus.Type = pb.PodStatus_SUCCEEDED
						}
						if cstatus.State.Terminated.Reason == "DeadlineExceeded" {
							podStatus.Type = pb.PodStatus_FAILED
						}
						if cstatus.State.Terminated.Reason == "Error" {
							podStatus.Type = pb.PodStatus_ABNORMAL
						}
					}
				}
				return
			}
			if !cstatus.Ready {
				events := f(clientset, pod)
				if events != nil {
					for _, evt := range events.Items {
						if strings.Contains(evt.Message, "Liveness probe failed") || strings.Contains(evt.Message, "Readiness probe failed") {
							podStatus.Type = pb.PodStatus_UNHEALTHY
							podStatus.Advice = PodStatusAdviceUnhealthy.String()
							return
						}
					}
				}
			}
			if !cstatus.Ready && cstatus.State.Waiting != nil {
				w := cstatus.State.Waiting
				if w.Reason != "PodInitializing" && w.Reason != "ContainerCreating" {
					podStatus.Type = pb.PodStatus_ABNORMAL
					return
				}
			}
		}
	}
}

// SortableConditionType implements sort.Interface for []PodCondition based on
// the Type field.
type SortableConditionType []corev1.PodCondition

var podConditionTbl = map[corev1.PodConditionType]int{
	corev1.PodScheduled:    0,
	corev1.PodInitialized:  1,
	corev1.PodReady:        2,
	corev1.ContainersReady: 3,
}

func (s SortableConditionType) Len() int {
	return len(s)
}
func (s SortableConditionType) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s SortableConditionType) Less(i, j int) bool {
	return podConditionTbl[s[i].Type] > podConditionTbl[s[j].Type]
}
