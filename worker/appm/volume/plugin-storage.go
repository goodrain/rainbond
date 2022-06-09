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

//PluginStorageVolume 插件新增存储
type PluginStorageVolume struct {
	Plugin api_model.PluginStorage
	Base
	PluginID string
	AS       *v1.AppService
}

//CreateVolume 创建插件存储或配置文件
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
