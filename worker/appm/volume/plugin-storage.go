// 本文件定义了与 Rainbond 平台中的插件存储卷相关的结构体和方法，
// 主要用于在应用服务中创建和管理这些插件相关的存储卷或配置文件。

// 文件内容包括以下几个主要部分：
// 1. `PluginStorageVolume` 结构体：这是一个与插件存储卷相关的结构体，继承自 `Base`，
//    主要负责处理插件存储卷的创建和管理操作。该结构体包括了插件的基本信息（如插件 ID、卷名等）
//    以及与应用服务相关的元数据信息。

// 2. `CreateVolume` 方法：该方法用于根据给定的定义对象（`Define`）创建插件存储卷或配置文件。
//    根据插件属性类型（`AttrType`），如果是配置文件类型（`config-file`），则会创建一个 Kubernetes 的 `ConfigMap` 对象，
//    并将插件内容写入其中；如果是存储类型（`storage`），则会根据插件的信息创建一个 Kubernetes 的 `PersistentVolumeClaim` 对象，
//    并将其附加到有状态组件（`StatefulSet`）或其他组件中。
//    通过这种方式，应用服务可以使用插件存储卷来扩展其功能或配置。

// 3. `CreateDependVolume` 方法：这是一个空方法，当前在插件存储卷的场景下没有具体实现。
//    该方法通常用于创建依赖于其他服务的卷，但在此场景下不需要额外的依赖卷处理。

// 4. `generateVolumeSubPath` 方法：该方法用于生成卷的子路径（`SubPath`）或子路径表达式（`SubPathExpr`）。
//    如果环境变量 `ENABLE_SUBPATH` 被设置为 "true"，则根据应用服务的租户 ID 和服务 ID 生成对应的子路径或子路径表达式，
//    并设置到卷挂载对象（`VolumeMount`）中。否则，直接返回原始的卷挂载对象。

// 5. `deleteVolume` 方法：该方法用于从定义对象（`Define`）的卷列表中删除指定的卷，
//    以便在需要时清理不再使用的卷。

// 通过这些结构体和方法，Rainbond 平台能够在应用服务中动态创建和配置与插件相关的存储卷，
// 使得应用服务可以通过插件来扩展其功能，满足不同的业务需求。

package volume

import (
	"fmt"
	api_model "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/util"
	v1 "github.com/goodrain/rainbond/worker/appm/types/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"path"
)

// PluginStorageVolume 插件新增存储
type PluginStorageVolume struct {
	Plugin api_model.PluginStorage
	Base
	PluginID string
	AS       *v1.AppService
}

// CreateVolume 创建插件存储或配置文件
func (v *PluginStorageVolume) CreateVolume(define *Define) error {
	v.as = v.AS
	if v.Plugin.AttrType == "config-file" {
		cmap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      util.NewUUID(),
				Namespace: v.as.GetNamespace(),
				Labels:    v.as.GetCommonLabels(),
			},
			Data: make(map[string]string),
		}
		cmap.Data[path.Base(v.Plugin.VolumePath)] = util.ParseVariable(v.Plugin.FileContent, map[string]string{})
		v.as.SetConfigMap(cmap)
		mode := int32(777)
		define.SetVolumeCMap(cmap, path.Base(v.Plugin.VolumePath), v.Plugin.VolumePath, false, &mode)
	}
	if v.Plugin.AttrType == "storage" {
		volumeMountName := fmt.Sprintf("plugin-%v-%v", v.PluginID, v.Plugin.VolumeName)
		volumeMountPath := v.Plugin.VolumePath
		volumeReadOnly := false
		var vm *corev1.VolumeMount
		annotations := map[string]string{"volume_name": v.Plugin.VolumeName}
		labels := v.as.GetCommonLabels(map[string]string{"volume_name": volumeMountName, "VolumeName": v.Plugin.VolumeName, "pluginID": v.PluginID})
		claim := newVolumeClaim(volumeMountName, volumeMountPath, "RWX", v1.RainbondStatefuleShareStorageClass, 0, labels, annotations)
		v.as.SetClaim(claim)
		if v.as.GetStatefulSet() == nil {
			v.as.SetClaimManually(claim)
		}
		vo := corev1.Volume{Name: volumeMountName}
		vo.PersistentVolumeClaim = &corev1.PersistentVolumeClaimVolumeSource{ClaimName: claim.GetName(), ReadOnly: volumeReadOnly}
		define.volumes = append(define.volumes, vo)
		vm = &corev1.VolumeMount{
			Name:      volumeMountName,
			MountPath: volumeMountPath,
			ReadOnly:  volumeReadOnly,
		}
		v.generateVolumeSubPath(define, vm)
		define.volumeMounts = append(define.volumeMounts, *vm)
	}
	return nil
}

// CreateDependVolume create depend volume
func (v *PluginStorageVolume) CreateDependVolume(define *Define) error {
	return nil
}

func (v *PluginStorageVolume) generateVolumeSubPath(define *Define, vm *corev1.VolumeMount) *corev1.VolumeMount {
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
			newVolumes := v.deleteVolume(define.volumes, delClaim)
			define.volumes = newVolumes
		}
		vm.Name = existClaimName
		subPathExpr := path.Join(fmt.Sprintf("tenant/%s/service/%s/", v.as.TenantID, v.as.ServiceID), vm.MountPath, "/$(POD_NAME)")
		vm.SubPathExpr = subPathExpr
		return vm
	}

	for _, delClaim := range needDeleteClaim {
		newVolumes := v.deleteVolume(define.volumes, delClaim)
		define.volumes = newVolumes
	}
	vm.Name = existClaimName
	vm.SubPath = path.Join(fmt.Sprintf("tenant/%s/service/%s/", v.as.TenantID, v.as.ServiceID), vm.MountPath)
	return vm
}

func (v *PluginStorageVolume) deleteVolume(claims []corev1.Volume, delClaim corev1.PersistentVolumeClaim) []corev1.Volume {
	newClaims := claims
	for i, claim := range claims {
		if claim.Name == delClaim.GetName() {
			newClaims = append(newClaims[0:i], newClaims[i+1:]...)
		}
	}
	return newClaims
}
