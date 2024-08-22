// 本文件提供了与 Kubernetes Pod 状态相关的工具函数，用于描述和处理 Pod 的状态信息，
// 这些工具函数在云原生环境下的应用程序监控和管理中至关重要。

// 1. `PodStatusAdvice` 类型：
//    - 定义了一组常量，用于提供 Pod 状态的建议。
//    - 包括内存不足 (OutOfMemory)、Pod 不健康 (Unhealthy) 和正在初始化 (Initiating) 等状态。

// 2. `DescribePodStatus` 函数：
//    - 该函数接收 Kubernetes 客户端集、Pod 对象和一个自定义 Pod 状态对象 `PodStatus`，
//      并根据 Pod 的当前状态设置 `PodStatus` 的类型、原因和消息。
//    - 函数通过检查 Pod 的删除时间戳、条件列表、容器状态等信息来确定 Pod 的当前状态，
//      并根据不同的条件设置对应的状态建议。
//    - 例如，如果 Pod 正在终止，函数会将状态设置为 `TERMINATING`，如果发现容器因内存不足而终止，
//      则将状态建议设置为 `OutOfMemory`。

// 3. `SortableConditionType` 类型：
//    - 该类型实现了排序接口，用于对 Pod 的条件 (Conditions) 进行排序。
//    - 条件类型按照优先级进行排序，如 `PodScheduled`、`PodInitialized`、`PodReady` 和 `ContainersReady`，
//      从而确保在处理 Pod 状态时能够识别出最重要的条件。

// 总的来说，本文件中的工具函数和类型为 Kubernetes Pod 状态的管理和监控提供了有效的支持，
// 帮助开发者和运维人员更好地理解和处理 Pod 在集群中的运行状况。

package util

import (
	"fmt"
	virtv1 "kubevirt.io/api/core/v1"
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
