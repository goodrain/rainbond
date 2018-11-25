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
	"encoding/json"
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/api/handler"
	api_model "github.com/goodrain/rainbond/api/model"
	httputil "github.com/goodrain/rainbond/util/http"
	"net/http"
	"net/url"
)

type GatewayStruct struct {
}

// HTTPRule is used to add, update or delete http rule which enables
// external traffic to access applications through the gateway
func (g *GatewayStruct) HttpRule(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		g.addHttpRule(w, r)
	case "PUT":
		g.updateHttpRule(w, r)
	case "DELETE":
		g.deleteHttpRule(w, r)
	}
}

func (g *GatewayStruct) addHttpRule(w http.ResponseWriter, r *http.Request) {
	logrus.Debugf("add http rule.")
	var req api_model.HTTPRuleStruct
	ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &req, nil)
	if !ok {
		return
	}
	reqJson, _ := json.Marshal(req)
	logrus.Debugf("Request is : %s", string(reqJson))

	// verify request
	values := url.Values{}
	if req.ServiceID == "" {
		values["service_id"] = []string{"The service_id field is required"}
	}
	if req.ContainerPort == 0 {
		values["container_port"] = []string{"The container_port field is required"}
	}
	if req.Domain == "" {
		values["domain"] = []string{"The domain field is required"}
	}
	if req.CertificateID != "" {
		if req.Certificate == "" {
			values["certificate"] = []string{"The certificate field is required"}
		}
		if req.PrivateKey == "" {
			values["private_key"] = []string{"The private_key field is required"}
		}
	}
	if len(values) != 0 {
		httputil.ReturnValidationError(r, w, values)
		return
	}

	h := handler.GetGatewayHandler()
	if err := h.AddHttpRule(&req); err != nil {
		httputil.ReturnError(r, w, 500, fmt.Sprintf("Unexpected error occorred while adding http rule: %v", err))
		return
	}

	httputil.ReturnSuccess(r, w, "success")
}

func (g *GatewayStruct) updateHttpRule(w http.ResponseWriter, r *http.Request) {
	logrus.Debugf("update http rule.")
	var req api_model.HTTPRuleStruct
	ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &req, nil)
	if !ok {
		return
	}
	reqJson, _ := json.Marshal(req)
	logrus.Debugf("Request is : %s", string(reqJson))

	h := handler.GetGatewayHandler()
	if err := h.UpdateHttpRule(&req); err != nil {
		httputil.ReturnError(r, w, 500, fmt.Sprintf("Unexpected error occorred while "+
			"updating http rule: %v", err))
		return
	}

	httputil.ReturnSuccess(r, w, "success")
}

func (g *GatewayStruct) deleteHttpRule(w http.ResponseWriter, r *http.Request) {
	logrus.Debugf("delete http rule.")
	var req api_model.HTTPRuleStruct
	ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &req, nil)
	if !ok {
		return
	}
	reqJson, _ := json.Marshal(req)
	logrus.Debugf("Request is : %s", string(reqJson))

	h := handler.GetGatewayHandler()
	err := h.DeleteHttpRule(&req)
	if err != nil {
		httputil.ReturnError(r, w, 500, fmt.Sprintf("Unexpected error occorred while delete http rule: %v", err))
		return
	}

	httputil.ReturnSuccess(r, w, "success")
}

// TCPRule is used to add, update or delete tcp rule which enables
// external traffic to access applications through the gateway
func (g *GatewayStruct) TcpRule(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		g.addTcpRule(w, r)
	case "PUT":
		g.updateTcpRule(w, r)
	case "DELETE":
		g.deleteTcpRule(w, r)
	}
}

func (g *GatewayStruct) addTcpRule(w http.ResponseWriter, r *http.Request) {
	logrus.Debugf("add tcp rule.")
	var req api_model.TCPRuleStruct
	ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &req, nil)
	if !ok {
		return
	}
	reqJson, _ := json.Marshal(req)
	logrus.Debugf("Request is : %s", string(reqJson))

	h := handler.GetGatewayHandler()
	if err := h.AddTcpRule(&req); err != nil {
		httputil.ReturnError(r, w, 500, fmt.Sprintf("Unexpected error occorred while "+
			"adding tcp rule: %v", err))
		return
	}

	httputil.ReturnSuccess(r, w, "success")
}

func (g *GatewayStruct) updateTcpRule(w http.ResponseWriter, r *http.Request) {
	logrus.Debugf("add tcp rule.")
	var req api_model.TCPRuleStruct
	ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &req, nil)
	if !ok {
		return
	}
	reqJson, _ := json.Marshal(req)
	logrus.Debugf("Request is : %s", string(reqJson))

	h := handler.GetGatewayHandler()
	if err := h.UpdateTcpRule(&req); err != nil {
		httputil.ReturnError(r, w, 500, fmt.Sprintf("Unexpected error occorred while "+
			"updating tcp rule: %v", err))
		return
	}

	httputil.ReturnSuccess(r, w, "success")
}

func (g *GatewayStruct) deleteTcpRule(w http.ResponseWriter, r *http.Request) {
	logrus.Debugf("delete tcp rule.")
	var req api_model.TCPRuleStruct
	ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &req, nil)
	if !ok {
		return
	}
	reqJson, _ := json.Marshal(req)
	logrus.Debugf("Request is : %s", string(reqJson))

	h := handler.GetGatewayHandler()
	if err := h.DeleteTcpRule(&req); err != nil {
		httputil.ReturnError(r, w, 500, fmt.Sprintf("Unexpected error occorred while "+
			"deleting tcp rule: %v", err))
		return
	}

	httputil.ReturnSuccess(r, w, "success")
}
