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

// VolumeTypeOptionsStruct volume option struct
type VolumeTypeOptionsStruct struct {
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
