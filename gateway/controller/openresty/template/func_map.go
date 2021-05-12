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
	"regexp"
	"strings"
	text_template "text/template"

	"github.com/golang/glog"
	"github.com/goodrain/rainbond/gateway/controller/openresty/model"
	v1 "github.com/goodrain/rainbond/gateway/v1"
	"github.com/sirupsen/logrus"
)

const (
	slash         = "/"
	nonIdempotent = "non_idempotent"
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
		"isValidByteSize":      isValidByteSize,
		"buildNextUpstream":    buildNextUpstream,
	}
)

func buildLuaHeaderRouter(input interface{}) string {
	loc, ok := input.(*model.Location)
	if !ok {
		glog.Errorf("expected an '*model.Location' type but %T was returned", input)
		return ""
	}
	_ = loc
	out := []string{"access_by_lua_block {"}

	priority := make([]string, 3)
	for name, c := range loc.NameCondition {
		switch c.Type {
		case v1.HeaderType:
			snippet := []string{}
			cond1 := []string{}
			cond2 := []string{}
			for key, val := range c.Value {
				snippet = append(snippet, fmt.Sprintf("\t\t\tlocal %s = ngx.var.http_%s", key, key))
				cond1 = append(cond1, key)
				cond2 = append(cond2, fmt.Sprintf("%s == \"%s\"", key, val))
			}
			snippet = append(snippet, fmt.Sprintf("\t\t\tif %s then", strings.Join(cond1, " and ")))
			snippet = append(snippet, fmt.Sprintf("\t\t\t\tif %s then", strings.Join(cond2, " and ")))
			snippet = append(snippet, fmt.Sprintf("\t\t\t\t\tngx.var.target = \"%s\"", name))
			snippet = append(snippet, "\t\t\t\t\telse")
			snippet = append(snippet, "\t\t\t\t\t\t\tngx.exit(404)")
			snippet = append(snippet, "\t\t\t\tend")
			snippet = append(snippet, "\t\t\telseif ngx.var.target == 'default' then")
			snippet = append(snippet, "\t\t\t\tngx.exit(404)")
			snippet = append(snippet, "\t\t\tend")
			priority[2] = strings.Join(snippet, "\n\r")
		case v1.CookieType:
			var snippet []string
			snippet = append(snippet, `
			string.split = function(s, p)
                local rt= {}
				string.gsub(s, '[^'..p..']+', function(w) table.insert(rt, w) end )
                return rt
            end
			local cookie = ngx.var.http_Cookie
			if cookie then
				local tbl = string.split(cookie, ";")
				local map = {}
				for _, v in pairs(tbl) do
					local list = string.split(v, "=")
					map[list[1]] = list[2]
				end
			`)
			var condition []string
			for key, val := range c.Value {
				condition = append(condition, fmt.Sprintf("map[\"%s\"] == \"%s\"", key, val))
			}
			snippet = append(snippet, fmt.Sprintf("\t\t\t\tif %s then", strings.Join(condition, " and ")))
			snippet = append(snippet, fmt.Sprintf("\t\t\t\t\tngx.var.target = \"%s\"", name))
			snippet = append(snippet, "\t\t\t\telse")
			snippet = append(snippet, "\t\t\t\t\tngx.exit(404)")
			snippet = append(snippet, "\t\t\t\tend")
			snippet = append(snippet, "\t\t\t\telseif ngx.var.target == 'default' then")
			snippet = append(snippet, "\t\t\t\t\tngx.exit(404)")
			snippet = append(snippet, "\t\t\tend")

			priority[1] = strings.Join(snippet, "\n\r")
		default:
			snippet := fmt.Sprintf("\t\t\tngx.var.target = \"%s\"", name)
			priority[0] = snippet
		}
	}

	for i := 0; i < 3; i++ {
		if priority[i] != "" {
			out = append(out, priority[i])
		}
	}

	out = append(out, "\t\t}")

	return strings.Join(out, "\n\r")
}

// refer to http://nginx.org/en/docs/syntax.html
// Nginx differentiates between size and offset
// offset directives support gigabytes in addition
var nginxSizeRegex = regexp.MustCompile("^[0-9]+[kKmM]{0,1}$")
var nginxOffsetRegex = regexp.MustCompile("^[0-9]+[kKmMgG]{0,1}$")

// isValidByteSize validates size units valid in nginx
// http://nginx.org/en/docs/syntax.html
func isValidByteSize(input interface{}, isOffset bool) bool {
	if _, ok := input.(int); ok {
		return true
	}
	s, ok := input.(string)
	if !ok {
		logrus.Errorf("expected an 'string' type but %T was returned", input)
		return false
	}

	s = strings.TrimSpace(s)
	if s == "" {
		logrus.Info("empty byte size, hence it will not be set")
		return false
	}

	if isOffset {
		return nginxOffsetRegex.MatchString(s)
	}

	return nginxSizeRegex.MatchString(s)
}

func buildNextUpstream(i, r interface{}) string {
	nextUpstream, ok := i.(string)
	if !ok {
		logrus.Errorf("expected a 'string' type but %T was returned", i)
		return ""
	}

	retryNonIdempotent := r.(bool)

	parts := strings.Split(nextUpstream, " ")

	nextUpstreamCodes := make([]string, 0, len(parts))
	for _, v := range parts {
		if v != "" && v != nonIdempotent {
			nextUpstreamCodes = append(nextUpstreamCodes, v)
		}

		if v == nonIdempotent {
			retryNonIdempotent = true
		}
	}

	if retryNonIdempotent {
		nextUpstreamCodes = append(nextUpstreamCodes, nonIdempotent)
	}

	return strings.Join(nextUpstreamCodes, " ")
}
