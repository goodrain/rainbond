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
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/goodrain/rainbond/api/middleware"
	api_model "github.com/goodrain/rainbond/api/model"
	dbmodel "github.com/goodrain/rainbond/db/model"

	"github.com/pquerna/ffjson/ffjson"

	"github.com/go-chi/chi"
	"github.com/jinzhu/gorm"
	validator "github.com/thedevsaddam/govalidator"

	"github.com/goodrain/rainbond/api/handler"
	"github.com/goodrain/rainbond/appruntimesync/client"
	httputil "github.com/goodrain/rainbond/util/http"

	"github.com/Sirupsen/logrus"
	"github.com/renstorm/fuzzysearch/fuzzy"
)

//V2Routes v2Routes
type V2Routes struct {
	TenantStruct
	AcpNodeStruct
	EntranceStruct
	EventLogStruct
	AppStruct
}

//Show test
func (v2 *V2Routes) Show(w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /v2/show v2 getApiVersion
	//
	// 显示当前的api version 信息
	//
	// show api version
	//
	// ---
	// produces:
	// - application/json
	// - application/xml
	// responses:
	//   default:
	//     schema:
	//       "$ref": "#/responses/commandResponse"
	//     description: 统一返回格式
	w.Write([]byte("v2 urls"))
}

// show health status
func (v2 *V2Routes) Health(w http.ResponseWriter, r *http.Request) {
	httputil.ReturnSuccess(r, w, map[string]string{"status": "health", "info": "api service health"})
}

func (v2 *V2Routes) AlertManagerWebHook(w http.ResponseWriter, r *http.Request) {
	fmt.Println("=======>webhook")
	in, err := ioutil.ReadAll(r.Body)
	if err != nil {
		fmt.Println(err)
		httputil.ReturnError(r, w, 400, "")
		return
	}
	fmt.Println("=====>body")
	fmt.Println(string(in))
	httputil.ReturnSuccess(r, w, "")

}

//TenantStruct tenant struct
type TenantStruct struct {
	StatusCli *client.AppRuntimeSyncClient
}

//AllTenantResources GetResources
func (t *TenantStruct) AllTenantResources(w http.ResponseWriter, r *http.Request) {
	tenants, err := handler.GetTenantManager().GetTenants()
	if err != nil {
		msg := httputil.ResponseBody{
			Msg: fmt.Sprintf("get tenant error, %v", err),
		}
		httputil.Return(r, w, 500, msg)
	}
	ts := &api_model.TotalStatsInfo{}
	for _, tenant := range tenants {
		services, err := handler.GetServiceManager().GetService(tenant.UUID)
		if err != nil {
			msg := httputil.ResponseBody{
				Msg: fmt.Sprintf("get service error, %v", err),
			}
			httputil.Return(r, w, 500, msg)
		}
		statsInfo, _ := handler.GetTenantManager().StatsMemCPU(services)
		statsInfo.UUID = tenant.UUID
		ts.Data = append(ts.Data, statsInfo)
	}
	httputil.ReturnSuccess(r, w, ts.Data)
}

//TenantResources TenantResources
func (t *TenantStruct) TenantResources(w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /v2/resources/tenants v2 tenantResources
	//
	// 租户资源使用情况
	//
	// get tenant resources
	//
	// ---
	// produces:
	// - application/json
	// - application/xml
	//
	// responses:
	//   default:
	//     schema:
	//       "$ref": "#/responses/commandResponse"
	//     description: 统一返回格式
	var tr api_model.TenantResources
	ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &tr.Body, nil)
	if !ok {
		return
	}
	rep, err := handler.GetTenantManager().GetTenantsResources(&tr)
	if err != nil {
		httputil.ReturnError(r, w, 500, fmt.Sprintf("get resources error, %v", err))
		return
	}
	var re []map[string]interface{}
	for _, v := range rep {
		if v != nil {
			re = append(re, v)
		}
	}
	httputil.ReturnSuccess(r, w, re)
	return
}

//ServiceResources ServiceResources
func (t *TenantStruct) ServiceResources(w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /v2/resources/services v2 serviceResources
	//
	// 应用资源使用情况
	//
	// get service resources
	//
	// ---
	// produces:
	// - application/json
	// - application/xml
	//
	// responses:
	//   default:
	//     schema:
	//       "$ref": "#/responses/commandResponse"
	//     description: 统一返回格式
	var tr api_model.ServicesResources
	ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &tr.Body, nil)
	if !ok {
		return
	}
	rep, err := handler.GetTenantManager().GetServicesResources(&tr)
	if err != nil {
		httputil.ReturnError(r, w, 500, fmt.Sprintf("get resources error, %v", err))
		return
	}
	httputil.ReturnSuccess(r, w, rep)
	return
}

//TenantsQuery TenantsQuery
func (t *TenantStruct) TenantsQuery(w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /v2/tenants/query/{tenant_name} v2 tenants
	//
	// 租户带资源列表
	//
	// get tenant resources
	//
	// ---
	// produces:
	// - application/json
	// - application/xml
	//
	// parameters:
	// - name: tenant_name
	//   in: path
	//   description: '123'
	//   required: true
	//   type: string
	//   format: string
	//
	// responses:
	//   default:
	//     schema:
	//       "$ref": "#/responses/commandResponse"
	//     description: 统一返回格式

	tenantName := strings.TrimSpace(chi.URLParam(r, "tenant_name"))

	rep, err := handler.GetTenantManager().GetTenantsName()
	if err != nil {
		httputil.ReturnError(r, w, 500, fmt.Sprintf("get tenants names error, %v", err))
		return
	}

	result := fuzzy.Find(tenantName, rep) // [cartwheel wheel]
	httputil.ReturnSuccess(r, w, result)
	return
}

//TenantsGetByName TenantsGetByName
func (t *TenantStruct) TenantsGetByName(w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /v2/tenants/{tenant_name}/res v2 tenants
	//
	// 租户带资源单个
	//
	// get tenant resources
	//
	// ---
	// produces:
	// - application/json
	// - application/xml
	//
	// parameters:
	// - name: tenant_name
	//   in: path
	//   description: '123'
	//   required: true
	//   type: string
	//   format: string
	//
	// responses:
	//   default:
	//     schema:
	//       "$ref": "#/responses/commandResponse"
	//     description: 统一返回格式

	tenantName := strings.TrimSpace(chi.URLParam(r, "tenant_name"))

	v, err := handler.GetTenantManager().GetTenantsByName(tenantName)
	if err != nil {
		httputil.ReturnError(r, w, 404, fmt.Sprintf("get tenants names error, %v", err))
		return
	}
	logrus.Infof("query tenant from db by name %s ,got %v", tenantName, v)

	tenantServiceRes, err := handler.GetServiceManager().GetTenantRes(v.UUID)
	if err != nil {
		httputil.ReturnError(r, w, 500, fmt.Sprintf("get tenants service total resources  error, %v", err))
		return
	}
	tenantServiceRes.UUID = v.UUID
	tenantServiceRes.Name = v.Name
	tenantServiceRes.EID = v.EID

	httputil.ReturnSuccess(r, w, tenantServiceRes)
	return
}

//TenantsWithResource TenantsWithResource
func (t *TenantStruct) TenantsWithResource(w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /v2/resources/tenants/res/page/{curPage}/size/{pageLen} v2 PagedTenantResList
	//
	// 租户带资源列表
	//
	// get paged tenant resources
	//
	// ---
	// produces:
	// - application/json
	// - application/xml
	//
	// parameters:
	// - name: curPage
	//   in: path
	//   description: '123'
	//   required: true
	//   type: string
	//   format: string
	// - name: pageLen
	//   in: path
	//   description: '25'
	//   required: true
	//   type: string
	//   format: string
	//
	// responses:
	//   default:
	//     schema:
	//       "$ref": "#/responses/commandResponse"
	//     description: 统一返回格式

	pageLenStr := strings.TrimSpace(chi.URLParam(r, "pageLen"))
	curPageStr := strings.TrimSpace(chi.URLParam(r, "curPage"))

	pageLen, err := strconv.Atoi(pageLenStr)
	if err != nil {
		httputil.ReturnError(r, w, 400, fmt.Sprintf("bad request, %v", err))
		return
	}
	curPage, err := strconv.Atoi(curPageStr)
	if err != nil {
		httputil.ReturnError(r, w, 400, fmt.Sprintf("bad request, %v", err))
		return
	}
	resource, count, err := handler.GetServiceManager().GetPagedTenantRes((curPage-1)*pageLen, pageLen)
	if err != nil {
		httputil.ReturnError(r, w, 500, fmt.Sprintf("get tenants  error, %v", err))
		return
	}
	var ret api_model.PagedTenantResList
	ret.List = resource
	ret.Length = count
	httputil.ReturnSuccess(r, w, ret)
	return
}

//SumTenants 统计租户数量
func (t *TenantStruct) SumTenants(w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /v2/resources/tenants/sum v2 sumTenants
	//
	// 获取租户数量
	//
	// get tenant resources
	//
	// ---
	// produces:
	// - application/json
	// - application/xml
	//
	// responses:
	//   default:
	//     schema:
	//       "$ref": "#/responses/commandResponse"
	//     description: 统一返回格式

	s, err := handler.GetTenantManager().TenantsSum()
	if err != nil {
		httputil.ReturnError(r, w, 500, fmt.Sprintf("sum tenants error, %v", err))
		return
	}
	rc := make(map[string]int)
	rc["num"] = s
	httputil.ReturnSuccess(r, w, rc)
}

//Tenant Tenant
func (t *TenantStruct) Tenant(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		t.AddTenant(w, r)
	case "GET":
		t.GetTenants(w, r)
	}
}

//AddTenant AddTenant
func (t *TenantStruct) AddTenant(w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /v2/tenants v2 addTenant
	//
	// 添加租户信息
	//
	// add tenant
	//
	// ---
	// consumes:
	// - application/json
	// - application/x-protobuf
	//
	// produces:
	// - application/json
	// - application/xml
	//
	// responses:
	//   default:
	//     schema:
	//       "$ref": "#/responses/commandResponse"
	//     description: 统一返回格式
	var ts api_model.AddTenantStruct
	ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &ts.Body, nil)
	if !ok {
		return
	}
	var dbts dbmodel.Tenants
	if ts.Body.Eid != "" {
		//新接口
		//TODO:生成tenant_id and tenant_name
		id, name, errN := handler.GetServiceManager().CreateTenandIDAndName(ts.Body.Eid)
		if errN != nil {
			httputil.ReturnError(r, w, 500, fmt.Sprintf("create tenant error, %v", errN))
			return
		}
		dbts.EID = ts.Body.Eid
		if ts.Body.TenantName == "" {
			dbts.Name = name
		} else {
			dbts.Name = ts.Body.TenantName
			name = ts.Body.TenantName
		}
		if ts.Body.TenantID == "" {
			dbts.UUID = id
		} else {
			dbts.UUID = ts.Body.TenantID
			id = ts.Body.TenantID
		}
		if err := handler.GetServiceManager().CreateTenant(&dbts); err != nil {
			if strings.HasSuffix(err.Error(), "is exist") {
				httputil.ReturnError(r, w, 400, err.Error())
				return
			}
			httputil.ReturnError(r, w, 500, fmt.Sprintf("create tenant error, %v", err))
			return
		}
		rc := make(map[string]string)
		rc["tenant_id"] = id
		rc["tenang_name"] = name
		rc["eid"] = ts.Body.Eid
		httputil.ReturnSuccess(r, w, rc)
		return
	}
	if ts.Body.TenantID != "" && ts.Body.TenantName != "" {
		//兼容旧接口
		dbts.Name = ts.Body.TenantName
		dbts.UUID = ts.Body.TenantID
		if err := handler.GetServiceManager().CreateTenant(&dbts); err != nil {
			if strings.HasSuffix(err.Error(), "is exist") {
				httputil.ReturnError(r, w, 400, err.Error())
				return
			}
			httputil.ReturnError(r, w, 500, fmt.Sprintf("create tenant error, %v", err))
			return
		}
		httputil.ReturnSuccess(r, w, nil)
		return
	}
	if ts.Body.Eid == "" && (ts.Body.TenantID == "" || ts.Body.TenantName == "") {
		httputil.ReturnError(r, w, 400, "args error")
		return
	}
	httputil.ReturnError(r, w, 400, "args error, need eid or tenatn_id / tenant_name")
	return
}

//GetTenants GetTenants
func (t *TenantStruct) GetTenants(w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /v2/tenants v2 getTenants
	//
	// 获取所有租户信息
	//
	// get tenant
	//
	// ---
	// consumes:
	// - application/json
	// - application/x-protobuf
	//
	// produces:
	// - application/json
	// - application/xml
	//
	// responses:
	//   default:
	//     schema:
	//       "$ref": "#/responses/commandResponse"
	//     description: 统一返回格式
	value := r.FormValue("eid")
	id := len(value)
	if id == 0 {
		tenants, err := handler.GetTenantManager().GetTenants()
		if err != nil {
			httputil.ReturnError(r, w, 500, "get tenant error")
			return
		}
		httputil.ReturnSuccess(r, w, tenants)
		return
	}

	tenants, err := handler.GetTenantManager().GetTenantsByEid(value)
	if err != nil {
		httputil.ReturnError(r, w, 500, "get tenant error")
		return
	}
	httputil.ReturnSuccess(r, w, tenants)
}

//DeleteTenant DeleteTenant
func (t *TenantStruct) DeleteTenant(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("delete tenant"))
}

//UpdateTenant UpdateTenant
func (t *TenantStruct) UpdateTenant(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("update tenant"))
}

//Get all apps and status
func (t *TenantStruct) ServicesCount(w http.ResponseWriter, r *http.Request) {
	allStatus := t.StatusCli.GetAllStatus()
	var closed int = 0
	var running int = 0
	var abnormal int = 0
	for _, v := range allStatus {
		switch v {
		case "closed":
			closed += 1
		case "running":
			running += 1
		case "abnormal":
			abnormal += 1

		}

	}
	serviceCount := map[string]int{"total": len(allStatus), "running": running, "closed": closed, "abnormal": abnormal}
	httputil.ReturnSuccess(r, w, serviceCount)
}

//ServicesInfo GetServiceInfo
func (t *TenantStruct) ServicesInfo(w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /v2/tenants/{tenant_name}/services v2 getServiceInfo
	//
	// 获取租户所有应用信息
	//
	// get services info in tenant
	//
	// ---
	// consumes:
	// - application/json
	// - application/x-protobuf
	//
	// produces:
	// - application/json
	// - application/xml
	//
	// responses:
	//   default:
	//     schema:
	//       "$ref": "#/responses/commandResponse"
	//     description: 统一返回格式
	tenantID := r.Context().Value(middleware.ContextKey("tenant_id")).(string)
	services, err := handler.GetServiceManager().GetService(tenantID)
	if err != nil {
		httputil.ReturnError(r, w, 500, "get tenant services error")
		return
	}
	httputil.ReturnSuccess(r, w, services)
	return
}

//CreateService create Service
func (t *TenantStruct) CreateService(w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /v2/tenants/{tenant_name}/services v2 createService
	//
	// 创建应用
	//
	// create service
	//
	// ---
	// consumes:
	// - application/json
	// - application/x-protobuf
	//
	// produces:
	// - application/json
	// - application/xml
	//
	// responses:
	//   default:
	//     schema:
	//       "$ref": "#/responses/commandResponse"
	//     description: 统一返回格式

	logrus.Debugf("trans create service service")
	var ss api_model.ServiceStruct
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		httputil.ReturnError(r, w, 500, err.Error())
		return
	}
	err = ffjson.Unmarshal(body, &ss)
	if err != nil {
		httputil.ReturnError(r, w, 500, err.Error())
		return
	}
	logrus.Debugf("data is %v", ss)

	tenantID := r.Context().Value(middleware.ContextKey("tenant_id")).(string)
	ss.TenantID = tenantID
	logrus.Debugf("begin to create service")
	if err := handler.GetServiceManager().ServiceCreate(&ss); err != nil {
		httputil.ReturnError(r, w, 500, fmt.Sprintf("create service error, %v", err))
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}

//UpdateService create Service
func (t *TenantStruct) UpdateService(w http.ResponseWriter, r *http.Request) {
	// swagger:operation PUT /v2/tenants/{tenant_name}/services/{service_alias} v2 updateService
	//
	// 应用更新
	//
	// update service
	//
	// ---
	// consumes:
	// - application/json
	// - application/x-protobuf
	//
	// produces:
	// - application/json
	// - application/xml
	//
	// responses:
	//   default:
	//     schema:
	//       "$ref": "#/responses/commandResponse"
	//     description: 统一返回格式

	logrus.Debugf("trans update service service")
	//目前提供三个元素的修改
	rules := validator.MapData{
		"container_cmd":    []string{},
		"image_name":       []string{},
		"container_memory": []string{},
	}
	//对应前方三个元素
	//mm := []string{"image_name", "container_cmd", "container_memory"}
	data, ok := httputil.ValidatorRequestMapAndErrorResponse(r, w, rules, nil)
	if !ok {
		return
	}
	serviceID := r.Context().Value(middleware.ContextKey("service_id")).(string)
	data["service_id"] = serviceID
	logrus.Debugf("begin to update service")
	if err := handler.GetServiceManager().ServiceUpdate(data); err != nil {
		httputil.ReturnError(r, w, 500, fmt.Sprintf("update service error, %v", err))
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}

//SetLanguage SetLanguage
func (t *TenantStruct) SetLanguage(w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST  /v2/tenants/{tenant_name}/services/{service_alias}/language v2 setLanguage
	//
	// 设置应用语言
	//
	// set language
	//
	// ---
	// consumes:
	// - application/json
	// - application/x-protobuf
	//
	// produces:
	// - application/json
	// - application/xml
	//
	// responses:
	//   default:
	//     schema:
	//       "$ref": "#/responses/commandResponse"
	//     description: 统一返回格式
	logrus.Debugf("trans language set service")
	rules := validator.MapData{
		"language": []string{"required"},
	}
	langS := &api_model.LanguageSet{}
	data, ok := httputil.ValidatorRequestMapAndErrorResponse(r, w, rules, nil)
	if !ok {
		return
	}
	langS.Language = data["language"].(string)
	langS.ServiceID = r.Context().Value(middleware.ContextKey("service_id")).(string)
	if err := handler.GetServiceManager().LanguageSet(langS); err != nil {
		httputil.ReturnError(r, w, 500, "set language error.")
		return
	}
	httputil.ReturnSuccess(r, w, nil)
	return
}

//StatusService StatusService
func (t *TenantStruct) StatusService(w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /v2/tenants/{tenant_name}/services/{service_alias}/status v2 serviceStatus
	//
	// 获取应用状态
	//
	// get service status
	//
	// ---
	// consumes:
	// - application/json
	// - application/x-protobuf
	//
	// produces:
	// - application/json
	// - application/xml
	//
	// responses:
	//   default:
	//     schema:
	//       "$ref": "#/responses/commandResponse"
	//     description: 统一返回格式
	serviceID := r.Context().Value(middleware.ContextKey("service_id")).(string)
	statusList, err := handler.GetServiceManager().GetStatus(serviceID)
	if err != nil {
		httputil.ReturnError(r, w, 500, fmt.Sprintf("get service list error,%v", err))
		return
	}
	httputil.ReturnSuccess(r, w, statusList)
	return
}

//PostStatusService PostStatusService
func (t *TenantStruct) PostStatusService(w http.ResponseWriter, r *http.Request) {
	logrus.Debugf("in status service serviceID")
}

//StatusServiceList service list status
func (t *TenantStruct) StatusServiceList(w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /v2/tenants/{tenant_name}/services_status v2 serviceStatuslist
	//
	// 获取应用状态
	//
	// get service statuslist
	//
	// ---
	// consumes:
	// - application/json
	// - application/x-protobuf
	//
	// produces:
	// - application/json
	// - application/xml
	//
	// responses:
	//   default:
	//     schema:
	//       "$ref": "#/responses/commandResponse"
	//     description: 统一返回格式
	var services api_model.StatusServiceListStruct
	ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &services.Body, nil)
	if !ok {
		return
	}
	//logrus.Info(services.Body.ServiceIDs)
	serviceList := services.Body.ServiceIDs
	tenantID := r.Context().Value(middleware.ContextKey("tenant_id")).(string)
	statusList := handler.GetServiceManager().GetServicesStatus(tenantID, serviceList)
	var info = make([]map[string]string, 0)
	if statusList != nil {
		for k, v := range statusList {
			info = append(info, map[string]string{"service_id": k, "status": v, "status_cn": TransStatus(v)})
		}
	}
	httputil.ReturnSuccess(r, w, info)
}

//ServiceLabel label
func (t *TenantStruct) ServiceLabel(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "PUT":
		t.UpdateServiceLabel(w, r)
	case "POST":
		t.AddServiceLabel(w, r)
	}
}

//AddServiceLabel AddServiceLabel
func (t *TenantStruct) AddServiceLabel(w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST  /v2/tenants/{tenant_name}/services/{service_alias}/service-label v2 addServiceLabel
	//
	// 添加应用标签
	//
	// add service label
	//
	// ---
	// consumes:
	// - application/json
	// - application/x-protobuf
	//
	// produces:
	// - application/json
	// - application/xml
	//
	// responses:
	//   default:
	//     schema:
	//       "$ref": "#/responses/commandResponse"
	//     description: 统一返回格式
	rules := validator.MapData{
		"label_values": []string{"required"},
	}
	data, ok := httputil.ValidatorRequestMapAndErrorResponse(r, w, rules, nil)
	if !ok {
		return
	}
	var valueList []string
	valueList = append(valueList, data["label_values"].(string))
	serviceID := r.Context().Value(middleware.ContextKey("service_id")).(string)
	if err := handler.GetServiceManager().AddLabel("service", serviceID, valueList); err != nil {
		httputil.ReturnError(r, w, 500, fmt.Sprintf("add service label error, %v", err))
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}

//UpdateServiceLabel UpdateServiceLabel
func (t *TenantStruct) UpdateServiceLabel(w http.ResponseWriter, r *http.Request) {
	// swagger:operation PUT /v2/tenants/{tenant_name}/services/{service_alias}/service-label v2 updateServiceLabel
	//
	// 更新应用标签
	//
	// delete service label
	//
	// ---
	// consumes:
	// - application/json
	// - application/x-protobuf
	//
	// produces:
	// - application/json
	// - application/xml
	//
	// responses:
	//   default:
	//     schema:
	//       "$ref": "#/responses/commandResponse"
	//     description: 统一返回格式
	rules := validator.MapData{
		"label_values": []string{"required"},
	}
	data, ok := httputil.ValidatorRequestMapAndErrorResponse(r, w, rules, nil)
	if !ok {
		return
	}
	value := data["label_values"].(string)
	serviceID := r.Context().Value(middleware.ContextKey("service_id")).(string)
	if err := handler.GetServiceManager().UpdateServiceLabel(serviceID, value); err != nil {
		httputil.ReturnError(r, w, 500, fmt.Sprintf("update service label error, %v", err))
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}

//NodeLabel label
func (t *TenantStruct) NodeLabel(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "DELETE":
		t.DeleteNodeLabel(w, r)
	case "POST":
		t.AddNodeLabel(w, r)
	}
}

//AddNodeLabel AddNodeLabel
func (t *TenantStruct) AddNodeLabel(w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST  /v2/tenants/{tenant_name}/services/{service_alias}/node-label v2 addNodeLabel
	//
	// 添加节点标签
	//
	// add node label
	//
	// ---
	// consumes:
	// - application/json
	// - application/x-protobuf
	//
	// produces:
	// - application/json
	// - application/xml
	//
	// responses:
	//   default:
	//     schema:
	//       "$ref": "#/responses/commandResponse"
	//     description: 统一返回格式

	var labels api_model.AddNodeLabelStruct
	ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &labels.Body, nil)
	if !ok {
		return
	}
	//logrus.Info(labels.Body.LabelValues)
	valueList := labels.Body.LabelValues
	serviceID := r.Context().Value(middleware.ContextKey("service_id")).(string)
	if err := handler.GetServiceManager().AddLabel("node", serviceID, valueList); err != nil {
		httputil.ReturnError(r, w, 500, fmt.Sprintf("add node label failure, %v", err))
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}

//DeleteNodeLabel DeleteLabel
func (t *TenantStruct) DeleteNodeLabel(w http.ResponseWriter, r *http.Request) {
	// swagger:operation DELETE  /v2/tenants/{tenant_name}/services/{service_alias}/node-label v2 deleteNodeLabel
	//
	// 删除节点标签
	//
	// delete node label
	//
	// ---
	// consumes:
	// - application/json
	// - application/x-protobuf
	//
	// produces:
	// - application/json
	// - application/xml
	//
	// responses:
	//   default:
	//     schema:
	//       "$ref": "#/responses/commandResponse"
	//     description: 统一返回格式

	var labels api_model.AddNodeLabelStruct
	ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &labels.Body, nil)
	if !ok {
		return
	}
	//logrus.Info(labels.Body.LabelValues)
	valueList := labels.Body.LabelValues
	serviceID := r.Context().Value(middleware.ContextKey("service_id")).(string)
	if err := handler.GetServiceManager().DeleteLabel("node", serviceID, valueList); err != nil {
		httputil.ReturnError(r, w, 500, fmt.Sprintf("delete node label failure, %v", err))
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}

//StatusContainerID StatusContainerID
func (t *TenantStruct) StatusContainerID(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("status container IDs list"))
}

//SingleServiceInfo SingleServiceInfo
func (t *TenantStruct) SingleServiceInfo(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "DELETE":
		t.DeleteSingleServiceInfo(w, r)
	case "GET":
		t.GetSingleServiceInfo(w, r)
	}
}

//GetSingleServiceInfo GetSingleServiceInfo
func (t *TenantStruct) GetSingleServiceInfo(w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /v2/tenants/{tenant_name}/services/{service_alias} v2 getService
	//
	// 获取应用信息
	//
	// get service info
	//
	// ---
	// consumes:
	// - application/json
	// - application/x-protobuf
	//
	// produces:
	// - application/json
	// - application/xml
	//
	// responses:
	//   default:
	//     schema:
	//       "$ref": "#/responses/commandResponse"
	//     description: 统一返回格式

	tenantID := r.Context().Value(middleware.ContextKey("tenant_id")).(string)
	serviceID := r.Context().Value(middleware.ContextKey("service_id")).(string)
	tenantName := r.Context().Value(middleware.ContextKey("tenant_name")).(string)
	serviceName := r.Context().Value(middleware.ContextKey("service_alias")).(string)
	result := make(map[string]string)
	result["tenantName"] = tenantName
	result["serviceAlias"] = serviceName
	result["tenantId"] = tenantID
	result["serviceId"] = serviceID
	httputil.ReturnSuccess(r, w, result)
}

//DeleteSingleServiceInfo DeleteService
// swagger:operation DELETE /v2/tenants/{tenant_name}/services/{service_alias} v2 deleteService
//
// 删除应用
//
// delete service
//
// ---
// consumes:
// - application/json
// - application/x-protobuf
//
// produces:
// - application/json
// - application/xml
//
// responses:
//   default:
//     schema:
//       "$ref": "#/responses/commandResponse"
//     description: 统一返回格式
func (t *TenantStruct) DeleteSingleServiceInfo(w http.ResponseWriter, r *http.Request) {
	serviceID := r.Context().Value(middleware.ContextKey("service_id")).(string)
	if err := handler.GetServiceManager().TransServieToDelete(serviceID); err != nil {
		if err == fmt.Errorf("unclosed") {
			httputil.ReturnError(r, w, 400, fmt.Sprintf("Service must be closed"))
			return
		}
		httputil.ReturnError(r, w, 500, fmt.Sprintf("delete service error, %v", err))
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}

//Dependency Dependency
func (t *TenantStruct) Dependency(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "DELETE":
		t.DeleteDependency(w, r)
	case "POST":
		t.AddDependency(w, r)
	}
}

//AddDependency AddDependency
// swagger:operation POST /v2/tenants/{tenant_name}/services/{service_alias}/dependency v2 addDependency
//
// 增加应用依赖关系
//
// add dependency
//
// ---
// consumes:
// - application/json
// - application/x-protobuf
//
// produces:
// - application/json
// - application/xml
//
// responses:
//   default:
//     schema:
//       "$ref": "#/responses/commandResponse"
//     description: 统一返回格式
func (t *TenantStruct) AddDependency(w http.ResponseWriter, r *http.Request) {
	rules := validator.MapData{
		"dep_service_id":   []string{"required"},
		"dep_service_type": []string{"required"},
		"dep_order":        []string{},
	}
	data, ok := httputil.ValidatorRequestMapAndErrorResponse(r, w, rules, nil)
	if !ok {
		return
	}
	ds := &api_model.DependService{
		TenantID:       r.Context().Value(middleware.ContextKey("tenant_id")).(string),
		ServiceID:      r.Context().Value(middleware.ContextKey("service_id")).(string),
		DepServiceID:   data["dep_service_id"].(string),
		DepServiceType: data["dep_service_type"].(string),
	}
	if err := handler.GetServiceManager().ServiceDepend("add", ds); err != nil {
		httputil.ReturnError(r, w, 500, fmt.Sprintf("add dependency error, %v", err))
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}

//DeleteDependency DeleteDependency
// swagger:operation DELETE /v2/tenants/{tenant_name}/services/{service_alias}/dependency v2 deleteDependency
//
// 删除应用依赖关系
//
// delete dependency
//
// ---
// consumes:
// - application/json
// - application/x-protobuf
//
// produces:
// - application/json
// - application/xml
//
// responses:
//   default:
//     schema:
//       "$ref": "#/responses/commandResponse"
//     description: 统一返回格式
func (t *TenantStruct) DeleteDependency(w http.ResponseWriter, r *http.Request) {
	logrus.Debugf("trans delete depend service ")
	rules := validator.MapData{
		"dep_service_id":   []string{"required"},
		"dep_service_type": []string{},
		"dep_order":        []string{},
	}
	data, ok := httputil.ValidatorRequestMapAndErrorResponse(r, w, rules, nil)
	if !ok {
		return
	}
	ds := &api_model.DependService{
		TenantID:     r.Context().Value(middleware.ContextKey("tenant_id")).(string),
		ServiceID:    r.Context().Value(middleware.ContextKey("service_id")).(string),
		DepServiceID: data["dep_service_id"].(string),
	}
	if err := handler.GetServiceManager().ServiceDepend("delete", ds); err != nil {
		httputil.ReturnError(r, w, 500, fmt.Sprintf("delete dependency error, %v", err))
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}

//Env Env
func (t *TenantStruct) Env(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "DELETE":
		t.DeleteEnv(w, r)
	case "POST":
		t.AddEnv(w, r)
	case "PUT":
		t.UpdateEnv(w, r)
	}
}

//AddEnv AddEnv
// swagger:operation POST /v2/tenants/{tenant_name}/services/{service_alias}/env v2 addEnv
//
// 增加环境变量
//
// add env var
//
// ---
// consumes:
// - application/json
// - application/x-protobuf
//
// produces:
// - application/json
// - application/xml
//
// responses:
//   default:
//     schema:
//       "$ref": "#/responses/commandResponse"
//     description: 统一返回格式
func (t *TenantStruct) AddEnv(w http.ResponseWriter, r *http.Request) {
	var envM api_model.AddTenantServiceEnvVar
	if !httputil.ValidatorRequestStructAndErrorResponse(r, w, &envM, nil) {
		return
	}
	tenantID := r.Context().Value(middleware.ContextKey("tenant_id")).(string)
	serviceID := r.Context().Value(middleware.ContextKey("service_id")).(string)
	var envD dbmodel.TenantServiceEnvVar
	envD.AttrName = envM.AttrName
	envD.AttrValue = envM.AttrValue
	envD.TenantID = tenantID
	envD.ServiceID = serviceID
	envD.ContainerPort = envM.ContainerPort
	envD.IsChange = envM.IsChange
	envD.Name = envM.Name
	envD.Scope = envM.Scope
	if err := handler.GetServiceManager().EnvAttr("add", &envD); err != nil {
		logrus.Errorf("Add env error, %v", err)
		httputil.ReturnError(r, w, 500, fmt.Sprintf("Add env error, %v", err))
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}

//UpdateEnv UpdateEnv
// swagger:operation PUT /v2/tenants/{tenant_name}/services/{service_alias}/env v2 update Env
//
// 修改环境变量
//
// update env var
//
// ---
// consumes:
// - application/json
// - application/x-protobuf
//
// produces:
// - application/json
// - application/xml
//
// responses:
//   default:
//     schema:
//       "$ref": "#/responses/commandResponse"
//     description: 统一返回格式
func (t *TenantStruct) UpdateEnv(w http.ResponseWriter, r *http.Request) {
	var envM api_model.AddTenantServiceEnvVar
	if !httputil.ValidatorRequestStructAndErrorResponse(r, w, &envM, nil) {
		return
	}
	tenantID := r.Context().Value(middleware.ContextKey("tenant_id")).(string)
	serviceID := r.Context().Value(middleware.ContextKey("service_id")).(string)
	var envD dbmodel.TenantServiceEnvVar
	envD.AttrName = envM.AttrName
	envD.AttrValue = envM.AttrValue
	envD.TenantID = tenantID
	envD.ServiceID = serviceID
	envD.ContainerPort = envM.ContainerPort
	envD.IsChange = envM.IsChange
	envD.Name = envM.Name
	envD.Scope = envM.Scope
	if err := handler.GetServiceManager().EnvAttr("update", &envD); err != nil {
		logrus.Errorf("update env error, %v", err)
		httputil.ReturnError(r, w, 500, fmt.Sprintf("update env error, %v", err))
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}

//DeleteEnv DeleteEnv
// swagger:operation DELETE /v2/tenants/{tenant_name}/services/{service_alias}/env v2 deleteEnv
//
// 删除环境变量
//
// delete env var
//
// ---
// consumes:
// - application/json
// - application/x-protobuf
//
// produces:
// - application/json
// - application/xml
//
// responses:
//   default:
//     schema:
//       "$ref": "#/responses/commandResponse"
//     description: 统一返回格式
func (t *TenantStruct) DeleteEnv(w http.ResponseWriter, r *http.Request) {
	var envM api_model.DelTenantServiceEnvVar
	if !httputil.ValidatorRequestStructAndErrorResponse(r, w, &envM, nil) {
		return
	}
	tenantID := r.Context().Value(middleware.ContextKey("tenant_id")).(string)
	serviceID := r.Context().Value(middleware.ContextKey("service_id")).(string)
	envM.TenantID = tenantID
	envM.ServiceID = serviceID
	var envD dbmodel.TenantServiceEnvVar
	envD.AttrName = envM.AttrName
	envD.AttrValue = envM.AttrValue
	envD.TenantID = tenantID
	envD.ServiceID = serviceID
	envD.ContainerPort = envM.ContainerPort
	envD.IsChange = envM.IsChange
	envD.Name = envM.Name
	envD.Scope = envM.Scope
	if err := handler.GetServiceManager().EnvAttr("delete", &envD); err != nil {
		logrus.Errorf("delete env error, %v", err)
		if err.Error() == gorm.ErrRecordNotFound.Error() {
			httputil.ReturnError(r, w, 404, "service port "+err.Error())
			return
		}
		httputil.ReturnError(r, w, 500, fmt.Sprintf("Delete env error, %v", err))
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}

//Ports 应用端口控制器
func (t *TenantStruct) Ports(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "DELETE":
		t.deletePortController(w, r)
	case "POST":
		t.addPortController(w, r)
	case "PUT":
		t.updatePortController(w, r)
	}
}

//PutPorts PortVar
// swagger:operation PUT /v2/tenants/{tenant_name}/services/{service_alias}/ports v2 updatePort
//
// 更新应用端口信息(旧)
//
// update port
//
// ---
// consumes:
// - application/json
// - application/x-protobuf
//
// produces:
// - application/json
// - application/xml
//
// responses:
//   default:
//     schema:
//       "$ref": "#/responses/commandResponse"
//     description: 统一返回格式
func (t *TenantStruct) PutPorts(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(middleware.ContextKey("tenant_id")).(string)
	serviceID := r.Context().Value(middleware.ContextKey("service_id")).(string)
	var ports api_model.ServicePorts
	if ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &ports, nil); !ok {
		return
	}
	if err := handler.GetServiceManager().PortVar("update", tenantID, serviceID, &ports, 0); err != nil {
		logrus.Errorf("update port error. %v", err)
		httputil.ReturnError(r, w, 500, err.Error())
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}

//AddPortVar PortVar
// swagger:operation POST /v2/tenants/{tenant_name}/services/{service_alias}/ports v2 addPort
//
// 增加应用端口,默认关闭对内和对外选项，需要开启使用相应接口
//
// add port
//
// ---
// consumes:
// - application/json
// - application/x-protobuf
//
// produces:
// - application/json
// - application/xml
//
// responses:
//   default:
//     schema:
//       "$ref": "#/responses/commandResponse"
//     description: 统一返回格式
func (t *TenantStruct) addPortController(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(middleware.ContextKey("tenant_id")).(string)
	serviceID := r.Context().Value(middleware.ContextKey("service_id")).(string)
	var ports api_model.ServicePorts
	if ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &ports, nil); !ok {
		return
	}
	if err := handler.GetServiceManager().PortVar("add", tenantID, serviceID, &ports, 0); err != nil {
		logrus.Errorf("add port error. %v", err)
		httputil.ReturnError(r, w, 500, err.Error())
		return
	}
	httputil.ReturnSuccess(r, w, ports.Port)
}

//UpdatePortVar PortVar
// swagger:operation PUT /v2/tenants/{tenant_name}/services/{service_alias}/ports/{port} v2 updatePort
//
// 更新应用端口信息
//
// update port
//
// ---
// consumes:
// - application/json
// - application/x-protobuf
//
// produces:
// - application/json
// - application/xml
//
// responses:
//   default:
//     schema:
//       "$ref": "#/responses/commandResponse"
//     description: 统一返回格式
func (t *TenantStruct) updatePortController(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(middleware.ContextKey("tenant_id")).(string)
	serviceID := r.Context().Value(middleware.ContextKey("service_id")).(string)
	portStr := chi.URLParam(r, "port")
	oldPort, err := strconv.Atoi(portStr)
	if err != nil {
		httputil.ReturnError(r, w, 400, "port must be a number")
		return
	}
	var ports api_model.ServicePorts
	if ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &ports, nil); !ok {
		return
	}
	if err := handler.GetServiceManager().PortVar("update", tenantID, serviceID, &ports, oldPort); err != nil {
		logrus.Errorf("update port error. %v", err)
		httputil.ReturnError(r, w, 500, err.Error())
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}

//DeletePortVar PortVar
// swagger:operation DELETE /v2/tenants/{tenant_name}/services/{service_alias}/ports/{port} v2 deletePort
//
// 删除端口变量
//
// delete port
//
// ---
// Consumes:
// - application/json
// - application/x-protobuf
//
// produces:
// - application/json
// - application/xml
//
// responses:
//   default:
//     schema:
//       "$ref": "#/responses/commandResponse"
//     description: 统一返回格式
func (t *TenantStruct) deletePortController(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(middleware.ContextKey("tenant_id")).(string)
	serviceID := r.Context().Value(middleware.ContextKey("service_id")).(string)
	portStr := chi.URLParam(r, "port")
	oldPort, err := strconv.Atoi(portStr)
	if err != nil {
		httputil.ReturnError(r, w, 400, "port must be a number")
		return
	}
	var port = &api_model.TenantServicesPort{
		TenantID:      tenantID,
		ServiceID:     serviceID,
		ContainerPort: oldPort,
	}
	var ports api_model.ServicePorts
	ports.Port = append(ports.Port, port)
	if err := handler.GetServiceManager().PortVar("delete", tenantID, serviceID, &ports, oldPort); err != nil {
		if err.Error() == gorm.ErrRecordNotFound.Error() {
			httputil.ReturnError(r, w, 404, "port can not found")
			return
		}
		httputil.ReturnError(r, w, 500, err.Error())
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}

//PortOuterController 开关端口对外服务
// swagger:operation PUT /v2/tenants/{tenant_name}/services/{service_alias}/ports/{port}/outer v2 PortOuterController
//
// 开关端口对外服务，应用无需重启自动生效
//
// add port
//
// ---
// Consumes:
// - application/json
// - application/x-protobuf
//
// produces:
// - application/json
// - application/xml
//
// responses:
//   default:
//     schema:
//       "$ref": "#/responses/commandResponse"
//     description: 统一返回格式
func (t *TenantStruct) PortOuterController(w http.ResponseWriter, r *http.Request) {
	var data api_model.ServicePortInnerOrOuter
	if !httputil.ValidatorRequestStructAndErrorResponse(r, w, &(data.Body), nil) {
		return
	}
	serviceID := r.Context().Value(middleware.ContextKey("service_id")).(string)
	tenantName := r.Context().Value(middleware.ContextKey("tenant_name")).(string)
	portStr := chi.URLParam(r, "port")
	containerPort, err := strconv.Atoi(portStr)
	if err != nil {
		httputil.ReturnError(r, w, 400, "port must be a number")
		return
	}
	vsPort, protocol, errV := handler.GetServiceManager().PortOuter(tenantName, serviceID, data.Body.Operation, containerPort)
	if errV != nil {
		if strings.HasSuffix(errV.Error(), gorm.ErrRecordNotFound.Error()) {
			httputil.ReturnError(r, w, 404, errV.Error())
			return
		}
		httputil.ReturnError(r, w, 500, errV.Error())
		return
	}
	rc := make(map[string]string)
	domain := os.Getenv("EX_DOMAIN")
	if domain == "" {
		httputil.ReturnError(r, w, 500, "have no EX_DOMAIN")
		return
	}
	mm := strings.Split(domain, ":")
	if protocol == "http" || protocol == "https" {
		rc["domain"] = mm[0]
		if len(mm) == 2 {
			rc["port"] = mm[1]
		} else {
			rc["port"] = "10080"
		}
	} else if vsPort != nil && vsPort.Port != 0 {
		rc["domain"] = mm[0]
		rc["port"] = fmt.Sprintf("%v", vsPort.Port)
	}
	httputil.ReturnSuccess(r, w, rc)
}

//PortInnerController 开关端口对内服务
// swagger:operation PUT /v2/tenants/{tenant_name}/services/{service_alias}/ports/{port}/inner v2 PortInnerController
//
// 开关对内服务，应用无需重启，自动生效
//
// add port
//
// ---
// Consumes:
// - application/json
// - application/x-protobuf
//
// produces:
// - application/json
// - application/xml
//
// responses:
//   default:
//     schema:
//       "$ref": "#/responses/commandResponse"
//     description: 统一返回格式
func (t *TenantStruct) PortInnerController(w http.ResponseWriter, r *http.Request) {
	var data api_model.ServicePortInnerOrOuter
	if !httputil.ValidatorRequestStructAndErrorResponse(r, w, &(data.Body), nil) {
		return
	}
	serviceID := r.Context().Value(middleware.ContextKey("service_id")).(string)
	tenantName := r.Context().Value(middleware.ContextKey("tenant_name")).(string)
	portStr := chi.URLParam(r, "port")
	containerPort, err := strconv.Atoi(portStr)
	if err != nil {
		httputil.ReturnError(r, w, 400, "port must be a number")
		return
	}
	if err := handler.GetServiceManager().PortInner(tenantName, serviceID, data.Body.Operation, containerPort); err != nil {
		if err.Error() == gorm.ErrRecordNotFound.Error() {
			httputil.ReturnError(r, w, 404, "service port "+err.Error())
			return
		} else if err.Error() == "already open" || err.Error() == "already close" {
			httputil.Return(r, w, 200, httputil.ResponseBody{Msg: err.Error()})
			return
		} else {
			httputil.ReturnError(r, w, 500, err.Error())
			return
		}
	}
	httputil.ReturnSuccess(r, w, nil)
}

//ChangeLBPort change lb mapping port
//only support change to existing port in this tenants
func (t *TenantStruct) ChangeLBPort(w http.ResponseWriter, r *http.Request) {
	serviceID := r.Context().Value(middleware.ContextKey("service_id")).(string)
	tenantID := r.Context().Value(middleware.ContextKey("tenant_id")).(string)
	portStr := chi.URLParam(r, "port")
	containerPort, err := strconv.Atoi(portStr)
	if err != nil {
		httputil.ReturnError(r, w, 400, "port must be a number")
		return
	}
	var data api_model.ServiceLBPortChange
	if !httputil.ValidatorRequestStructAndErrorResponse(r, w, &(data.Body), nil) {
		return
	}
	mapport, errc := handler.GetServiceManager().ChangeLBPort(tenantID, serviceID, containerPort, data.Body.ChangePort)
	if errc != nil {
		errc.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, mapport)
}

//Pods pods
// swagger:operation GET  /v2/tenants/{tenant_name}/services/{service_alias}/pods v2 getPodsInfo
//
// 获取pods信息
//
// get pods info
//
// ---
// consumes:
// - application/json
// - application/x-protobuf
//
// produces:
// - application/json
// - application/xml
//
// responses:
//   default:
//     schema:
//       "$ref": "#/responses/commandResponse"
//     description: 统一返回格式
func (t *TenantStruct) Pods(w http.ResponseWriter, r *http.Request) {
	serviceID := r.Context().Value(middleware.ContextKey("service_id")).(string)
	pods, err := handler.GetServiceManager().GetPods(serviceID)
	if err != nil {
		if err.Error() == gorm.ErrRecordNotFound.Error() {
			logrus.Error("record notfound:",err)
			httputil.ReturnError(r, w, 404, fmt.Sprintf("get pods error, %v", err))
			return
		}
		logrus.Error("get pods error:",err)
		httputil.ReturnError(r, w, 500, fmt.Sprintf("get pods error, %v", err))
		return
	}
	httputil.ReturnSuccess(r, w, pods)
}

//Probe probe
func (t *TenantStruct) Probe(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "PUT":
		t.UpdateProbe(w, r)
	case "DELETE":
		t.DeleteProbe(w, r)
	case "POST":
		t.AddProbe(w, r)
	}
}

//AddProbe add probe
// swagger:operation POST /v2/tenants/{tenant_name}/services/{service_alias}/probe v2 addProbe
//
// 增加应用探针
//
// add probe
//
// ---
// consumes:
// - application/json
// - application/x-protobuf
//
// produces:
// - application/json
// - application/xml
//
// responses:
//   default:
//     schema:
//       "$ref": "#/responses/commandResponse"
//     description: 统一返回格式
func (t *TenantStruct) AddProbe(w http.ResponseWriter, r *http.Request) {
	serviceID := r.Context().Value(middleware.ContextKey("service_id")).(string)
	var tsp api_model.ServiceProbe
	if ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &tsp, nil); !ok {
		return
	}
	var tspD dbmodel.ServiceProbe
	tspD.ServiceID = serviceID
	tspD.Cmd = tsp.Cmd
	tspD.FailureThreshold = tsp.FailureThreshold
	tspD.HTTPHeader = tsp.HTTPHeader
	tspD.InitialDelaySecond = tsp.InitialDelaySecond
	tspD.IsUsed = tsp.IsUsed
	tspD.Mode = tsp.Mode
	tspD.Path = tsp.Path
	tspD.PeriodSecond = tsp.PeriodSecond
	tspD.Port = tsp.Port
	tspD.ProbeID = tsp.ProbeID
	tspD.Scheme = tsp.Scheme
	tspD.SuccessThreshold = tsp.SuccessThreshold
	tspD.TimeoutSecond = tsp.TimeoutSecond
	//注意端口问题
	if err := handler.GetServiceManager().ServiceProbe(&tspD, "add"); err != nil {
		httputil.ReturnError(r, w, 500, fmt.Sprintf("add service probe error, %v", err))
		return
	}
}

//UpdateProbe update probe
// swagger:operation PUT /v2/tenants/{tenant_name}/services/{service_alias}/probe v2 updateProbe
//
// 更新应用探针信息, *注意此处为全量更新
//
// update probe
//
// ---
// consumes:
// - application/json
// - application/x-protobuf
//
// produces:
// - application/json
// - application/xml
//
// responses:
//   default:
//     schema:
//       "$ref": "#/responses/commandResponse"
//     description: 统一返回格式
func (t *TenantStruct) UpdateProbe(w http.ResponseWriter, r *http.Request) {
	serviceID := r.Context().Value(middleware.ContextKey("service_id")).(string)
	var tsp api_model.ServiceProbe
	if ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &tsp, nil); !ok {
		return
	}
	var tspD dbmodel.ServiceProbe
	tspD.ServiceID = serviceID
	tspD.Cmd = tsp.Cmd
	tspD.FailureThreshold = tsp.FailureThreshold
	tspD.HTTPHeader = tsp.HTTPHeader
	tspD.InitialDelaySecond = tsp.InitialDelaySecond
	tspD.IsUsed = tsp.IsUsed
	tspD.Mode = tsp.Mode
	tspD.Path = tsp.Path
	tspD.PeriodSecond = tsp.PeriodSecond
	tspD.Port = tsp.Port
	tspD.ProbeID = tsp.ProbeID
	tspD.Scheme = tsp.Scheme
	tspD.SuccessThreshold = tsp.SuccessThreshold
	tspD.TimeoutSecond = tsp.TimeoutSecond
	//注意端口问题
	if err := handler.GetServiceManager().ServiceProbe(&tspD, "update"); err != nil {
		if err.Error() == gorm.ErrRecordNotFound.Error() {
			httputil.ReturnError(r, w, 404, fmt.Sprintf("update prob error, %v", err))
			return
		}
		httputil.ReturnError(r, w, 500, fmt.Sprintf("update service probe error, %v", err))
		return
	}
}

//DeleteProbe delete probe
// swagger:operation DELETE /v2/tenants/{tenant_name}/services/{service_alias}/probe v2 deleteProbe
//
// 删除应用探针
//
// delete probe
//
// ---
// consumes:
// - application/json
// - application/x-protobuf
//
// produces:
// - application/json
// - application/xml
//
// responses:
//   default:
//     schema:
//       "$ref": "#/responses/commandResponse"
//     description: 统一返回格式
func (t *TenantStruct) DeleteProbe(w http.ResponseWriter, r *http.Request) {
	serviceID := r.Context().Value(middleware.ContextKey("service_id")).(string)
	var tsp api_model.ServiceProbe
	if ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &tsp, nil); !ok {
		return
	}
	var tspD dbmodel.ServiceProbe
	tspD.ServiceID = serviceID
	tspD.ProbeID = tsp.ProbeID
	//注意端口问题
	if err := handler.GetServiceManager().ServiceProbe(&tspD, "delete"); err != nil {
		if err.Error() == gorm.ErrRecordNotFound.Error() {
			httputil.ReturnError(r, w, 404, fmt.Sprintf("delete prob error, %v", err))
			return
		}
		httputil.ReturnError(r, w, 500, fmt.Sprintf("delete service probe error, %v", err))
		return
	}
}

//Port Port
func (t *TenantStruct) Port(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "PUT":
		t.UpdatePort(w, r)
	case "DELETE":
		t.DeletePort(w, r)
	case "POST":
		t.AddPort(w, r)
	}
}

//AddPort add port
func (t *TenantStruct) AddPort(w http.ResponseWriter, r *http.Request) {
}

//DeletePort delete port
func (t *TenantStruct) DeletePort(w http.ResponseWriter, r *http.Request) {
}

//UpdatePort Update port
func (t *TenantStruct) UpdatePort(w http.ResponseWriter, r *http.Request) {
}

//SingleTenantResources SingleTenantResources
// swagger:operation GET /v2/tenants/{tenant_name}/resources v2 singletenantResources
//
// 指定租户资源使用情况
//
// get tenant resources
//
// ---
// produces:
// - application/json
// - application/xml
// parameters:
// - name: tenant_name
//   in: path
//   description: tenant name
//   required: true
//   type: string
//
// responses:
//   default:
//     schema:
//       "$ref": "#/responses/commandResponse"
//     description: 统一返回格式
func (t *TenantStruct) SingleTenantResources(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(middleware.ContextKey("tenant_id")).(string)
	//11ms
	services, err := handler.GetServiceManager().GetService(tenantID)
	if err != nil {
		msg := httputil.ResponseBody{
			Msg: fmt.Sprintf("get service error, %v", err),
		}
		httputil.Return(r, w, 500, msg)
	}
	//19ms
	statsInfo, _ := handler.GetTenantManager().StatsMemCPU(services)
	//900ms
	statsInfo.UUID = tenantID
	httputil.ReturnSuccess(r, w, statsInfo)
	return
}

//GetSupportProtocols GetSupportProtocols
// swagger:operation GET /v2/tenants/{tenant_name}/protocols v2 getSupportProtocols
//
// 获取当前数据中心支持的protocols
//
// get region protocols
//
// ---
// produces:
// - application/json
// - application/xml
// parameters:
// - name: tenant_name
//   in: path
//   description: tenant name
//   required: true
//   type: string
//
// responses:
//   default:
//     schema:
//       "$ref": "#/responses/commandResponse"
//     description: 统一返回格式
func (t *TenantStruct) GetSupportProtocols(w http.ResponseWriter, r *http.Request) {
	rps, err := handler.GetTenantManager().GetProtocols()
	if err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, rps)
	return
}

//TransPlugins transPlugins
// swagger:operation POST /v2/tenants/{tenant_name}/transplugins v2 transPlugins
//
// 安装云帮默认plugins
//
// trans plugins
//
// ---
// produces:
// - application/json
// - application/xml
// parameters:
// - name: tenant_name
//   in: path
//   description: tenant name
//   required: true
//   type: string
//
// responses:
//   default:
//     schema:
//       "$ref": "#/responses/commandResponse"
//     description: 统一返回格式
func (t *TenantStruct) TransPlugins(w http.ResponseWriter, r *http.Request) {
	var tps api_model.TransPlugins
	ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &tps.Body, nil)
	if !ok {
		return
	}
	tenantID := r.Context().Value(middleware.ContextKey("tenant_id")).(string)
	tenantName := r.Context().Value(middleware.ContextKey("tenant_name")).(string)
	rc := make(map[string]string)
	err := handler.GetTenantManager().TransPlugins(tenantID, tenantName, tps.Body.FromTenantName, tps.Body.PluginsID)
	if err != nil {
		err.Handle(r, w)
		return
	}
	rc["result"] = "success"
	httputil.ReturnSuccess(r, w, rc)
	return
}
