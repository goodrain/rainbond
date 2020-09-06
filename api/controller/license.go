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
	"errors"
	"fmt"
	"net/http"

	"github.com/goodrain/rainbond/api/handler"

	httputil "github.com/goodrain/rainbond/util/http"

	validator "github.com/goodrain/rainbond/util/govalidator"
	"github.com/sirupsen/logrus"
)

//LicenseManager license manager
type LicenseManager struct{}

var licenseManager *LicenseManager

//GetLicenseManager get license Manager
func GetLicenseManager() *LicenseManager {
	if licenseManager != nil {
		return licenseManager
	}
	licenseManager = &LicenseManager{}
	return licenseManager
}

//AnalystLicense AnalystLicense
// swagger:operation POST /license license SendLicense
//
// 提交license
//
// post license & get token
//
// ---
// produces:
// - application/json
// - application/xml
// parameters:
// - name: license
//   in: form
//   description: license
//   required: true
//   type: string
//   format: string
//
// Responses:
//   '200':
//	   description: '{"bean":"{\"token\": \"Q3E5OXdoZDZDX3drN0QtV2gtVmpRaGtlcHJQYmFK\"}"}'
func (l *LicenseManager) AnalystLicense(w http.ResponseWriter, r *http.Request) {
	rule := validator.MapData{
		"license": []string{"required"},
	}
	data, ok := httputil.ValidatorRequestMapAndErrorResponse(r, w, rule, nil)
	if !ok {
		return
	}
	license := data["license"].(string)
	logrus.Debugf("license is %v", license)
	/*
		text, err := handler.GetLicenseHandler().PackLicense(license)
		if err != nil {
			httputil.ReturnError(r, w, 500, fmt.Sprintf("%v", err))
			return
		}
	*/
	token, errT := handler.BasePack([]byte(license))
	if errT != nil {
		httputil.ReturnError(r, w, 500, "pack license error")
		return
	}
	logrus.Debugf("token is %v", token)
	if err := handler.GetLicenseHandler().StoreLicense(license, token); err != nil {
		logrus.Debugf("%s", err)
		logrus.Debugf("%s", fmt.Errorf("license is exist"))
		if err == errors.New("license is exist") {
			//err  license is exist
			httputil.ReturnError(r, w, 400, fmt.Sprintf("storage token error, %v", err))
		}
		httputil.ReturnError(r, w, 500, fmt.Sprintf("storage token error, %v", err))
		return
	}
	rc := fmt.Sprintf(`{"token": "%v"}`, token)
	httputil.ReturnSuccess(r, w, &rc)
}
