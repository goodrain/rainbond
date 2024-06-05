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

package middleware

import (
	"encoding/base64"
	"io"
	"net/http"
	"strings"

	"github.com/goodrain/rainbond/api/handler"
	"github.com/goodrain/rainbond/api/util"
)

// FullToken token api校验
func FullToken(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.RequestURI, "/docs") {
			auth := r.Header.Get("Authorization")
			if auth == "" {
				w.Header().Set("WWW-Authenticate", `Basic realm="Dotcoo User Login"`)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			auths := strings.SplitN(auth, " ", 2)
			if len(auths) != 2 {
				return
			}
			authMethod := auths[0]
			authB64 := auths[1]
			switch authMethod {
			case "Basic":
				authstr, err := base64.StdEncoding.DecodeString(authB64)
				if err != nil {
					io.WriteString(w, "Unauthorized!\n")
					return
				}
				userPwd := strings.SplitN(string(authstr), ":", 2)
				if len(userPwd) != 2 {
					io.WriteString(w, "Unauthorized!\n")
					return
				}
				username := userPwd[0]
				password := userPwd[1]
				if username == "goodrain" && password == "goodrain-api-test" {
					next.ServeHTTP(w, r)
					return
				}
			default:
				io.WriteString(w, "Unauthorized!\n")
				return
			}
			w.Header().Set("WWW-Authenticate", `Basic realm="Dotcoo User Login"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		//logrus.Debugf("request uri is %s", r.RequestURI)
		t := r.Header.Get("Authorization")
		if tt := strings.Split(t, " "); len(tt) == 2 {
			if handler.GetTokenIdenHandler().CheckToken(tt[1], r.RequestURI) {
				next.ServeHTTP(w, r)
				return
			}
		}
		util.CloseRequest(r)
		w.WriteHeader(http.StatusUnauthorized)
	}
	return http.HandlerFunc(fn)
}
