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

package template

import (
	"fmt"
	"github.com/golang/glog"
	"github.com/goodrain/rainbond/gateway/controller/openresty/model"
	"strings"
	text_template "text/template"
)

var (
	funcMap = text_template.FuncMap{
		"empty": func(input interface{}) bool {
			check, ok := input.(string)
			if ok {
				return len(check) == 0
			}
			return true
		},
		"buildLuaHeaderRouter": buildLuaHeaderRouter,
	}
)

func buildLuaHeaderRouter(input interface{}) string {
	loc, ok := input.(*model.Location)
	if !ok {
		glog.Errorf("expected an '*model.Location' type but %T was returned", input)
		return ""
	}
	out := []string{
		"access_by_lua_block {",
	}
	if loc.Header != nil {
		var condition []string
		for key, val := range loc.Header {
			str := fmt.Sprintf("\t\t\tlocal %s = ngx.var.http_%s", key, key)
			out = append(out, str)
			condition = append(condition, fmt.Sprintf("%s == \"%s\"", key, val))
		}
		cond := strings.Join(condition, " and ")
		out = append(out, fmt.Sprintf("\t\t\tif %s then", cond))
		out = append(out, fmt.Sprintf("\t\t\t\tngx.var.target = \"%s\"", loc.ProxyPass))
		out = append(out, fmt.Sprintf("\t\t\tend\n\r\t\t}\n\r"))

		return strings.Join(out, "\n\r")
	} else if loc.Cookie != nil {
		var condition []string
		out = append(out, `
			local common = require("common")

			local cookie = ngx.var.http_set_cookie
			local tbl = common.split(cookie, ";")
			local map = {}
			for _, v in pairs(tbl) do
                local list = common.split(v, "=")
                map[list[1]] = list[2]
            end
			`)
		for key, val := range loc.Cookie {
			condition = append(condition, fmt.Sprintf("map[\"%s\"] == \"%s\"", key, val))
		}
		cond := strings.Join(condition, " and ")
		out = append(out, fmt.Sprintf("\t\t\tif %s then", cond))
		out = append(out, fmt.Sprintf("\t\t\tngx.var.target = \"%s\"", loc.ProxyPass))
		out = append(out, fmt.Sprintf("\t\t\tend\n\r\t\t}"))

		return strings.Join(out, "\n\r")
	} else {
		return fmt.Sprintf(`
		access_by_lua_block {
			ngx.var.target = '%s';
		}
		`, loc.ProxyPass)
	}
}
