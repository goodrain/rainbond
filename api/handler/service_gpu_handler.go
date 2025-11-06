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

// ServiceGPUHandler 组件GPU配置处理器
type ServiceGPUHandler interface {
	// GetServiceGPUConfig 获取组件GPU配置
	GetServiceGPUConfig(ctx context.Context, serviceID string) (*model.ServiceGPUConfigResp, error)
	// SetServiceGPUConfig 设置组件GPU配置
	SetServiceGPUConfig(ctx context.Context, tenantID, serviceID string, req *model.ServiceGPUConfigReq) error
	// DeleteServiceGPUConfig 删除组件GPU配置
	DeleteServiceGPUConfig(ctx context.Context, serviceID string) error
}

type serviceGPUHandler struct {
	tenantGPUHandler TenantGPUHandler
}

// NewServiceGPUHandler 创建组件GPU配置处理器
func NewServiceGPUHandler() ServiceGPUHandler {
	return &serviceGPUHandler{
		tenantGPUHandler: NewTenantGPUHandler(),
	}
}

// GetServiceGPUConfig 获取组件GPU配置
func (h *serviceGPUHandler) GetServiceGPUConfig(ctx context.Context, serviceID string) (*model.ServiceGPUConfigResp, error) {
	// 从数据库获取配置
	config, err := db.GetManager().TenantServiceGPUDao().GetByServiceID(serviceID)
	if err != nil {
		logrus.Errorf("Failed to get GPU config for service %s: %v", serviceID, err)
		return nil, fmt.Errorf("failed to get GPU config: %v", err)
	}

	// 如果没有配置，返回默认配置（GPU禁用）
	if config == nil {
		return &model.ServiceGPUConfigResp{
			ServiceID:          serviceID,
			EnableGPU:          false,
			GPUCount:           0,
			GPUMemory:          0,
			GPUCores:           0,
			GPUModelPreference: "",
		}, nil
	}

	// 转换为响应模型
	resp := &model.ServiceGPUConfigResp{
		ServiceID:          config.ServiceID,
		EnableGPU:          config.EnableGPU,
		GPUCount:           config.GPUCount,
		GPUMemory:          config.GPUMemory,
		GPUCores:           config.GPUCores,
		GPUModelPreference: config.GPUModelPreference,
		CreateTime:         config.CreateTime.Format(time.RFC3339),
		UpdateTime:         config.UpdateTime.Format(time.RFC3339),
	}

	return resp, nil
}

// SetServiceGPUConfig 设置组件GPU配置
func (h *serviceGPUHandler) SetServiceGPUConfig(ctx context.Context, tenantID, serviceID string, req *model.ServiceGPUConfigReq) error {
	// 参数验证
	if req.GPUCount < 0 {
		return fmt.Errorf("gpu_count must be non-negative")
	}
	if req.GPUMemory < 0 {
		return fmt.Errorf("gpu_memory must be non-negative")
	}
	if req.GPUCores < 0 || req.GPUCores > 100 {
		return fmt.Errorf("gpu_cores must be between 0 and 100")
	}

	// 如果启用GPU，验证是否超过团队配额
	if req.EnableGPU {
		// 获取团队配额
		quota, err := db.GetManager().TenantGPUQuotaDao().GetByTenantID(tenantID)
		if err != nil {
			logrus.Errorf("Failed to get team quota: %v", err)
			return fmt.Errorf("failed to get team quota: %v", err)
		}

		// 如果配额不为0，需要验证
		if quota != nil && (quota.GPULimit > 0 || quota.GPUMemoryLimit > 0) {
			// 获取团队所有启用GPU的服务配置
			allConfigs, err := db.GetManager().TenantServiceGPUDao().GetByTenantID(tenantID)
			if err != nil {
				logrus.Errorf("Failed to get team GPU configs: %v", err)
				return fmt.Errorf("failed to get team GPU configs: %v", err)
			}

			// 计算当前使用量（排除当前服务）
			usedGPU := 0
			usedMemory := int64(0)
			for _, cfg := range allConfigs {
				if cfg.ServiceID != serviceID && cfg.EnableGPU {
					usedGPU += cfg.GPUCount
					usedMemory += cfg.GPUMemory
				}
			}

			// 加上新的配置
			usedGPU += req.GPUCount
			usedMemory += req.GPUMemory

			// 验证配额
			if quota.GPULimit > 0 && usedGPU > quota.GPULimit {
				return fmt.Errorf("GPU quota exceeded: used %d, limit %d", usedGPU, quota.GPULimit)
			}
			if quota.GPUMemoryLimit > 0 && usedMemory > quota.GPUMemoryLimit {
				return fmt.Errorf("GPU memory quota exceeded: used %d MB, limit %d MB", usedMemory, quota.GPUMemoryLimit)
			}
		}
	}

	// 检查配置是否已存在
	existingConfig, err := db.GetManager().TenantServiceGPUDao().GetByServiceID(serviceID)
	if err != nil {
		logrus.Errorf("Failed to check existing config for service %s: %v", serviceID, err)
		return fmt.Errorf("failed to check existing config: %v", err)
	}

	now := time.Now()
	if existingConfig == nil {
		// 创建新配置
		newConfig := &dbmodel.TenantServiceGPU{
			ServiceID:          serviceID,
			EnableGPU:          req.EnableGPU,
			GPUCount:           req.GPUCount,
			GPUMemory:          req.GPUMemory,
			GPUCores:           req.GPUCores,
			GPUModelPreference: req.GPUModelPreference,
			CreateTime:         now,
			UpdateTime:         now,
		}

		if err := db.GetManager().TenantServiceGPUDao().Create(newConfig); err != nil {
			logrus.Errorf("Failed to create GPU config for service %s: %v", serviceID, err)
			return fmt.Errorf("failed to create GPU config: %v", err)
		}

		logrus.Infof("Created GPU config for service %s: EnableGPU=%v, GPU=%d, Memory=%dMB, Cores=%d",
			serviceID, req.EnableGPU, req.GPUCount, req.GPUMemory, req.GPUCores)
	} else {
		// 更新现有配置
		existingConfig.EnableGPU = req.EnableGPU
		existingConfig.GPUCount = req.GPUCount
		existingConfig.GPUMemory = req.GPUMemory
		existingConfig.GPUCores = req.GPUCores
		existingConfig.GPUModelPreference = req.GPUModelPreference
		existingConfig.UpdateTime = now

		if err := db.GetManager().TenantServiceGPUDao().Update(existingConfig); err != nil {
			logrus.Errorf("Failed to update GPU config for service %s: %v", serviceID, err)
			return fmt.Errorf("failed to update GPU config: %v", err)
		}

		logrus.Infof("Updated GPU config for service %s: EnableGPU=%v, GPU=%d, Memory=%dMB, Cores=%d",
			serviceID, req.EnableGPU, req.GPUCount, req.GPUMemory, req.GPUCores)
	}

	return nil
}

// DeleteServiceGPUConfig 删除组件GPU配置
func (h *serviceGPUHandler) DeleteServiceGPUConfig(ctx context.Context, serviceID string) error {
	if err := db.GetManager().TenantServiceGPUDao().Delete(serviceID); err != nil {
		logrus.Errorf("Failed to delete GPU config for service %s: %v", serviceID, err)
		return fmt.Errorf("failed to delete GPU config: %v", err)
	}

	logrus.Infof("Deleted GPU config for service %s", serviceID)
	return nil
}
