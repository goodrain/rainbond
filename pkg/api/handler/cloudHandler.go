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

//CloudHandler define source handler
type CloudHandler interface {
	TokenDispatcher(gt *api_model.GetUserToken) (*api_model.TokenInfo, *util.APIHandleError)
	GetTokenInfo(eid string) (*dbmodel.RegionUserInfo, *util.APIHandleError)
	UpdateTokenTime(eid string, vd int) *util.APIHandleError
}

var defaultCloudHandler CloudHandler

//CreateCloudHandler create define sources handler
func CreateCloudHandler(conf option.Config) error {
	var err error
	if defaultCloudHandler != nil {
		return nil
	}
	defaultCloudHandler, err = CreateCloudManager(conf)
	if err != nil {
		return err
	}
	return nil
}

//GetCloudManager get manager
func GetCloudManager() CloudHandler {
	return defaultCloudHandler
}
