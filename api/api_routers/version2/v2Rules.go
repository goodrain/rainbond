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
	"github.com/go-chi/chi"
)

// PluginRouter plugin router
func (v2 *V2) rulesRouter() chi.Router {
	r := chi.NewRouter()
	// service rule
	// url: v2/tenant/{tenant_name}/services/{service_alias}/net-rule/xxx
	// -- --
	//upstream
	r.Mount("/upstream", v2.upstreamRouter())
	return r
}

func (v2 *V2) upstreamRouter() chi.Router {
	r := chi.NewRouter()
	return r
}
