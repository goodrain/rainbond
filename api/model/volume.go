// Copyright (C) 2014-2018 Goodrain Co., Ltd.
// RAINBOND, Application Management Platform

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

package model

// VolumeBestReqStruct request for volumebest
type VolumeBestReqStruct struct {
	VolumeType   string `json:"volume_type" validate:"volume_type|required|in:share-file,local,memoryfs,config-file,ceph-rbd"`
	AccessMode   string `json:"access_mode"`
	SharePolicy  string `json:"share_policy"`
	BackupPolicy string `json:"backup_policy"`
}

// VolumeBestRespStruct response for volumebest
type VolumeBestRespStruct struct {
	Changed    bool   `json:"changed"`
	VolumeType string `json:"volume_type"`
}

// VolumeOptionsStruct volume option struct
type VolumeOptionsStruct struct {
	VolumeType           string                 `json:"volume_type"`
	NameShow             string                 `json:"name_show"`
	VolumeProviderName   string                 `json:"volume_provider_name"`
	CapacityValidation   map[string]interface{} `json:"capacity_validation"`
	Description          string                 `json:"description"`
	AccessMode           []string               `json:"access_mode"`
	SharePolicy          []string               `json:"share_policy"`           //共享模式
	BackupPolicy         []string               `json:"backup_policy"`          // 备份策略
	ReclaimPolicy        string                 `json:"reclaim_policy"`         // 回收策略,delete, retain, recyle
	VolumeBindingMode    string                 `json:"volume_binding_mode"`    // 绑定模式,Immediate,WaitForFirstConsumer
	AllowVolumeExpansion *bool                  `json:"allow_volume_expansion"` // 是否支持扩展
	Sort                 int                    `json:"sort"`                   // 排序
}

// VolumeProviderDetail volume provider detail
// Attention accessMode/sharerPolicy/backupPolicy都是结合业务进行添加字段，需自己补充
// Provisioner/reclaimPolicy/volumeBindingMode/allowVolumeExpansion为StorageClass内置参数
type VolumeProviderDetail struct {
	Name                 string   `json:"name"`                   //StorageClass名字
	Provisioner          string   `json:"provisioner"`            //提供者，如ceph.com/rbd、kubernetes.io/rbd
	VolumeBindingMode    string   `json:"volume_binding_mode"`    // 绑定模式,Immediate,WaitForFirstConsumer
	AllowVolumeExpansion *bool    `json:"allow_volume_expansion"` // 是否支持扩展
	AccessMode           []string `json:"access_mode"`            // 读写模式（Important! A volume can only be mounted using one access mode at a time, even if it supports many. For example, a GCEPersistentDisk can be mounted as ReadWriteOnce by a single node or ReadOnlyMany by many nodes, but not at the same time. #https://kubernetes.io/docs/concepts/storage/persistent-volumes/#access-modes）
	SharePolicy          []string `json:"share_policy"`           //共享模式
	BackupPolicy         []string `json:"backup_policy"`          // 备份策略
	ReclaimPolicy        string   `json:"reclaim_policy"`         // 回收策略,delete, retain, recyle
}

//AddVolumeStruct AddVolumeStruct
//swagger:parameters addVolumes
type AddVolumeStruct struct {
	// in: path
	// required: true
	TenantName string `json:"tenant_name"`
	// in: path
	// required: true
	ServiceAlias string `json:"service_alias"`
	// in: body
	Body struct {
		// 类型 "application;app_publish"
		// in: body
		// required: true
		Category string `json:"category"`
		// 容器挂载目录
		// in: body
		// required: true
		VolumePath string `json:"volume_path" validate:"volume_path|required|regex:^/"`
		//存储类型（share,local,tmpfs）
		// in: body
		// required: true
		VolumeType string `json:"volume_type" validate:"volume_type|required"`
		// 存储名称(同一个应用唯一)
		// in: body
		// required: true
		VolumeName  string `json:"volume_name" validate:"volume_name|required|max:50"`
		FileContent string `json:"file_content"`
		// 存储驱动别名（StorageClass别名）
		VolumeProviderName string `json:"volume_provider_name"`
		IsReadOnly         bool   `json:"is_read_only"`
		// VolumeCapacity 存储大小
		VolumeCapacity int64 `json:"volume_capacity"` // 单位: Mi
		// AccessMode 读写模式（Important! A volume can only be mounted using one access mode at a time, even if it supports many. For example, a GCEPersistentDisk can be mounted as ReadWriteOnce by a single node or ReadOnlyMany by many nodes, but not at the same time. #https://kubernetes.io/docs/concepts/storage/persistent-volumes/#access-modes）
		AccessMode string `json:"access_mode"`
		// SharePolicy 共享模式
		SharePolicy string `json:"share_policy"`
		// BackupPolicy 备份策略
		BackupPolicy string `json:"backup_policy"`
		// ReclaimPolicy 回收策略
		ReclaimPolicy string `json:"reclaim_policy"`
		// AllowExpansion 是否支持扩展
		AllowExpansion bool `json:"allow_expansion"`
	}
}

//DeleteVolumeStruct DeleteVolumeStruct
//swagger:parameters deleteVolumes
type DeleteVolumeStruct struct {
	// in: path
	// required: true
	TenantName string `json:"tenant_name"`
	// in: path
	// required: true
	ServiceAlias string `json:"service_alias"`
	// 存储名称
	// in: path
	// required: true
	VolumeName string `json:"volume_name"`
}

//AddVolumeDependencyStruct AddVolumeDependencyStruct
//swagger:parameters addDepVolume
type AddVolumeDependencyStruct struct {
	// in: path
	// required: true
	TenantName string `json:"tenant_name"`
	// in: path
	// required: true
	ServiceAlias string `json:"service_alias"`
	// in: body
	Body struct {
		// 依赖的服务id
		// in: body
		// required: true
		DependServiceID string `json:"depend_service_id"  validate:"depend_service_id|required"`
		// 容器挂载目录
		// in: body
		// required: true
		VolumePath string `json:"volume_path" validate:"volume_path|required|regex:^/"`
		// 依赖存储名称
		// in: body
		// required: true
		VolumeName string `json:"volume_name" validate:"volume_name|required|max:50"`

		VolumeType string `json:"volume_type" validate:"volume_type|required|in:share-file,config-file"`
	}
}

//DeleteVolumeDependencyStruct DeleteVolumeDependencyStruct
//swagger:parameters  delDepVolume
type DeleteVolumeDependencyStruct struct {
	// in: path
	// required: true
	TenantName string `json:"tenant_name"`
	// in: path
	// required: true
	ServiceAlias string `json:"service_alias"`
	// in: body
	Body struct {
		// 依赖的服务id
		// in: body
		// required: true
		DependServiceID string `json:"depend_service_id" validate:"depend_service_id|required|max:32"`
		// 依赖存储名称
		// in: body
		// required: true
		VolumeName string `json:"volume_name" validate:"volume_name|required|max:50"`
	}
}

//以下为v2旧版API参数定义

//V2AddVolumeStruct AddVolumeStruct
//swagger:parameters addVolume
type V2AddVolumeStruct struct {
	// in: path
	// required: true
	TenantName string `json:"tenant_name"`
	// in: path
	// required: true
	ServiceAlias string `json:"service_alias"`
	// in: body
	Body struct {
		// 类型 "application;app_publish"
		// in: body
		// required: true
		Category string `json:"category"`
		// 容器挂载目录
		// in: body
		// required: true
		VolumePath string `json:"volume_path" validate:"volume_path|required|regex:^/"`
		// 宿主机挂载目录
		// in: body
		// required: true
		HostPath string `json:"host_path" validate:"volume_path|required|regex:^/"`
		//存储驱动名称
		VolumeProviderName string `json:"volume_provider_name"`
		// 存储大小
		VolumeCapacity int64 `json:"volume_capacity" validate:"volume_capacity|required|min:1"` // 单位Mi
		// AccessMode 读写模式（Important! A volume can only be mounted using one access mode at a
		AccessMode string `gorm:"column:access_mode" json:"access_mode"`
		// SharePolicy 共享模式
		SharePolicy string `gorm:"column:share_policy" json:"share_policy"`
		// BackupPolicy 备份策略
		BackupPolicy string `gorm:"column:backup_policy" json:"backup_policy"`
		// ReclaimPolicy 回收策略
		ReclaimPolicy string `json:"reclaim_policy"`
		// AllowExpansion 是否支持扩展
		AllowExpansion bool `gorm:"column:allow_expansion" json:"allow_expansion"`
	}
}

//V2DelVolumeStruct AddVolumeStruct
//swagger:parameters deleteVolume
type V2DelVolumeStruct struct {
	// in: path
	// required: true
	TenantName string `json:"tenant_name"`
	// in: path
	// required: true
	ServiceAlias string `json:"service_alias"`
	// in: body
	Body struct {
		// 类型 "application;app_publish"
		// in: body
		// required: true
		Category string `json:"category"`
		// 容器挂载目录
		// in: body
		// required: true
		VolumePath string `json:"volume_path" validate:"volume_path|required|regex:^/"`
	}
}

//V2AddVolumeDependencyStruct AddVolumeDependencyStruct
//swagger:parameters addVolumeDependency
type V2AddVolumeDependencyStruct struct {
	// in: path
	// required: true
	TenantName string `json:"tenant_name"`
	// in: path
	// required: true
	ServiceAlias string `json:"service_alias"`
	// in: body
	Body struct {
		// 依赖的服务id
		// in: body
		// required: true
		DependServiceID string `json:"depend_service_id"  validate:"depend_service_id|required"`
		// 挂载目录
		// in: body
		// required: true
		MntDir string `json:"mnt_dir" validate:"mnt_dir|required"`
		// 挂载容器内目录名称
		// in: body
		// required: true
		MntName string `json:"mnt_name" validate:"mnt_name|required"`
	}
}

//V2DelVolumeDependencyStruct V2DelVolumeDependencyStruct
//swagger:parameters deleteVolumeDependency
type V2DelVolumeDependencyStruct struct {
	// in: path
	// required: true
	TenantName string `json:"tenant_name"`
	// in: path
	// required: true
	ServiceAlias string `json:"service_alias"`
	// in: body
	Body struct {
		// 依赖的服务id
		// in: body
		// required: true
		DependServiceID string `json:"depend_service_id"  validate:"depend_service_id|required"`
	}
}

// UpdVolumeReq is a value struct holding request for updating volume.
type UpdVolumeReq struct {
	VolumeName  string `json:"volume_name" validate:"required"`
	VolumeType  string `json:"volume_type" validate:"volume_type|required|in:share-file,local,memoryfs,config-file,alicloud-disk"`
	FileContent string `json:"file_content"`
	VolumePath  string `json:"volume_path"`
}

// VolumeWithStatusResp volume status
type VolumeWithStatusResp struct {
	ServiceID string `json:"service_id"`
	//存储名称
	Status map[string]string `json:"status"`
}
