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
	"fmt"
	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/api/handler"
	"github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/db"
	"github.com/jinzhu/gorm"
	"github.com/sirupsen/logrus"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"

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

// MavenSettingList maven setting list
func (t *ClusterController) MavenSettingList(w http.ResponseWriter, r *http.Request) {
	httputil.ReturnSuccess(r, w, handler.GetClusterHandler().MavenSettingList(r.Context()))
}

// MavenSettingAdd maven setting add
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

// MavenSettingUpdate maven setting file update
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

// MavenSettingDelete maven setting file delete
func (t *ClusterController) MavenSettingDelete(w http.ResponseWriter, r *http.Request) {
	err := handler.GetClusterHandler().MavenSettingDelete(r.Context(), chi.URLParam(r, "name"))
	if err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}

// MavenSettingDetail maven setting file delete
func (t *ClusterController) MavenSettingDetail(w http.ResponseWriter, r *http.Request) {
	c, err := handler.GetClusterHandler().MavenSettingDetail(r.Context(), chi.URLParam(r, "name"))
	if err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, c)
}

// GetExceptionNodeInfo -
func (t *ClusterController) GetExceptionNodeInfo(w http.ResponseWriter, r *http.Request) {
	nodes, err := handler.GetClusterHandler().GetExceptionNodeInfo(r.Context())
	if err != nil {
		httputil.ReturnError(r, w, 500, err.Error())
		return
	}
	httputil.ReturnSuccess(r, w, nodes)
}

// BatchGetGateway batch get resource gateway
func (t *ClusterController) BatchGetGateway(w http.ResponseWriter, r *http.Request) {
	ns, err := handler.GetClusterHandler().BatchGetGateway(r.Context())
	if err != nil {
		logrus.Errorf(err.Error())
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, ns)
}

// GetNamespace Get the unconnected namespaces under the current cluster
func (t *ClusterController) GetNamespace(w http.ResponseWriter, r *http.Request) {
	content := r.FormValue("content")
	ns, err := handler.GetClusterHandler().GetNamespace(r.Context(), content)
	if err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, ns)
}

// GetNamespaceResource Get all resources in the current namespace
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

// ConvertResource Get the resources under the current namespace to the rainbond platform
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

// ResourceImport Import the converted k8s resources into recognition
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

// GetResource -
func (t *ClusterController) GetResource(w http.ResponseWriter, r *http.Request) {
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
func (t *ClusterController) AddResource(w http.ResponseWriter, r *http.Request) {
	var hr model.AddHandleResource
	if ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &hr, nil); !ok {
		return
	}
	rri, err := handler.GetClusterHandler().AddAppK8SResource(r.Context(), hr.Namespace, hr.AppID, hr.ResourceYaml, hr.Recover)
	if err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, rri)
}

// UpdateResource -
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

// DeleteResource -
func (t *ClusterController) DeleteResource(w http.ResponseWriter, r *http.Request) {
	var hr model.HandleResource
	if ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &hr, nil); !ok {
		return
	}
	if hr.State == model.CreateSuccess || hr.State == model.UpdateSuccess {
		handler.GetClusterHandler().DeleteAppK8SResource(r.Context(), hr.Namespace, hr.AppID, hr.Name, hr.ResourceYaml, hr.Kind)
	}
	httputil.ReturnSuccess(r, w, nil)
}

// BatchDeleteResource -
func (c *ClusterController) BatchDeleteResource(w http.ResponseWriter, r *http.Request) {
	httputil.ReturnSuccess(r, w, nil)
}

// SyncResource -
func (t *ClusterController) SyncResource(w http.ResponseWriter, r *http.Request) {
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

// YamlResourceDetailed -
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

// YamlResourceImport -
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
	ac, err := handler.GetClusterHandler().AppYamlResourceImport(yr.Namespace, yr.TenantID, yr.AppID, ar)
	if err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, ac)
}

// CreateShellPod -
func (t *ClusterController) CreateShellPod(w http.ResponseWriter, r *http.Request) {
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
func (t *ClusterController) DeleteShellPod(w http.ResponseWriter, r *http.Request) {
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
func (t *ClusterController) RbdLog(w http.ResponseWriter, r *http.Request) {
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
func (t *ClusterController) GetRbdPods(w http.ResponseWriter, r *http.Request) {
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
func (t *ClusterController) ListPlugins(w http.ResponseWriter, r *http.Request) {
	official, _ := strconv.ParseBool(r.URL.Query().Get("official"))
	res, err := handler.GetClusterHandler().ListPlugins(official)
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	httputil.ReturnSuccess(r, w, res)
}

// ListAbilities -
func (t *ClusterController) ListAbilities(w http.ResponseWriter, r *http.Request) {
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

// GetAbility -
func (t *ClusterController) GetAbility(w http.ResponseWriter, r *http.Request) {
	abilityID := chi.URLParam(r, "ability_id")
	res, err := handler.GetClusterHandler().GetAbility(abilityID)
	if err != nil {
		httputil.ReturnError(r, w, 400, err.Error())
		return
	}
	httputil.ReturnSuccess(r, w, res)
}

// UpdateAbility -
func (t *ClusterController) UpdateAbility(w http.ResponseWriter, r *http.Request) {
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

// GetLangVersion Get the unconnected namespaces under the current cluster
func (c *ClusterController) GetLangVersion(w http.ResponseWriter, r *http.Request) {
	// language：查询的语言
	// show：版本信息
	language := r.URL.Query().Get("language")
	show := r.URL.Query().Get("show")
	// 获取版本列表
	versions, err := db.GetManager().LongVersionDao().ListVersionByLanguage(language, show)
	if err != nil {
		httputil.ReturnBcodeError(r, w, fmt.Errorf("update lang version failure: %v", err))
		return
	}
	httputil.ReturnSuccess(r, w, versions)
}

// UpdateLangVersion -
func (c *ClusterController) UpdateLangVersion(w http.ResponseWriter, r *http.Request) {
	var lang model.UpdateLangVersion
	if ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &lang, nil); !ok {
		httputil.ReturnError(r, w, 400, "failed to parse parameters")
		return
	}
	// 更新默认语言版本
	err := db.GetManager().LongVersionDao().DefaultLangVersion(lang.Lang, lang.Version, lang.Show, lang.FirstChoice)
	if err != nil {
		httputil.ReturnError(r, w, 400, fmt.Sprintf("update lang version failure: %v", err))
		return
	}
	httputil.ReturnSuccess(r, w, "更新成功")
}

// CreateLangVersion -
func (c *ClusterController) CreateLangVersion(w http.ResponseWriter, r *http.Request) {
	var lang model.UpdateLangVersion
	if ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &lang, nil); !ok {
		httputil.ReturnError(r, w, 400, "failed to parse parameters")
		return
	}
	// 根据语言标识和版本号，获取语言版本信息。
	_, err := db.GetManager().LongVersionDao().GetVersionByLanguageAndVersion(lang.Lang, lang.Version)
	if err != nil && err != gorm.ErrRecordNotFound {
		httputil.ReturnError(r, w, 400, fmt.Sprintf("get lang version failure: %v", err))
		return
	}
	if err == gorm.ErrRecordNotFound {
		// 创建新的语言版本记录。
		err := db.GetManager().LongVersionDao().CreateLangVersion(lang.Lang, lang.Version, lang.EventID, lang.FileName, lang.Show)
		if err != nil {
			httputil.ReturnError(r, w, 400, fmt.Sprintf("create lang version failure: %v", err))
			return
		}
		sourceDir := path.Join(LSUploadPath, lang.EventID)
		destinationDir := path.Join(BaseUploadPath, lang.EventID)
		err = copyDirectory(sourceDir, destinationDir)
		if err != nil {
			httputil.ReturnError(r, w, 400, fmt.Sprintf("rename lang version failure: %v", err))
			return
		}
		httputil.ReturnSuccess(r, w, "创建成功")
		return
	}
	httputil.ReturnSuccess(r, w, "exist")
	return
}

// DeleteLangVersion -
func (c *ClusterController) DeleteLangVersion(w http.ResponseWriter, r *http.Request) {
	var lang model.UpdateLangVersion
	if ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &lang, nil); !ok {
		httputil.ReturnError(r, w, 400, "failed to parse parameters")
		return
	}
	// 根据语言标识和版本号，删除该语言版本
	eventID, err := db.GetManager().LongVersionDao().DeleteLangVersion(lang.Lang, lang.Version)
	if err != nil {
		httputil.ReturnError(r, w, 400, fmt.Sprintf("delete lang version failure: %v", err))
		return
	}
	// 删除本地文件
	err = os.RemoveAll(path.Join(BaseUploadPath, eventID))
	if err != nil {
		httputil.ReturnError(r, w, 400, fmt.Sprintf("delete lang version pack failure: %v", err))
		return
	}
	httputil.ReturnSuccess(r, w, "删除成功")
}

// GetRegionStatus -
func (c *ClusterController) GetRegionStatus(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if token != os.Getenv("HELM_TOKEN") {
		httputil.ReturnError(r, w, 400, "failed to verify token")
		return
	}
	regionInfo, err := handler.GetClusterHandler().GetClusterRegionStatus()
	if err != nil {
		httputil.ReturnError(r, w, 400, err.Error())
		return
	}
	httputil.ReturnSuccess(r, w, regionInfo)
}

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destinationFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destinationFile.Close()
	_, err = io.Copy(destinationFile, sourceFile)
	if err != nil {
		return err
	}
	err = destinationFile.Sync()
	if err != nil {
		return err
	}
	return nil
}

func copyDirectory(srcDir, dstDir string) error {
	err := os.MkdirAll(dstDir, os.ModePerm)
	if err != nil {
		return err
	}
	err = filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relativePath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dstDir, relativePath)
		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("skipping symbolic link: %s", path)
		}
		return copyFile(path, dstPath)
	})
	return err
}
