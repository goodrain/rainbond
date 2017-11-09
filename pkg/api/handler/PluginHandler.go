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
	dbmodel "github.com/goodrain/rainbond/pkg/db/model"
)

//PluginHandler plugin handler
type PluginHandler interface {
	CreatePluginAct(cps *api_model.CreatePluginStruct) *util.APIHandleError
	UpdatePluginAct(pluginID string, cps *api_model.UpdatePluginStruct) *util.APIHandleError
	DeletePluginAct(pluginID string) *util.APIHandleError
	GetPlugins(tenantID string) ([]*dbmodel.TenantPlugin, *util.APIHandleError)
	AddDefaultEnv(est *api_model.ENVStruct) *util.APIHandleError
	UpdateDefaultEnv(est *api_model.ENVStruct) *util.APIHandleError
	DeleteDefaultEnv(pluginID, ENVName string) *util.APIHandleError
	BuildPluginManual(bps *api_model.BuildPluginStruct) (string, *util.APIHandleError)
	GetAllPluginBuildVersions(pluginID string) ([]*dbmodel.TenantPluginBuildVersion, *util.APIHandleError)
	GetPluginBuildVersion(pluginID, versionID string) (*dbmodel.TenantPluginBuildVersion, *util.APIHandleError)
	DeletePluginBuildVersion(pluginID, versionID string) *util.APIHandleError
	GetDefaultEnv(pluginID string) ([]*dbmodel.TenantPluginDefaultENV, *util.APIHandleError)
	GetEnvsWhichCanBeSet(serviceID, pluginID string) (interface{}, *util.APIHandleError)
}

var defaultPluginHandler PluginHandler

//CreatePluginHandler create plugin handler
func CreatePluginHandler(conf option.Config) error {
	var err error
	if defaultPluginHandler != nil {
		return nil
	}
	defaultPluginHandler, err = CreatePluginManager(conf)
	if err != nil {
		return err
	}
	return nil
}

//GetPluginManager get manager
func GetPluginManager() PluginHandler {
	return defaultPluginHandler
}
