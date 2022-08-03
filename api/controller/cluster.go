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
	"github.com/goodrain/rainbond/api/model"
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
	nodes, err := handler.GetClusterHandler().GetClusterInfo(r.Context())
	if err != nil {
		logrus.Errorf("get cluster info: %v", err)
		httputil.ReturnError(r, w, 500, err.Error())
		return
	}

	httputil.ReturnSuccess(r, w, nodes)
}

//MavenSettingList maven setting list
func (t *ClusterController) MavenSettingList(w http.ResponseWriter, r *http.Request) {
	httputil.ReturnSuccess(r, w, handler.GetClusterHandler().MavenSettingList(r.Context()))
}

//MavenSettingAdd maven setting add
func (t *ClusterController) MavenSettingAdd(w http.ResponseWriter, r *http.Request) {
	var set handler.MavenSetting
	if ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &set, nil); !ok {
		return
	}
	if err := handler.GetClusterHandler().MavenSettingAdd(r.Context(), &set); err != nil {
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
	if err := handler.GetClusterHandler().MavenSettingUpdate(r.Context(), set); err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, set)
}

//MavenSettingDelete maven setting file delete
func (t *ClusterController) MavenSettingDelete(w http.ResponseWriter, r *http.Request) {
	err := handler.GetClusterHandler().MavenSettingDelete(r.Context(), chi.URLParam(r, "name"))
	if err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}

//MavenSettingDetail maven setting file delete
func (t *ClusterController) MavenSettingDetail(w http.ResponseWriter, r *http.Request) {
	c, err := handler.GetClusterHandler().MavenSettingDetail(r.Context(), chi.URLParam(r, "name"))
	if err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, c)
}

//GetNamespace Get the unconnected namespaces under the current cluster
func (t *ClusterController) GetNamespace(w http.ResponseWriter, r *http.Request) {
	content := r.FormValue("content")
	ns, err := handler.GetClusterHandler().GetNamespace(r.Context(), content)
	if err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, ns)
}

//GetNamespaceResource Get all resources in the current namespace
func (t *ClusterController) GetNamespaceResource(w http.ResponseWriter, r *http.Request) {
	content := r.FormValue("content")
	namespace := r.FormValue("namespace")
	rs, err := handler.GetClusterHandler().GetNamespaceSource(r.Context(), content, namespace)
	if err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, rs)
}

//ConvertResource Get the resources under the current namespace to the rainbond platform
func (t *ClusterController) ConvertResource(w http.ResponseWriter, r *http.Request) {
	content := r.FormValue("content")
	namespace := r.FormValue("namespace")
	rs, err := handler.GetClusterHandler().GetNamespaceSource(r.Context(), content, namespace)
	if err != nil {
		err.Handle(r, w)
		return
	}
	appsServices, err := handler.GetClusterHandler().ConvertResource(r.Context(), namespace, rs)
	if err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, appsServices)
}

//ResourceImport Import the converted k8s resources into recognition
func (t *ClusterController) ResourceImport(w http.ResponseWriter, r *http.Request) {
	content := r.FormValue("content")
	namespace := r.FormValue("namespace")
	eid := r.FormValue("eid")
	rs, err := handler.GetClusterHandler().GetNamespaceSource(r.Context(), content, namespace)
	if err != nil {
		err.Handle(r, w)
		return
	}
	appsServices, err := handler.GetClusterHandler().ConvertResource(r.Context(), namespace, rs)
	if err != nil {
		err.Handle(r, w)
		return
	}
	rri, err := handler.GetClusterHandler().ResourceImport(namespace, appsServices, eid)
	if err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, rri)
}

//AddResource -
func (t *ClusterController) AddResource(w http.ResponseWriter, r *http.Request) {
	var hr model.AddHandleResource
	if ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &hr, nil); !ok {
		return
	}
	rri, err := handler.GetClusterHandler().AddAppK8SResource(r.Context(), hr.Namespace, hr.AppID, hr.ResourceYaml)
	if err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, rri)
}

//UpdateResource -
func (t *ClusterController) UpdateResource(w http.ResponseWriter, r *http.Request) {
	var hr model.HandleResource
	if ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &hr, nil); !ok {
		return
	}
	rri, err := handler.GetClusterHandler().UpdateAppK8SResource(r.Context(), hr.Namespace, hr.AppID, hr.Name, hr.ResourceYaml, hr.Kind)
	if err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, rri)
}

//DeleteResource -
func (t *ClusterController) DeleteResource(w http.ResponseWriter, r *http.Request) {
	var hr model.HandleResource
	if ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &hr, nil); !ok {
		return
	}
	err := handler.GetClusterHandler().DeleteAppK8SResource(r.Context(), hr.Namespace, hr.AppID, hr.Name, hr.ResourceYaml, hr.Kind)
	if err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}

//YamlResourceName -
func (t *ClusterController) YamlResourceName(w http.ResponseWriter, r *http.Request) {
	var yr model.YamlResource
	if ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &yr, nil); !ok {
		return
	}
	h, err := handler.GetClusterHandler().AppYamlResourceName(yr)
	if err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, h)
}

//YamlResourceDetailed -
func (t *ClusterController) YamlResourceDetailed(w http.ResponseWriter, r *http.Request) {
	var yr model.YamlResource
	if ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &yr, nil); !ok {
		return
	}
	h, err := handler.GetClusterHandler().AppYamlResourceDetailed(yr, false)
	if err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, h)
}

//YamlResourceImport -
func (t *ClusterController) YamlResourceImport(w http.ResponseWriter, r *http.Request) {
	var yr model.YamlResource
	if ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &yr, nil); !ok {
		return
	}
	ar, err := handler.GetClusterHandler().AppYamlResourceDetailed(yr, true)
	if err != nil {
		err.Handle(r, w)
		return
	}
	ac, err := handler.GetClusterHandler().AppYamlResourceImport(yr, ar)
	if err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, ac)
}
