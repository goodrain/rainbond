// RAINBOND, Application Management Platform
// Copyright (C) 2014-2017 Goodrain Co., Ltd.

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
	"time"
)

type MockHttpRuleDaoImpl struct {
}

func (h *MockHttpRuleDaoImpl) AddModel(model.Interface) error {
	return nil
}

func (h *MockHttpRuleDaoImpl) UpdateModel(model.Interface) error {
	return nil
}

// GetHttpRuleByServiceIDAndContainerPort gets a HttpRule based on serviceID and containerPort
func (h *MockHttpRuleDaoImpl) GetHttpRuleByServiceIDAndContainerPort(serviceID string,
	containerPort int) (*model.HttpRule, error) {
	createTime, _ := time.Parse(time.RFC3339, "2018-11-18T10:24:43Z")
	httpRule := &model.HttpRule{
		Model: model.Model{
			ID:        1,
			CreatedAt: createTime,
		},
		ServiceID:        serviceID,
		ContainerPort:    containerPort,
		Domain:           "dummy-domain",
		Path:             "/",
		LoadBalancerType: model.RoundRobinLBType,
	}

	return httpRule, nil
}

type MockStreamRuleDaoTmpl struct {
	DB *gorm.DB
}

func (s *MockStreamRuleDaoTmpl) AddModel(model.Interface) error {
	return nil
}

func (s *MockStreamRuleDaoTmpl) UpdateModel(model.Interface) error {
	return nil
}

// GetStreamRuleByServiceIDAndContainerPort gets a StreamRule based on serviceID and containerPort
func (s *MockStreamRuleDaoTmpl) GetStreamRuleByServiceIDAndContainerPort(serviceID string,
	containerPort int) (*model.StreamRule, error) {
	//createTime, _ := time.Parse(time.RFC3339, "2018-11-18T10:24:43Z")
	//streamRule := &model.StreamRule{
	//	Model: model.Model{
	//		ID:        3,
	//		CreatedAt: createTime,
	//	},
	//	ServiceID:     serviceID,
	//	ContainerPort: containerPort,
	//	IP: "127.0.0.1",
	//}

	return nil, nil
}
