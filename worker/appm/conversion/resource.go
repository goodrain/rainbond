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

package conversion

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

//Allocate the CPU at the ratio of 4g memory to 1 core CPU
func createResourcesByDefaultCPU(memory int, setCPURequest, setCPULimit int64) corev1.ResourceRequirements {
	var cpuRequest, cpuLimit int64
	base := int64(memory) / 128
	if base <= 0 {
		base = 1
	}
	if memory > 0 {
		if memory < 512 {
			cpuRequest, cpuLimit = base*30, base*80
		} else if memory <= 1024 {
			cpuRequest, cpuLimit = base*30, base*160
		} else {
			cpuRequest, cpuLimit = base*30, (int64(memory)-1024)/1024*500+1280
		}
	} else {
		memory = 0
	}
	if setCPULimit >= 0 {
		cpuLimit = setCPULimit
	}
	if setCPURequest >= 0 {
		cpuRequest = setCPURequest
	}

	limits := corev1.ResourceList{}
	limits[corev1.ResourceCPU] = *resource.NewMilliQuantity(cpuLimit, resource.DecimalSI)
	limits[corev1.ResourceMemory] = *resource.NewQuantity(int64(memory*1024*1024), resource.BinarySI)

	request := corev1.ResourceList{}
	request[corev1.ResourceCPU] = *resource.NewMilliQuantity(cpuRequest, resource.DecimalSI)
	request[corev1.ResourceMemory] = *resource.NewQuantity(int64(memory*1024*1024), resource.BinarySI)

	return corev1.ResourceRequirements{
		Limits:   limits,
		Requests: request,
	}
}
