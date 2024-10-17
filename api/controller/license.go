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

package controller

import (
	"github.com/goodrain/rainbond/api/middleware"
	"github.com/goodrain/rainbond/api/util/license"
	httputil "github.com/goodrain/rainbond/util/http"
	"net/http"
)

// LicenseManager license manager
type LicenseManager struct{}

var licenseManager *LicenseManager

// GetLicenseManager get license Manager
func GetLicenseManager() *LicenseManager {
	if licenseManager != nil {
		return licenseManager
	}
	licenseManager = &LicenseManager{}
	return licenseManager
}

// Deprecated
func (l *LicenseManager) GetlicenseFeature(w http.ResponseWriter, r *http.Request) {
	//The following features are designed for license control.
	// GPU Support
	// Windows Container Support
	// Gateway security control
	features := []license.Feature{}
	httputil.ReturnSuccess(r, w, features)
}

// Getlicense -
func (l *LicenseManager) Getlicense(w http.ResponseWriter, r *http.Request) {
	err := middleware.LicenseVerification(r, true)
	if err != nil {
		httputil.ReturnError(r, w, 400, err.Error())
		return
	}
	httputil.ReturnSuccess(r, w, middleware.LicenseCache.Data)
}
