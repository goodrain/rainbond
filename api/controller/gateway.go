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
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/util"
	httputil "github.com/goodrain/rainbond/util/http"
	"net/http"
)

type GatewayStruct struct {
}

// HttpRule is used to add, update or delete http rule which enables
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
	var req api_model.HttpRuleStruct
	ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &req, nil)
	if !ok {
		return
	}
	reqJson, _ := json.Marshal(req)
	logrus.Debugf("Request is : %s", string(reqJson))

	// TODO: shouldn't write the business logic here
	httpRule := &model.HttpRule{
		UUID:             util.NewUUID(),
		ServiceID:        req.ServiceID,
		ContainerPort:    req.ContainerPort,
		Domain:           req.Domain,
		Path:             req.Path,
		Header:           req.Header,
		Cookie:           req.Cookie,
		IP:               req.IP,
		LoadBalancerType: req.LoadBalancerType,
		CertificateID:    req.CertificateID,
	}

	h := handler.GetGatewayHandler()
	tx := db.GetManager().Begin()
	if err := h.AddHttpRule(httpRule, tx); err != nil {
		tx.Rollback()
		httputil.ReturnError(r, w, 500, fmt.Sprintf("Unexpected error occorred while adding http rule: %v", err))
		return
	}

	if req.CertificateID != "" {
		if err := h.AddCertificate(&req, tx); err != nil {
			httputil.ReturnError(r, w, 500, fmt.Sprintf("Unexpected error occorred while adding certificate: %v", err))
			return
		}
	}

	err := h.AddRuleExtensions(httpRule.UUID, req.RuleExtensions, tx)
	if err != nil {
		tx.Rollback()
		httputil.ReturnError(r, w, 500, fmt.Sprintf("Unexpected error occorred while adding rule extensions: %v", err))
		return
	}

	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		httputil.ReturnError(r, w, 500, fmt.Sprintf("Unexpected error occorred while commit transaction: %v", err))
		return
	}

	httputil.ReturnSuccess(r, w, "success")
}

func (g *GatewayStruct) updateHttpRule(w http.ResponseWriter, r *http.Request) {
	logrus.Debugf("update http rule.")
	var req api_model.HttpRuleStruct
	ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &req, nil)
	if !ok {
		return
	}
	reqJson, _ := json.Marshal(req)
	logrus.Debugf("Request is : %s", string(reqJson))

	// TODO: shouldn't write the business logic here
	// begin transaction
	tx := db.GetManager().Begin()
	h := handler.GetGatewayHandler()
	httpRule, err := h.UpdateHttpRule(&req, tx)
	if err != nil {
		tx.Rollback()
		httputil.ReturnError(r, w, 500, fmt.Sprintf("Unexpected error occorred while "+
			"updating http rule: %v", err))
		return
	}

	if err := h.AddCertificate(&req, tx); err != nil {
		tx.Rollback()
		httputil.ReturnError(r, w, 500, fmt.Sprintf("Unexpected error occorred while "+
			"updating certificate: %v", err))
		return
	}

	if err := h.AddRuleExtensions(httpRule.UUID, req.RuleExtensions, tx); err != nil {
		tx.Rollback()
		httputil.ReturnError(r, w, 500, fmt.Sprintf("Unexpected error occorred while adding rule extensions: %v", err))
		return
	}

	// end transaction
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		httputil.ReturnError(r, w, 500, fmt.Sprintf("Unexpected error occorred while committing transaction: %v", err))
		return
	}

	httputil.ReturnSuccess(r, w, "success")
}

func (g *GatewayStruct) deleteHttpRule(w http.ResponseWriter, r *http.Request) {
	logrus.Debugf("delete http rule.")
	var req api_model.HttpRuleStruct
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

// TcpRule is used to add, update or delete tcp rule which enables
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
	var req api_model.TcpRuleStruct
	ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &req, nil)
	if !ok {
		return
	}
	reqJson, _ := json.Marshal(req)
	logrus.Debugf("Request is : %s", string(reqJson))

	h := handler.GetGatewayHandler()
	if err := h.AddTcpRule(&req); err != nil {
		httputil.ReturnError(r, w, 500, fmt.Sprintf("Unexpected error occorred while " +
			"adding tcp rule: %v", err))
		return
	}

	httputil.ReturnSuccess(r, w, "success")
}

func (g *GatewayStruct) updateTcpRule(w http.ResponseWriter, r *http.Request) {
	logrus.Debugf("add tcp rule.")
	var req api_model.TcpRuleStruct
	ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &req, nil)
	if !ok {
		return
	}
	reqJson, _ := json.Marshal(req)
	logrus.Debugf("Request is : %s", string(reqJson))

	h := handler.GetGatewayHandler()
	if err := h.UpdateTcpRule(&req); err != nil {
		httputil.ReturnError(r, w, 500, fmt.Sprintf("Unexpected error occorred while " +
			"updating tcp rule: %v", err))
		return
	}

	httputil.ReturnSuccess(r, w, "success")
}

func (g *GatewayStruct) deleteTcpRule(w http.ResponseWriter, r *http.Request) {
	logrus.Debugf("delete tcp rule.")
	var req api_model.TcpRuleStruct
	ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &req, nil)
	if !ok {
		return
	}
	reqJson, _ := json.Marshal(req)
	logrus.Debugf("Request is : %s", string(reqJson))

	h := handler.GetGatewayHandler()
	if err := h.DeleteTcpRule(&req); err != nil {
		httputil.ReturnError(r, w, 500, fmt.Sprintf("Unexpected error occorred while " +
			"deleting tcp rule: %v", err))
		return
	}

	httputil.ReturnSuccess(r, w, "success")
}
