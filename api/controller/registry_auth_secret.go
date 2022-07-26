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
	"fmt"
	"net/http"

	"github.com/goodrain/rainbond/api/handler"
	api_model "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/cmd/api/option"
	"github.com/goodrain/rainbond/mq/client"
	httputil "github.com/goodrain/rainbond/util/http"
)

// RegistryAuthSecretStruct -
type RegistryAuthSecretStruct struct {
	MQClient client.MQClient
	cfg      *option.Config
}

// RegistryAuthSecret http handler for registry auth secret
func (g *RegistryAuthSecretStruct) RegistryAuthSecret(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST", "PUT":
		g.addOrUpdateRegistryAuthSecret(w, r)
	case "DELETE":
		g.deleteRegistryAuthSecret(w, r)
	}
}

func (g *RegistryAuthSecretStruct) addOrUpdateRegistryAuthSecret(w http.ResponseWriter, r *http.Request) {
	var req api_model.AddOrUpdateRegistryAuthSecretStruct
	ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &req, nil)
	if !ok {
		return
	}

	h := handler.GetRegistryAuthSecretHandler()
	err := h.AddOrUpdateRegistryAuthSecret(&req)
	if err != nil {
		httputil.ReturnError(r, w, 500, fmt.Sprintf("Unexpected error occorred while adding auth secret: %v", err))
		return
	}

	httputil.ReturnSuccess(r, w, req)
}

func (g *RegistryAuthSecretStruct) deleteRegistryAuthSecret(w http.ResponseWriter, r *http.Request) {
	var req api_model.DeleteRegistryAuthSecretStruct
	ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &req, nil)
	if !ok {
		return
	}

	h := handler.GetRegistryAuthSecretHandler()
	err := h.DeleteRegistryAuthSecret(&req)
	if err != nil {
		httputil.ReturnError(r, w, 500, fmt.Sprintf("Unexpected error occorred while delete registry auth secret: %v", err))
		return
	}

	httputil.ReturnSuccess(r, w, "success")
}
