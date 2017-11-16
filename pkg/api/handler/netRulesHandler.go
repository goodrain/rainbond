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
	"github.com/goodrain/rainbond/cmd/api/option"
	api_model "github.com/goodrain/rainbond/pkg/api/model"
	"github.com/goodrain/rainbond/pkg/api/util"
)

//NetRulesHandler net rules handler
type NetRulesHandler interface {
	CreateDownStreamNetRules(
		tenantID string,
		rs *api_model.SetNetDownStreamRuleStruct) *util.APIHandleError
	GetDownStreamNetRule(
		tenantID,
		serviceAlias,
		destServiceAlias,
		port string) (*api_model.NetRulesDownStreamBody, *util.APIHandleError)
	UpdateDownStreamNetRule(
		tenantID string,
		urs *api_model.UpdateNetDownStreamRuleStruct) *util.APIHandleError
}

var defaultNetRulesHandler NetRulesHandler

//CreateNetRulesHandler create plugin handler
func CreateNetRulesHandler(conf option.Config) error {
	var err error
	if defaultNetRulesHandler != nil {
		return nil
	}
	defaultNetRulesHandler, err = CreateNetRulesManager(conf)
	if err != nil {
		return err
	}
	return nil
}

//GetRulesManager get manager
func GetRulesManager() NetRulesHandler {
	return defaultNetRulesHandler
}
