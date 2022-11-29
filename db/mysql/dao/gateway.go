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
	gormbulkups "github.com/atcdot/gorm-bulk-upsert"
	"reflect"

	"github.com/goodrain/rainbond/api/util/bcode"
	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

//CertificateDaoImpl -
type CertificateDaoImpl struct {
	DB *gorm.DB
}

//AddModel add model
func (c *CertificateDaoImpl) AddModel(mo model.Interface) error {
	certificate, ok := mo.(*model.Certificate)
	if !ok {
		return fmt.Errorf("can't convert %s to %s", reflect.TypeOf(mo).String(), "*model.Certificate")
	}
	var old model.Certificate
	if ok := c.DB.Where("uuid = ?", certificate.UUID).Find(&old).RecordNotFound(); ok {
		if err := c.DB.Create(certificate).Error; err != nil {
			return err
		}
	} else {
		return fmt.Errorf("certificate already exists based on certificateID(%s)", certificate.UUID)
	}
	return nil
}

//UpdateModel update Certificate
func (c *CertificateDaoImpl) UpdateModel(mo model.Interface) error {
	cert, ok := mo.(*model.Certificate)
	if !ok {
		return fmt.Errorf("failed to convert %s to *model.Certificate", reflect.TypeOf(mo).String())
	}
	return c.DB.Table(cert.TableName()).
		Where("uuid = ?", cert.UUID).
		Save(cert).Error
}

//AddOrUpdate add or update Certificate
func (c *CertificateDaoImpl) AddOrUpdate(mo model.Interface) error {
	cert, ok := mo.(*model.Certificate)
	if !ok {
		return fmt.Errorf("failed to convert %s to *model.Certificate", reflect.TypeOf(mo).String())
	}

	var old model.Certificate
	if err := c.DB.Where("uuid = ?", cert.UUID).Find(&old).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.DB.Create(cert).Error
		}
		return err
	}

	// update certificate
	old.Certificate = cert.Certificate
	old.PrivateKey = cert.PrivateKey
	return c.DB.Table(cert.TableName()).Where("uuid = ?", cert.UUID).Save(&old).Error
}

// GetCertificateByID gets a certificate by matching id
func (c *CertificateDaoImpl) GetCertificateByID(certificateID string) (*model.Certificate, error) {
	var certificate model.Certificate
	if err := c.DB.Where("uuid = ?", certificateID).Find(&certificate).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		logrus.Errorf("error getting certificate by id: %s", err.Error())
		return nil, err
	}
	return &certificate, nil
}

//RuleExtensionDaoImpl rule extension dao
type RuleExtensionDaoImpl struct {
	DB *gorm.DB
}

//AddModel add
func (c *RuleExtensionDaoImpl) AddModel(mo model.Interface) error {
	re, ok := mo.(*model.RuleExtension)
	if !ok {
		return fmt.Errorf("can't convert %s to %s", reflect.TypeOf(mo).String(), "*model.RuleExtension")
	}
	var old model.RuleExtension
	if ok := c.DB.Where("rule_id = ? and value = ?", re.RuleID, re.Value).Find(&old).RecordNotFound(); ok {
		return c.DB.Create(re).Error
	}
	return fmt.Errorf("RuleExtension already exists based on RuleID(%s) and Value(%s)",
		re.RuleID, re.Value)
}

//UpdateModel update model,do not impl
func (c *RuleExtensionDaoImpl) UpdateModel(model.Interface) error {
	return nil
}

//GetRuleExtensionByRuleID get extension by rule
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

// DeleteByRuleIDs deletes rule extentions based on the given ruleIDs.
func (c *RuleExtensionDaoImpl) DeleteByRuleIDs(ruleIDs []string) error {
	if err := c.DB.Where("rule_id in (?)", ruleIDs).Delete(&model.RuleExtension{}).Error; err != nil {
		return errors.Wrap(err, "delete rule extentions")
	}
	return nil
}

// CreateOrUpdateRuleExtensionsInBatch -
func (c *RuleExtensionDaoImpl) CreateOrUpdateRuleExtensionsInBatch(exts []*model.RuleExtension) error {
	dbType := c.DB.Dialect().GetName()
	if dbType == "sqlite3" {
		for _, ext := range exts {
			if ok := c.DB.Where("ID=? ", ext.ID).Find(&ext).RecordNotFound(); !ok {
				if err := c.DB.Model(&ext).Where("ID = ?", ext.ID).Update(ext).Error; err != nil {
					logrus.Error("batch Update or update ext error:", err)
					return err
				}
			} else {
				if err := c.DB.Create(&ext).Error; err != nil {
					logrus.Error("batch create ext error:", err)
					return err
				}
			}
		}
		return nil
	}
	var objects []interface{}
	for _, ext := range exts {
		objects = append(objects, *ext)
	}
	if err := gormbulkups.BulkUpsert(c.DB, objects, 2000); err != nil {
		return errors.Wrap(err, "create or update rule extensions in batch")
	}
	return nil
}

//HTTPRuleDaoImpl http rule
type HTTPRuleDaoImpl struct {
	DB *gorm.DB
}

//AddModel -
func (h *HTTPRuleDaoImpl) AddModel(mo model.Interface) error {
	httpRule, ok := mo.(*model.HTTPRule)
	if !ok {
		return fmt.Errorf("can't not convert %s to *model.HTTPRule", reflect.TypeOf(mo).String())
	}
	var oldHTTPRule model.HTTPRule
	if ok := h.DB.Where("uuid=?", httpRule.UUID).Find(&oldHTTPRule).RecordNotFound(); ok {
		if err := h.DB.Create(httpRule).Error; err != nil {
			return err
		}
	} else {
		return fmt.Errorf("HTTPRule already exists based on uuid(%s)", httpRule.UUID)
	}
	return nil
}

//UpdateModel -
func (h *HTTPRuleDaoImpl) UpdateModel(mo model.Interface) error {
	hr, ok := mo.(*model.HTTPRule)
	if !ok {
		return fmt.Errorf("failed to convert %s to *model.HTTPRule", reflect.TypeOf(mo).String())
	}
	return h.DB.Save(hr).Error
}

// GetHTTPRuleByID gets a HTTPRule based on uuid
func (h *HTTPRuleDaoImpl) GetHTTPRuleByID(id string) (*model.HTTPRule, error) {
	httpRule := &model.HTTPRule{}
	if err := h.DB.Where("uuid = ?", id).Find(httpRule).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return httpRule, nil
		}
		return nil, err
	}
	return httpRule, nil
}

// GetHTTPRuleByServiceIDAndContainerPort gets a HTTPRule based on serviceID and containerPort
func (h *HTTPRuleDaoImpl) GetHTTPRuleByServiceIDAndContainerPort(serviceID string,
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

// GetHTTPRulesByCertificateID get http rules by certificateID
func (h *HTTPRuleDaoImpl) GetHTTPRulesByCertificateID(certificateID string) ([]*model.HTTPRule, error) {
	var httpRules []*model.HTTPRule
	if err := h.DB.Where("certificate_id = ?", certificateID).Find(&httpRules).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return httpRules, nil
		}
		return nil, err
	}
	return httpRules, nil
}

//DeleteHTTPRuleByID delete http rule by rule id
func (h *HTTPRuleDaoImpl) DeleteHTTPRuleByID(id string) error {
	httpRule := &model.HTTPRule{}
	if err := h.DB.Where("uuid = ? ", id).Delete(httpRule).Error; err != nil {
		return err
	}

	return nil
}

// DeleteByComponentPort deletes http rules based on componentID and port.
func (h *HTTPRuleDaoImpl) DeleteByComponentPort(componentID string, port int) error {
	if err := h.DB.Where("service_id=? and container_port=?", componentID, port).Delete(&model.HTTPRule{}).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return errors.Wrap(bcode.ErrIngressHTTPRuleNotFound, "delete http rules")
		}
		return err
	}
	return nil
}

//DeleteHTTPRuleByServiceID delete http rule by service id
func (h *HTTPRuleDaoImpl) DeleteHTTPRuleByServiceID(serviceID string) error {
	httpRule := &model.HTTPRule{}
	if err := h.DB.Where("service_id = ? ", serviceID).Delete(httpRule).Error; err != nil {
		return err
	}
	return nil
}

// ListByServiceID lists all HTTPRules matching serviceID
func (h *HTTPRuleDaoImpl) ListByServiceID(serviceID string) ([]*model.HTTPRule, error) {
	var rules []*model.HTTPRule
	if err := h.DB.Where("service_id = ?", serviceID).Find(&rules).Error; err != nil {
		return nil, err
	}
	return rules, nil
}

// ListByComponentPort lists http rules based on the given componentID and port.
func (h *HTTPRuleDaoImpl) ListByComponentPort(componentID string, port int) ([]*model.HTTPRule, error) {
	var rules []*model.HTTPRule
	if err := h.DB.Where("service_id=? and container_port=?", componentID, port).Find(&rules).Error; err != nil {
		return nil, errors.Wrap(err, "list http rules")
	}
	return rules, nil
}

// ListByCertID lists all HTTPRules matching certificate id
func (h *HTTPRuleDaoImpl) ListByCertID(certID string) ([]*model.HTTPRule, error) {
	var rules []*model.HTTPRule
	if err := h.DB.Where("certificate_id = ?", certID).Find(&rules).Error; err != nil {
		return nil, err
	}
	return rules, nil
}

//DeleteByComponentIDs delete http rule by component ids
func (h *HTTPRuleDaoImpl) DeleteByComponentIDs(componentIDs []string) error {
	return h.DB.Where("service_id in (?) ", componentIDs).Delete(&model.HTTPRule{}).Error
}

// CreateOrUpdateHTTPRuleInBatch Batch insert or update http rule
func (h *HTTPRuleDaoImpl) CreateOrUpdateHTTPRuleInBatch(httpRules []*model.HTTPRule) error {
	dbType := h.DB.Dialect().GetName()
	if dbType == "sqlite3" {
		for _, httpRule := range httpRules {
			if ok := h.DB.Where("ID=? ", httpRule.ID).Find(&httpRule).RecordNotFound(); !ok {
				if err := h.DB.Model(&httpRule).Where("ID = ?", httpRule.ID).Update(httpRule).Error; err != nil {
					logrus.Error("batch Update or update httpRule error:", err)
					return err
				}
			} else {
				if err := h.DB.Create(&httpRule).Error; err != nil {
					logrus.Error("batch create httpRule error:", err)
					return err
				}
			}
		}
		return nil
	}
	var objects []interface{}
	for _, httpRule := range httpRules {
		objects = append(objects, *httpRule)
	}
	if err := gormbulkups.BulkUpsert(h.DB, objects, 2000); err != nil {
		return errors.Wrap(err, "create or update http rule in batch")
	}
	return nil
}

// ListByComponentIDs -
func (h *HTTPRuleDaoImpl) ListByComponentIDs(componentIDs []string) ([]*model.HTTPRule, error) {
	var rules []*model.HTTPRule
	if err := h.DB.Where("service_id in (?) ", componentIDs).Find(&rules).Error; err != nil {
		return nil, err
	}
	return rules, nil
}

// HTTPRuleRewriteDaoTmpl is a implementation of HTTPRuleRewriteDao
type HTTPRuleRewriteDaoTmpl struct {
	DB *gorm.DB
}

//AddModel -
func (h *HTTPRuleRewriteDaoTmpl) AddModel(mo model.Interface) error {
	httpRuleRewrite, ok := mo.(*model.HTTPRuleRewrite)
	if !ok {
		return fmt.Errorf("can't not convert %s to *model.HTTPRuleRewrite", reflect.TypeOf(mo).String())
	}
	var oldHTTPRuleRewrite model.HTTPRuleRewrite
	if ok := h.DB.Where("uuid = ?", httpRuleRewrite.UUID).Find(&oldHTTPRuleRewrite).RecordNotFound(); !ok {
		return fmt.Errorf("HTTPRuleRewrite already exists based on uuid(%s)", httpRuleRewrite.UUID)
	}
	return h.DB.Create(httpRuleRewrite).Error
}

//UpdateModel -
func (h *HTTPRuleRewriteDaoTmpl) UpdateModel(mo model.Interface) error {
	hr, ok := mo.(*model.HTTPRuleRewrite)
	if !ok {
		return fmt.Errorf("failed to convert %s to *model.HTTPRuleRewrite", reflect.TypeOf(mo).String())
	}
	return h.DB.Save(hr).Error
}

// CreateOrUpdateHTTPRuleRewriteInBatch -
func (h *HTTPRuleRewriteDaoTmpl) CreateOrUpdateHTTPRuleRewriteInBatch(httpRuleRewrites []*model.HTTPRuleRewrite) error {
	dbType := h.DB.Dialect().GetName()
	if dbType == "sqlite3" {
		for _, httpRuleRewrite := range httpRuleRewrites {
			if ok := h.DB.Where("ID=? ", httpRuleRewrite.ID).Find(&httpRuleRewrite).RecordNotFound(); !ok {
				if err := h.DB.Model(&httpRuleRewrite).Where("ID = ?", httpRuleRewrite.ID).Update(httpRuleRewrite).Error; err != nil {
					logrus.Error("batch Update or update httpRuleRewrite error:", err)
					return err
				}
			} else {
				if err := h.DB.Create(&httpRuleRewrite).Error; err != nil {
					logrus.Error("batch create httpRuleRewrite error:", err)
					return err
				}
			}
		}
		return nil
	}
	var objects []interface{}
	for _, httpRuleRewrites := range httpRuleRewrites {
		objects = append(objects, *httpRuleRewrites)
	}
	if err := gormbulkups.BulkUpsert(h.DB, objects, 2000); err != nil {
		return errors.Wrap(err, "create or update http rule rewrite in batch")
	}
	return nil
}

// ListByHTTPRuleID -
func (h *HTTPRuleRewriteDaoTmpl) ListByHTTPRuleID(httpRuleID string) ([]*model.HTTPRuleRewrite, error) {
	var rewrites []*model.HTTPRuleRewrite
	if err := h.DB.Where("http_rule_id = ?", httpRuleID).Find(&rewrites).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return rewrites, nil
		}
		return nil, err
	}
	return rewrites, nil
}

// DeleteByHTTPRuleID -
func (h *HTTPRuleRewriteDaoTmpl) DeleteByHTTPRuleID(httpRuleID string) error {
	return h.DB.Where("http_rule_id in (?) ", httpRuleID).Delete(&model.HTTPRuleRewrite{}).Error
}

// DeleteByHTTPRuleIDs deletes http rule rewrites by given httpRuleIDs.
func (h *HTTPRuleRewriteDaoTmpl) DeleteByHTTPRuleIDs(httpRuleIDs []string) error {
	if err := h.DB.Where("http_rule_id in (?)", httpRuleIDs).Delete(&model.HTTPRuleRewrite{}).Error; err != nil {
		return errors.Wrap(err, "delete http rule rewrites")
	}
	return nil
}

// TCPRuleDaoTmpl is a implementation of TcpRuleDao
type TCPRuleDaoTmpl struct {
	DB *gorm.DB
}

// AddModel adds model.TCPRule
func (t *TCPRuleDaoTmpl) AddModel(mo model.Interface) error {
	tcpRule := mo.(*model.TCPRule)
	var oldTCPRule model.TCPRule
	if ok := t.DB.Where("uuid = ? or (ip=? and port=?)", tcpRule.UUID, tcpRule.IP, tcpRule.Port).Find(&oldTCPRule).RecordNotFound(); ok {
		if err := t.DB.Create(tcpRule).Error; err != nil {
			return err
		}
	} else {
		return fmt.Errorf("TCPRule already exists based on uuid(%s) or host %s and port %d exist", tcpRule.UUID, tcpRule.IP, tcpRule.Port)
	}
	return nil
}

// UpdateModel updates model.TCPRule
func (t *TCPRuleDaoTmpl) UpdateModel(mo model.Interface) error {
	tr, ok := mo.(*model.TCPRule)
	if !ok {
		return fmt.Errorf("failed to convert %s to *model.TCPRule", reflect.TypeOf(mo).String())
	}

	return t.DB.Save(tr).Error
}

// GetTCPRuleByServiceIDAndContainerPort gets a TCPRule based on serviceID and containerPort
func (t *TCPRuleDaoTmpl) GetTCPRuleByServiceIDAndContainerPort(serviceID string,
	containerPort int) ([]*model.TCPRule, error) {
	var result []*model.TCPRule
	if err := t.DB.Where("service_id = ? and container_port = ?", serviceID,
		containerPort).Find(&result).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return result, nil
		}
		return nil, err
	}
	return result, nil
}

// GetTCPRuleByID gets a TCPRule based on tcpRuleID
func (t *TCPRuleDaoTmpl) GetTCPRuleByID(id string) (*model.TCPRule, error) {
	result := &model.TCPRule{}
	if err := t.DB.Where("uuid = ?", id).Find(result).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return result, nil
}

// GetTCPRuleByServiceID gets a TCPRules based on service id.
func (t *TCPRuleDaoTmpl) GetTCPRuleByServiceID(sid string) ([]*model.TCPRule, error) {
	var result []*model.TCPRule
	if err := t.DB.Where("service_id = ?", sid).Find(&result).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return result, nil
}

// DeleteByID deletes model.TCPRule
func (t *TCPRuleDaoTmpl) DeleteByID(uuid string) error {
	return t.DB.Where("uuid = ?", uuid).Delete(&model.TCPRule{}).Error
}

// DeleteTCPRuleByServiceID deletes model.TCPRule
func (t *TCPRuleDaoTmpl) DeleteTCPRuleByServiceID(serviceID string) error {
	var tcpRule = &model.TCPRule{}
	return t.DB.Where("service_id = ?", serviceID).Delete(tcpRule).Error
}

// DeleteByComponentPort deletes tcp rules based on the given component id and port.
func (t *TCPRuleDaoTmpl) DeleteByComponentPort(componentID string, port int) error {
	if err := t.DB.Where("service_id=? and container_port=?", componentID, port).Delete(&model.TCPRule{}).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return errors.Wrap(bcode.ErrIngressTCPRuleNotFound, "delete tcp rules")
		}
		return errors.Wrap(err, "delete tcp rules")
	}
	return nil
}

//GetUsedPortsByIP get used port by ip
//sort by port
func (t *TCPRuleDaoTmpl) GetUsedPortsByIP(ip string) ([]*model.TCPRule, error) {
	var rules []*model.TCPRule
	if ip == "0.0.0.0" {
		if err := t.DB.Order("port asc").Find(&rules).Error; err != nil {
			return nil, err
		}
		return rules, nil
	}
	if err := t.DB.Where("ip = ? or ip = ?", ip, "0.0.0.0").Order("port asc").Find(&rules).Error; err != nil {
		return nil, err
	}
	return rules, nil
}

// ListByServiceID lists all TCPRules matching serviceID
func (t *TCPRuleDaoTmpl) ListByServiceID(serviceID string) ([]*model.TCPRule, error) {
	var rules []*model.TCPRule
	if err := t.DB.Where("service_id = ?", serviceID).Find(&rules).Error; err != nil {
		return nil, err
	}
	return rules, nil
}

//DeleteByComponentIDs delete tcp rule by component ids
func (t *TCPRuleDaoTmpl) DeleteByComponentIDs(componentIDs []string) error {
	return t.DB.Where("service_id in (?) ", componentIDs).Delete(&model.TCPRule{}).Error
}

// CreateOrUpdateTCPRuleInBatch Batch insert or update tcp rule
func (t *TCPRuleDaoTmpl) CreateOrUpdateTCPRuleInBatch(tcpRules []*model.TCPRule) error {
	dbType := t.DB.Dialect().GetName()
	if dbType == "sqlite3" {
		for _, tcpRule := range tcpRules {
			if ok := t.DB.Where("ID=? ", tcpRule.ID).Find(&tcpRule).RecordNotFound(); !ok {
				if err := t.DB.Model(&tcpRule).Where("ID = ?", tcpRule.ID).Update(tcpRule).Error; err != nil {
					logrus.Error("batch Update or update tcpRule error:", err)
					return err
				}
			} else {
				if err := t.DB.Create(&tcpRule).Error; err != nil {
					logrus.Error("batch create tcpRule error:", err)
					return err
				}
			}
		}
		return nil
	}
	var objects []interface{}
	for _, tcpRule := range tcpRules {
		objects = append(objects, *tcpRule)
	}
	if err := gormbulkups.BulkUpsert(t.DB, objects, 2000); err != nil {
		return errors.Wrap(err, "create or update tcp rule in batch")
	}
	return nil
}

// GwRuleConfigDaoImpl is a implementation of GwRuleConfigDao.
type GwRuleConfigDaoImpl struct {
	DB *gorm.DB
}

// AddModel creates a new gateway rule config.
func (t *GwRuleConfigDaoImpl) AddModel(mo model.Interface) error {
	cfg := mo.(*model.GwRuleConfig)
	var old model.GwRuleConfig
	err := t.DB.Where("`rule_id` = ? and `key` = ?", cfg.RuleID, cfg.Key).Find(&old).Error
	if err == gorm.ErrRecordNotFound {
		if err := t.DB.Create(cfg).Error; err != nil {
			return err
		}
	} else {
		return fmt.Errorf("RuleID: %s; Key: %s; %v", cfg.RuleID, cfg.Key, err)
	}
	return nil
}

// UpdateModel updates a gateway rule config.
func (t *GwRuleConfigDaoImpl) UpdateModel(mo model.Interface) error {
	return nil
}

// DeleteByRuleID deletes gateway rule configs by rule id.
func (t *GwRuleConfigDaoImpl) DeleteByRuleID(rid string) error {
	return t.DB.Where("rule_id=?", rid).Delete(&model.GwRuleConfig{}).Error
}

// ListByRuleID lists GwRuleConfig by rule id.
func (t *GwRuleConfigDaoImpl) ListByRuleID(rid string) ([]*model.GwRuleConfig, error) {
	var res []*model.GwRuleConfig
	err := t.DB.Where("rule_id = ?", rid).Find(&res).Error
	if err != nil {
		return nil, err
	}
	return res, nil
}

// DeleteByRuleIDs deletes rule configs based on the given ruleIDs.
func (t *GwRuleConfigDaoImpl) DeleteByRuleIDs(ruleIDs []string) error {
	if err := t.DB.Where("rule_id in (?)", ruleIDs).Delete(&model.GwRuleConfig{}).Error; err != nil {
		return errors.Wrap(err, "delete rule configs")
	}
	return nil
}

// CreateOrUpdateGwRuleConfigsInBatch creates or updates rule configs in batch.
func (t *GwRuleConfigDaoImpl) CreateOrUpdateGwRuleConfigsInBatch(ruleConfigs []*model.GwRuleConfig) error {
	dbType := t.DB.Dialect().GetName()
	if dbType == "sqlite3" {
		for _, ruleConfig := range ruleConfigs {
			if ok := t.DB.Where("ID=? ", ruleConfig.ID).Find(&ruleConfig).RecordNotFound(); !ok {
				if err := t.DB.Model(&ruleConfig).Where("ID = ?", ruleConfig.ID).Update(ruleConfig).Error; err != nil {
					logrus.Error("batch Update or update ruleConfig error:", err)
					return err
				}
			} else {
				if err := t.DB.Create(&ruleConfig).Error; err != nil {
					logrus.Error("batch create ruleConfig error:", err)
					return err
				}
			}
		}
		return nil
	}
	var objects []interface{}
	for _, ruleConfig := range ruleConfigs {
		objects = append(objects, *ruleConfig)
	}
	if err := gormbulkups.BulkUpsert(t.DB, objects, 2000); err != nil {
		return errors.Wrap(err, "create or update rule configs in batch")
	}
	return nil
}
