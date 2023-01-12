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
	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/api/handler"
	"github.com/goodrain/rainbond/api/model"
	"github.com/sirupsen/logrus"
	"net/http"
	"strconv"

	httputil "github.com/goodrain/rainbond/util/http"
)

// ClusterController -
type ClusterController struct {
}

// GetClusterInfo -
func (c *ClusterController) GetClusterInfo(w http.ResponseWriter, r *http.Request) {
	nodes, err := handler.GetClusterHandler().GetClusterInfo(r.Context())
	if err != nil {
		logrus.Errorf("get cluster info: %v", err)
		httputil.ReturnError(r, w, 500, err.Error())
		return
	}

	httputil.ReturnSuccess(r, w, nodes)
}

// MavenSettingList maven setting list
func (c *ClusterController) MavenSettingList(w http.ResponseWriter, r *http.Request) {
	httputil.ReturnSuccess(r, w, handler.GetClusterHandler().MavenSettingList(r.Context()))
}

// MavenSettingAdd maven setting add
func (c *ClusterController) MavenSettingAdd(w http.ResponseWriter, r *http.Request) {
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

// MavenSettingUpdate maven setting file update
func (c *ClusterController) MavenSettingUpdate(w http.ResponseWriter, r *http.Request) {
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

// MavenSettingDelete maven setting file delete
func (c *ClusterController) MavenSettingDelete(w http.ResponseWriter, r *http.Request) {
	err := handler.GetClusterHandler().MavenSettingDelete(r.Context(), chi.URLParam(r, "name"))
	if err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}

// MavenSettingDetail maven setting file delete
func (c *ClusterController) MavenSettingDetail(w http.ResponseWriter, r *http.Request) {
	setting, err := handler.GetClusterHandler().MavenSettingDetail(r.Context(), chi.URLParam(r, "name"))
	if err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, setting)
}

// GetNamespace Get the unconnected namespaces under the current cluster
func (c *ClusterController) GetNamespace(w http.ResponseWriter, r *http.Request) {
	content := r.FormValue("content")
	ns, err := handler.GetClusterHandler().GetNamespace(r.Context(), content)
	if err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, ns)
}

// GetNamespaceResource Get all resources in the current namespace
func (c *ClusterController) GetNamespaceResource(w http.ResponseWriter, r *http.Request) {
	content := r.FormValue("content")
	namespace := r.FormValue("namespace")
	rs, err := handler.GetClusterHandler().GetNamespaceSource(r.Context(), content, namespace)
	if err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, rs)
}

// ConvertResource Get the resources under the current namespace to the rainbond platform
func (c *ClusterController) ConvertResource(w http.ResponseWriter, r *http.Request) {
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

// ResourceImport Import the converted k8s resources into recognition
func (c *ClusterController) ResourceImport(w http.ResponseWriter, r *http.Request) {
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

// GetResource -
func (c *ClusterController) GetResource(w http.ResponseWriter, r *http.Request) {
	var hr model.HandleResource
	if ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &hr, nil); !ok {
		return
	}
	rri, err := handler.GetClusterHandler().GetAppK8SResource(r.Context(), hr.Namespace, hr.AppID, hr.Name, hr.ResourceYaml, hr.Kind)
	if err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, rri)
}

// AddResource -
func (c *ClusterController) AddResource(w http.ResponseWriter, r *http.Request) {
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

// UpdateResource -
func (c *ClusterController) UpdateResource(w http.ResponseWriter, r *http.Request) {
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

// DeleteResource -
func (c *ClusterController) DeleteResource(w http.ResponseWriter, r *http.Request) {
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

// SyncResource -
func (c *ClusterController) SyncResource(w http.ResponseWriter, r *http.Request) {
	var req model.SyncResources
	if ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &req, nil); !ok {
		return
	}
	resources, err := handler.GetClusterHandler().SyncAppK8SResources(r.Context(), &req)
	if err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, resources)
}

// YamlResourceName -
func (c *ClusterController) YamlResourceName(w http.ResponseWriter, r *http.Request) {
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

// YamlResourceDetailed -
func (c *ClusterController) YamlResourceDetailed(w http.ResponseWriter, r *http.Request) {
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

// YamlResourceImport -
func (c *ClusterController) YamlResourceImport(w http.ResponseWriter, r *http.Request) {
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

// CreateShellPod -
func (c *ClusterController) CreateShellPod(w http.ResponseWriter, r *http.Request) {
	var sp model.ShellPod
	if ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &sp, nil); !ok {
		return
	}
	pod, err := handler.GetClusterHandler().CreateShellPod(sp.RegionName)
	if err != nil {
		logrus.Error("create shell pod error:", err)
		return
	}
	httputil.ReturnSuccess(r, w, pod)
}

// DeleteShellPod -
func (c *ClusterController) DeleteShellPod(w http.ResponseWriter, r *http.Request) {
	var sp model.ShellPod
	if ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &sp, nil); !ok {
		return
	}
	err := handler.GetClusterHandler().DeleteShellPod(sp.PodName)
	if err != nil {
		logrus.Error("delete shell pod error:", err)
		return
	}
	httputil.ReturnSuccess(r, w, "")
}

// RbdLog -
func (c *ClusterController) RbdLog(w http.ResponseWriter, r *http.Request) {
	podName := r.URL.Query().Get("pod_name")
	follow, _ := strconv.ParseBool(r.URL.Query().Get("follow"))
	err := handler.GetClusterHandler().RbdLog(w, r, podName, follow)

	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	return
}

// GetRbdPods -
func (c *ClusterController) GetRbdPods(w http.ResponseWriter, r *http.Request) {
	res, err := handler.GetClusterHandler().GetRbdPods()
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	httputil.ReturnSuccess(r, w, res)
}

// ListRainbondComponents -
func (t *ClusterController) ListRainbondComponents(w http.ResponseWriter, r *http.Request) {
	components, err := handler.GetClusterHandler().ListRainbondComponents(r.Context())
	if err != nil {
		logrus.Errorf("get rainbond components error: %v", err)
		httputil.ReturnError(r, w, 500, err.Error())
		return
	}
	httputil.ReturnSuccess(r, w, components)
}

// ListPlugins -
func (c *ClusterController) ListPlugins(w http.ResponseWriter, r *http.Request) {
	res, err := handler.GetClusterHandler().ListPlugins()
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	httputil.ReturnSuccess(r, w, res)
}

// ListAbilities -
func (c *ClusterController) ListAbilities(w http.ResponseWriter, r *http.Request) {
	res, err := handler.GetClusterHandler().ListAbilities()
	if err != nil {
		httputil.ReturnError(r, w, 400, err.Error())
		return
	}
	var abilities []model.AbilityResp
	for _, ability := range res {
		abilities = append(abilities, model.AbilityResp{
			Name:       ability.GetName(),
			Kind:       ability.GetKind(),
			APIVersion: ability.GetAPIVersion(),
			AbilityID:  handler.GetClusterHandler().GenerateAbilityID(&ability),
		})
	}
	httputil.ReturnSuccess(r, w, abilities)
}

func (c *ClusterController) GetAbility(w http.ResponseWriter, r *http.Request) {
	abilityID := chi.URLParam(r, "ability_id")
	res, err := handler.GetClusterHandler().GetAbility(abilityID)
	if err != nil {
		httputil.ReturnError(r, w, 400, err.Error())
		return
	}
	httputil.ReturnSuccess(r, w, res)
}

// UpdateAbility -
func (c *ClusterController) UpdateAbility(w http.ResponseWriter, r *http.Request) {
	abilityID := chi.URLParam(r, "ability_id")
	var req model.UpdateAbilityReq
	if ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &req, nil); !ok {
		return
	}
	if err := handler.GetClusterHandler().UpdateAbility(abilityID, req.Object); err != nil {
		httputil.ReturnError(r, w, 400, err.Error())
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}
