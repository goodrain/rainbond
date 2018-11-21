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
	"github.com/goodrain/rainbond/api/middleware"
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
		logrus.Debug("Update application rule.")
	case "Delete":
		g.deleteHttpRule(w, r)
		logrus.Debugf("Delete application rule.")
	}
}

func (g *GatewayStruct) addHttpRule(w http.ResponseWriter, r *http.Request) {
	logrus.Debugf("add application rule.")
	var req api_model.AppRuleStruct
	ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &req, nil)
	if !ok {
		return
	}
	reqJson, _ := json.Marshal(req)
	logrus.Debugf("Request is : %s", string(reqJson))

	serviceID := r.Context().Value(middleware.ContextKey("service_id")).(string)

	httpRule := &model.HttpRule{
		UUID:             util.NewUUID(),
		ServiceID:        serviceID,
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
		cert := &model.Certificate{
			UUID:            req.CertificateID,
			CertificateName: req.CertificateName,
			Certificate:     req.Certificate,
			PrivateKey:      req.PrivateKey,
		}
		if err := h.AddCertificate(cert, tx); err != nil {
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

}

func (g *GatewayStruct) deleteHttpRule(w http.ResponseWriter, r *http.Request) {

}
