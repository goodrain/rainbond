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

// VolumeProviderStruct volume provider struct
type VolumeProviderStruct struct {
	Kind        string                 `json:"kind"`
	Provisioner []VolumeProviderDetail `json:"provisioner"`
}

// VolumeProviderDetail volume provider detail
type VolumeProviderDetail struct {
	Name                 string `json:"name"`
	Provisioner          string `json:"provisioner"`
	ReclaimPolicy        string `json:"reclaim_policy"`
	VolumeBindingMode    string `json:"volume_binding_mode"`
	AllowVolumeExpansion *bool  `json:"allow_volume_expansion"`
	// TODO AccessMode
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
		VolumeType string `json:"volume_type" validate:"volume_type|required|in:share-file,local,memoryfs,config-file,ceph-rbd"`
		// 存储名称(同一个应用唯一)
		// in: body
		// required: true
		VolumeName  string `json:"volume_name" validate:"volume_name|required|max:50"`
		FileContent string `json:"file_content"`
		// 存储驱动别名（StorageClass别名）
		VolumeProviderName string `json:"volume_provider_name"`
		IsReadOnly         bool   `json:"is_read_only"`
		// VolumeCapacity 存储大小
		VolumeCapacity int64 `json:"volume_capacity"` // TODO 单位
		// AccessMode 读写模式
		AccessMode string `json:"access_mode"`
		// SharePolicy 共享模式
		SharePolicy string `json:"share_policy"`
		// BackupPolicy 备份策略
		BackupPolicy string `json:"backup_policy"`
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
	VolumeType  string `json:"volume_type" validate:"volume_type|required|in:share-file,local,memoryfs,config-file"`
	FileContent string `json:"file_content"`
	VolumePath  string `json:"volume_path"`
}
