// RAINBOND, Application Management Platform
// Copyright (C) 2014-2019 Goodrain Co., Ltd.

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version. For any non-GPL usage of Rainbond,
// one or multiple Commercial Licenses authorized by Goodrain Co., Ltd.
// must be obtained first.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.
// 该文件定义了两个用于判断 Kubernetes Pod 状态的核心函数。
// 这些函数用于检测 Pod 是否处于终止状态或由于节点丢失而被驱逐。

// 文件的主要内容包括：
// 1. `IsPodTerminated` 方法：该方法用于判断一个 Pod 是否处于终止状态。
//    如果 Pod 的状态不是 Pending、Running、Unknown、Succeeded 或 Failed，
//    则认为该 Pod 已终止，返回 `true`，否则返回 `false`。
//    该方法主要用于识别那些已经不再活动的 Pod，以便进行后续处理，如清理资源等。

// 2. `IsPodNodeLost` 方法：该方法用于检测一个 Pod 是否因为节点丢失而被驱逐。
//    当 Pod 的删除时间戳不为空且状态原因是 "NodeLost" 时，返回 `true`，否则返回 `false`。
//    该方法主要用于处理由于节点故障或不可达导致的 Pod 异常情况，
//    以便在系统中及时采取相应的恢复或迁移措施。

// 总体来说，该文件为 Rainbond 平台中的 Pod 状态检测提供了必要的工具，
// 这些工具可以帮助平台有效地管理和监控 Pod 的生命周期，确保集群中应用的高可用性和稳定性。

package v1

import (
	corev1 "k8s.io/api/core/v1"
)

// IsPodTerminated Exception evicted pod
func IsPodTerminated(pod *corev1.Pod) bool {
	if phase := pod.Status.Phase; phase != corev1.PodPending && phase != corev1.PodRunning && phase != corev1.PodUnknown && phase != corev1.PodSucceeded && phase != corev1.PodFailed {
		return true
	}
	return false
}

// IsPodNodeLost node loss pod
func IsPodNodeLost(pod *corev1.Pod) bool {
	if pod.DeletionTimestamp != nil && pod.Status.Reason == "NodeLost" {
		return true
	}
	return false
}
