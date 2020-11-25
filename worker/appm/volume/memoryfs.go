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
