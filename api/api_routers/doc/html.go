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

package doc

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi"

	"github.com/sirupsen/logrus"
)

//Routes routes
func Routes() chi.Router {
	r := chi.NewRouter()
	workDir, _ := os.Getwd()
	//logrus.Debugf("workdir is %v", workDir)
	filesDir := filepath.Join(workDir, "html")
	//filesDir := "/Users/qingguo/gopath/src/github.com/goodrain/rainbond/hack/contrib/docker/api/html"
	logrus.Debugf("filesdir is %v", filesDir)
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("/docs"))
	})
	FileServer(r, "/docs", http.Dir(filesDir))
	FileServer(r, "/docs/", http.Dir(filesDir))
	return r
}

//FileServer file server
func FileServer(r chi.Router, path string, root http.FileSystem) {
	if strings.ContainsAny(path, "{}*") {
		panic("FileServer does not permit URL parameters.")
	}

	fs := http.StripPrefix(path, http.FileServer(root))

	if path != "/" && path[len(path)-1] != '/' {
		r.Get(path, http.RedirectHandler(path+"/", 301).ServeHTTP)
		path += "/"
	}
	path += "*"

	r.Get(path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fs.ServeHTTP(w, r)
	}))
}
