package util

import (
	"fmt"

	"github.com/goodrain/rainbond/worker/server/pb"
	corev1 "k8s.io/api/core/v1"
)

var podStatusTbl = map[string]pb.PodStatus_Type{
	string(corev1.PodPending):     pb.PodStatus_PENDING,
	string(corev1.PodRunning):     pb.PodStatus_RUNNING,
	string(corev1.PodSucceeded):   pb.PodStatus_SUCCEEDED,
	string(corev1.PodFailed):      pb.PodStatus_FAILED,
	string(corev1.PodUnknown):     pb.PodStatus_UNKNOWN,
	string(corev1.PodReady):       pb.PodStatus_ABNORMAL,
	string(corev1.PodInitialized): pb.PodStatus_INITIATING,
	string(corev1.PodScheduled):   pb.PodStatus_SCHEDULING,
}

func DescribePodStatus(pod *corev1.Pod, podStatus *pb.PodStatus) {
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
		// TODO: advice
	} else {
		// schedule, ready, init
		podStatus.Type = pb.PodStatus_RUNNING
		// TODO: sort important.
		for _, condition := range pod.Status.Conditions {
			if condition.Status == corev1.ConditionTrue {
				continue
			}
			podStatus.Type = podStatusTbl[string(condition.Type)]
			podStatus.Reason = condition.Reason
			podStatus.Message = condition.Message
		}
		// TODO: advice
	}
}
