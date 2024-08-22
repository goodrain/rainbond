// RAINBOND, Application Management Platform
// Copyright (C) 2014-2017 Goodrain Co., Ltd.

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

// 本文件定义了与 Rainbond 平台中的本地存储卷相关的结构体和方法，
// 主要用于在应用服务中创建和管理本地存储类型的存储卷。

// 文件内容包括以下几个主要部分：
// 1. `LocalVolume` 结构体：这是一个与本地存储卷相关的结构体，继承自 `Base`，主要负责处理与本地存储卷的创建和管理操作。
//    本地存储卷通常用于需要在本地持久化数据的有状态组件（如 StatefulSet）。

// 2. `CreateVolume` 方法：该方法用于根据给定的定义对象（`Define`）创建本地存储卷。首先，它会生成一个卷声明（`PersistentVolumeClaim`），
//    该声明包含了卷的挂载路径、访问模式、存储类、容量等信息，并将其添加到有状态集（StatefulSet）的卷声明模板中。
//    接着，会将该卷声明对应的卷（`Volume`）对象添加到定义对象的卷列表中，
//    并将卷挂载信息（`VolumeMount`）添加到定义对象的卷挂载列表中，
//    从而使得应用服务可以使用该本地存储卷。

// 3. `CreateDependVolume` 方法：这是一个空方法，当前在本地存储卷的场景下没有具体实现。
//    该方法通常用于创建依赖于其他服务的卷，但在本地卷场景下不需要额外的依赖卷处理。

// 通过这些结构体和方法，Rainbond 平台可以在有状态组件中动态创建和配置本地存储卷，
// 使得应用服务能够利用本地磁盘进行数据存储，从而满足对持久化存储的需求。

package volume

import (
	"fmt"

	"github.com/goodrain/rainbond/node/nodem/client"
	v1 "github.com/goodrain/rainbond/worker/appm/types/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
)

// LocalVolume local volume struct
type LocalVolume struct {
	Base
}

// CreateVolume local volume create volume
func (v *LocalVolume) CreateVolume(define *Define) error {
	volumeMountName := fmt.Sprintf("manual%d", v.svm.ID)
	volumeMountPath := v.svm.VolumePath
	volumeReadOnly := v.svm.IsReadOnly
	statefulset := v.as.GetStatefulSet()
	if statefulset == nil {
		logrus.Warning("local volume must be used state compoment")
		return nil
	}
	labels := v.as.GetCommonLabels(map[string]string{"volume_name": v.svm.VolumeName, "version": v.as.DeployVersion})
	annotations := map[string]string{"volume_name": v.svm.VolumeName}
	claim := newVolumeClaim(volumeMountName, volumeMountPath, v.svm.AccessMode, v1.RainbondStatefuleLocalStorageClass, v.svm.VolumeCapacity, labels, annotations)
	claim.Annotations = map[string]string{
		client.LabelOS: func() string {
			if v.as.IsWindowsService {
				return "windows"
			}
			return "linux"
		}(),
	}
	v.as.SetClaim(claim)
	vo := corev1.Volume{Name: volumeMountName}
	vo.PersistentVolumeClaim = &corev1.PersistentVolumeClaimVolumeSource{ClaimName: claim.GetName(), ReadOnly: volumeReadOnly}
	define.volumes = append(define.volumes, vo)
	statefulset.Spec.VolumeClaimTemplates = append(statefulset.Spec.VolumeClaimTemplates, *claim)

	vm := corev1.VolumeMount{
		Name:      volumeMountName,
		MountPath: volumeMountPath,
		ReadOnly:  volumeReadOnly,
	}
	define.volumeMounts = append(define.volumeMounts, vm)
	return nil
}

// CreateDependVolume empty func
func (v *LocalVolume) CreateDependVolume(define *Define) error {
	return nil
}
