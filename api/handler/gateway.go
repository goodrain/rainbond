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

func (g *GatewayAction) AddHttpRule(httpRule *model.HttpRule, tx *gorm.DB) error {
	return g.dbmanager.HttpRuleDaoTransactions(tx).AddModel(httpRule)
}

// AddCertificate adds certificate to db if is doesn't exists
func (g *GatewayAction) AddCertificate(certificate *model.Certificate, tx *gorm.DB) error {
	return g.dbmanager.CertificateDaoTransactions(tx).AddModel(certificate)
}

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
