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

// TenantServiceGPU 组件GPU配置模型
type TenantServiceGPU struct {
	ServiceID          string    `gorm:"column:service_id;primary_key" json:"service_id"`
	EnableGPU          bool      `gorm:"column:enable_gpu" json:"enable_gpu"`                      // 是否启用GPU
	GPUCount           int       `gorm:"column:gpu_count" json:"gpu_count"`                        // GPU卡数
	GPUMemory          int64     `gorm:"column:gpu_memory" json:"gpu_memory"`                      // GPU显存(MB)
	GPUCores           int       `gorm:"column:gpu_cores" json:"gpu_cores"`                        // GPU算力百分比(0-100)
	GPUModelPreference string    `gorm:"column:gpu_model_preference" json:"gpu_model_preference"`  // GPU型号偏好，逗号分隔
	CreateTime         time.Time `gorm:"column:create_time;type:datetime" json:"create_time"`
	UpdateTime         time.Time `gorm:"column:update_time;type:datetime" json:"update_time"`
}

// TableName 返回表名
func (TenantServiceGPU) TableName() string {
	return "tenant_service_gpu"
}
