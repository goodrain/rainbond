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

package controller

import (
	"net/http"

	"github.com/pquerna/ffjson/ffjson"

	"github.com/go-chi/chi"

	"github.com/goodrain/rainbond/pkg/api/util"
	"github.com/goodrain/rainbond/pkg/node/api/handler"
)

//ServiceDiscover service discover service
func ServiceDiscover(w http.ResponseWriter, r *http.Request) {
	serviceInfo := chi.URLParam(r, "service_name")
	//eg: serviceInfo := test_gr123456_201711031246
	sds, err := handler.GetDiscoverManager().DiscoverService(serviceInfo)
	if err != nil {
		err.Handle(r, w)
		return
	}
	sdsJ, errJ := ffjson.Marshal(sds)
	if errJ != nil {
		util.CreateAPIHandleError(500, errJ).Handle(r, w)
		return
	}
	w.WriteHeader(200)
	w.Write([]byte(sdsJ))
}
