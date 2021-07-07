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
	"github.com/goodrain/rainbond/api/region"
	"net/http"

	"github.com/goodrain/rainbond/api/util/license"
	httputil "github.com/goodrain/rainbond/util/http"
)

//LicenseManager license manager
type LicenseManager struct {
	RegionClient region.Region
}

var licenseManager *LicenseManager

//GetLicenseManager get license Manager
func GetLicenseManager() *LicenseManager {
	if licenseManager != nil {
		return licenseManager
	}
	regionClient, _ := region.NewRegion(region.APIConf{
		Endpoints: []string{"http://127.0.0.1:8888"},
	})
	licenseManager = &LicenseManager{
		RegionClient: regionClient,
	}
	return licenseManager
}

func (l *LicenseManager) GetlicenseFeature(w http.ResponseWriter, r *http.Request) {
	//The following features are designed for license control.
	// GPU Support
	// Windows Container Support
	// Gateway security control
	features := []license.Feature{}
	if lic := license.ReadLicense(); lic != nil {
		features = lic.Features
	}
	httputil.ReturnSuccess(r, w, features)
}

func (l *LicenseManager) Getlicense(w http.ResponseWriter, r *http.Request) {
	var resp = license.LicenseResp{}
	if lic := license.ReadLicense(); lic != nil {
		resp.License = lic
	}
	nodes, err := l.RegionClient.Nodes().List()
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	resp.ActualNode = int64(len(nodes))
	httputil.ReturnSuccess(r, w, resp)
}
