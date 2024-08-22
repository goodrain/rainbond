// Copyright (C) 2014-2019 Goodrain Co., Ltd.
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
// 文件: getter.go
// 说明: 该文件实现了数据获取的相关功能。文件中定义了用于从不同数据源中获取和处理数据的方法，
// 以支持平台中的各种功能模块。通过这些方法，Rainbond 平台能够高效地从外部资源中获取所需的数据，
// 并确保数据的准确性和及时性。

package web

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi"
	httputil "github.com/goodrain/rainbond/util/http"
)

// getDockerLogs get history docker logs
func (s *SocketServer) getDockerLogs(w http.ResponseWriter, r *http.Request) {
	rows, _ := strconv.Atoi(r.URL.Query().Get("rows"))
	serviceID := chi.URLParam(r, "serviceID")
	if rows == 0 {
		rows = 100
	}
	loglist := s.storemanager.GetDockerLogs(serviceID, rows)
	httputil.ReturnSuccess(r, w, loglist)
}
