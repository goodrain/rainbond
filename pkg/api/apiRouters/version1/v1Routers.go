
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

package version1

import (
	"github.com/go-chi/chi"
)

//Routes routes
func (v1 *V1Routes) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", Show)
	r.NotFound(v1.UnFoundRequest)
	r.Mount("/services", v1.servicesRouter())

	return r
}

func (v1 *V1Routes) servicesRouter() chi.Router {
	//v1 services router
	r := chi.NewRouter()
	r.Post("/lifecycle/{service_id}/stop", v1.APIFuncV1.StopService)
	r.Post("/lifecycle/{service_id}/stop/", v1.APIFuncV1.StopService)
	r.Post("/lifecycle/{service_id}/start", v1.APIFuncV1.StartService)
	r.Post("/lifecycle/{service_id}/start/", v1.APIFuncV1.StartService)
	r.Post("/lifecycle/{service_id}/restart", v1.APIFuncV1.RestartService)
	r.Post("/lifecycle/{service_id}/restart/", v1.APIFuncV1.RestartService)
	r.Post("/lifecycle/{service_id}/vertical", v1.APIFuncV1.VerticalService)
	r.Post("/lifecycle/{service_id}/vertical/", v1.APIFuncV1.VerticalService)
	r.Put("/lifecycle/{service_id}/horizontal", v1.APIFuncV1.HorizontalService)
	r.Put("/lifecycle/{service_id}/horizontal/", v1.APIFuncV1.HorizontalService)
	r.Post("/lifecycle/{service_id}/deploy", v1.APIFuncV1.DeployService)
	r.Post("/lifecycle/{service_id}/deploy/", v1.APIFuncV1.DeployService)
	r.Post("/lifecycle/{service_id}/upgrade", v1.APIFuncV1.UpgradeService)
	r.Put("/lifecycle/{service_id}/upgrade", v1.APIFuncV1.UpgradeService)
	r.Post("/lifecycle/{service_id}/upgrade/", v1.APIFuncV1.UpgradeService)
	r.Put("/lifecycle/{service_id}/upgrade/", v1.APIFuncV1.UpgradeService)
	r.Post("/lifecycle/{service_id}/build/", v1.APIFuncV1.BuildService)
	r.Post("/lifecycle/{service_id}/build", v1.APIFuncV1.BuildService)
	r.Post("/lifecycle/{service_id}/status/", v1.APIFuncV1.StatusService)
	r.Get("/lifecycle/{service_id}/status/", v1.APIFuncV1.StatusService)
	r.Post("/lifecycle/{service_id}/status", v1.APIFuncV1.StatusService)
	r.Get("/lifecycle/{service_id}/status", v1.APIFuncV1.StatusService)
	r.Post("/lifecycle/status", v1.APIFuncV1.StatusServiceList)
	r.Post("/lifecycle/status/", v1.APIFuncV1.StatusServiceList)
	r.Post("/lifecycle/{service_id}/containerIds", v1.APIFuncV1.StatusContainerID)
	r.Post("/lifecycle/{service_id}/containerIds/", v1.APIFuncV1.StatusContainerID)
	return r
}

func (v1 *V1Routes) tenantRouter() chi.Router {
	r := chi.NewRouter()
	return r
}

func (v1 *V1Routes) lbRouter() chi.Router {
	r := chi.NewRouter()
	return r
}
