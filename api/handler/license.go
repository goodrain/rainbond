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
	apimodel "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"

	"github.com/jinzhu/gorm"
	"github.com/pquerna/ffjson/ffjson"
	"github.com/sirupsen/logrus"
)

// LicenseAction LicenseAction
type LicenseAction struct{}

// PackLicense PackLicense
func (l *LicenseAction) PackLicense(encrypted string) ([]byte, error) {
	return decrypt(key, encrypted)
}

// StoreLicense StoreLicense
func (l *LicenseAction) StoreLicense(license, token string) error {

	ls := &dbmodel.LicenseInfo{
		Token:   token,
		License: license,
	}
	if err := db.GetManager().LicenseDao().AddModel(ls); err != nil {
		return err
	}
	return nil
}

// LicensesInfos LicensesInfos
// 验证
type LicensesInfos struct {
	Infos map[string]*apimodel.LicenseInfo
}

// ShowInfos ShowInfos
func (l *LicensesInfos) ShowInfos() (map[string]*apimodel.LicenseInfo, error) {
	return l.Infos, nil
}

// ListLicense lise license
func ListLicense() (map[string]*apimodel.LicenseInfo, error) {
	licenses, err := db.GetManager().LicenseDao().ListLicenses()
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, err
		}
	}
	LMlicense := make(map[string]*apimodel.LicenseInfo)
	for _, license := range licenses {
		mLicense := &apimodel.LicenseInfo{}
		lc, err := GetLicenseHandler().PackLicense(license.License)
		if err != nil {
			logrus.Errorf("init license error.")
			continue
		}
		if err := ffjson.Unmarshal(lc, mLicense); err != nil {
			logrus.Errorf("unmashal license error, %v", err)
			continue
		}
		LMlicense[license.Token] = mLicense
	}
	return LMlicense, nil
}
