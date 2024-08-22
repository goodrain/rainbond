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

// 本文件定义了与 Rainbond 平台中的其他类型存储卷（例如 Ceph RBD、阿里云存储等）相关的结构体和方法，
// 主要用于在应用服务中创建和管理这些存储卷。

// 文件内容包括以下几个主要部分：
// 1. `OtherVolume` 结构体：这是一个与其他类型存储卷相关的结构体，继承自 `Base`，
//    主要负责处理其他类型存储卷的创建和管理操作。其他类型存储卷可以包括 Ceph RBD、阿里云存储等，
//    这些存储卷通常用于更复杂的存储需求，如分布式存储或云存储。

// 2. `CreateVolume` 方法：该方法用于根据给定的定义对象（`Define`）创建其他类型的存储卷。
//    首先，方法会根据存储卷类型（`VolumeType`）从数据库中获取对应的存储类型信息，
//    并验证存储容量是否符合要求。
//    然后，根据存储卷的相关信息（如挂载路径、访问模式等）创建一个 Kubernetes 的 `PersistentVolumeClaim` 对象，
//    并将其附加到有状态组件（`StatefulSet`）或其他组件中。
//    通过这种方式，应用服务可以使用其他类型的存储卷来满足特定的存储需求。

// 3. `CreateDependVolume` 方法：这是一个空方法，当前在其他类型存储卷的场景下没有具体实现。
//    该方法通常用于创建依赖于其他服务的卷，但在此场景下不需要额外的依赖卷处理。

// 通过这些结构体和方法，Rainbond 平台能够在应用服务中动态创建和配置其他类型的存储卷，
// 使得应用服务可以利用分布式存储或云存储来满足不同的存储需求，并确保数据的可靠性和可扩展性。

package volume

import (
	"fmt"
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/node/nodem/client"
	workerutil "github.com/goodrain/rainbond/worker/util"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
)

// OtherVolume ali cloud volume struct
type OtherVolume struct {
	Base
}

// CreateVolume ceph rbd volume create volume
func (v *OtherVolume) CreateVolume(define *Define) error {
	var shareFile bool
	if v.svm.VolumeType == dbmodel.ShareFileVolumeType.String() {
		v.svm.VolumeType = v.as.SharedStorageClass
		shareFile = true
	}
	volumeType, err := db.GetManager().VolumeTypeDao().GetVolumeTypeByType(v.svm.VolumeType)
	if err != nil {
		logrus.Errorf("get volume type by type error: %s", err.Error())
		return fmt.Errorf("validate volume capacity error")
	}
	if err := workerutil.ValidateVolumeCapacity(volumeType.CapacityValidation, v.svm.VolumeCapacity); err != nil {
		logrus.Errorf("validate volume capacity[%v] error: %s", v.svm.VolumeCapacity, err.Error())
		return err
	}
	v.svm.VolumeProviderName = volumeType.Provisioner
	volumeMountName := fmt.Sprintf("manual%d", v.svm.ID)
	volumeMountPath := v.svm.VolumePath
	volumeReadOnly := v.svm.IsReadOnly
	labels := v.as.GetCommonLabels(map[string]string{"volume_name": v.svm.VolumeName, "version": v.as.DeployVersion, "reclaim_policy": v.svm.ReclaimPolicy})
	annotations := map[string]string{"volume_name": v.svm.VolumeName}
	claim := newVolumeClaim(volumeMountName, volumeMountPath, v.svm.AccessMode, v.svm.VolumeType, v.svm.VolumeCapacity, labels, annotations)
	logrus.Debugf("storage class is : %s, claim value is : %s", v.svm.VolumeType, claim.GetName())
	claim.Annotations = map[string]string{
		client.LabelOS: func() string {
			if v.as.IsWindowsService {
				return "windows"
			}
			return "linux"
		}(),
	}
	v.as.SetClaim(claim)                 // store claim to appService
	statefulset := v.as.GetStatefulSet() //有状态组件
	vo := corev1.Volume{Name: volumeMountName}
	vo.PersistentVolumeClaim = &corev1.PersistentVolumeClaimVolumeSource{ClaimName: claim.GetName(), ReadOnly: volumeReadOnly}
	if statefulset != nil {
		statefulset.Spec.VolumeClaimTemplates = append(statefulset.Spec.VolumeClaimTemplates, *claim)
		logrus.Debugf("stateset.Spec.VolumeClaimTemplates: %+v", statefulset.Spec.VolumeClaimTemplates)
	} else {
		if shareFile {
			v.as.SetClaimManually(claim)
		}
		define.volumes = append(define.volumes, vo)
	}

	vm := corev1.VolumeMount{
		Name:      volumeMountName,
		MountPath: volumeMountPath,
		ReadOnly:  volumeReadOnly,
	}
	define.volumeMounts = append(define.volumeMounts, vm)
	return nil
}

// CreateDependVolume create depend volume
func (v *OtherVolume) CreateDependVolume(define *Define) error {
	return nil
}
