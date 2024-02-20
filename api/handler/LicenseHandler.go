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

import apimodel "github.com/goodrain/rainbond/api/model"

// LicenseHandler LicenseAction
type LicenseHandler interface {
	PackLicense(encrypted string) ([]byte, error)
	StoreLicense(license, token string) error
}

var defaultLicenseHandler LicenseHandler

// CreateLicenseManger create service manager
func CreateLicenseManger() error {
	defaultLicenseHandler = &LicenseAction{}
	return nil
}

// GetLicenseHandler get license handler
func GetLicenseHandler() LicenseHandler {
	return defaultLicenseHandler
}

//license验证

// LicenseInfoHandler LicenseInfoHandler
type LicenseInfoHandler interface {
	ShowInfos() (map[string]*apimodel.LicenseInfo, error)
}

var defaultLicensesInfosHandler LicenseInfoHandler

// CreateLicensesInfoManager CreateLicensesInfoManager
func CreateLicensesInfoManager() error {
	if defaultLicensesInfosHandler == nil {
		listInfos, err := ListLicense()
		if err != nil {
			return err
		}
		defaultLicensesInfosHandler = &LicensesInfos{
			Infos: listInfos,
		}
	}
	return nil
}

// GetLicensesInfosHandler GetLicensesInfosHandler
func GetLicensesInfosHandler() LicenseInfoHandler {
	return defaultLicensesInfosHandler
}
