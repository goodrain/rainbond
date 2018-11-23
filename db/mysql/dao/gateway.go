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
	"reflect"
)

//TenantServicesDaoImpl -
type CertificateDaoImpl struct {
	DB *gorm.DB
}

func (c *CertificateDaoImpl) AddModel(mo model.Interface) error {
	certificate, ok := mo.(*model.Certificate)
	if !ok {
		return fmt.Errorf("Can't convert %s to %s", reflect.TypeOf(mo).String(), "*model.Certificate")
	}
	var old model.Certificate
	if ok := c.DB.Where("uuid = ?", certificate.UUID).Find(&old).RecordNotFound(); ok {
		if err := c.DB.Create(certificate).Error; err != nil {
			return err
		}
	} else {
		return fmt.Errorf("Certificate already exists based on certificateID(%s)",
			certificate.UUID)
	}
	return nil
}

func (c *CertificateDaoImpl) UpdateModel(mo model.Interface) error {
	cert, ok := mo.(*model.Certificate)
	if !ok {
		return fmt.Errorf("Failed to convert %s to *model.Certificate", reflect.TypeOf(mo).String())
	}

	return c.DB.Table(cert.TableName()).
		Where("uuid = ?", cert.UUID).
		Update(cert).Error
}

// GetCertificateByID gets a certificate by matching id
func (c *CertificateDaoImpl) GetCertificateByID(certificateID string) (*model.Certificate, error) {
	var certificate *model.Certificate
	if err := c.DB.Where("id = ?", certificateID).Find(&certificate).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return certificate, nil
		}
		return nil, err
	}
	return certificate, nil
}

func (c *CertificateDaoImpl) DeleteCertificateByID(certificateID string) error {
	cert := &model.Certificate{
		UUID: certificateID,
	}
	return c.DB.Where("uuid=?", certificateID).Delete(cert).Error
}

type RuleExtensionDaoImpl struct {
	DB *gorm.DB
}

func (c *RuleExtensionDaoImpl) AddModel(mo model.Interface) error {
	re, ok := mo.(*model.RuleExtension)
	if !ok {
		return fmt.Errorf("Can't convert %s to %s", reflect.TypeOf(mo).String(), "*model.RuleExtension")
	}
	var old model.RuleExtension
	if ok := c.DB.Where("rule_id = ? and value = ?", re.RuleID, re.Value).Find(&old).RecordNotFound(); ok {
		return c.DB.Create(re).Error
	} else {
		return fmt.Errorf("RuleExtension already exists based on RuleID(%s) and Value(%s)",
			re.RuleID, re.Value)
	}
}

func (c *RuleExtensionDaoImpl) UpdateModel(model.Interface) error {
	return nil
}

func (c *RuleExtensionDaoImpl) GetRuleExtensionByRuleID(ruleID string) ([]*model.RuleExtension, error) {
	var ruleExtension []*model.RuleExtension
	if err := c.DB.Where("rule_id = ?", ruleID).Find(&ruleExtension).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return ruleExtension, nil
		}
		return nil, err
	}
	return ruleExtension, nil
}

// DeleteRuleExtensionByRuleID delete rule extensions by ruleID
func (c *RuleExtensionDaoImpl) DeleteRuleExtensionByRuleID(ruleID string) error {
	re := &model.RuleExtension{
		RuleID: ruleID,
	}
	return c.DB.Where("rule_id=?", ruleID).Delete(re).Error
}

type HttpRuleDaoImpl struct {
	DB *gorm.DB
}

func (h *HttpRuleDaoImpl) AddModel(mo model.Interface) error {
	httpRule, ok := mo.(*model.HttpRule)
	if !ok {
		return fmt.Errorf("Can't not convert %s to *model.HttpRule", reflect.TypeOf(mo).String())
	}
	var oldHttpRule model.HttpRule
	if ok := h.DB.Where("uuid=?", httpRule.UUID).Find(&oldHttpRule).RecordNotFound(); ok {
		if err := h.DB.Create(httpRule).Error; err != nil {
			return err
		}
	} else {
		return fmt.Errorf("HttpRule already exists based on uuid(%s)", httpRule.UUID)
	}
	return nil
}

func (h *HttpRuleDaoImpl) UpdateModel(mo model.Interface) error {
	hr, ok := mo.(*model.HttpRule)
	if !ok {
		return fmt.Errorf("Failed to convert %s to *model.HttpRule", reflect.TypeOf(mo).String())
	}

	return h.DB.Table(hr.TableName()).
		Where("uuid = ?", hr.UUID).
		Update(hr).Error
}

// GetHttpRuleByServiceIDAndContainerPort gets a HttpRule based on uuid
func (h *HttpRuleDaoImpl) GetHttpRuleByID(id string) (*model.HttpRule, error) {
	httpRule := &model.HttpRule{}
	if err := h.DB.Where("uuid = ?", id).Find(httpRule).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return httpRule, nil
		}
		return nil, err
	}
	return httpRule, nil
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

func (h *HttpRuleDaoImpl) DeleteHttpRuleByServiceIDAndContainerPort(serviceID string,
	containerPort int) (*model.HttpRule, error) {
	httpRule, err := h.GetHttpRuleByServiceIDAndContainerPort(serviceID, containerPort)
	if err != nil {
		return nil, err
	}
	if err := h.DB.Where("service_id = ? and container_port = ?", serviceID,
		containerPort).Delete(httpRule).Error; err != nil {
		return nil, err
	}

	return httpRule, nil
}

func (h *HttpRuleDaoImpl) DeleteHttpRuleByID(id string) error {
	httpRule := &model.HttpRule{
		UUID: id,
	}
	if err := h.DB.Where("uuid = ? ", id).Delete(httpRule).Error; err != nil {
		return err
	}

	return nil
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
		return fmt.Errorf("TcpRule already exists based on ServiceID(%s) and ContainerPort(%v)",
			tcpRule.ServiceID, tcpRule.ContainerPort)
	}
	return nil
}

func (t *TcpRuleDaoTmpl) UpdateModel(mo model.Interface) error {
	tr, ok := mo.(*model.TcpRule)
	if !ok {
		return fmt.Errorf("Failed to convert %s to *model.TcpRule", reflect.TypeOf(mo).String())
	}

	return t.DB.Table(tr.TableName()).
		Where("uuid = ?", tr.UUID).
		Update(tr).Error
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

// GetTcpRuleByID gets a TcpRule based on tcpRuleID
func (s *TcpRuleDaoTmpl) GetTcpRuleByID(id string) (*model.TcpRule, error) {
	result := &model.TcpRule{}
	if err := s.DB.Where("uuid = ?", id).Find(result).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return result, nil
		}
		return nil, err
	}
	return result, nil
}

func (s *TcpRuleDaoTmpl) DeleteTcpRule(tcpRule *model.TcpRule) error {
	return s.DB.Where("uuid = ?", tcpRule.UUID).Delete(tcpRule).Error
}
