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
	"os"

	"github.com/goodrain/rainbond/cmd/api/option"
	dbmodel "github.com/goodrain/rainbond/pkg/db/model"
)

//TokenMapHandler DefaultTokenMapHandler
type TokenMapHandler interface {
	AddTokenIntoMap(rui *dbmodel.RegionUserInfo)
	CheckToken(token, uri string) bool
	InitTokenMap() error
}

var defaultTokenIdenHandler TokenMapHandler

//TokenMap TokenMap
type TokenMap map[string]*dbmodel.RegionUserInfo

var defaultTokenMap map[string]*dbmodel.RegionUserInfo

var defaultSourceURI map[string]int

//CreateTokenIdenHandler create token identification handler
func CreateTokenIdenHandler(conf option.Config) error {
	CreateDefaultTokenMap(conf)
	var err error
	if defaultTokenIdenHandler != nil {
		return nil
	}
	defaultTokenIdenHandler, err = CreateTokenIdenManager(conf)
	if err != nil {
		return err
	}
	return nil
}

func createDefaultSourceURI() {
	if defaultSourceURI != nil {
		return
	}
	SourceURI := make(map[string]int)
	SourceURI["nodes"] = 1
	SourceURI["tasks"] = 1
	SourceURI["tasktemps"] = 1
	SourceURI["taskgroups"] = 1
	SourceURI["configs"] = 1
	defaultSourceURI = SourceURI
	return
}

//CreateDefaultTokenMap CreateDefaultTokenMap
func CreateDefaultTokenMap(conf option.Config) {
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
		APIRange:       "all",
		ValidityPeriod: 3257894000,
	}
	tokenMap := make(map[string]*dbmodel.RegionUserInfo)
	tokenMap[consoleToken] = rui
	defaultTokenMap = tokenMap
	return
}

//GetTokenIdenHandler GetTokenIdenHandler
func GetTokenIdenHandler() TokenMapHandler {
	return defaultTokenIdenHandler
}

//GetDefaultTokenMap GetDefaultTokenMap
func GetDefaultTokenMap() map[string]*dbmodel.RegionUserInfo {
	return defaultTokenMap
}

//GetDefaultSourceURI GetDefaultSourceURI
func GetDefaultSourceURI() map[string]int {
	return defaultSourceURI
}
