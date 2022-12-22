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

package controller

import (
	"context"
	"encoding/json"
	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/api/handler"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	v12 "k8s.io/api/core/v1"
	"net/http"

	httputil "github.com/goodrain/rainbond/util/http"
)

// NodesController -
type NodesController struct {
}

// GetNodes -
func (n *NodesController) GetNodes(w http.ResponseWriter, r *http.Request) {
	nodes, err := handler.GetNodesHandler().GetNodes(context.Background())
	if err != nil {
		logrus.Errorf("get node list info: %v", err)
		httputil.ReturnError(r, w, 500, err.Error())
		return
	}
	httputil.ReturnSuccess(r, w, nodes)
}

// GetNode -
func (n *NodesController) GetNode(w http.ResponseWriter, r *http.Request) {
	nodeName := chi.URLParam(r, "node_name")
	nodes, err := handler.GetNodesHandler().GetNodeInfo(context.Background(), nodeName)
	if err != nil {
		logrus.Errorf("get node info error: %v", err)
		httputil.ReturnError(r, w, 500, err.Error())
		return
	}
	httputil.ReturnSuccess(r, w, nodes)
}

// NodeAction -
func (n *NodesController) NodeAction(w http.ResponseWriter, r *http.Request) {
	nodeName := chi.URLParam(r, "node_name")
	action := chi.URLParam(r, "action")
	err := handler.GetNodesHandler().NodeAction(context.Background(), nodeName, action)
	if err != nil {
		logrus.Errorf("operate node action error: %v", err)
		httputil.ReturnError(r, w, 500, err.Error())
		return
	}
	httputil.ReturnSuccess(r, w, "ok")
}

// GetLabels -
func (n *NodesController) GetLabels(w http.ResponseWriter, r *http.Request) {
	nodeName := chi.URLParam(r, "node_name")
	labels, err := handler.GetNodesHandler().GetLabels(context.Background(), nodeName)
	if err != nil {
		logrus.Errorf("get node labels error: %v", err)
		httputil.ReturnError(r, w, 500, err.Error())
		return
	}
	httputil.ReturnSuccess(r, w, labels)
}

// UpdateLabels -
func (n *NodesController) UpdateLabels(w http.ResponseWriter, r *http.Request) {
	nodeName := chi.URLParam(r, "node_name")
	var labels = make(map[string]string)
	in, err := ioutil.ReadAll(r.Body)
	if err != nil {
		logrus.Error("error read from request, details:", err)
		httputil.ReturnError(r, w, 500, err.Error())
		return
	}
	err = json.Unmarshal(in, &labels)
	if err != nil {
		logrus.Error("error unmarshal labels, details:", err)
		httputil.ReturnError(r, w, 500, err.Error())
		return
	}
	res, err := handler.GetNodesHandler().UpdateLabels(context.Background(), nodeName, labels)
	if err != nil {
		logrus.Error("update node labels error: ", err)
		httputil.ReturnError(r, w, 500, err.Error())
		return
	}
	httputil.ReturnSuccess(r, w, res)
}

// GetTaints -
func (n *NodesController) GetTaints(w http.ResponseWriter, r *http.Request) {
	nodeName := chi.URLParam(r, "node_name")
	labels, err := handler.GetNodesHandler().GetTaints(context.Background(), nodeName)
	if err != nil {
		logrus.Errorf("get node taints error: %v", err)
		httputil.ReturnError(r, w, 500, err.Error())
		return
	}
	httputil.ReturnSuccess(r, w, labels)
}

// UpdateTaints -
func (n *NodesController) UpdateTaints(w http.ResponseWriter, r *http.Request) {
	nodeName := chi.URLParam(r, "node_name")
	var taints []v12.Taint
	in, err := ioutil.ReadAll(r.Body)
	if err != nil {
		logrus.Error("error read from request, details:", err)
		httputil.ReturnError(r, w, 500, err.Error())
		return
	}
	err = json.Unmarshal(in, &taints)
	if err != nil {
		logrus.Error("error unmarshal labels, details:", err)
		httputil.ReturnError(r, w, 500, err.Error())
		return
	}
	labels, err := handler.GetNodesHandler().UpdateTaints(context.Background(), nodeName, taints)
	if err != nil {
		logrus.Errorf("update node taints error: %v", err)
		httputil.ReturnError(r, w, 500, err.Error())
		return
	}
	httputil.ReturnSuccess(r, w, labels)
}
