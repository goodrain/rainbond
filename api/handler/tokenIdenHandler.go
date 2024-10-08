// Copyright (C) 2014-2018 Goodrain Co., Ltd.
// RAINBOND, Application Management Platform

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
	"os"

	apimodel "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/api/util"
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
)

// TokenMapHandler DefaultTokenMapHandler
type TokenMapHandler interface {
	AddTokenIntoMap(rui *dbmodel.RegionUserInfo)
	DeleteTokenFromMap(oldtoken string, rui *dbmodel.RegionUserInfo)
	CheckToken(token, uri string) bool
	GetAPIManager() map[string][]*dbmodel.RegionAPIClass
	AddAPIManager(am *apimodel.APIManager) *util.APIHandleError
	DeleteAPIManager(am *apimodel.APIManager) *util.APIHandleError
	InitTokenMap() error
}

var defaultTokenIdenHandler TokenMapHandler

// TokenMap TokenMap
type TokenMap map[string]*dbmodel.RegionUserInfo

var defaultTokenMap map[string]*dbmodel.RegionUserInfo

var defaultSourceURI map[string][]*dbmodel.RegionAPIClass

// CreateTokenIdenHandler create token identification handler
func CreateTokenIdenHandler() error {
	CreateDefaultTokenMap()
	var err error
	if defaultTokenIdenHandler != nil {
		return nil
	}
	defaultTokenIdenHandler, err = CreateTokenIdenManager()
	if err != nil {
		return err
	}
	return defaultTokenIdenHandler.InitTokenMap()
}

func createDefaultSourceURI() error {
	if defaultSourceURI != nil {
		return nil
	}
	var err error
	defaultSourceURI, err = resourceURI()
	if err != nil {
		return err
	}
	return nil
}

func resourceURI() (map[string][]*dbmodel.RegionAPIClass, error) {
	sourceMap := make(map[string][]*dbmodel.RegionAPIClass)
	nodeSource, err := db.GetManager().RegionAPIClassDao().GetPrefixesByClass(dbmodel.NODEMANAGER)
	if err != nil {
		return nil, err
	}
	sourceMap[dbmodel.NODEMANAGER] = nodeSource

	serverSource, err := db.GetManager().RegionAPIClassDao().GetPrefixesByClass(dbmodel.SERVERSOURCE)
	if err != nil {
		return nil, err
	}
	sourceMap[dbmodel.SERVERSOURCE] = serverSource
	return sourceMap, nil
}

// CreateDefaultTokenMap CreateDefaultTokenMap
func CreateDefaultTokenMap() {
	createDefaultSourceURI()
	if defaultTokenMap != nil {
		return
	}
	consoleToken := "defaulttokentoken"
	if os.Getenv("TOKEN") != "" {
		consoleToken = os.Getenv("TOKEN")
	}
	rui := &dbmodel.RegionUserInfo{
		Token:          consoleToken,
		APIRange:       dbmodel.ALLPOWER,
		ValidityPeriod: 3257894000,
	}
	tokenMap := make(map[string]*dbmodel.RegionUserInfo)
	tokenMap[consoleToken] = rui
	defaultTokenMap = tokenMap
	return
}

// GetTokenIdenHandler GetTokenIdenHandler
func GetTokenIdenHandler() TokenMapHandler {
	return defaultTokenIdenHandler
}

// GetDefaultTokenMap GetDefaultTokenMap
func GetDefaultTokenMap() map[string]*dbmodel.RegionUserInfo {
	return defaultTokenMap
}

// SetTokenCache SetTokenCache
func SetTokenCache(info *dbmodel.RegionUserInfo) {
	defaultTokenMap[info.Token] = info
}

// GetDefaultSourceURI GetDefaultSourceURI
func GetDefaultSourceURI() map[string][]*dbmodel.RegionAPIClass {
	return defaultSourceURI
}
