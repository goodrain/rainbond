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
func (g *GatewayAction) AddHttpRule(httpRule *model.HttpRule, tx *gorm.DB) error {
	return g.dbmanager.HttpRuleDaoTransactions(tx).AddModel(httpRule)
}

func (g *GatewayAction) UpdateHttpRule(req *apimodel.HttpRuleStruct,
	tx *gorm.DB) (httpRule *model.HttpRule, err error) {
	rule, err := g.dbmanager.HttpRuleDaoTransactions(tx).GetHttpRuleByServiceIDAndContainerPort(req.ServiceID,
		req.ContainerPort)
	if err != nil {
		return nil, err
	}
	if rule == nil {
		return nil, fmt.Errorf("HttpRule dosen't exist based on ServiceID(%s) " +
			"and ContainerPort(%v)", req.ServiceID, req.ContainerPort)
	}
	// delete old Certificate
	if err := g.dbmanager.CertificateDaoTransactions(tx).DeleteCertificateByID(rule.CertificateID); err != nil {
		return nil, err
	}
	// delete old RuleExtensions
	if err := g.dbmanager.RuleExtensionDaoTransactions(tx).DeleteRuleExtensionByRuleID(rule.UUID); err != nil {
		return nil, err
	}

	rule.Path = req.Path
	rule.Domain = req.Domain
	rule.Header = req.Header
	rule.Cookie = req.Cookie
	rule.LoadBalancerType = req.LoadBalancerType
	rule.CertificateID = req.CertificateID

	return rule, g.dbmanager.HttpRuleDaoTransactions(tx).UpdateModel(rule)
}

// DeleteHttpRule deletes http rule, including certificate and rule extensions
func (g *GatewayAction) DeleteHttpRule(req *apimodel.HttpRuleStruct) error {
	// begin transaction
	tx := db.GetManager().Begin()
	// delete http rule
	httpRule, err := g.dbmanager.HttpRuleDaoTransactions(tx).DeleteHttpRuleByServiceIDAndContainerPort(
		req.ServiceID, req.ContainerPort)
	if err != nil {
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
func (g *GatewayAction) AddCertificate(req *apimodel.HttpRuleStruct, tx *gorm.DB) error {
	cert := &model.Certificate{
		UUID:            req.CertificateID,
		CertificateName: req.CertificateName,
		Certificate:     req.Certificate,
		PrivateKey:      req.PrivateKey,
	}

	return g.dbmanager.CertificateDaoTransactions(tx).AddModel(cert)
}

func (g *GatewayAction) UpdateCertificate(req apimodel.HttpRuleStruct, httpRule *model.HttpRule, tx *gorm.DB) error {
	// delete old certificate
	cert, err := g.dbmanager.CertificateDaoTransactions(tx).GetCertificateByID(req.CertificateID)
	if err != nil {
		return err
	}
	if cert == nil {
		return fmt.Errorf("Certificate doesn't exist based on certificateID(%s)", req.CertificateID)
	}

	cert.CertificateName = req.CertificateName
	cert.Certificate = req.Certificate
	cert.PrivateKey = req.PrivateKey
	return g.dbmanager.CertificateDaoTransactions(tx).UpdateModel(cert)
}

// AddRuleExtensions adds rule extensions to db if any of they doesn't exists
func (g *GatewayAction) AddRuleExtensions(ruleID string, ruleExtensions []*apimodel.RuleExtensionStruct, tx *gorm.DB) error {
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
