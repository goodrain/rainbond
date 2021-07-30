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
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
)

// ComponentIngressTask -
type ComponentIngressTask struct {
	ComponentID string `json:"service_id"`
	Action      string `json:"action"`
	Port        int    `json:"port"`
	IsInner     bool   `json:"is_inner"`
}

//GatewayHandler gateway api handler
type GatewayHandler interface {
	AddHTTPRule(req *apimodel.AddHTTPRuleStruct) error
	CreateHTTPRule(tx *gorm.DB, req *apimodel.AddHTTPRuleStruct) error
	UpdateHTTPRule(req *apimodel.UpdateHTTPRuleStruct) error
	DeleteHTTPRule(req *apimodel.DeleteHTTPRuleStruct) error
	DeleteHTTPRuleByServiceIDWithTransaction(sid string, tx *gorm.DB) error

	AddCertificate(req *apimodel.AddHTTPRuleStruct, tx *gorm.DB) error
	UpdateCertificate(req apimodel.AddHTTPRuleStruct, httpRule *dbmodel.HTTPRule, tx *gorm.DB) error

	AddTCPRule(req *apimodel.AddTCPRuleStruct) error
	CreateTCPRule(tx *gorm.DB, req *apimodel.AddTCPRuleStruct) error
	UpdateTCPRule(req *apimodel.UpdateTCPRuleStruct, minPort int) error
	DeleteTCPRule(req *apimodel.DeleteTCPRuleStruct) error
	DeleteTCPRuleByServiceIDWithTransaction(sid string, tx *gorm.DB) error
	AddRuleExtensions(ruleID string, ruleExtensions []*apimodel.RuleExtensionStruct, tx *gorm.DB) error
	GetAvailablePort(ip string, lock bool) (int, error)
	TCPIPPortExists(ip string, port int) bool
	// Deprecated.
	SendTaskDeprecated(in map[string]interface{}) error
	SendTask(task *ComponentIngressTask) error
	RuleConfig(req *apimodel.RuleConfigReq) error
	UpdCertificate(req *apimodel.UpdCertificateReq) error
	GetGatewayIPs() []IPAndAvailablePort
	ListHTTPRulesByCertID(certID string) ([]*dbmodel.HTTPRule, error)
	DeleteIngressRulesByComponentPort(tx *gorm.DB, componentID string, port int) error
	SyncHTTPRules(tx *gorm.DB, components []*apimodel.Component) error
	SyncTCPRules(tx *gorm.DB, components []*apimodel.Component) error
	SyncRuleConfigs(tx *gorm.DB, components []*apimodel.Component) error
}
