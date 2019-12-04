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

package web

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi"
)

//getDockerLogs get history docker logs
func (s *SocketServer) getDockerLogs(w http.ResponseWriter, r *http.Request) {
	rows, _ := strconv.Atoi(r.URL.Query().Get("rows"))
	serviceID := chi.URLParam(r, "serviceID")
	loglist := s.storemanager.GetDockerLogs(serviceID, rows)
	content := strings.Join(loglist, "\n")
	w.Write([]byte(content))
}
