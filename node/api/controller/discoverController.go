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

	"github.com/goodrain/rainbond/api/util"

	"github.com/go-chi/chi"
	httputil "github.com/goodrain/rainbond/util/http"
)

//PluginResourcesConfig discover plugin config
func PluginResourcesConfig(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "tenant_id")
	serviceAlias := chi.URLParam(r, "service_alias")
	pluginID := chi.URLParam(r, "plugin_id")
	ss, err := discoverService.GetPluginConfigs(r.Context(), namespace, serviceAlias, pluginID)
	if err != nil {
		util.CreateAPIHandleError(500, err).Handle(r, w)
		return
	}
	if ss == nil {
		util.CreateAPIHandleError(404, err).Handle(r, w)
	}
	httputil.ReturnNoFomart(r, w, 200, ss)
}
