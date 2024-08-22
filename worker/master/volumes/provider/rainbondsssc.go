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

// 本文件实现了一个用于 Rainbond 平台的本地共享存储卷（Persistent Volume）提供者，主要功能是为 Rainbond 应用创建和管理持久化存储卷。

// 1. `rainbondssscProvisioner` 结构体：
//    - 该结构体实现了 `controller.Provisioner` 接口，用于在指定目录下创建存储卷，并返回代表存储卷的 PV（Persistent Volume）对象。
//    - 结构体包含 `pvDir` 属性，用于存储 PV 相关的目录路径，`name` 属性用于表示提供者的名称。

// 2. `NewRainbondssscProvisioner` 函数：
//    - 该函数根据环境变量创建并返回一个 `rainbondssscProvisioner` 实例。
//    - 如果环境变量 `ALLINONE_MODE` 设置为 `true`，则使用 Rancher 的 `local-path` 作为提供者名称，否则使用 Rainbond 自定义的 `provisioner-sssc` 名称。

// 3. `Provision` 方法：
//    - 该方法用于创建持久化存储卷，并返回对应的 PV 对象。
//    - 它首先根据 PVC 的标签信息获取租户 ID 和服务 ID，并根据这些信息生成存储路径（`hostpath`），如果路径不存在，则创建该路径。
//    - 如果存储路径为空或不存在，则会根据不同的场景（如是否为无状态、是否有插件 ID）来决定路径的生成方式。
//    - 然后，使用生成的路径更新持久化存储卷源（`PersistentVolumeSource`），并构建 PV 对象，最后返回该对象。

// 4. `Delete` 方法：
//    - 该方法用于删除由 `Provision` 方法创建的持久化存储卷，目前未实现删除逻辑。

// 5. `Name` 方法：
//    - 该方法返回提供者的名称。

// 6. `getPodNameByPVCName` 函数：
//    - 该函数通过 PVC 名称解析并返回关联的 Pod 名称。

// 7. `getVolumeIDByPVCName` 函数：
//    - 该函数通过 PVC 名称解析并返回关联的卷 ID，如果解析失败则返回 0。

// 8. `updatePathForPersistentVolumeSource` 函数：
//    - 该函数根据生成的存储路径更新持久化存储卷源（`PersistentVolumeSource`），并返回更新后的持久化存储卷源。
//    - 支持多种存储卷源类型，如 NFS、Glusterfs、HostPath 等，并根据不同的存储类型对路径进行处理和转换。

// 总体而言，本文件的主要功能是通过实现自定义的存储卷提供者，在 Rainbond 平台上为应用创建和管理基于本地文件系统的共享存储卷。通过灵活处理不同类型的存储源，确保了存储卷的创建和管理过程高效且稳定。

package provider

import (
	"encoding/json"
	"fmt"
	api_model "github.com/goodrain/rainbond/api/model"
	"github.com/jinzhu/gorm"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/util"
	"github.com/goodrain/rainbond/worker/master/volumes/provider/lib/controller"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type rainbondssscProvisioner struct {
	// The directory to create PV-backing directories in
	pvDir string
	name  string
}

// NewRainbondssscProvisioner creates a new Rainbond statefulset share volume provisioner
func NewRainbondssscProvisioner() controller.Provisioner {
	sharePath := os.Getenv("SHARE_DATA_PATH")
	if sharePath == "" {
		sharePath = "/grdata"
	}
	if os.Getenv("ALLINONE_MODE") == "true" {
		return &rainbondssscProvisioner{
			pvDir: sharePath,
			name:  "rancher.io/local-path",
		}
	}
	return &rainbondssscProvisioner{
		pvDir: sharePath,
		name:  "rainbond.io/provisioner-sssc",
	}
}

var _ controller.Provisioner = &rainbondssscProvisioner{}

// Provision creates a storage asset and returns a PV object representing it.
func (p *rainbondssscProvisioner) Provision(options controller.VolumeOptions) (*v1.PersistentVolume, error) {
	logrus.Debugf("[rainbondssscProvisioner] start creating PV object. paramters: %+v", options.Parameters)

	tenantID := options.PVC.Labels["tenant_id"]
	serviceID := options.PVC.Labels["service_id"]
	_, stateless := options.PVC.Labels["stateless"]
	// v5.0.4 Previous versions
	hostpath := path.Join(p.pvDir, "tenant", tenantID, "service", serviceID, options.PVC.Name)
	pluginID, ok := options.PVC.Labels["pluginID"]
	// after v5.0.4,change host path
	// Directory path has nothing to do with volume ID
	// Directory path bound to volume mount path
	if util.DirIsEmpty(hostpath) {
		podName := getPodNameByPVCName(options.PVC.Name)
		volumeID := getVolumeIDByPVCName(options.PVC.Name)
		if volumeID != 0 {
			volume, err := db.GetManager().TenantServiceVolumeDao().GetVolumeByID(volumeID)
			if err != nil {
				logrus.Errorf("get volume by id %d failure %s", volumeID, err.Error())
				return nil, err
			}
			hostpath = volume.HostPath
			if !stateless {
				hostpath = path.Join(volume.HostPath, podName)
			}
		} else if ok {
			config, err := db.GetManager().TenantPluginVersionConfigDao().GetPluginConfig(serviceID, pluginID)
			if err != nil && err != gorm.ErrRecordNotFound {
				logrus.Errorf("get service plugin config from db failure %s", err.Error())
			}
			if config == nil {
				return nil, fmt.Errorf("can not parse volume id")
			}
			configStr := config.ConfigStr
			var oldConfig api_model.ResourceSpec
			if err := json.Unmarshal([]byte(configStr), &oldConfig); err == nil {
				for _, plugin := range oldConfig.BaseNormal.Options {
					var pluginStorage api_model.PluginStorage
					jsonValue, ok := plugin.(string)
					if ok {
						if err := json.Unmarshal([]byte(jsonValue), &pluginStorage); err == nil {
							if pluginStorage.VolumeName == options.PVC.Labels["VolumeName"] {
								hostpath = path.Join("/grdata/tenant/", tenantID, "service", serviceID, pluginStorage.VolumePath, podName)
							}
						}
					}
				}

			}
		} else {
			return nil, fmt.Errorf("can not parse volume id")
		}
	}
	if err := util.CheckAndCreateDirByMode(hostpath, 0777); err != nil {
		return nil, err
	}
	// new volume path
	persistentVolumeSource, err := updatePathForPersistentVolumeSource(&options.PersistentVolumeSource, hostpath)
	if err != nil {
		return nil, err
	}

	pv := &v1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:   options.PVName,
			Labels: options.PVC.Labels,
		},
		Spec: v1.PersistentVolumeSpec{
			PersistentVolumeReclaimPolicy: options.PersistentVolumeReclaimPolicy,
			AccessModes:                   options.PVC.Spec.AccessModes,
			Capacity: v1.ResourceList{
				v1.ResourceStorage: options.PVC.Spec.Resources.Requests[v1.ResourceStorage],
			},
			MountOptions:           options.MountOptions,
			PersistentVolumeSource: *persistentVolumeSource,
		},
	}
	logrus.Infof("create rainbondsssc pv %s for pvc %s", pv.Name, options.PVC.Name)
	return pv, nil
}

// Delete removes the storage asset that was created by Provision represented
// by the given PV.
func (p *rainbondssscProvisioner) Delete(volume *v1.PersistentVolume) error {

	return nil
}

func (p *rainbondssscProvisioner) Name() string {
	return p.name
}

func getPodNameByPVCName(pvcName string) string {
	pvcNames := strings.SplitN(pvcName, "-", 2)
	if len(pvcNames) == 2 {
		return pvcNames[1]
	}
	return pvcName
}

func getVolumeIDByPVCName(pvcName string) int {
	logrus.Debug("parse volume id from pvc name", pvcName)
	pvcNames := strings.SplitN(pvcName, "-", 2)
	// pvcNames 通常情况下为 "manual15-zk-zk-gr3cd1a1-0" 或 "manual6", 但是在使用 Helm 部署时，由于存储使用集群内部的 StorageClass
	// 并不是 rainbondsssc 或 rainbondsslc，所以此时的 pvcNames 可能是 "data-sonar-gra7c815-0", 此时就会触发切片越界，但实际上对于
	// 这类存储，应该交给 K8s 集群中的 StorageClass 处理
	if len(pvcNames) == 2 {
		if len(pvcNames[0]) > 6 {
			idStr := pvcNames[0][6:]
			id, _ := strconv.Atoi(idStr)
			return id
		}
		return 0
	}
	if strings.HasPrefix(pvcName, "manual") {
		idStr := strings.TrimPrefix(pvcName, "manual")
		id, _ := strconv.Atoi(idStr)
		return id
	}
	return 0
}

func updatePathForPersistentVolumeSource(persistentVolumeSource *v1.PersistentVolumeSource, hostpath string) (*v1.PersistentVolumeSource, error) {
	newPath := func(new string) string {
		p := strings.Replace(hostpath, "/grdata", "", 1)
		return path.Join(new, p)
	}
	source := &v1.PersistentVolumeSource{}
	switch {
	case persistentVolumeSource.NFS != nil:
		source.NFS = persistentVolumeSource.NFS
		source.NFS.Path = newPath(persistentVolumeSource.NFS.Path)
	case persistentVolumeSource.CSI != nil && persistentVolumeSource.CSI.Driver == "nasplugin.csi.alibabacloud.com":
		// convert aliyun nas to nfs
		if persistentVolumeSource.CSI.VolumeAttributes != nil {
			source.NFS = &v1.NFSVolumeSource{
				Server: persistentVolumeSource.CSI.VolumeAttributes["server"],
				Path:   newPath(persistentVolumeSource.CSI.VolumeAttributes["path"]),
			}
		}
	case persistentVolumeSource.Glusterfs != nil:
		//glusterfs:
		//	endpoints: glusterfs-cluster
		//	path: myVol1
		glusterfs := &v1.GlusterfsPersistentVolumeSource{
			EndpointsName:      persistentVolumeSource.Glusterfs.EndpointsName,
			EndpointsNamespace: persistentVolumeSource.Glusterfs.EndpointsNamespace,
			Path:               newPath(persistentVolumeSource.Glusterfs.Path),
		}
		source.Glusterfs = glusterfs
	case persistentVolumeSource.HostPath != nil:
		source.HostPath = &v1.HostPathVolumeSource{
			Path: newPath(persistentVolumeSource.HostPath.Path),
			Type: persistentVolumeSource.HostPath.Type,
		}
	case persistentVolumeSource.CSI != nil:
		source.CSI = persistentVolumeSource.CSI
	default:
		return nil, fmt.Errorf("unsupported persistence volume source")
	}
	return source, nil
}
