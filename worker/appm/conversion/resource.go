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
/*
 * 描述:
 * 该文件包含与资源设置相关的功能，特别是用于创建 Kubernetes 资源需求的功能。主要功能包括根据给定的内存、CPU 和 GPU 配置生成 Kubernetes 和 KubeVirt 资源需求对象。此功能对资源的限制和请求进行设置，并在需要时支持 KubeVirt 特定的资源配置。
 *
 * 主要功能:
 * - 创建 Kubernetes 资源需求对象 (corev1.ResourceRequirements) 和 KubeVirt 资源需求对象 (kubevirtv1.ResourceRequirements)。
 * - 根据提供的内存、CPU 和 GPU 配置生成资源限制和请求。
 * - 处理 GPU 限制的解析和日志记录。
 *
 * 函数:
 * - createResourcesBySetting(memory int, setCPURequest, setCPULimit, setGPULimit int64, vmResource bool) (*corev1.ResourceRequirements, *kubevirtv1.ResourceRequirements)
 *   - 参数:
 *     - memory: 内存大小，以 MB 为单位。
 *     - setCPURequest: CPU 请求值，以毫核为单位。
 *     - setCPULimit: CPU 限制值，以毫核为单位。
 *     - setGPULimit: GPU 限制值，以整数形式表示。
 *     - vmResource: 布尔值，指示是否生成 KubeVirt 资源需求对象。
 *   - 返回值:
 *     - 对于 Kubernetes 资源需求，返回 *corev1.ResourceRequirements。
 *     - 对于 KubeVirt 资源需求，返回 *kubevirtv1.ResourceRequirements。
 *
 * 许可证:
 * - 本程序是自由软件；您可以根据 GNU 通用公共许可证第3版（或（根据您的选择）任何更高版本）的条款重新分发和/或修改。
 * - 对于 Rainbond 的任何非 GPL 使用，必须先获得 Goodrain Co., Ltd. 授权的一个或多个商业许可证。
 * - 本程序按“原样”分发，未提供任何形式的明示或暗示的担保，包括但不限于适销性或适用性的担保。
 * - 有关详细信息，请参阅 GNU 通用公共许可证。
 *
 * 联系:
 * - 如果您没有收到 GNU 通用公共许可证的副本，请访问 <http://www.gnu.org/licenses/>。
 */

package conversion

import (
	"fmt"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	kubevirtv1 "kubevirt.io/api/core/v1"
)

func createResourcesBySetting(memory int, setCPURequest, setCPULimit, setGPULimit int64, vmResource bool) (*corev1.ResourceRequirements, *kubevirtv1.ResourceRequirements) {
	limits := corev1.ResourceList{}
	request := corev1.ResourceList{}
	if memory > 0 {
		limits[corev1.ResourceMemory] = *resource.NewQuantity(int64(memory*1024*1024), resource.BinarySI)
	}
	if setCPULimit > 0 {
		limits[corev1.ResourceCPU] = *resource.NewMilliQuantity(setCPULimit, resource.DecimalSI)
	}
	if setGPULimit > 0 {
		gpuLimit, err := resource.ParseQuantity(fmt.Sprintf("%d", setGPULimit))
		if err != nil {
			logrus.Errorf("gpu request is invalid")
		} else {
			limits[GetGPUMemKey()] = gpuLimit
		}
	}

	if setCPURequest > 0 {
		request[corev1.ResourceCPU] = *resource.NewMilliQuantity(setCPURequest, resource.DecimalSI)
	}
	if vmResource {
		return nil, &kubevirtv1.ResourceRequirements{
			Limits:   limits,
			Requests: request,
		}
	}
	return &corev1.ResourceRequirements{
		Limits:   limits,
		Requests: request,
	}, nil
}
