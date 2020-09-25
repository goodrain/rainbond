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
	"net/http"

	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/api/handler"
	"github.com/sirupsen/logrus"

	httputil "github.com/goodrain/rainbond/util/http"
)

// ClusterController -
type ClusterController struct {
}

// GetClusterInfo -
func (t *ClusterController) GetClusterInfo(w http.ResponseWriter, r *http.Request) {
	nodes, err := handler.GetClusterHandler().GetClusterInfo()
	if err != nil {
		logrus.Errorf("get cluster info: %v", err)
		httputil.ReturnError(r, w, 500, err.Error())
		return
	}

	httputil.ReturnSuccess(r, w, nodes)
}

//MavenSettingList maven setting list
func (t *ClusterController) MavenSettingList(w http.ResponseWriter, r *http.Request) {
	httputil.ReturnSuccess(r, w, handler.GetClusterHandler().MavenSettingList())
}

//MavenSettingAdd maven setting add
func (t *ClusterController) MavenSettingAdd(w http.ResponseWriter, r *http.Request) {
	var set handler.MavenSetting
	if ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &set, nil); !ok {
		return
	}
	if err := handler.GetClusterHandler().MavenSettingAdd(&set); err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, &set)
}

//MavenSettingUpdate maven setting file update
func (t *ClusterController) MavenSettingUpdate(w http.ResponseWriter, r *http.Request) {
	type SettingUpdate struct {
		Content string `json:"content" validate:"required"`
	}
	var su SettingUpdate
	if ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &su, nil); !ok {
		return
	}
	set := &handler.MavenSetting{
		Name:    chi.URLParam(r, "name"),
		Content: su.Content,
	}
	if err := handler.GetClusterHandler().MavenSettingUpdate(set); err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, set)
}

//MavenSettingDelete maven setting file delete
func (t *ClusterController) MavenSettingDelete(w http.ResponseWriter, r *http.Request) {
	err := handler.GetClusterHandler().MavenSettingDelete(chi.URLParam(r, "name"))
	if err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}

//MavenSettingDetail maven setting file delete
func (t *ClusterController) MavenSettingDetail(w http.ResponseWriter, r *http.Request) {
	c, err := handler.GetClusterHandler().MavenSettingDetail(chi.URLParam(r, "name"))
	if err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, c)
}
