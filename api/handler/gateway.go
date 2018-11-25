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

package handler

import (
	"fmt"
	apimodel "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/util"
	"github.com/jinzhu/gorm"
)

type GatewayAction struct {
	dbmanager db.Manager
}

//CreateManager creates gateway manager.
func CreateGatewayManager(dbmanager db.Manager) *GatewayAction {
	return &GatewayAction{
		dbmanager: dbmanager,
	}
}

// AddHttpRule adds http rule to db if it doesn't exists.
func (g *GatewayAction) AddHttpRule(req *apimodel.HTTPRuleStruct) error {
	httpRule := &model.HTTPRule{
		UUID:          req.HTTPRuleID,
		ServiceID:     req.ServiceID,
		ContainerPort: req.ContainerPort,
		Domain:        req.Domain,
		Path:          req.Path,
		Header:        req.Header,
		Cookie:        req.Cookie,
		IP:            req.IP,
		CertificateID: req.CertificateID,
	}

	// begin transaction
	tx := db.GetManager().Begin()
	if err := db.GetManager().HttpRuleDaoTransactions(tx).AddModel(httpRule); err != nil {
		tx.Rollback()
		return err
	}

	cert := &model.Certificate{
		UUID:            req.CertificateID,
		CertificateName: fmt.Sprintf("cert-%s", util.NewUUID()[0:8]),
		Certificate:     req.Certificate,
		PrivateKey:      req.PrivateKey,
	}
	if err := db.GetManager().CertificateDaoTransactions(tx).AddModel(cert); err != nil {
		tx.Rollback()
		return err
	}

	for _, ruleExtension := range req.RuleExtensions {
		re := &model.RuleExtension{
			UUID:   util.NewUUID(),
			RuleID: httpRule.UUID,
			Key:    ruleExtension.Key,
			Value:  ruleExtension.Value,
		}
		if err := db.GetManager().RuleExtensionDaoTransactions(tx).AddModel(re); err != nil {
			tx.Rollback()
			return err
		}
	}

	// end transaction
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return err
	}
	return nil
}

// UpdateHttpRule updates http rule
func (g *GatewayAction) UpdateHttpRule(req *apimodel.HTTPRuleStruct) error {
	tx := db.GetManager().Begin()
	rule, err := g.dbmanager.HttpRuleDaoTransactions(tx).GetHttpRuleByID(req.HTTPRuleID)

	if err != nil {
		tx.Rollback()
		return err
	}
	if rule == nil {
		tx.Rollback()
		return fmt.Errorf("HTTPRule dosen't exist based on uuid(%s)", req.HTTPRuleID)
	}
	// delete old Certificate
	if err := g.dbmanager.CertificateDaoTransactions(tx).DeleteCertificateByID(rule.CertificateID); err != nil {
		tx.Rollback()
		return err
	}
	// delete old RuleExtensions
	if err := g.dbmanager.RuleExtensionDaoTransactions(tx).DeleteRuleExtensionByRuleID(rule.UUID); err != nil {
		tx.Rollback()
		return err
	}
	// add new certificate
	cert := &model.Certificate{
		UUID:            req.CertificateID,
		CertificateName: fmt.Sprintf("cert-%s", util.NewUUID()[0:8]),
		Certificate:     req.Certificate,
		PrivateKey:      req.PrivateKey,
	}
	if err := g.dbmanager.CertificateDaoTransactions(tx).AddModel(cert); err != nil {
		tx.Rollback()
		return err
	}
	// add new rule extensions
	for _, ruleExtension := range req.RuleExtensions {
		re := &model.RuleExtension{
			UUID:   util.NewUUID(),
			RuleID: rule.UUID,
			Key:    ruleExtension.Key,
			Value:  ruleExtension.Value,
		}
		if err := db.GetManager().RuleExtensionDaoTransactions(tx).AddModel(re); err != nil {
			tx.Rollback()
			return err
		}
	}
	// update http rule
	rule.ServiceID = req.ServiceID
	rule.ContainerPort = req.ContainerPort
	rule.Domain = req.Domain
	rule.Path = req.Path
	rule.Header = req.Header
	rule.Cookie = req.Cookie
	rule.CertificateID = req.CertificateID
	if err := db.GetManager().HttpRuleDaoTransactions(tx).UpdateModel(rule); err != nil {
		tx.Rollback()
		return err
	}
	// end transaction
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return err
	}
	return nil
}

// DeleteHttpRule deletes http rule, including certificate and rule extensions
func (g *GatewayAction) DeleteHttpRule(req *apimodel.HTTPRuleStruct) error {
	// begin transaction
	tx := db.GetManager().Begin()
	// delete http rule
	httpRule, err := g.dbmanager.HttpRuleDaoTransactions(tx).GetHttpRuleByID(req.HTTPRuleID)
	if err != nil {
		tx.Rollback()
		return err
	}
	if err := g.dbmanager.HttpRuleDaoTransactions(tx).DeleteHttpRuleByID(httpRule.UUID); err != nil {
		tx.Rollback()
		return err
	}
	// delete certificate
	if err := g.dbmanager.CertificateDaoTransactions(tx).DeleteCertificateByID(httpRule.CertificateID); err != nil {
		tx.Rollback()
		return err
	}
	// delete rule extension
	if err := g.dbmanager.RuleExtensionDaoTransactions(tx).DeleteRuleExtensionByRuleID(httpRule.UUID); err != nil {
		tx.Rollback()
		return err
	}
	// end transaction
	if err := tx.Commit().Error; err != nil {
		return err
	}

	return nil
}

// AddCertificate adds certificate to db if it doesn't exists
func (g *GatewayAction) AddCertificate(req *apimodel.HTTPRuleStruct, tx *gorm.DB) error {
	cert := &model.Certificate{
		UUID:            req.CertificateID,
		CertificateName: fmt.Sprintf("cert-%s", util.NewUUID()[0:8]),
		Certificate:     req.Certificate,
		PrivateKey:      req.PrivateKey,
	}

	return g.dbmanager.CertificateDaoTransactions(tx).AddModel(cert)
}

func (g *GatewayAction) UpdateCertificate(req apimodel.HTTPRuleStruct, httpRule *model.HTTPRule,
	tx *gorm.DB) error {
	// delete old certificate
	cert, err := g.dbmanager.CertificateDaoTransactions(tx).GetCertificateByID(req.CertificateID)
	if err != nil {
		return err
	}
	if cert == nil {
		return fmt.Errorf("Certificate doesn't exist based on certificateID(%s)", req.CertificateID)
	}

	cert.CertificateName = fmt.Sprintf("cert-%s", util.NewUUID()[0:8])
	cert.Certificate = req.Certificate
	cert.PrivateKey = req.PrivateKey
	return g.dbmanager.CertificateDaoTransactions(tx).UpdateModel(cert)
}

// AddTcpRule adds tcp rule.
func (g *GatewayAction) AddTcpRule(req *apimodel.TCPRuleStruct) error {
	tcpRule := &model.TCPRule{
		UUID:          req.TCPRuleID,
		ServiceID:     req.ServiceID,
		ContainerPort: req.ContainerPort,
		IP:            req.IP,
		Port:          req.Port,
	}

	// begin transaction
	tx := db.GetManager().Begin()
	// add tcp rule
	if err := g.dbmanager.TcpRuleDaoTransactions(tx).AddModel(tcpRule); err != nil {
		tx.Rollback()
		return err
	}

	// add rule extensions
	for _, ruleExtension := range req.RuleExtensions {
		re := &model.RuleExtension{
			UUID:   util.NewUUID(),
			RuleID: tcpRule.UUID,
			Value:  ruleExtension.Value,
		}
		if err := g.dbmanager.RuleExtensionDaoTransactions(tx).AddModel(re); err != nil {
			tx.Rollback()
			return err
		}
	}

	// end transaction
	if err := tx.Commit().Error; err != nil {
		return err
	}

	return nil
}

func (g *GatewayAction) UpdateTcpRule(req *apimodel.TCPRuleStruct) error {
	// begin transaction
	tx := db.GetManager().Begin()
	// get old tcp rule
	tcpRule, err := g.dbmanager.TcpRuleDaoTransactions(tx).GetTcpRuleByID(req.TCPRuleID)
	if err != nil {
		tx.Rollback()
		return err
	}
	// delete old rule extensions
	if err := g.dbmanager.RuleExtensionDaoTransactions(tx).DeleteRuleExtensionByRuleID(tcpRule.UUID); err != nil {
		tx.Rollback()
		return err
	}
	// update tcp rule
	tcpRule.ServiceID = req.ServiceID
	tcpRule.ContainerPort = req.ContainerPort
	tcpRule.IP = req.IP
	tcpRule.Port = req.Port
	if err := g.dbmanager.TcpRuleDaoTransactions(tx).UpdateModel(tcpRule); err != nil {
		tx.Rollback()
		return err
	}
	// add new rule extensions
	for _, ruleExtension := range req.RuleExtensions {
		re := &model.RuleExtension{
			UUID:   util.NewUUID(),
			RuleID: tcpRule.UUID,
			Value:  ruleExtension.Value,
		}
		if err := g.dbmanager.RuleExtensionDaoTransactions(tx).AddModel(re); err != nil {
			tx.Rollback()
			return err
		}
	}

	// end transaction
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return err
	}

	return nil
}

func (g *GatewayAction) DeleteTcpRule(req *apimodel.TCPRuleStruct) error {
	// begin transaction
	tx := db.GetManager().Begin()
	tcpRule, err := db.GetManager().TcpRuleDaoTransactions(tx).GetTcpRuleByID(req.TCPRuleID)
	if err != nil {
		tx.Rollback()
		return err
	}
	// delete rule extensions
	if err := db.GetManager().RuleExtensionDaoTransactions(tx).DeleteRuleExtensionByRuleID(tcpRule.UUID); err != nil {
		tx.Rollback()
		return err
	}
	// delete tcp rule
	if err := db.GetManager().TcpRuleDaoTransactions(tx).DeleteTcpRule(tcpRule); err != nil {
		tx.Rollback()
		return err
	}
	// end transaction
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return err
	}
	return nil
}

// AddRuleExtensions adds rule extensions to db if any of they doesn't exists
func (g *GatewayAction) AddRuleExtensions(ruleID string, ruleExtensions []*apimodel.RuleExtensionStruct,
	tx *gorm.DB) error {
	for _, ruleExtension := range ruleExtensions {
		re := &model.RuleExtension{
			UUID:   util.NewUUID(),
			RuleID: ruleID,
			Value:  ruleExtension.Value,
		}
		err := g.dbmanager.RuleExtensionDaoTransactions(tx).AddModel(re)
		if err != nil {
			return err
		}
	}
	return nil
}
