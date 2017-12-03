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

package middleware

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/goodrain/rainbond/pkg/api/handler"
	"github.com/goodrain/rainbond/pkg/api/util"

	"github.com/Sirupsen/logrus"
)

//Token 简单token验证
func Token(next http.Handler) http.Handler {
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
				fmt.Println("error")
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
		token := os.Getenv("TOKEN")
		t := r.Header.Get("Authorization")
		if tt := strings.Split(t, " "); len(tt) == 2 {
			if tt[1] == token {
				next.ServeHTTP(w, r)
				return
			}
		}
		util.CloseRequest(r)
		w.WriteHeader(http.StatusUnauthorized)
	}
	return http.HandlerFunc(fn)
}

//FullToken token api校验
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
				fmt.Println("error")
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
		uris := strings.Split(r.RequestURI, "/")
		if len(uris) < 3 {
			util.CloseRequest(r)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		sourceURI := uris[2]
		logrus.Debugf("request uri is %s", sourceURI)
		t := r.Header.Get("Authorization")
		if tt := strings.Split(t, " "); len(tt) == 2 {
			if handler.GetTokenIdenHandler().CheckToken(tt[1], sourceURI) {
				next.ServeHTTP(w, r)
				return
			}
		}
		util.CloseRequest(r)
		w.WriteHeader(http.StatusUnauthorized)
	}
	return http.HandlerFunc(fn)
}

//License license验证
func License(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		licenseMap, err := handler.GetLicensesInfosHandler().ShowInfos()
		if err != nil {
			logrus.Errorf("Get license map error, %v", err)
			return
		}
		l := r.Header.Get("License")
		//logrus.Debugf("Request License is :" + l)
		if l == "" {
			logrus.Debugf("need license")
			util.CloseRequest(r)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if _, ok := licenseMap[l]; !ok {
			logrus.Debugf("have no license suit for %v", l)
			util.CloseRequest(r)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		logrus.Debugf("license info for %v is :%v", l, licenseMap[l])
		//TODO: 细致校验，时间，接口
		next.ServeHTTP(w, r)
		return
	}
	return http.HandlerFunc(fn)
}
