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
	"os"
	"path"
	"strings"
)

// ShareFileVolume nfs volume struct
type ShareFileVolume struct {
	Base
}

// CreateVolume share file volume create volume
func (v *ShareFileVolume) CreateVolume(define *Define) error {
	volumeMountName := fmt.Sprintf("manual%d", v.svm.ID)
	volumeMountPath := v.svm.VolumePath
	volumeReadOnly := v.svm.IsReadOnly

	var vm *corev1.VolumeMount
	if v.as.GetStatefulSet() != nil {
		statefulset := v.as.GetStatefulSet()

		labels := v.as.GetCommonLabels(map[string]string{"volume_name": volumeMountName})
		annotations := map[string]string{"volume_name": v.svm.VolumeName}
		claim := newVolumeClaim(volumeMountName, volumeMountPath, v.svm.AccessMode, v1.RainbondStatefuleShareStorageClass, v.svm.VolumeCapacity, labels, annotations)
		vm = &corev1.VolumeMount{
			Name:      volumeMountName,
			MountPath: volumeMountPath,
			ReadOnly:  volumeReadOnly,
		}
		v.as.SetClaim(claim)
		statefulset.Spec.VolumeClaimTemplates = append(statefulset.Spec.VolumeClaimTemplates, *claim)
		vo := corev1.Volume{Name: volumeMountName}
		vo.PersistentVolumeClaim = &corev1.PersistentVolumeClaimVolumeSource{ClaimName: claim.GetName(), ReadOnly: volumeReadOnly}
		define.volumes = append(define.volumes, vo)
		v.generateVolumeSubPath(define, vm)
	} else {
		for _, m := range define.volumeMounts {
			if m.MountPath == volumeMountPath { // TODO move to prepare
				logrus.Warningf("found the same mount path: %s, skip it", volumeMountPath)
				return nil
			}
		}

		labels := v.as.GetCommonLabels(map[string]string{
			"volume_name": volumeMountName,
			"stateless":   "",
		})
		annotations := map[string]string{"volume_name": v.svm.VolumeName}
		claim := newVolumeClaim(volumeMountName, volumeMountPath, v.svm.AccessMode, v1.RainbondStatefuleShareStorageClass, v.svm.VolumeCapacity, labels, annotations)
		vm = &corev1.VolumeMount{
			Name:      volumeMountName,
			MountPath: volumeMountPath,
			ReadOnly:  volumeReadOnly,
		}
		v.as.SetClaim(claim)
		v.as.SetClaimManually(claim)
		vo := corev1.Volume{Name: volumeMountName}
		vo.PersistentVolumeClaim = &corev1.PersistentVolumeClaimVolumeSource{ClaimName: claim.GetName(), ReadOnly: volumeReadOnly}
		define.volumes = append(define.volumes, vo)
		v.generateVolumeSubPath(define, vm)
	}
	define.volumeMounts = append(define.volumeMounts, *vm)

	return nil
}

// CreateDependVolume create depend volume
func (v *ShareFileVolume) CreateDependVolume(define *Define) error {
	volumeMountName := fmt.Sprintf("mnt%d", v.smr.ID)
	volumeMountPath := v.smr.VolumePath
	for _, m := range define.volumeMounts {
		if m.MountPath == volumeMountPath {
			logrus.Warningf("found the same mount path: %s, skip it", volumeMountPath)
			return nil
		}
	}

	vo := corev1.Volume{Name: volumeMountName}
	claimName := fmt.Sprintf("manual%d", v.svm.ID)
	vo.PersistentVolumeClaim = &corev1.PersistentVolumeClaimVolumeSource{ClaimName: claimName, ReadOnly: false}
	define.volumes = append(define.volumes, vo)
	vm := &corev1.VolumeMount{
		Name:      volumeMountName,
		MountPath: volumeMountPath,
		ReadOnly:  false,
	}
	v.generateVolumeSubPath(define, vm)
	define.volumeMounts = append(define.volumeMounts, *vm)
	return nil
}

func (v *ShareFileVolume) generateVolumeSubPath(define *Define, vm *corev1.VolumeMount) *corev1.VolumeMount {
	if os.Getenv("ENABLE_SUBPATH") != "true" {
		return vm
	}
	var existClaimName string
	var needDeleteClaim []corev1.PersistentVolumeClaim
	for _, claim := range v.as.GetClaims() {
		if existClaimName != "" && *claim.Spec.StorageClassName == v1.RainbondStatefuleShareStorageClass {
			v.as.DeleteClaim(claim)
			if v.as.GetStatefulSet() == nil {
				v.as.DeleteClaimManually(claim)
			}
			needDeleteClaim = append(needDeleteClaim, *claim)
			continue
		}
		if *claim.Spec.StorageClassName == v1.RainbondStatefuleShareStorageClass {
			existClaimName = claim.GetName()
		}
	}
	if v.as.GetStatefulSet() != nil {
		for _, delClaim := range needDeleteClaim {
			newClaimTmpls := v.deleteClaim(v.as.GetStatefulSet().Spec.VolumeClaimTemplates, delClaim)
			v.as.GetStatefulSet().Spec.VolumeClaimTemplates = newClaimTmpls
			newVolumes := v.deleteVolume(define.volumes, delClaim)
			define.volumes = newVolumes
		}
		vm.Name = existClaimName
		subPathExpr := path.Join(strings.TrimPrefix(v.svm.HostPath, "/grdata/"), "$(POD_NAME)")
		vm.SubPathExpr = subPathExpr
		return vm
	}

	for _, delClaim := range needDeleteClaim {
		newVolumes := v.deleteVolume(define.volumes, delClaim)
		define.volumes = newVolumes
	}
	vm.Name = existClaimName
	vm.SubPath = strings.TrimPrefix(v.svm.HostPath, "/grdata/")
	return vm
}

func (v *ShareFileVolume) deleteClaim(claims []corev1.PersistentVolumeClaim, delClaim corev1.PersistentVolumeClaim) []corev1.PersistentVolumeClaim {
	newClaims := claims
	for i, claim := range claims {
		if claim.GetName() == delClaim.GetName() {
			newClaims = append(newClaims[0:i], newClaims[i+1:]...)
		}
	}
	return newClaims
}

func (v *ShareFileVolume) deleteVolume(claims []corev1.Volume, delClaim corev1.PersistentVolumeClaim) []corev1.Volume {
	newClaims := claims
	for i, claim := range claims {
		if claim.Name == delClaim.GetName() {
			newClaims = append(newClaims[0:i], newClaims[i+1:]...)
		}
	}
	return newClaims
}
