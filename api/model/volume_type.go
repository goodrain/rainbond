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

// VolumeTypeStruct volume option struct
type VolumeTypeStruct struct {
	VolumeType         string                 `json:"volume_type" validate:"volume_type|required"`
	NameShow           string                 `json:"name_show"`
	CapacityValidation map[string]interface{} `json:"capacity_validation"`
	Description        string                 `json:"description"`
	AccessMode         []string               `json:"access_mode"`    // 读写模式（Important! A volume can only be mounted using one access mode at a time, even if it supports many. For example, a GCEPersistentDisk can be mounted as ReadWriteOnce by a single node or ReadOnlyMany by many nodes, but not at the same time. #https://kubernetes.io/docs/concepts/storage/persistent-volumes/#access-modes）
	SharePolicy        []string               `json:"share_policy"`   //共享模式
	BackupPolicy       []string               `json:"backup_policy"`  // 备份策略
	ReclaimPolicy      string                 `json:"reclaim_policy"` // 回收策略,delete, retain, recyle
	Provisioner        string                 `json:"provisioner"`    //存储提供方
	StorageClassDetail map[string]interface{} `json:"storage_class_detail" validate:"storage_class_detail|required"`
	Sort               int                    `json:"sort"`   // 排序
	Enable             bool                   `json:"enable"` // 是否生效
}

// VolumeTypePageStruct volume option struct with page
type VolumeTypePageStruct struct {
	list     *VolumeTypeStruct
	page     int
	pageSize int
	count    int
}
