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
	"net/http"
	"strings"

	httputil "github.com/goodrain/rainbond/util/http"

	"github.com/go-chi/chi"
	"github.com/sirupsen/logrus"
)

// APPRegister 服务注册
func APPRegister(w http.ResponseWriter, r *http.Request) {
	appName := strings.TrimSpace(chi.URLParam(r, "app_name"))
	logrus.Infof(appName)
}

//APPDiscover 服务发现
//用于实时性要求不高的场景，例如docker发现event_log地址
//请求API返回可用地址

func APPDiscover(w http.ResponseWriter, r *http.Request) {
	appName := strings.TrimSpace(chi.URLParam(r, "app_name"))
	endpoints := appService.FindAppEndpoints(appName)
	if endpoints == nil || len(endpoints) == 0 {
		httputil.ReturnError(r, w, 404, "app endpoints not found")
		return
	}
	httputil.ReturnSuccess(r, w, endpoints)
}

// APPList 列出已注册应用
func APPList(w http.ResponseWriter, r *http.Request) {

}
