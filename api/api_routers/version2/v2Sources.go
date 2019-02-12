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
	"github.com/goodrain/rainbond/api/controller"
)

//PluginRouter plugin router
func (v2 *V2) defineSourcesRouter() chi.Router {
	r := chi.NewRouter()
	// service rule
	// url: v2/tenant/{tenant_name}/sources/{source_alias}/env-name
	// --- ---
	r.Post("/{source_alias}", controller.GetManager().SetDefineSource)
	r.Delete("/{source_alias}/{env_name}", controller.GetManager().DeleteDefineSource)
	r.Get("/{source_alias}/{env_name}", controller.GetManager().GetDefineSource)
	r.Put("/{source_alias}/{env_name}", controller.GetManager().UpdateDefineSource)
	return r
}

/*
	用户自定义资源使用
	1. 用户请求node discover地址使用 注入的环境变量BASEURL, 用以区分不同租户的设定与获取
	BASEURL=DOCKERURL/TENANT_ID
	DOCKERURL="http://172.30.42.1"
	TENANT_ID="10bsdfafadfa1231231231"
*/
