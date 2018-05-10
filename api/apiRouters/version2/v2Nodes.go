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

package version2

import (
	"github.com/goodrain/rainbond/api/controller"

	"github.com/go-chi/chi"
)

func (v2 *V2) nodesRouter() chi.Router {
	r := chi.NewRouter()
	//node uri
	r.Get("/", controller.GetManager().Nodes)
	r.Get("/resources",controller.GetManager().Nodes)
	r.Get("/{node}/details", controller.GetManager().Nodes)
	r.Get("/{node}/basic", controller.GetManager().Nodes)
	r.Delete("/{node}", controller.GetManager().Nodes)
	r.Post("/{node}/label", controller.GetManager().Nodes)
	r.Put("/{node}/reschedulable", controller.GetManager().Nodes)
	r.Put("/{node}/unschedulable", controller.GetManager().Nodes)
	r.Put("/{node}", controller.GetManager().Nodes)
	r.Post("/{node}", controller.GetManager().Nodes)
	r.Post("/{node}/down", controller.GetManager().Nodes)


	r.Put("/login", controller.GetManager().Nodes)
	r.Put("/{ip}/init",controller.GetManager().Nodes)
	r.Get("/{ip}/init/status",controller.GetManager().Nodes)
	r.Get("/{ip}/install/status", controller.GetManager().Nodes)
	r.Put("/{ip}/install", controller.GetManager().Nodes)
	return r
}

func (v2 *V2) appsRouter() chi.Router {
	r := chi.NewRouter()
	//job uri


	//r.Get("/{app_name}/register", controller.GetManager().Apps)
	r.Get("/{app_name}/discover", controller.GetManager().Apps)
	//r.Get("/", controller.GetManager().Apps)
	return r
}
func (v2 *V2) jobsRouter() chi.Router {
	r := chi.NewRouter()
	//job uri
	r.Get("/", controller.GetManager().Jobs)
	r.Put("/", controller.GetManager().Jobs)
	r.Get("/group", controller.GetManager().Jobs)
	r.Get("/{group}-{id}", controller.GetManager().Jobs)
	r.Post("/{group}-{id}", controller.GetManager().Jobs)
	r.Delete("/{group}-{id}", controller.GetManager().Jobs)
	r.Put("/{group}-{id}/execute/{name}", controller.GetManager().Jobs)

	r.Get("/{group}-{id}/nodes", controller.GetManager().Jobs)
	return r
}
