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

package volume

import (
	"fmt"
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
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
	statefulset := v.as.GetStatefulSet() //有状态组件
	if v.svm.VolumeType == dbmodel.ShareFileVolumeType.String() {
		v.svm.VolumeType = v.as.SharedStorageClass
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
	if statefulset == nil {
		v.svm.AccessMode = "RWX"
	}
	claim := newVolumeClaim(volumeMountName, volumeMountPath, v.svm.AccessMode, v.svm.VolumeType, v.svm.VolumeCapacity, labels, annotations)
	logrus.Debugf("storage class is : %s, claim value is : %s", v.svm.VolumeType, claim.GetName())
	v.as.SetClaim(claim) // store claim to appService
	vo := corev1.Volume{Name: volumeMountName}
	vo.PersistentVolumeClaim = &corev1.PersistentVolumeClaimVolumeSource{ClaimName: claim.GetName(), ReadOnly: volumeReadOnly}
	if statefulset != nil {
		statefulset.Spec.VolumeClaimTemplates = append(statefulset.Spec.VolumeClaimTemplates, *claim)
		logrus.Debugf("stateset.Spec.VolumeClaimTemplates: %+v", statefulset.Spec.VolumeClaimTemplates)
	} else {
		v.as.SetClaimManually(claim)
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
