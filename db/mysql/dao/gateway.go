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
	"github.com/Sirupsen/logrus"
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

func (c *CertificateDaoImpl) AddOrUpdate(mo model.Interface) error {
	cert, ok := mo.(*model.Certificate)
	if !ok {
		return fmt.Errorf("Failed to convert %s to *model.Certificate", reflect.TypeOf(mo).String())
	}
	var result model.Certificate
	if err := c.DB.Where("uuid = ?", cert.UUID).Assign(cert).FirstOrCreate(&result).Error; err != nil {
		return fmt.Errorf("Unexpected error occurred while adding or updating certficate: %v", err)
	}

	return nil
}

// GetCertificateByID gets a certificate by matching id
func (c *CertificateDaoImpl) GetCertificateByID(certificateID string) (*model.Certificate, error) {
	var certificate model.Certificate
	if err := c.DB.Where("uuid = ?", certificateID).Find(&certificate).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return &certificate, nil
		}
		logrus.Error("error getting certificate by id: %v", err)
		return nil, err
	}
	return &certificate, nil
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
	httpRule, ok := mo.(*model.HTTPRule)
	if !ok {
		return fmt.Errorf("Can't not convert %s to *model.HTTPRule", reflect.TypeOf(mo).String())
	}
	var oldHttpRule model.HTTPRule
	if ok := h.DB.Where("uuid=?", httpRule.UUID).Find(&oldHttpRule).RecordNotFound(); ok {
		if err := h.DB.Create(httpRule).Error; err != nil {
			return err
		}
	} else {
		return fmt.Errorf("HTTPRule already exists based on uuid(%s)", httpRule.UUID)
	}
	return nil
}

func (h *HttpRuleDaoImpl) UpdateModel(mo model.Interface) error {
	hr, ok := mo.(*model.HTTPRule)
	if !ok {
		return fmt.Errorf("Failed to convert %s to *model.HTTPRule", reflect.TypeOf(mo).String())
	}

	return h.DB.Table(hr.TableName()).
		Where("uuid = ?", hr.UUID).
		Update(hr).Error
}

// GetHttpRuleByServiceIDAndContainerPort gets a HTTPRule based on uuid
func (h *HttpRuleDaoImpl) GetHttpRuleByID(id string) (*model.HTTPRule, error) {
	httpRule := &model.HTTPRule{}
	if err := h.DB.Where("uuid = ?", id).Find(httpRule).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return httpRule, nil
		}
		return nil, err
	}
	return httpRule, nil
}

// GetHttpRuleByServiceIDAndContainerPort gets a HTTPRule based on serviceID and containerPort
func (h *HttpRuleDaoImpl) GetHttpRuleByServiceIDAndContainerPort(serviceID string,
	containerPort int) ([]*model.HTTPRule, error) {
	var httpRule []*model.HTTPRule
	if err := h.DB.Where("service_id = ? and container_port = ?", serviceID,
		containerPort).Find(&httpRule).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return httpRule, nil
		}
		return nil, err
	}
	return httpRule, nil
}

func (h *HttpRuleDaoImpl) DeleteHttpRuleByID(id string) error {
	httpRule := &model.HTTPRule{
		UUID: id,
	}
	if err := h.DB.Where("uuid = ? ", id).Delete(httpRule).Error; err != nil {
		return err
	}

	return nil
}

// ListByServiceID lists all HTTPRules matching serviceID
func (h *HttpRuleDaoImpl) ListByServiceID(serviceID string) ([]*model.HTTPRule, error) {
	var rules []*model.HTTPRule
	if err := h.DB.Where("service_id = ?", serviceID).Find(&rules).Error; err != nil {
		return nil, err
	}
	return rules, nil
}

// TcpRuleDaoTmpl is a implementation of TcpRuleDao
type TcpRuleDaoTmpl struct {
	DB *gorm.DB
}

// AddModel adds model.TCPRule
func (t *TcpRuleDaoTmpl) AddModel(mo model.Interface) error {
	tcpRule := mo.(*model.TCPRule)
	var oldTcpRule model.TCPRule
	if ok := t.DB.Where("uuid = ?", tcpRule.UUID).Find(&oldTcpRule).RecordNotFound(); ok {
		if err := t.DB.Create(tcpRule).Error; err != nil {
			return err
		}
	} else {
		return fmt.Errorf("TCPRule already exists based on uuid(%s)", tcpRule.UUID)
	}
	return nil
}

// UpdateModel updates model.TCPRule
func (t *TcpRuleDaoTmpl) UpdateModel(mo model.Interface) error {
	tr, ok := mo.(*model.TCPRule)
	if !ok {
		return fmt.Errorf("Failed to convert %s to *model.TCPRule", reflect.TypeOf(mo).String())
	}

	return t.DB.Table(tr.TableName()).
		Where("uuid = ?", tr.UUID).
		Update(tr).Error
}

// GetTcpRuleByServiceIDAndContainerPort gets a TCPRule based on serviceID and containerPort
func (s *TcpRuleDaoTmpl) GetTcpRuleByServiceIDAndContainerPort(serviceID string,
	containerPort int) ([]*model.TCPRule, error) {
	var result []*model.TCPRule
	if err := s.DB.Where("service_id = ? and container_port = ?", serviceID,
		containerPort).Find(&result).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return result, nil
		}
		return nil, err
	}
	return result, nil
}

// GetTcpRuleByID gets a TCPRule based on tcpRuleID
func (s *TcpRuleDaoTmpl) GetTcpRuleByID(id string) (*model.TCPRule, error) {
	result := &model.TCPRule{}
	if err := s.DB.Where("uuid = ?", id).Find(result).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return result, nil
}

// DeleteTcpRule deletes model.TCPRule
func (s *TcpRuleDaoTmpl) DeleteTcpRule(tcpRule *model.TCPRule) error {
	return s.DB.Where("uuid = ?", tcpRule.UUID).Delete(tcpRule).Error
}

// ListByServiceID lists all TCPRules matching serviceID
func (h *TcpRuleDaoTmpl) ListByServiceID(serviceID string) ([]*model.TCPRule, error) {
	var rules []*model.TCPRule
	if err := h.DB.Where("service_id = ?", serviceID).Find(&rules).Error; err != nil {
		return nil, err
	}
	return rules, nil
}

// IPPortImpl is an implementation of dao.IPPortDao
type IPPortImpl struct {
	DB *gorm.DB
}

// AddModel adds model.IPPort
func (i *IPPortImpl) AddModel(mo model.Interface) error {
	ipport := mo.(*model.IPPort)
	var old model.TCPRule
	if ok := i.DB.Where("ip = ? and port = ?", ipport.IP, ipport.Port).Find(&old).RecordNotFound(); ok {
		if err := i.DB.Create(ipport).Error; err != nil {
			return err
		}
	} else {
		return fmt.Errorf("IPPort already exists(ip=%s, port=%d)", ipport.IP, ipport.Port)
	}
	return nil
}

// UpdateModel updates model.IPPort
func (i *IPPortImpl) UpdateModel(mo model.Interface) error {
	ipport, ok := mo.(*model.IPPort)
	if !ok {
		return fmt.Errorf("Failed to convert %s to *model.IPPort", reflect.TypeOf(mo).String())
	}

	return i.DB.Table(ipport.TableName()).
		Where("uuid = ?", ipport.UUID).
		Update(ipport).Error
}

// DeleteByIPAndPort deletes an IPPort that matches ip and port
func (i *IPPortImpl) DeleteByIPAndPort(ip string, port int) error {
	return i.DB.Where("ip = ? and port = ?", ip, port).Delete(model.IPPort{}).Error
}

// GetIPByPort returns an array of ip by port
func (i *IPPortImpl) GetIPByPort(port int) ([]*model.IPPort, error) {
	var result []*model.IPPort
	if err := i.DB.Where("port = ?", port).Find(&result).Error; err != nil {
		return nil, err
	}
	return result, nil
}

// GetIPByPort returns an IPPort that matches ip and port
func (i *IPPortImpl) GetIPPortByIPAndPort(ip string, port int) (*model.IPPort, error) {
	var result model.IPPort
	if err := i.DB.Where("ip = ? and port = ?", ip, port).Find(&result).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &result, nil
}

// IPPoolImpl is an implementation of dao.IPPoolDao
type IPPoolImpl struct {
	DB *gorm.DB
}

// AddModel adds model.IPPool
func (i *IPPoolImpl) AddModel(mo model.Interface) error {
	ippool, ok := mo.(*model.IPPool)
	if !ok {
		return fmt.Errorf("Can't not convert %s to *model.IPPool", reflect.TypeOf(mo).String())
	}
	if ok = i.DB.Where("eid = ?", ippool.EID).Find(&model.IPPool{}).RecordNotFound(); ok {
		if err := i.DB.Create(ippool).Error; err != nil {
			return err
		}
	} else {
		return fmt.Errorf("IPPool for EID(%s) exists", ippool.EID)
	}
	return nil
}

// UpdateModel updates model.IPPool
func (i *IPPoolImpl) UpdateModel(mo model.Interface) error {
	ippool, ok := mo.(*model.IPPool)
	if !ok {
		return fmt.Errorf("Can't not convert %s to *model.IPPool", reflect.TypeOf(mo).String())
	}
	return i.DB.Table(ippool.TableName()).
		Where("eid = ?", ippool.EID).
		Update(ippool).Error
}

// GetIPPoolByEID returns model.IPPool that matches eid.
func (i *IPPoolImpl) GetIPPoolByEID(eid string) (*model.IPPool, error) {
	var result model.IPPool
	if err := i.DB.Where("eid = ?", eid).Find(&result).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &result, nil
}