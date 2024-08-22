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

// 本文件定义了 Rainbond 平台中的存储卷（Volume）管理模块，包括各种类型存储卷的创建、配置和管理操作。
// 存储卷是 Rainbond 应用服务的重要组成部分，用于在容器中持久化数据或提供配置文件。

// 文件内容主要包括以下几个部分：
// 1. `Volume` 接口：该接口定义了存储卷的基本操作，包括创建卷（CreateVolume）和创建依赖卷（CreateDependVolume）。
//    所有具体的存储卷类型都需要实现这个接口。

// 2. `NewVolumeManager` 函数：该函数是存储卷管理器的构造函数，根据传入的存储卷类型创建相应的存储卷对象。
//    不同类型的存储卷包括共享文件卷（ShareFileVolume）、配置文件卷（ConfigFileVolume）、内存文件系统卷（MemoryFSVolume）、
//    本地卷（LocalVolume）、插件存储卷（PluginStorageVolume）、NFS 卷（NFSVolume）等。

// 3. `Base` 结构体：这是一个基础结构体，存储了与应用服务和存储卷相关的基本信息，例如应用服务对象、存储卷模型、
//    版本信息和数据库管理器等。所有具体的存储卷类型都继承自这个基础结构体，并通过 `setBaseInfo` 方法进行初始化。

// 4. `Define` 结构体：该结构体用于定义应用服务的存储卷，包括卷的挂载路径、卷对象和虚拟机卷等。
//    通过该结构体，Rainbond 可以统一管理和访问应用服务的存储卷信息。

// 5. `newVolumeClaim` 函数：该函数用于创建 Kubernetes 中的 `PersistentVolumeClaim` 对象。
//    该对象表示对持久化存储资源的请求，包含存储卷的名称、访问模式、存储类和资源请求等信息。

// 6. `SetVolume` 和 `SetVolumeCMap` 方法：这些方法用于在 `Define` 结构体中设置存储卷和配置卷。
//    `SetVolume` 方法支持内存文件系统卷和共享文件卷的设置，而 `SetVolumeCMap` 方法则用于设置配置卷，
//    其中配置卷的数据来源于 Kubernetes 中的 ConfigMap 对象。

// 7. `convertRulesToEnvs` 函数：该函数用于将 HTTP 规则转换为环境变量，
//    并将这些环境变量注入到应用服务的容器中。通过环境变量，应用服务可以访问与其端口相关的域名和协议信息。

// 8. `RewriteHostPathInWindows` 和 `RewriteContainerPathInWindows` 函数：
//    这些函数用于在 Windows 环境中重写主机路径和容器路径，以适应 Windows 的路径格式。

// 通过这些结构体和方法，Rainbond 平台能够灵活地创建和管理应用服务的存储卷，
// 满足不同类型服务的持久化存储需求，并支持在不同操作系统环境中的兼容性。

package volume

import (
	"fmt"
	kubevirtv1 "kubevirt.io/api/core/v1"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/model"
	dbmodel "github.com/goodrain/rainbond/db/model"
	v1 "github.com/goodrain/rainbond/worker/appm/types/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Volume volume function interface
type Volume interface {
	CreateVolume(define *Define) error       // use serviceVolume
	CreateDependVolume(define *Define) error // use serviceMountR
	setBaseInfo(as *v1.AppService, serviceVolume *model.TenantServiceVolume, serviceMountR *model.TenantServiceMountRelation, version *dbmodel.VersionInfo, dbmanager db.Manager)
}

// NewVolumeManager create volume
func NewVolumeManager(as *v1.AppService,
	serviceVolume *model.TenantServiceVolume,
	serviceMountR *model.TenantServiceMountRelation,
	version *dbmodel.VersionInfo,
	envs []corev1.EnvVar,
	envVarSecrets []*corev1.Secret,
	dbmanager db.Manager) Volume {
	var v Volume
	volumeType := ""
	if serviceVolume != nil {
		volumeType = serviceVolume.VolumeType
	}
	if serviceMountR != nil {
		volumeType = serviceMountR.VolumeType
		as.SharedStorageClass = ""
	}
	if volumeType == "" {
		logrus.Warn("unknown volume Type, can't create volume")
		return nil
	}
	switch volumeType {
	case dbmodel.ShareFileVolumeType.String():
		v = new(ShareFileVolume)
		css := as.SharedStorageClass
		if css != "" {
			v = new(OtherVolume)
		}
	case dbmodel.ConfigFileVolumeType.String():
		v = &ConfigFileVolume{envs: envs, envVarSecrets: envVarSecrets}
	case dbmodel.VMVolumeType.String():
		v = new(ShareFileVolume)
	case dbmodel.MemoryFSVolumeType.String():
		v = new(MemoryFSVolume)
	case dbmodel.LocalVolumeType.String():
		v = new(LocalVolume)
	case dbmodel.PluginStorageType.String():
		v = new(PluginStorageVolume)
	case dbmodel.NFSdVolumeType.String():
		v = new(NFSVolume)
	default:
		logrus.Warnf("other volume type[%s]", volumeType)
		v = new(OtherVolume)
	}
	v.setBaseInfo(as, serviceVolume, serviceMountR, version, dbmanager)
	return v
}

// Base volume base
type Base struct {
	as        *v1.AppService
	svm       *model.TenantServiceVolume
	smr       *model.TenantServiceMountRelation
	version   *dbmodel.VersionInfo
	dbmanager db.Manager
}

func (b *Base) setBaseInfo(as *v1.AppService, serviceVolume *model.TenantServiceVolume, serviceMountR *model.TenantServiceMountRelation, version *dbmodel.VersionInfo, dbmanager db.Manager) {
	b.as = as
	b.svm = serviceVolume
	b.smr = serviceMountR
	b.version = version
	b.dbmanager = dbmanager
}

func prepare() {
	// TODO prepare volume info, create volume just create volume and return volumeMount, do not process anything else
}

func newVolumeClaim(name, volumePath, accessMode, storageClassName string, capacity int64, labels, annotations map[string]string) *corev1.PersistentVolumeClaim {
	logrus.Debugf("volume annotaion is %+v", annotations)
	if capacity == 0 {
		logrus.Warnf("claim[%s] capacity is 0, set 10G default", name)
		capacity = 10
	}
	resourceStorage, _ := resource.ParseQuantity(fmt.Sprintf("%dGi", capacity)) // 统一单位使用G
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Labels:      labels,
			Annotations: annotations,
			Namespace:   "string",
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes:      []corev1.PersistentVolumeAccessMode{parseAccessMode(accessMode)},
			StorageClassName: &storageClassName,
			Resources: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceStorage: resourceStorage,
				},
			},
		},
	}
}

/*
RWO - ReadWriteOnce
ROX - ReadOnlyMany
RWX - ReadWriteMany
*/
func parseAccessMode(accessMode string) corev1.PersistentVolumeAccessMode {
	accessMode = strings.ToUpper(accessMode)
	switch accessMode {
	case "RWO":
		return corev1.ReadWriteOnce
	case "ROX":
		return corev1.ReadOnlyMany
	case "RWX":
		return corev1.ReadWriteMany
	default:
		return corev1.ReadWriteOnce
	}
}

// Define define volume
type Define struct {
	as           *v1.AppService
	volumeMounts []corev1.VolumeMount
	volumes      []corev1.Volume
	vmVolume     []kubevirtv1.Volume
	vmDisk       []kubevirtv1.Disk
}

// GetVolumes get define volumes
func (v *Define) GetVolumes() []corev1.Volume {
	return v.volumes
}

// GetVMVolume get define vm volumes
func (v *Define) GetVMVolume() []kubevirtv1.Volume {
	return v.vmVolume
}

// GetVMDisk get define vm devices
func (v *Define) GetVMDisk() []kubevirtv1.Disk {
	return v.vmDisk
}

// GetVolumeMounts get define volume mounts
func (v *Define) GetVolumeMounts() []corev1.VolumeMount {
	return v.volumeMounts
}

// SetVolume define set volume
func (v *Define) SetVolume(VolumeType dbmodel.VolumeType, name, mountPath, hostPath string, hostPathType corev1.HostPathType, readOnly bool) {
	for _, m := range v.volumeMounts {
		if m.MountPath == mountPath {
			return
		}
	}
	switch VolumeType {
	case dbmodel.MemoryFSVolumeType:
		vo := corev1.Volume{Name: name}
		// V5.2 do not use memory as medium of emptyDir
		vo.EmptyDir = &corev1.EmptyDirVolumeSource{}
		v.volumes = append(v.volumes, vo)
		if mountPath != "" {
			vm := corev1.VolumeMount{
				MountPath: mountPath,
				Name:      name,
				ReadOnly:  readOnly,
				SubPath:   "",
			}
			v.volumeMounts = append(v.volumeMounts, vm)
		}
	case dbmodel.ShareFileVolumeType:
		if hostPath != "" {
			vo := corev1.Volume{
				Name: name,
			}
			vo.HostPath = &corev1.HostPathVolumeSource{
				Path: hostPath,
				Type: &hostPathType,
			}
			v.volumes = append(v.volumes, vo)
			if mountPath != "" {
				vm := corev1.VolumeMount{
					MountPath: mountPath,
					Name:      name,
					ReadOnly:  readOnly,
					SubPath:   "",
				}
				v.volumeMounts = append(v.volumeMounts, vm)
			}
		}
	case dbmodel.LocalVolumeType:
		//no support
		return
	}
}

// SetVolumeCMap sets volumes and volumeMounts. The type of volumes is configMap.
func (v *Define) SetVolumeCMap(cmap *corev1.ConfigMap, k, p string, isReadOnly bool, mode *int32) {
	vm := corev1.VolumeMount{
		MountPath: p,
		Name:      cmap.Name,
		ReadOnly:  false,
		SubPath:   path.Base(p),
	}
	v.volumeMounts = append(v.volumeMounts, vm)
	var defaultMode int32 = 0777
	if mode != nil {
		// convert int to octal
		octal, _ := strconv.ParseInt(strconv.Itoa(int(*mode)), 8, 64)
		defaultMode = int32(octal)
	}
	vo := corev1.Volume{
		Name: cmap.Name,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: cmap.Name,
				},
				DefaultMode: &defaultMode,
				Items: []corev1.KeyToPath{
					{
						Key:  k,
						Path: path.Base(p), // subpath
						Mode: &defaultMode,
					},
				},
			},
		},
	}
	v.volumes = append(v.volumes, vo)
}

func convertRulesToEnvs(as *v1.AppService, dbmanager db.Manager, ports []*dbmodel.TenantServicesPort) (re []corev1.EnvVar) {
	defDomain := fmt.Sprintf(".%s.%s.", as.ServiceAlias, as.TenantName)
	httpRules, _ := dbmanager.HTTPRuleDao().ListByServiceID(as.ServiceID)
	portDomainEnv := make(map[int][]corev1.EnvVar)
	portProtocolEnv := make(map[int][]corev1.EnvVar)
	for i := range httpRules {
		rule := httpRules[i]
		portDomainEnv[rule.ContainerPort] = append(portDomainEnv[rule.ContainerPort], corev1.EnvVar{
			Name:  fmt.Sprintf("DOMAIN_%d", rule.ContainerPort),
			Value: rule.Domain,
		})
		portProtocolEnv[rule.ContainerPort] = append(portProtocolEnv[rule.ContainerPort], corev1.EnvVar{
			Name: fmt.Sprintf("DOMAIN_PROTOCOL_%d", rule.ContainerPort),
			Value: func() string {
				if rule.CertificateID != "" {
					return "https"
				}
				return "http"
			}(),
		})
	}
	var portInts []int
	for _, port := range ports {
		if *port.IsOuterService {
			portInts = append(portInts, port.ContainerPort)
		}
	}
	sort.Ints(portInts)
	var gloalDomain, gloalDomainProcotol string
	var firstDomain, firstDomainProcotol string
	for _, p := range portInts {
		if len(portDomainEnv[p]) == 0 {
			continue
		}
		var portDomain, portDomainProcotol string
		for i, renv := range portDomainEnv[p] {
			//custom http rule
			if !strings.Contains(renv.Value, defDomain) {
				if gloalDomain == "" {
					gloalDomain = renv.Value
					gloalDomainProcotol = portProtocolEnv[p][i].Value
				}
				portDomain = renv.Value
				portDomainProcotol = portProtocolEnv[p][i].Value
				break
			}
			if firstDomain == "" {
				firstDomain = renv.Value
				firstDomainProcotol = portProtocolEnv[p][i].Value
			}
		}
		if portDomain == "" {
			portDomain = portDomainEnv[p][0].Value
			portDomainProcotol = portProtocolEnv[p][0].Value
		}
		re = append(re, corev1.EnvVar{
			Name:  fmt.Sprintf("DOMAIN_%d", p),
			Value: portDomain,
		})
		re = append(re, corev1.EnvVar{
			Name:  fmt.Sprintf("DOMAIN_PROTOCOL_%d", p),
			Value: portDomainProcotol,
		})
	}
	if gloalDomain == "" {
		gloalDomain = firstDomain
		gloalDomainProcotol = firstDomainProcotol
	}
	if gloalDomain != "" {
		re = append(re, corev1.EnvVar{
			Name:  "DOMAIN",
			Value: gloalDomain,
		})
		re = append(re, corev1.EnvVar{
			Name:  "DOMAIN_PROTOCOL",
			Value: gloalDomainProcotol,
		})
	}
	return
}

// RewriteHostPathInWindows rewrite host path
func RewriteHostPathInWindows(hostPath string) string {
	localPath := os.Getenv("LOCAL_DATA_PATH")
	sharePath := os.Getenv("SHARE_DATA_PATH")
	if localPath == "" {
		localPath = "/grlocaldata"
	}
	if sharePath == "" {
		sharePath = "/grdata"
	}
	hostPath = strings.Replace(hostPath, "/grdata", `z:`, 1)
	hostPath = strings.Replace(hostPath, "/", `\`, -1)
	return hostPath
}

// RewriteContainerPathInWindows mount path in windows
func RewriteContainerPathInWindows(mountPath string) string {
	if mountPath == "" {
		return ""
	}
	if mountPath[0] == '/' {
		mountPath = `c:\` + mountPath[1:]
	}
	mountPath = strings.Replace(mountPath, "/", `\`, -1)
	return mountPath
}
