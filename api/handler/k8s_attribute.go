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

package handler

import (
	api_model "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/db"
)

// CreateK8sAttribute -
func (s *ServiceAction) CreateK8sAttribute(tenantID, componentID string, k8sAttr *api_model.ComponentK8sAttribute) error {
	return db.GetManager().ComponentK8sAttributeDao().AddModel(k8sAttr.DbModel(tenantID, componentID))
}

// UpdateK8sAttribute -
func (s *ServiceAction) UpdateK8sAttribute(componentID string, k8sAttributes *api_model.ComponentK8sAttribute) error {
	attr, err := db.GetManager().ComponentK8sAttributeDao().GetByComponentIDAndName(componentID, k8sAttributes.Name)
	if err != nil {
		return err
	}
	attr.AttributeValue = k8sAttributes.AttributeValue
	return db.GetManager().ComponentK8sAttributeDao().UpdateModel(attr)
}

// DeleteK8sAttribute -
func (s *ServiceAction) DeleteK8sAttribute(componentID, name string) error {
	return db.GetManager().ComponentK8sAttributeDao().DeleteByComponentIDAndName(componentID, name)
}
