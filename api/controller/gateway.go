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
	"net/http"
	"net/url"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/api/handler"
	api_model "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/cmd/api/option"
	"github.com/goodrain/rainbond/mq/client"
	httputil "github.com/goodrain/rainbond/util/http"
)

// GatewayStruct -
type GatewayStruct struct {
	MQClient client.MQClient
	cfg      *option.Config
}

// HTTPRule is used to add, update or delete http rule which enables
// external traffic to access applications through the gateway
func (g *GatewayStruct) HTTPRule(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		g.addHTTPRule(w, r)
	case "PUT":
		g.updateHTTPRule(w, r)
	case "DELETE":
		g.deleteHTTPRule(w, r)
	}
}

func (g *GatewayStruct) addHTTPRule(w http.ResponseWriter, r *http.Request) {
	logrus.Debugf("add http rule.")
	var req api_model.AddHTTPRuleStruct
	ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &req, nil)
	if !ok {
		return
	}
	reqJSON, _ := json.Marshal(req)
	logrus.Debugf("Request is : %s", string(reqJSON))

	// verify request
	values := url.Values{}
	if req.ContainerPort == 0 {
		values["container_port"] = []string{"The container_port field is required"}
	}
	if strings.Replace(req.CertificateID, " ", "", -1) != "" {
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
	sid, err := h.AddHTTPRule(&req)
	if err != nil {
		httputil.ReturnError(r, w, 500, fmt.Sprintf("Unexpected error occorred while adding http rule: %v", err))
		return
	}

	if err := handler.GetGatewayHandler().SendTask(sid, "add-http-rule"); err != nil {
		logrus.Errorf("send runtime message about gateway failure %s", err.Error())
	}

	httputil.ReturnSuccess(r, w, "success")
}

func (g *GatewayStruct) updateHTTPRule(w http.ResponseWriter, r *http.Request) {
	logrus.Debugf("update http rule.")
	var req api_model.UpdateHTTPRuleStruct
	ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &req, nil)
	if !ok {
		return
	}
	reqJSON, _ := json.Marshal(req)
	logrus.Debugf("Request is : %s", string(reqJSON))

	// verify request
	values := url.Values{}
	if strings.Replace(req.CertificateID, " ", "", -1) != "" {
		if req.Certificate == "" {
			values["certificate"] = []string{"The certificate field is required"}
		}
		if req.PrivateKey == "" {
			values["private_key"] = []string{"The private_key field is required"}
		}
	}
	if len(req.RuleExtensions) > 0 {
		for _, re := range req.RuleExtensions {
			if re.Key == "" {
				values["key"] = []string{"The key field is required"}
				break
			}
			if re.Value == "" {
				values["value"] = []string{"The value field is required"}
				break
			}
		}
	}
	if len(values) != 0 {
		httputil.ReturnValidationError(r, w, values)
		return
	}

	h := handler.GetGatewayHandler()
	sid, err := h.UpdateHTTPRule(&req)
	if err != nil {
		httputil.ReturnError(r, w, 500, fmt.Sprintf("Unexpected error occorred while "+
			"updating http rule: %v", err))
		return
	}

	if err := handler.GetGatewayHandler().SendTask(sid, "update-http-rule"); err != nil {
		logrus.Errorf("send runtime message about gateway failure %s", err.Error())
	}

	httputil.ReturnSuccess(r, w, "success")
}

func (g *GatewayStruct) deleteHTTPRule(w http.ResponseWriter, r *http.Request) {
	logrus.Debugf("delete http rule.")
	var req api_model.DeleteHTTPRuleStruct
	ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &req, nil)
	if !ok {
		return
	}
	reqJSON, _ := json.Marshal(req)
	logrus.Debugf("Request is : %s", string(reqJSON))

	h := handler.GetGatewayHandler()
	serviceID, err := h.DeleteHTTPRule(&req)
	if err != nil {
		httputil.ReturnError(r, w, 500, fmt.Sprintf("Unexpected error occorred while delete http rule: %v", err))
		return
	}

	if err := handler.GetGatewayHandler().SendTask(serviceID, "delete-http-rule"); err != nil {
		logrus.Errorf("send runtime message about gateway failure %s", err.Error())
	}

	httputil.ReturnSuccess(r, w, "success")
}

// TCPRule is used to add, update or delete tcp rule which enables
// external traffic to access applications through the gateway
func (g *GatewayStruct) TCPRule(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		g.AddTCPRule(w, r)
	case "PUT":
		g.updateTCPRule(w, r)
	case "DELETE":
		g.deleteTCPRule(w, r)
	}
}

// AddTCPRule adds a tcp rule
func (g *GatewayStruct) AddTCPRule(w http.ResponseWriter, r *http.Request) {
	logrus.Debugf("add tcp rule.")
	var req api_model.AddTCPRuleStruct
	ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &req, nil)
	if !ok {
		return
	}
	reqJSON, _ := json.Marshal(req)
	logrus.Debugf("Request is : %s", string(reqJSON))

	h := handler.GetGatewayHandler()
	// verify request
	values := url.Values{}
	if req.ContainerPort == 0 {
		values["container_port"] = []string{"The container_port field is required"}
	}
	if req.Port == 0 {
		values["port"] = []string{"The port field is required"}
	} else if req.Port <= g.cfg.MinExtPort {
		values["port"] = []string{fmt.Sprintf("The port field should be greater than %d", g.cfg.MinExtPort)}
	} else {
		// check if the port exists
		if h.PortExists(req.Port) {
			values["port"] = []string{fmt.Sprintf("The port(%v) already exists", req.Port)}
		}
	}
	if len(req.RuleExtensions) > 0 {
		for _, re := range req.RuleExtensions {
			if re.Key == "" {
				values["key"] = []string{"The key field is required"}
				break
			}
			if re.Value == "" {
				values["value"] = []string{"The value field is required"}
				break
			}
		}
	}
	if len(values) != 0 {
		httputil.ReturnValidationError(r, w, values)
		return
	}
	sid, err := h.AddTCPRule(&req)
	if err != nil {
		httputil.ReturnError(r, w, 500, fmt.Sprintf("Unexpected error occorred while "+
			"adding tcp rule: %v", err))
		return
	}

	if err := handler.GetGatewayHandler().SendTask(sid, "add-tcp-rule"); err != nil {
		logrus.Errorf("send runtime message about gateway failure %s", err.Error())
	}

	httputil.ReturnSuccess(r, w, "success")
}

func (g *GatewayStruct) updateTCPRule(w http.ResponseWriter, r *http.Request) {
	logrus.Debugf("update tcp rule.")
	var req api_model.UpdateTCPRuleStruct
	ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &req, nil)
	if !ok {
		return
	}
	reqJSON, _ := json.Marshal(req)
	logrus.Debugf("Request is : %s", string(reqJSON))

	h := handler.GetGatewayHandler()
	// verify reqeust
	values := url.Values{}
	if req.Port != 0 && req.Port <= g.cfg.MinExtPort {
		values["port"] = []string{fmt.Sprintf("The port field should be greater than %d", g.cfg.MinExtPort)}
	} else {
		// check if the port exists
		if h.PortExists(req.Port) {
			values["port"] = []string{fmt.Sprintf("The port(%v) already exists", req.Port)}
		}
	}
	if len(req.RuleExtensions) > 0 {
		for _, re := range req.RuleExtensions {
			if re.Key == "" {
				values["key"] = []string{"The key field is required"}
				break
			}
			if re.Value == "" {
				values["value"] = []string{"The value field is required"}
				break
			}
		}
	}
	if len(values) != 0 {
		httputil.ReturnValidationError(r, w, values)
		return
	}

	sid, err := h.UpdateTCPRule(&req, g.cfg.MinExtPort)
	if err != nil {
		httputil.ReturnError(r, w, 500, fmt.Sprintf("Unexpected error occorred while "+
			"updating tcp rule: %v", err))
		return
	}

	if err := handler.GetGatewayHandler().SendTask(sid, "update-tcp-rule"); err != nil {
		logrus.Errorf("send runtime message about gateway failure %s", err.Error())
	}

	httputil.ReturnSuccess(r, w, "success")
}

func (g *GatewayStruct) deleteTCPRule(w http.ResponseWriter, r *http.Request) {
	logrus.Debugf("delete TCP rule.")
	var req api_model.DeleteTCPRuleStruct
	ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &req, nil)
	if !ok {
		return
	}
	reqJSON, _ := json.Marshal(req)
	logrus.Debugf("Request is : %s", string(reqJSON))

	h := handler.GetGatewayHandler()
	sid, err := h.DeleteTCPRule(&req)
	if err != nil {
		httputil.ReturnError(r, w, 500, fmt.Sprintf("Unexpected error occorred while "+
			"deleting tcp rule: %v", err))
		return
	}

	if err := handler.GetGatewayHandler().SendTask(sid, "delete-tcp-rule"); err != nil {
		logrus.Errorf("send runtime message about gateway failure %s", err.Error())
	}

	httputil.ReturnSuccess(r, w, "success")
}

// GetAvailablePort returns a available port
func (g *GatewayStruct) GetAvailablePort(w http.ResponseWriter, r *http.Request) {
	logrus.Debugf("get available port.")
	h := handler.GetGatewayHandler()

	res, err := h.GetAvailablePort()
	if err != nil {
		httputil.ReturnError(r, w, 500, fmt.Sprintf("Unexpected error occorred while "+
			"getting available port: %v", err))
		return
	}

	httputil.ReturnSuccess(r, w, res)
}