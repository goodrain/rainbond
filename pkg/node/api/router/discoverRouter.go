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

package router

import (
	"github.com/goodrain/rainbond/pkg/node/api/controller"

	"github.com/go-chi/chi"
)

//DisconverRoutes 发现服务api
func DisconverRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/ping", controller.Ping)
	r.Mount("/listeners", ListenersRoutes())
	r.Mount("/clusters", ClustersRoutes())
	r.Mount("/registration", RegistrationRoutes())
	return r
}

//ListenersRoutes listeners routes lds
//GET /v1/listeners/(string: service_cluster)/(string: service_node)
func ListenersRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/ping", controller.Ping)
	r.Get("/{tenant_name}/{service_nodes}", controller.ListenerDiscover)
	return r
}

//ClustersRoutes cds
//GET /v1/clusters/(string: service_cluster)/(string: service_node)
func ClustersRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/ping", controller.Ping)
	return r
}

//RegistrationRoutes sds
//GET /v1/registration/(string: service_name)
func RegistrationRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/ping", controller.Ping)
	r.Get("/{service_name}", controller.ServiceDiscover)
	return r
}
