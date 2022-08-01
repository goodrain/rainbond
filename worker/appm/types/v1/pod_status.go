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

package v1

import (
	corev1 "k8s.io/api/core/v1"
)

//IsPodTerminated Exception evicted pod
func IsPodTerminated(pod *corev1.Pod) bool {
	if phase := pod.Status.Phase; phase != corev1.PodPending && phase != corev1.PodRunning && phase != corev1.PodUnknown && phase != corev1.PodSucceeded && phase != corev1.PodFailed {
		return true
	}
	return false
}

//IsPodNodeLost node loss pod
func IsPodNodeLost(pod *corev1.Pod) bool {
	if pod.DeletionTimestamp != nil && pod.Status.Reason == "NodeLost" {
		return true
	}
	return false
}
