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
// 本文件定义了与 Rainbond 平台中的 NFS（Network File System，网络文件系统）卷相关的结构体和方法，
// 主要用于在应用服务中创建和管理 NFS 类型的存储卷。

// 文件内容包括以下几个主要部分：
// 1. `NFSVolume` 结构体：这是一个与 NFS 卷相关的结构体，继承自 `Base`，
//    主要负责处理与 NFS 卷的创建和管理操作。
//    NFS 卷允许多个服务器之间共享文件系统，因此在需要跨多个节点共享文件的场景下非常有用。

// 2. `CreateVolume` 方法：该方法用于根据给定的定义对象（`Define`）创建 NFS 卷。
//    首先，方法会创建一个 Kubernetes 的 `Volume` 对象，并将其类型设置为 `NFS`，
//    同时指定 NFS 服务器地址和路径。
//    接着，将该卷和对应的挂载路径信息添加到定义对象的卷列表和卷挂载列表中。
//    通过这种方式，应用服务可以访问和使用 NFS 卷来存储和共享数据。

// 3. `CreateDependVolume` 方法：这是一个空方法，当前在 NFS 卷的场景下没有具体实现。
//    该方法通常用于创建依赖于其他服务的卷，但在 NFS 卷场景下不需要额外的依赖卷处理。

// 通过这些结构体和方法，Rainbond 平台能够在应用服务中动态创建和配置 NFS 卷，
// 使得应用服务可以跨多个节点共享文件和数据，满足分布式文件存储的需求。

package volume

import (
	"fmt"
	corev1 "k8s.io/api/core/v1"
)

// NFSVolume NFS volume struct
type NFSVolume struct {
	Base
}

// CreateVolume nfs create volume
func (n *NFSVolume) CreateVolume(define *Define) error {
	NFSName := fmt.Sprintf("nfs-%d", n.svm.ID)
	volumes := corev1.Volume{
		Name: NFSName,
		VolumeSource: corev1.VolumeSource{
			NFS: &corev1.NFSVolumeSource{
				Server: n.svm.NFSServer,
				Path:   n.svm.NFSPath,
			},
		},
	}
	define.volumes = append(define.volumes, volumes)
	volumeMounts := corev1.VolumeMount{
		Name:      NFSName,
		MountPath: n.svm.VolumePath,
	}
	define.volumeMounts = append(define.volumeMounts, volumeMounts)
	return nil
}

// CreateDependVolume nfs create depend volume
func (n *NFSVolume) CreateDependVolume(define *Define) error {
	return nil
}
