package util

import (
	"fmt"
	virtv1 "kubevirt.io/api/core/v1"
	"sort"
	"strings"

	rainbondutil "github.com/goodrain/rainbond/util"
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
	string(corev1.PodPending):             pb.PodStatus_INITIATING,
	string(corev1.PodRunning):             pb.PodStatus_RUNNING,
	string(corev1.PodSucceeded):           pb.PodStatus_ABNORMAL,
	string(corev1.PodFailed):              pb.PodStatus_ABNORMAL,
	string(corev1.PodUnknown):             pb.PodStatus_UNKNOWN,
	string(corev1.PodReady):               pb.PodStatus_NOTREADY,
	string(corev1.PodInitialized):         pb.PodStatus_INITIATING,
	string(corev1.PodScheduled):           pb.PodStatus_SCHEDULING,
	string(corev1.ContainersReady):        pb.PodStatus_NOTREADY,
	string(virtv1.VirtualMachineUnpaused): pb.PodStatus_RUNNING,
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
			// Translate common Pod-level failure reasons to user-friendly messages
			podStatus.Message = translatePodReason(pod.Status.Reason, pod.Status.Message)
		}
		if len(pod.Status.Message) > 0 && podStatus.Message == "" {
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
			// Translate condition reasons to user-friendly messages
			podStatus.Message = translateConditionReason(condition.Reason, condition.Message)
		}
	}
	if podStatus.Type == pb.PodStatus_PENDING {
		for _, cstatus := range pod.Status.ContainerStatuses {
			for _, OwnerReference := range pod.OwnerReferences {
				if OwnerReference.Kind == "Job" {
					if cstatus.State.Terminated != nil {
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
			}
			return
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
					podStatus.Message = rainbondutil.Translation("Deployment failed: container out of memory killed")
				} else if cstatus.State.Terminated.Reason != "" {
					podStatus.Message = translateContainerTerminatedReason(cstatus.State.Terminated.Reason, cstatus.State.Terminated.Message)
				}
				for _, OwnerReference := range pod.OwnerReferences {
					if OwnerReference.Kind == "Job" {
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
						if strings.Contains(evt.Message, "Liveness probe failed") {
							podStatus.Type = pb.PodStatus_UNHEALTHY
							podStatus.Advice = PodStatusAdviceUnhealthy.String()
							podStatus.Message = rainbondutil.Translation("Deployment failed: liveness probe failed")
							return
						}
						if strings.Contains(evt.Message, "Readiness probe failed") {
							podStatus.Type = pb.PodStatus_UNHEALTHY
							podStatus.Advice = PodStatusAdviceUnhealthy.String()
							podStatus.Message = rainbondutil.Translation("Deployment failed: readiness probe failed")
							return
						}
						if strings.Contains(evt.Message, "Startup probe failed") {
							podStatus.Type = pb.PodStatus_UNHEALTHY
							podStatus.Advice = PodStatusAdviceUnhealthy.String()
							podStatus.Message = rainbondutil.Translation("Deployment failed: startup probe failed")
							return
						}
					}
				}
			}
			if !cstatus.Ready && cstatus.State.Waiting != nil {
				w := cstatus.State.Waiting
				if w.Reason != "PodInitializing" && w.Reason != "ContainerCreating" {
					podStatus.Type = pb.PodStatus_ABNORMAL
					// Translate container waiting reasons to user-friendly messages
					podStatus.Message = translateContainerWaitingReason(w.Reason, w.Message)
					return
				}
			}
		}
	}
}

// translatePodReason translates Pod-level failure reasons to user-friendly messages
func translatePodReason(reason, message string) string {
	switch reason {
	case "OutOfMemory":
		return rainbondutil.Translation("Deployment failed: container out of memory killed")
	case "Evicted":
		if strings.Contains(message, "node") {
			return rainbondutil.Translation("Deployment failed: no nodes available for scheduling")
		}
		return fmt.Sprintf("%s: %s", rainbondutil.Translation("Deployment failed: container configuration error"), message)
	default:
		if message != "" {
			return message
		}
		return reason
	}
}

// translateConditionReason translates Pod condition reasons to user-friendly messages
func translateConditionReason(reason, message string) string {
	switch reason {
	case "Unschedulable":
		if strings.Contains(message, "Insufficient cpu") {
			return rainbondutil.Translation("Deployment failed: insufficient CPU resources")
		}
		if strings.Contains(message, "Insufficient memory") {
			return rainbondutil.Translation("Deployment failed: insufficient memory resources")
		}
		if strings.Contains(message, "Insufficient") {
			return rainbondutil.Translation("Deployment failed: insufficient storage resources")
		}
		if strings.Contains(message, "node(s)") {
			return rainbondutil.Translation("Deployment failed: no nodes available for scheduling")
		}
		return fmt.Sprintf("%s: %s", rainbondutil.Translation("Pod scheduling failed"), message)
	case "ContainersNotReady":
		return rainbondutil.Translation("Deployment failed: container creation failed")
	case "PodInitializing":
		return rainbondutil.Translation("Pod is initializing")
	default:
		if message != "" {
			return message
		}
		return reason
	}
}

// translateContainerWaitingReason translates container waiting reasons to user-friendly messages
func translateContainerWaitingReason(reason, message string) string {
	switch reason {
	case "ErrImagePull", "ImagePullBackOff":
		if strings.Contains(message, "not found") || strings.Contains(message, "manifest unknown") {
			return rainbondutil.Translation("Deployment failed: image not found")
		}
		if strings.Contains(message, "unauthorized") || strings.Contains(message, "authentication") {
			return rainbondutil.Translation("Deployment failed: image pull authentication failed")
		}
		return rainbondutil.Translation("Deployment failed: image pull failed")
	case "InvalidImageName":
		return rainbondutil.Translation("Deployment failed: invalid image name")
	case "CreateContainerConfigError":
		return rainbondutil.Translation("Deployment failed: container configuration error")
	case "CreateContainerError":
		return rainbondutil.Translation("Deployment failed: container creation failed")
	case "CrashLoopBackOff":
		return rainbondutil.Translation("Deployment failed: container is being terminated repeatedly")
	case "RunContainerError":
		return rainbondutil.Translation("Deployment failed: container startup failed")
	default:
		if message != "" {
			return fmt.Sprintf("%s: %s", reason, message)
		}
		return reason
	}
}

// translateContainerTerminatedReason translates container terminated reasons to user-friendly messages
func translateContainerTerminatedReason(reason, message string) string {
	switch reason {
	case "OOMKilled":
		return rainbondutil.Translation("Deployment failed: container out of memory killed")
	case "Error":
		return rainbondutil.Translation("Deployment failed: container startup failed")
	case "ContainerCannotRun":
		return rainbondutil.Translation("Deployment failed: container configuration error")
	default:
		if message != "" {
			return fmt.Sprintf("%s: %s", reason, message)
		}
		return reason
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
