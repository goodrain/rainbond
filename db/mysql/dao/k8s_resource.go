// RAINBOND, Application Management Platform
// Copyright (C) 2022-2022 Goodrain Co., Ltd.

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
)

// K8sResourceDaoImpl k8s resource dao
type K8sResourceDaoImpl struct {
	DB *gorm.DB
}

// AddModel add model
func (t *K8sResourceDaoImpl) AddModel(mo model.Interface) error {
	return nil
}

// UpdateModel update model
func (t *K8sResourceDaoImpl) UpdateModel(mo model.Interface) error {
	return nil
}

// ListByAppID list by app id
func (t *K8sResourceDaoImpl) ListByAppID(appID string) ([]model.K8sResource, error) {
	var resources []model.K8sResource
	if err := t.DB.Where("app_id = ?", appID).Find(&resources).Error; err != nil {
		return nil, err
	}
	return resources, nil
}
