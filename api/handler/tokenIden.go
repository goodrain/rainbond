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
	"fmt"
	"strings"
	"time"

	apimodel "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/api/util"
	"github.com/goodrain/rainbond/cmd/api/option"
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
)

// TokenIdenAction  TokenIdenAction
type TokenIdenAction struct{}

// CreateTokenIdenManager token identification
func CreateTokenIdenManager(conf option.Config) (*TokenIdenAction, error) {
	return &TokenIdenAction{}, nil
}

// AddTokenIntoMap AddTokenIntoMap
func (t *TokenIdenAction) AddTokenIntoMap(rui *dbmodel.RegionUserInfo) {
	m := GetDefaultTokenMap()
	m[rui.Token] = rui
}

// DeleteTokenFromMap DeleteTokenFromMap
func (t *TokenIdenAction) DeleteTokenFromMap(oldtoken string, rui *dbmodel.RegionUserInfo) {
	m := GetDefaultTokenMap()
	t.AddTokenIntoMap(rui)
	delete(m, oldtoken)
}

// GetAPIManager GetAPIManager
func (t *TokenIdenAction) GetAPIManager() map[string][]*dbmodel.RegionAPIClass {
	return GetDefaultSourceURI()
}

// AddAPIManager AddAPIManager
func (t *TokenIdenAction) AddAPIManager(am *apimodel.APIManager) *util.APIHandleError {
	m := GetDefaultSourceURI()
	ra := &dbmodel.RegionAPIClass{
		ClassLevel: am.Body.ClassLevel,
		Prefix:     am.Body.Prefix,
	}
	if sourceList, ok := m[am.Body.ClassLevel]; ok {
		sourceList = append(sourceList, ra)
	} else {
		//支持新增类型
		newL := []*dbmodel.RegionAPIClass{ra}
		m[am.Body.ClassLevel] = newL
	}
	ra.URI = am.Body.URI
	ra.Alias = am.Body.Alias
	ra.Remark = am.Body.Remark
	if err := db.GetManager().RegionAPIClassDao().AddModel(ra); err != nil {
		return util.CreateAPIHandleErrorFromDBError("add api manager", err)
	}
	return nil
}

// DeleteAPIManager DeleteAPIManager
func (t *TokenIdenAction) DeleteAPIManager(am *apimodel.APIManager) *util.APIHandleError {
	m := GetDefaultSourceURI()
	if sourceList, ok := m[am.Body.ClassLevel]; ok {
		var newL []*dbmodel.RegionAPIClass
		for _, s := range sourceList {
			if s.Prefix == am.Body.Prefix {
				continue
			}
			newL = append(newL, s)
		}
		if len(newL) == 0 {
			//该级别分组为空时，删除该资源分组
			delete(m, am.Body.ClassLevel)
		} else {
			m[am.Body.ClassLevel] = newL
		}
	} else {
		return util.CreateAPIHandleError(400, fmt.Errorf("have no api class level about %v", am.Body.ClassLevel))
	}
	if err := db.GetManager().RegionAPIClassDao().DeletePrefixInClass(am.Body.ClassLevel, am.Body.Prefix); err != nil {
		return util.CreateAPIHandleErrorFromDBError("delete api prefix", err)
	}
	return nil
}

// CheckToken CheckToken
func (t *TokenIdenAction) CheckToken(token, uri string) bool {
	m := GetDefaultTokenMap()
	//logrus.Debugf("default token map is %v", m)
	regionInfo, ok := m[token]
	if !ok {
		var err error
		regionInfo, err = db.GetManager().RegionUserInfoDao().GetTokenByTokenID(token)
		if err != nil {
			return false
		}
		SetTokenCache(regionInfo)
	}
	if regionInfo.ValidityPeriod < int(time.Now().Unix()) {
		return false
	}
	switch regionInfo.APIRange {
	case dbmodel.ALLPOWER:
		return true
	case dbmodel.SERVERSOURCE:
		sm := GetDefaultSourceURI()
		smL, ok := sm[dbmodel.SERVERSOURCE]
		if !ok {
			return false
		}
		rc := false
		for _, urinfo := range smL {
			if strings.HasPrefix(uri, urinfo.Prefix) {
				rc = true
			}
		}
		return rc
	case dbmodel.NODEMANAGER:
		sm := GetDefaultSourceURI()
		smL, ok := sm[dbmodel.NODEMANAGER]
		if !ok {
			return false
		}
		rc := false
		for _, urinfo := range smL {
			if strings.HasPrefix(uri, urinfo.Prefix) {
				rc = true
			}
		}
		return rc
	}
	return false
}

// InitTokenMap InitTokenMap
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
