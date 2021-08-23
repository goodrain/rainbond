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
		v1.LabelOS: func() string {
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
