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

//GatewayHandler gateway api handler
type GatewayHandler interface {
	AddHTTPRule(req *apimodel.AddHTTPRuleStruct) (string, error)
	UpdateHTTPRule(req *apimodel.UpdateHTTPRuleStruct) (string, error)
	DeleteHTTPRule(req *apimodel.DeleteHTTPRuleStruct) (string, error)

	AddCertificate(req *apimodel.AddHTTPRuleStruct, tx *gorm.DB) error
	UpdateCertificate(req apimodel.AddHTTPRuleStruct, httpRule *dbmodel.HTTPRule, tx *gorm.DB) error

	AddTCPRule(req *apimodel.AddTCPRuleStruct) (string, error)
	UpdateTCPRule(req *apimodel.UpdateTCPRuleStruct, minPort int) (string, error)
	DeleteTCPRule(req *apimodel.DeleteTCPRuleStruct) (string, error)

	AddRuleExtensions(ruleID string, ruleExtensions []*apimodel.RuleExtensionStruct, tx *gorm.DB) error

	GetAvailablePort() (int, error)
	PortExists(port int) bool
	SendTask(serviceID string, action string) error
	TCPAvailable(ip string, port int, ruleID string) bool
	AddIPPool(req *apimodel.IPPoolStruct) error
}
