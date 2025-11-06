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

package handler

import (
	"context"
	"fmt"
	"time"

	"github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/sirupsen/logrus"
)

// TenantGPUHandler 团队GPU配额管理处理器
type TenantGPUHandler interface {
	// GetTenantGPUQuota 获取团队GPU配额
	GetTenantGPUQuota(ctx context.Context, tenantID string) (*model.TenantGPUQuotaResp, error)
	// SetTenantGPUQuota 设置团队GPU配额
	SetTenantGPUQuota(ctx context.Context, tenantID string, req *model.TenantGPUQuotaReq) error
	// GetTenantGPUUsage 获取团队GPU使用情况
	GetTenantGPUUsage(ctx context.Context, tenantID string) (*model.TenantGPUUsageResp, error)
}

type tenantGPUHandler struct{}

// NewTenantGPUHandler 创建团队GPU配额管理处理器
func NewTenantGPUHandler() TenantGPUHandler {
	return &tenantGPUHandler{}
}

// GetTenantGPUQuota 获取团队GPU配额
func (h *tenantGPUHandler) GetTenantGPUQuota(ctx context.Context, tenantID string) (*model.TenantGPUQuotaResp, error) {
	// 从数据库获取配额
	quota, err := db.GetManager().TenantGPUQuotaDao().GetByTenantID(tenantID)
	if err != nil {
		logrus.Errorf("Failed to get GPU quota for tenant %s: %v", tenantID, err)
		return nil, fmt.Errorf("failed to get GPU quota: %v", err)
	}

	// 如果没有配置，返回默认配额（0表示不限制）
	if quota == nil {
		return &model.TenantGPUQuotaResp{
			TenantID:       tenantID,
			GPULimit:       0,
			GPUMemoryLimit: 0,
		}, nil
	}

	// 转换为响应模型
	resp := &model.TenantGPUQuotaResp{
		TenantID:       quota.TenantID,
		GPULimit:       quota.GPULimit,
		GPUMemoryLimit: quota.GPUMemoryLimit,
		CreateTime:     quota.CreateTime.Format(time.RFC3339),
		UpdateTime:     quota.UpdateTime.Format(time.RFC3339),
	}

	return resp, nil
}

// SetTenantGPUQuota 设置团队GPU配额
func (h *tenantGPUHandler) SetTenantGPUQuota(ctx context.Context, tenantID string, req *model.TenantGPUQuotaReq) error {
	// 参数验证
	if req.GPULimit < 0 {
		return fmt.Errorf("gpu_limit must be non-negative")
	}
	if req.GPUMemoryLimit < 0 {
		return fmt.Errorf("gpu_memory_limit must be non-negative")
	}

	// 检查配额是否已存在
	existingQuota, err := db.GetManager().TenantGPUQuotaDao().GetByTenantID(tenantID)
	if err != nil {
		logrus.Errorf("Failed to check existing quota for tenant %s: %v", tenantID, err)
		return fmt.Errorf("failed to check existing quota: %v", err)
	}

	now := time.Now()
	if existingQuota == nil {
		// 创建新配额
		newQuota := &dbmodel.TenantGPUQuota{
			TenantID:       tenantID,
			GPULimit:       req.GPULimit,
			GPUMemoryLimit: req.GPUMemoryLimit,
			CreateTime:     now,
			UpdateTime:     now,
		}

		if err := db.GetManager().TenantGPUQuotaDao().Create(newQuota); err != nil {
			logrus.Errorf("Failed to create GPU quota for tenant %s: %v", tenantID, err)
			return fmt.Errorf("failed to create GPU quota: %v", err)
		}

		logrus.Infof("Created GPU quota for tenant %s: GPU=%d, Memory=%dMB", tenantID, req.GPULimit, req.GPUMemoryLimit)
	} else {
		// 更新现有配额
		existingQuota.GPULimit = req.GPULimit
		existingQuota.GPUMemoryLimit = req.GPUMemoryLimit
		existingQuota.UpdateTime = now

		if err := db.GetManager().TenantGPUQuotaDao().Update(existingQuota); err != nil {
			logrus.Errorf("Failed to update GPU quota for tenant %s: %v", tenantID, err)
			return fmt.Errorf("failed to update GPU quota: %v", err)
		}

		logrus.Infof("Updated GPU quota for tenant %s: GPU=%d, Memory=%dMB", tenantID, req.GPULimit, req.GPUMemoryLimit)
	}

	return nil
}

// GetTenantGPUUsage 获取团队GPU使用情况
func (h *tenantGPUHandler) GetTenantGPUUsage(ctx context.Context, tenantID string) (*model.TenantGPUUsageResp, error) {
	// 获取团队配额
	quota, err := db.GetManager().TenantGPUQuotaDao().GetByTenantID(tenantID)
	if err != nil {
		logrus.Errorf("Failed to get GPU quota for tenant %s: %v", tenantID, err)
		return nil, fmt.Errorf("failed to get GPU quota: %v", err)
	}

	// 默认配额（0表示不限制）
	gpuLimit := 0
	gpuMemoryLimit := int64(0)
	if quota != nil {
		gpuLimit = quota.GPULimit
		gpuMemoryLimit = quota.GPUMemoryLimit
	}

	// 获取团队所有启用GPU的服务配置
	serviceConfigs, err := db.GetManager().TenantServiceGPUDao().GetByTenantID(tenantID)
	if err != nil {
		logrus.Errorf("Failed to get GPU service configs for tenant %s: %v", tenantID, err)
		return nil, fmt.Errorf("failed to get GPU service configs: %v", err)
	}

	// 计算已使用的GPU资源
	usedGPU := 0
	usedGPUMemory := int64(0)

	for _, config := range serviceConfigs {
		if config.EnableGPU {
			usedGPU += config.GPUCount
			usedGPUMemory += config.GPUMemory
		}
	}

	// 计算使用率
	usageRate := 0.0
	if gpuLimit > 0 {
		// 如果配额不为0，计算GPU卡数使用率
		usageRate = float64(usedGPU) / float64(gpuLimit) * 100
	} else if gpuMemoryLimit > 0 {
		// 如果只限制显存，计算显存使用率
		usageRate = float64(usedGPUMemory) / float64(gpuMemoryLimit) * 100
	}

	// 限制使用率在0-100之间
	if usageRate > 100 {
		usageRate = 100
	}

	resp := &model.TenantGPUUsageResp{
		TenantID:       tenantID,
		UsedGPU:        usedGPU,
		UsedGPUMemory:  usedGPUMemory,
		GPULimit:       gpuLimit,
		GPUMemoryLimit: gpuMemoryLimit,
		UsageRate:      usageRate,
	}

	return resp, nil
}
