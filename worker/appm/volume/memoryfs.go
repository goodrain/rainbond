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

// 本文件定义了与 Rainbond 平台中的内存文件系统卷（MemoryFS Volume）相关的结构体和方法，
// 主要用于在应用服务中创建和管理内存文件系统类型的存储卷。

// 文件内容包括以下几个主要部分：
// 1. `MemoryFSVolume` 结构体：这是一个与内存文件系统卷相关的结构体，继承自 `Base`，
//    主要负责处理与内存文件系统卷的创建和管理操作。
//    内存文件系统卷通过 Kubernetes 的 `emptyDir` 机制实现，
//    该卷类型存储的数据会保存在节点的内存中，适用于需要高速缓存或临时存储的场景。

// 2. `CreateVolume` 方法：该方法用于根据给定的定义对象（`Define`）创建内存文件系统卷。
//    首先，方法会检查卷的挂载路径是否为空或是否存在重复挂载路径，
//    如果满足上述情况则会跳过卷的创建。
//    接着，方法会创建一个 Kubernetes 的 `Volume` 对象，并将其类型设置为 `emptyDir`，
//    该 `emptyDir` 的默认存储介质是节点的存储介质（通常是磁盘）。
//    另外，如果服务的环境变量中指定了使用内存作为 `emptyDir` 的存储介质，
//    那么该卷的介质会被设置为内存（`Memory`），
//    最后将该卷和对应的挂载信息添加到定义对象的卷列表和卷挂载列表中。

// 3. `CreateDependVolume` 方法：这是一个空方法，当前在内存文件系统卷的场景下没有具体实现。
//    该方法通常用于创建依赖于其他服务的卷，但在内存文件系统卷场景下不需要额外的依赖卷处理。

// 通过这些结构体和方法，Rainbond 平台可以在应用服务中动态创建和配置内存文件系统卷，
// 使得应用服务能够利用内存进行高速缓存或临时数据存储，满足对高性能和临时存储的需求。

package volume

import (
	"fmt"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
)

// MemoryFSVolume memory fs volume struct
type MemoryFSVolume struct {
	Base
}

// CreateVolume memory fs volume create volume
func (v *MemoryFSVolume) CreateVolume(define *Define) error {
	logrus.Debugf("create emptyDir volume type for: %s", v.svm.VolumePath)
	volumeMountName := fmt.Sprintf("manual%d", v.svm.ID)
	volumeMountPath := v.svm.VolumePath
	volumeReadOnly := false
	if volumeMountPath == "" {
		logrus.Warningf("service[%s]'s mount path is empty, skip create memoryfs", v.version.ServiceID)
		return nil
	}
	for _, m := range define.volumeMounts {
		if m.MountPath == volumeMountPath {
			logrus.Warningf("service[%s]'s found the same mount path: %s, skip create memoryfs", v.version.ServiceID, volumeMountPath)
			return nil
		}
	}
	vo := corev1.Volume{Name: volumeMountName} // !!!: volumeMount name of k8s model must equal to volume name of k8s model

	// V5.2  emptyDir's medium use default "" which means to use the node's default medium
	vo.EmptyDir = &corev1.EmptyDirVolumeSource{}

	// get service custom env
	es, err := v.dbmanager.TenantServiceEnvVarDao().GetServiceEnvs(v.as.ServiceID, []string{"inner"})
	if err != nil {
		logrus.Errorf("get service[%s] env failed: %s", v.as.ServiceID, err.Error())
		return err
	}
	for _, env := range es {
		// still support for memory medium
		if env.AttrName == "ES_ENABLE_EMPTYDIR_MEDIUM_MEMORY" && env.AttrValue == "true" {
			logrus.Debugf("use memory as medium of emptyDir for volume[name: %s; path: %s]", volumeMountName, volumeMountPath)
			vo.EmptyDir.Medium = corev1.StorageMediumMemory
		}
	}
	define.volumes = append(define.volumes, vo)
	vm := corev1.VolumeMount{
		MountPath: volumeMountPath,
		Name:      volumeMountName,
		ReadOnly:  volumeReadOnly,
		SubPath:   "",
	}
	define.volumeMounts = append(define.volumeMounts, vm)
	return nil
}

// CreateDependVolume empty func
func (v *MemoryFSVolume) CreateDependVolume(define *Define) error {
	return nil
}
