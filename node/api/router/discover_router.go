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

package router

import (
	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/node/api/controller"
)

//DisconverRoutes common plugin discover plugin config
func DisconverRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/ping", controller.Ping)
	r.Mount("/resources", SourcesRoutes())
	return r
}

//SourcesRoutes SourcesRoutes
//GET /v1/resources/(string: tenant_id)/(string: service_alias)/(string: plugin_id)
func SourcesRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/{tenant_id}/{service_alias}/{plugin_id}", controller.PluginResourcesConfig)
	return r
}
