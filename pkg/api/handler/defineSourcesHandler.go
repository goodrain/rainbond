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

//SourcesHandler define source handler
type SourcesHandler interface {
	CreateDefineSources(
		tenantID string,
		ss *api_model.SetDefineSourcesStruct) *util.APIHandleError
}

var defaultSourcesHandler SourcesHandler

//CreateSourcesHandler create define sources handler
func CreateSourcesHandler(conf option.Config) error {
	var err error
	if defaultSourcesHandler != nil {
		return nil
	}
	defaultSourcesHandler, err = CreateSourcesManager(conf)
	if err != nil {
		return err
	}
	return nil
}

//GetSourcesManager get manager
func GetSourcesManager() SourcesHandler {
	return defaultSourcesHandler
}
