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
	"time"

	"github.com/goodrain/rainbond/cmd/api/option"
	"github.com/goodrain/rainbond/pkg/db"
	dbmodel "github.com/goodrain/rainbond/pkg/db/model"
)

//TokenIdenAction  TokenIdenAction
type TokenIdenAction struct{}

//CreateTokenIdenManager token identification
func CreateTokenIdenManager(conf option.Config) (*TokenIdenAction, error) {
	return &TokenIdenAction{}, nil
}

//AddTokenIntoMap AddTokenIntoMap
func (t *TokenIdenAction) AddTokenIntoMap(rui *dbmodel.RegionUserInfo) {
	m := GetDefaultTokenMap()
	m[rui.Token] = rui
}

//CheckToken CheckToken
func (t *TokenIdenAction) CheckToken(token, uri string) bool {
	m := GetDefaultTokenMap()
	regionInfo, ok := m[token]
	if !ok {
		return false
	}
	if regionInfo.ValidityPeriod < int(time.Now().Unix()) {
		return false
	}
	switch regionInfo.Range {
	case "source":
		sm := GetDefaultSourceURI()
		if _, ok := sm[uri]; ok {
			return false
		}
		return true
	case "all":
		return true
	case "node":
		sm := GetDefaultSourceURI()
		if _, ok := sm[uri]; ok {
			return true
		}
		return false
	}
	return false
}

//InitTokenMap InitTokenMap
func (t *TokenIdenAction) InitTokenMap() error {
	ruis, err := db.GetManager().RegionUserInfoDao().GetALLTokenInValidityPeriod()
	if err != nil {
		return err
	}
	m := GetDefaultTokenMap()
	for _, rui := range ruis {
		m[rui.Token] = rui
	}
	return nil
}
