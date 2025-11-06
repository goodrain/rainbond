// Copyright (C) 2014-2024 Goodrain Co., Ltd.
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

import (
	"time"
)

// TenantGPUQuota 团队GPU配额模型
type TenantGPUQuota struct {
	TenantID       string    `gorm:"column:tenant_id;primary_key" json:"tenant_id"`
	GPULimit       int       `gorm:"column:gpu_limit" json:"gpu_limit"`                  // GPU卡数限制，0表示不限制
	GPUMemoryLimit int64     `gorm:"column:gpu_memory_limit" json:"gpu_memory_limit"`    // GPU显存限制(MB)，0表示不限制
	CreateTime     time.Time `gorm:"column:create_time;type:datetime" json:"create_time"`
	UpdateTime     time.Time `gorm:"column:update_time;type:datetime" json:"update_time"`
}

// TableName 返回表名
func (TenantGPUQuota) TableName() string {
	return "tenant_gpu_quota"
}
