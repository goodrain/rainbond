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

package dao

import (
	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
	"github.com/sirupsen/logrus"
)

// TenantGPUQuotaDao 团队GPU配额数据访问接口
type TenantGPUQuotaDao interface {
	GetByTenantID(tenantID string) (*model.TenantGPUQuota, error)
	Create(quota *model.TenantGPUQuota) error
	Update(quota *model.TenantGPUQuota) error
	Delete(tenantID string) error
	ListAll() ([]*model.TenantGPUQuota, error)
}

// TenantGPUQuotaDaoImpl 团队GPU配额数据访问实现
type TenantGPUQuotaDaoImpl struct {
	DB *gorm.DB
}

// GetByTenantID 根据租户ID获取GPU配额
func (t *TenantGPUQuotaDaoImpl) GetByTenantID(tenantID string) (*model.TenantGPUQuota, error) {
	var quota model.TenantGPUQuota
	if err := t.DB.Where("tenant_id = ?", tenantID).First(&quota).Error; err != nil {
		if gorm.IsRecordNotFoundError(err) {
			return nil, nil
		}
		logrus.Errorf("get tenant gpu quota by tenant_id error: %v", err)
		return nil, err
	}
	return &quota, nil
}

// Create 创建GPU配额
func (t *TenantGPUQuotaDaoImpl) Create(quota *model.TenantGPUQuota) error {
	if err := t.DB.Create(quota).Error; err != nil {
		logrus.Errorf("create tenant gpu quota error: %v", err)
		return err
	}
	return nil
}

// Update 更新GPU配额
func (t *TenantGPUQuotaDaoImpl) Update(quota *model.TenantGPUQuota) error {
	if err := t.DB.Model(&model.TenantGPUQuota{}).
		Where("tenant_id = ?", quota.TenantID).
		Updates(map[string]interface{}{
			"gpu_limit":        quota.GPULimit,
			"gpu_memory_limit": quota.GPUMemoryLimit,
		}).Error; err != nil {
		logrus.Errorf("update tenant gpu quota error: %v", err)
		return err
	}
	return nil
}

// Delete 删除GPU配额
func (t *TenantGPUQuotaDaoImpl) Delete(tenantID string) error {
	if err := t.DB.Where("tenant_id = ?", tenantID).
		Delete(&model.TenantGPUQuota{}).Error; err != nil {
		logrus.Errorf("delete tenant gpu quota error: %v", err)
		return err
	}
	return nil
}

// ListAll 获取所有GPU配额
func (t *TenantGPUQuotaDaoImpl) ListAll() ([]*model.TenantGPUQuota, error) {
	var quotas []*model.TenantGPUQuota
	if err := t.DB.Find(&quotas).Error; err != nil {
		logrus.Errorf("list all tenant gpu quota error: %v", err)
		return nil, err
	}
	return quotas, nil
}

// TenantServiceGPUDao 组件GPU配置数据访问接口
type TenantServiceGPUDao interface {
	GetByServiceID(serviceID string) (*model.TenantServiceGPU, error)
	GetByTenantID(tenantID string) ([]*model.TenantServiceGPU, error)
	Create(config *model.TenantServiceGPU) error
	Update(config *model.TenantServiceGPU) error
	Delete(serviceID string) error
	GetEnabledGPUServices(tenantID string) ([]*model.TenantServiceGPU, error)
}

// TenantServiceGPUDaoImpl 组件GPU配置数据访问实现
type TenantServiceGPUDaoImpl struct {
	DB *gorm.DB
}

// GetByServiceID 根据服务ID获取GPU配置
func (t *TenantServiceGPUDaoImpl) GetByServiceID(serviceID string) (*model.TenantServiceGPU, error) {
	var config model.TenantServiceGPU
	if err := t.DB.Where("service_id = ?", serviceID).First(&config).Error; err != nil {
		if gorm.IsRecordNotFoundError(err) {
			return nil, nil
		}
		logrus.Errorf("get tenant service gpu by service_id error: %v", err)
		return nil, err
	}
	return &config, nil
}

// GetByTenantID 根据租户ID获取所有GPU配置
func (t *TenantServiceGPUDaoImpl) GetByTenantID(tenantID string) ([]*model.TenantServiceGPU, error) {
	var configs []*model.TenantServiceGPU
	// 需要通过tenant_service表关联查询
	query := `
		SELECT tsg.* FROM tenant_service_gpu tsg
		INNER JOIN tenant_service ts ON tsg.service_id = ts.service_id
		WHERE ts.tenant_id = ?
	`
	if err := t.DB.Raw(query, tenantID).Scan(&configs).Error; err != nil {
		logrus.Errorf("get tenant service gpu by tenant_id error: %v", err)
		return nil, err
	}
	return configs, nil
}

// Create 创建GPU配置
func (t *TenantServiceGPUDaoImpl) Create(config *model.TenantServiceGPU) error {
	if err := t.DB.Create(config).Error; err != nil {
		logrus.Errorf("create tenant service gpu error: %v", err)
		return err
	}
	return nil
}

// Update 更新GPU配置
func (t *TenantServiceGPUDaoImpl) Update(config *model.TenantServiceGPU) error {
	if err := t.DB.Model(&model.TenantServiceGPU{}).
		Where("service_id = ?", config.ServiceID).
		Updates(map[string]interface{}{
			"enable_gpu":           config.EnableGPU,
			"gpu_count":            config.GPUCount,
			"gpu_memory":           config.GPUMemory,
			"gpu_cores":            config.GPUCores,
			"gpu_model_preference": config.GPUModelPreference,
		}).Error; err != nil {
		logrus.Errorf("update tenant service gpu error: %v", err)
		return err
	}
	return nil
}

// Delete 删除GPU配置
func (t *TenantServiceGPUDaoImpl) Delete(serviceID string) error {
	if err := t.DB.Where("service_id = ?", serviceID).
		Delete(&model.TenantServiceGPU{}).Error; err != nil {
		logrus.Errorf("delete tenant service gpu error: %v", err)
		return err
	}
	return nil
}

// GetEnabledGPUServices 获取启用GPU的服务列表
func (t *TenantServiceGPUDaoImpl) GetEnabledGPUServices(tenantID string) ([]*model.TenantServiceGPU, error) {
	var configs []*model.TenantServiceGPU
	query := `
		SELECT tsg.* FROM tenant_service_gpu tsg
		INNER JOIN tenant_service ts ON tsg.service_id = ts.service_id
		WHERE ts.tenant_id = ? AND tsg.enable_gpu = true
	`
	if err := t.DB.Raw(query, tenantID).Scan(&configs).Error; err != nil {
		logrus.Errorf("get enabled gpu services error: %v", err)
		return nil, err
	}
	return configs, nil
}
