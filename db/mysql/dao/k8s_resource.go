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
	"fmt"
	gormbulkups "github.com/atcdot/gorm-bulk-upsert"
	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
	pkgerr "github.com/pkg/errors"
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
	resource, ok := mo.(*model.K8sResource)
	if !ok {
		return fmt.Errorf("mo.(*model.K8sResource) err")
	}
	return t.DB.Save(resource).Error
}

// ListByAppID list by app id
func (t *K8sResourceDaoImpl) ListByAppID(appID string) ([]model.K8sResource, error) {
	var resources []model.K8sResource
	if err := t.DB.Where("app_id = ?", appID).Find(&resources).Error; err != nil {
		return nil, err
	}
	return resources, nil
}

//CreateK8sResourceInBatch -
func (t *K8sResourceDaoImpl) CreateK8sResourceInBatch(k8sResources []*model.K8sResource) error {
	var objects []interface{}
	for _, cg := range k8sResources {
		objects = append(objects, *cg)
	}
	if err := gormbulkups.BulkUpsert(t.DB, objects, 2000); err != nil {
		return pkgerr.Wrap(err, "create K8sResource groups in batch")
	}
	return nil
}

//DeleteK8sResourceInBatch -
func (t *K8sResourceDaoImpl) DeleteK8sResourceInBatch(appID, name string, kind string) error {
	return t.DB.Where("app_id=? and name=? and kind=?", appID, name, kind).Delete(&model.K8sResource{}).Error
}

//GetK8sResourceByNameInBatch -
func (t *K8sResourceDaoImpl) GetK8sResourceByNameInBatch(appID, name, kind string) ([]model.K8sResource, error) {
	var resources []model.K8sResource
	if err := t.DB.Where("app_id=? and name=? and kind=?", appID, name, kind).Find(&resources).Error; err != nil {
		return nil, err
	}
	return resources, nil
}
