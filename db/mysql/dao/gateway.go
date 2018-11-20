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
	"fmt"
	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
)

//TenantServicesDaoImpl -
type CertificateDaoImpl struct {
	DB *gorm.DB
}

func (c *CertificateDaoImpl) AddModel(model.Interface) error {
	return nil
}

func (c *CertificateDaoImpl) UpdateModel(model.Interface) error {
	return nil
}

// GetCertificateByID gets a certificate by matching id
func (c *CertificateDaoImpl) GetCertificateByID(certificateID string) (*model.Certificate, error) {
	var certificate *model.Certificate
	if err := c.DB.Where("id = ?", certificateID).Find(&certificate).Error; err != nil {
		return nil, err
	}
	return certificate, nil
}

type RuleExtensionDaoImpl struct {
	DB *gorm.DB
}

func (c *RuleExtensionDaoImpl) AddModel(model.Interface) error {
	return nil
}

func (c *RuleExtensionDaoImpl) UpdateModel(model.Interface) error {
	return nil
}

func (c *RuleExtensionDaoImpl) GetRuleExtensionByServiceID(serviceID string) ([]*model.RuleExtension, error) {
	var ruleExtension []*model.RuleExtension
	if err := c.DB.Where("service_id = ?", serviceID).Find(&ruleExtension).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return ruleExtension, nil
		}
		return nil, err
	}
	return ruleExtension, nil
}

type HttpRuleDaoImpl struct {
	DB *gorm.DB
}

func (h *HttpRuleDaoImpl) AddModel(mo model.Interface) error {
	httpRule := mo.(*model.HttpRule)
	var oldHttpRule model.HttpRule
	if ok := h.DB.Where("service_id = ? and container_port=?", httpRule.ServiceID,
		httpRule.ContainerPort).Find(&oldHttpRule).RecordNotFound(); ok {
		if err := h.DB.Create(httpRule).Error; err != nil {
			return err
		}
	} else {
		return fmt.Errorf("HttpRule already exists based on ServiceID(%s) and ContainerPort(%s)",
			httpRule.ServiceID, httpRule.ContainerPort)
	}
	return nil
}

func (h *HttpRuleDaoImpl) UpdateModel(model.Interface) error {
	return nil
}

// GetHttpRuleByServiceIDAndContainerPort gets a HttpRule based on serviceID and containerPort
func (h *HttpRuleDaoImpl) GetHttpRuleByServiceIDAndContainerPort(serviceID string,
	containerPort int) (*model.HttpRule, error) {
	httpRule := &model.HttpRule{}
	if err := h.DB.Where("service_id = ? and container_port = ?", serviceID,
		containerPort).Find(httpRule).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return httpRule, nil
		}
		return nil, err
	}
	return httpRule, nil
}

type TcpRuleDaoTmpl struct {
	DB *gorm.DB
}

func (t *TcpRuleDaoTmpl) AddModel(mo model.Interface) error {
	tcpRule := mo.(*model.TcpRule)
	var oldTcpRule model.TcpRule
	if ok := t.DB.Where("service_id = ? and container_port=?", tcpRule.ServiceID,
		tcpRule.ContainerPort).Find(&oldTcpRule).RecordNotFound(); ok {
		if err := t.DB.Create(tcpRule).Error; err != nil {
			return err
		}
	} else {
		return fmt.Errorf("TcpRule already exists based on ServiceID(%s) and ContainerPort(%s)",
			tcpRule.ServiceID, tcpRule.ContainerPort)
	}
	return nil
}

func (s *TcpRuleDaoTmpl) UpdateModel(model.Interface) error {
	return nil
}

// GetTcpRuleByServiceIDAndContainerPort gets a TcpRule based on serviceID and containerPort
func (s *TcpRuleDaoTmpl) GetTcpRuleByServiceIDAndContainerPort(serviceID string,
	containerPort int) (*model.TcpRule, error) {
	result := &model.TcpRule{}
	if err := s.DB.Where("service_id = ? and container_port = ?", serviceID,
		containerPort).Find(result).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return result, nil
		}
		return nil, err
	}
	return result, nil
}
