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

// 本文件定义了 Rainbond 平台中的共享文件存储卷（ShareFileVolume）的结构体和相关操作方法。
// 这些存储卷通常用于为有状态组件（StatefulSet）、虚拟机（VirtualMachine）或其他类型的服务提供持久化存储。

// 文件内容主要包括以下几个部分：
// 1. `ShareFileVolume` 结构体：这是一个继承自 `Base` 的结构体，表示共享文件存储卷。它封装了与存储卷相关的属性和操作方法，
//    例如存储卷的挂载路径、读写权限、容量等。

// 2. `CreateVolume` 方法：该方法用于为应用服务创建共享文件存储卷。
//    如果是有状态组件（StatefulSet），它会创建一个 `PersistentVolumeClaim` 对象，并将其附加到组件的 `VolumeClaimTemplates` 中。
//    如果是虚拟机（VirtualMachine），则会创建一个与虚拟机磁盘相关的 `PersistentVolumeClaim` 对象。
//    对于其他类型的服务，该方法会检查是否存在重复的挂载路径，并根据需要创建新的存储卷挂载。
//    最终将创建的卷挂载（VolumeMount）对象添加到定义（Define）对象中。

// 3. `CreateDependVolume` 方法：该方法用于创建依赖卷，它会根据给定的卷信息创建相应的 `PersistentVolumeClaim` 对象，
//    并将其附加到定义（Define）对象的卷列表中。
//    此方法主要用于处理那些需要依赖其他服务的卷，确保在主服务启动前这些依赖卷已就绪。

// 4. `generateVolumeSubPath` 方法：该方法用于生成卷的子路径（SubPath）或子路径表达式（SubPathExpr）。
//    如果环境变量 `ENABLE_SUBPATH` 被设置为 "true"，则会根据存储卷的主机路径（HostPath）生成相应的子路径，
//    并将其设置到卷挂载（VolumeMount）对象中。否则，直接返回原始的卷挂载对象。

// 5. `deleteClaim` 和 `deleteVolume` 方法：这些方法用于从存储卷的声明列表（VolumeClaimTemplates）或定义对象（Define）的卷列表中删除指定的卷。
//    这些操作在需要清理不再使用的卷时非常有用，确保系统中不会遗留多余的存储卷资源。

// 通过这些结构体和方法，Rainbond 平台能够在应用服务中灵活地创建和管理共享文件存储卷，
// 满足不同类型服务的持久化存储需求，同时支持多种存储策略和卷类型的配置。

package volume

import (
	"fmt"
	v1 "github.com/goodrain/rainbond/worker/appm/types/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	kubevirtv1 "kubevirt.io/api/core/v1"
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
		define.volumeMounts = append(define.volumeMounts, *vm)
	} else if v.as.GetVirtualMachine() != nil {
		labels := v.as.GetCommonLabels(map[string]string{
			"volume_name": volumeMountName,
			"stateless":   "",
		})
		annotations := map[string]string{"volume_name": v.svm.VolumeName}
		claim := newVolumeClaim(volumeMountName, path.Join(volumeMountPath, volumeMountName), v.svm.AccessMode, v1.RainbondStatefuleLocalStorageClass, v.svm.VolumeCapacity, labels, annotations)
		v.as.SetClaim(claim)
		v.as.SetClaimManually(claim)
		vo := kubevirtv1.Volume{
			Name: volumeMountName,
			VolumeSource: kubevirtv1.VolumeSource{
				PersistentVolumeClaim: &kubevirtv1.PersistentVolumeClaimVolumeSource{
					PersistentVolumeClaimVolumeSource: corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: claim.Name,
					},
					Hotpluggable: false,
				},
			},
		}
		var dd kubevirtv1.DiskDevice
		switch volumeMountPath {
		case "/disk":
			dd = kubevirtv1.DiskDevice{
				Disk: &kubevirtv1.DiskTarget{
					Bus: kubevirtv1.DiskBusSATA,
				},
			}
		case "/lun":
			dd = kubevirtv1.DiskDevice{
				LUN: &kubevirtv1.LunTarget{
					Bus: kubevirtv1.DiskBusSATA,
				},
			}
		case "/cdrom":
			dd = kubevirtv1.DiskDevice{
				CDRom: &kubevirtv1.CDRomTarget{
					Bus: kubevirtv1.DiskBusSATA,
				},
			}
		}
		bootOrder := uint(len(define.vmDisk) + 1)
		dk := kubevirtv1.Disk{
			BootOrder:  &bootOrder,
			DiskDevice: dd,
			Name:       volumeMountName,
		}
		define.vmDisk = append(define.vmDisk, dk)
		define.vmVolume = append(define.vmVolume, vo)
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
		define.volumeMounts = append(define.volumeMounts, *vm)
	}

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
