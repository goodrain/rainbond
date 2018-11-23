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

package controller

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/node/api"
	httputil "github.com/goodrain/rainbond/util/http"
)

//SetAPIRoute set api route
func (m *ManagerService) SetAPIRoute(apim *api.Manager) error {
	apim.GetRouter().Post("/services/{service_name}/stop", m.StopServiceAPI)
	apim.GetRouter().Post("/services/{service_name}/start", m.StartServiceAPI)
	apim.GetRouter().Post("/services/update", m.UpdateConfigAPI)
	return nil
}

//StartServiceAPI start a service
func (m *ManagerService) StartServiceAPI(w http.ResponseWriter, r *http.Request) {
	serviceName := strings.TrimSpace(chi.URLParam(r, "service_name"))
	if err := m.StartService(serviceName); err != nil {
		httputil.ReturnError(r, w, 400, err.Error())
	}
	httputil.ReturnSuccess(r, w, nil)
}

//StopServiceAPI stop a service
func (m *ManagerService) StopServiceAPI(w http.ResponseWriter, r *http.Request) {
	serviceName := strings.TrimSpace(chi.URLParam(r, "service_name"))
	if err := m.StopService(serviceName); err != nil {
		httputil.ReturnError(r, w, 400, err.Error())
	}
	httputil.ReturnSuccess(r, w, nil)
}

//UpdateConfigAPI update service config
func (m *ManagerService) UpdateConfigAPI(w http.ResponseWriter, r *http.Request) {
	if err := m.ReLoadServices(); err != nil {
		httputil.ReturnError(r, w, 400, err.Error())
	}
	httputil.ReturnSuccess(r, w, nil)
}
