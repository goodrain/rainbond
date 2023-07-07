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

// TenantServiceVolumeType tenant service volume type
type TenantServiceVolumeType struct {
	Model
	VolumeType         string `gorm:"column:volume_type; size:64" json:"volume_type"`
	NameShow           string `gorm:"column:name_show; size:64" json:"name_show"`
	CapacityValidation string `gorm:"column:capacity_validation; size:1024" json:"capacity_validation"`
	Description        string `gorm:"column:description; size:1024" json:"description"`
	AccessMode         string `gorm:"column:access_mode; size:128" json:"access_mode"`
	BackupPolicy       string `gorm:"column:backup_policy; size:128" json:"backup_policy"`
	ReclaimPolicy      string `gorm:"column:reclaim_policy; size:20" json:"reclaim_policy"`
	SharePolicy        string `gorm:"share_policy; size:128" json:"share_policy"`
	Provisioner        string `gorm:"provisioner; size:128" json:"provisioner"`
	StorageClassDetail string `gorm:"storage_class_detail; type:longtext" json:"storage_class_detail"`
	Sort               int    `gorm:"sort; default:9999" json:"sort"`
	Enable             bool   `gorm:"enable" json:"enable"`
}

// TableName 表名
func (t *TenantServiceVolumeType) TableName() string {
	return "tenant_services_volume_type"
}
