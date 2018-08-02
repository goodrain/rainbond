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

package api

import (
	"net/http"

	"github.com/goodrain/rainbond/util"

	"github.com/go-chi/chi"
	httputil "github.com/goodrain/rainbond/util/http"
	"github.com/goodrain/rainbond/monitor/api/controller"
)

func APIServer(c *controller.ControllerManager) *chi.Mux {
	r := chi.NewRouter()
	r.Route("/monitor", func(r chi.Router) {
		r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
			bean := map[string]string{"status": "health", "info": "monitor service health"}
			httputil.ReturnSuccess(r, w, bean)
		})
	})
	r.Route("/v2/rules", func(r chi.Router) {
			r.Post("/", c.AddRules)
			r.Put("/{rules_name}", c.RegRules)
			r.Delete("/{rules_name}", c.DelRules)
			r.Get("/{rules_name}", c.GetRules)
			r.Get("/all", c.GetAllRules)
	})
	util.ProfilerSetup(r)
	return r
}
