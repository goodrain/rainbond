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
	"net/http"

	"github.com/goodrain/rainbond/api/middleware"
	api_model "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/worker/discover/model"

	"time"

	"os"

	"github.com/Sirupsen/logrus"
	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/api/handler"
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	tutil "github.com/goodrain/rainbond/util"
	httputil "github.com/goodrain/rainbond/util/http"
	"github.com/jinzhu/gorm"
	"github.com/thedevsaddam/govalidator"
)

//TIMELAYOUT timelayout
const TIMELAYOUT = "2006-01-02T15:04:05"

func createEvent(eventID, serviceID, optType, tenantID, deployVersion string) (*dbmodel.ServiceEvent, int, error) {
	if eventID == "" {
		eventID = tutil.NewUUID()
	}
	event := dbmodel.ServiceEvent{}
	event.EventID = eventID
	event.ServiceID = serviceID
	event.OptType = optType
	event.TenantID = tenantID
	now := time.Now()
	timeNow := now.Format(TIMELAYOUT)
	event.StartTime = timeNow
	event.UserName = "system"
	version := deployVersion
	oldDeployVersion := ""
	if deployVersion == "" {
		service, err := db.GetManager().TenantServiceDao().GetServiceByID(serviceID)
		if err != nil {
			return nil, 3, nil
		}
		version = service.DeployVersion
	}
	events, err := db.GetManager().ServiceEventDao().GetEventByServiceID(serviceID)
	if err != nil {

	}
	if len(events) != 0 {
		latestEvent := events[0]
		oldDeployVersion = latestEvent.DeployVersion
	}

	event.DeployVersion = version
	event.OldDeployVersion = oldDeployVersion

	status, err := checkCanAddEvent(serviceID, event.EventID)
	if err != nil {
		logrus.Errorf("error check event", err.Error())
		return nil, status, nil
	}
	if status == 0 {
		db.GetManager().ServiceEventDao().AddModel(&event)
		return &event, status, nil
	}
	return nil, status, nil
}

func checkCanAddEvent(s, eventID string) (int, error) {
	events, err := db.GetManager().ServiceEventDao().GetEventByServiceID(s)
	if err != nil {
		return 3, err
	}
	if len(events) == 0 {
		//service 首个event
		return 0, nil
	}
	latestEvent := events[0]
	if latestEvent.EventID == eventID {
		return 0, nil
	}
	if latestEvent.FinalStatus == "" {
		//未完成
		timeOut, err := checkEventTimeOut(latestEvent)
		if err != nil {
			return 3, err
		}
		if timeOut {
			//未完成，超时
			return 0, nil
		}
		//未完成，未超时

		return 2, nil
	}
	//已完成
	return 0, nil
}
func getOrNilEventID(data map[string]interface{}) string {
	if eventID, ok := data["event_id"]; ok {
		return eventID.(string)
	}
	return ""
}
func checkEventTimeOut(event *dbmodel.ServiceEvent) (bool, error) {
	startTime := event.StartTime
	start, err := time.Parse(TIMELAYOUT, startTime)
	if err != nil {
		return true, err
	}
	if event.OptType == "deploy" || event.OptType == "create" {

		end := start.Add(3 * time.Minute)
		if time.Now().After(end) {
			event.FinalStatus = "timeout"
			err = db.GetManager().ServiceEventDao().UpdateModel(event)
			return true, err
		}

	} else {
		end := start.Add(30 * time.Second)
		if time.Now().After(end) {
			event.FinalStatus = "timeout"
			err = db.GetManager().ServiceEventDao().UpdateModel(event)
			return true, err
		}
	}

	return false, nil
}

func handleStatus(status int, err error, w http.ResponseWriter, r *http.Request) {
	if status != 0 {
		//logrus.Error("应用启动任务发送失败 "+err.Error(), map[string]string{"step": "callback", "status": "failure"})
		if status == 2 {
			httputil.ReturnError(r, w, 400, "last event unfinish.")
			return
		}
		httputil.ReturnError(r, w, 400, "create event info error.")
		return
	}
}

//StartService StartService
// swagger:operation POST /v2/tenants/{tenant_name}/services/{service_alias}/start  v2 startService
//
// 启动应用
//
// start service
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
func (t *TenantStruct) StartService(w http.ResponseWriter, r *http.Request) {
	rules := validator.MapData{
		"event_id": []string{},
	}
	data, ok := httputil.ValidatorRequestMapAndErrorResponse(r, w, rules, nil)
	if !ok {
		return
	}
	// TODO:
	// if os.Getenv("PUBLIC_CLOUD") == "true" {
	// 	tenant := r.Context().Value(middleware.ContextKey("tenant")).(*dbmodel.Tenants)
	// 	service := r.Context().Value(middleware.ContextKey("service")).(*dbmodel.TenantServices)
	// 	if err := publiccloud.ChargeSverify(tenant, service.ContainerMemory*service.Replicas, "start"); err != nil {
	// 		err.Handle(r, w)
	// 		return
	// 	}
	// }
	tenantID := r.Context().Value(middleware.ContextKey("tenant_id")).(string)
	serviceID := r.Context().Value(middleware.ContextKey("service_id")).(string)

	sEvent, status, err := createEvent(getOrNilEventID(data), serviceID, "start", tenantID, "")
	handleStatus(status, err, w, r)
	if status != 0 {
		return
	}

	eventID := sEvent.EventID

	logger := event.GetManager().GetLogger(eventID)
	defer event.CloseManager()
	startStopStruct := &api_model.StartStopStruct{
		TenantID:  tenantID,
		ServiceID: serviceID,
		EventID:   eventID,
		TaskType:  "start",
	}
	if err := handler.GetServiceManager().StartStopService(startStopStruct); err != nil {
		logger.Error("应用启动任务发送失败 "+err.Error(), map[string]string{"step": "callback", "status": "failure"})
		httputil.ReturnError(r, w, 500, "get service info error.")
		return
	}
	logger.Info("应用启动任务发送成功 ", map[string]string{"step": "start-service", "status": "starting"})
	httputil.ReturnSuccess(r, w, sEvent)
	return
}

//StopService StopService
// swagger:operation POST /v2/tenants/{tenant_name}/services/{service_alias}/stop v2 stopService
//
// 关闭应用
//
// stop service
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
func (t *TenantStruct) StopService(w http.ResponseWriter, r *http.Request) {
	rules := validator.MapData{
		"event_id": []string{},
	}
	data, ok := httputil.ValidatorRequestMapAndErrorResponse(r, w, rules, nil)
	if !ok {
		return
	}
	tenantID := r.Context().Value(middleware.ContextKey("tenant_id")).(string)
	serviceID := r.Context().Value(middleware.ContextKey("service_id")).(string)
	sEvent, status, err := createEvent(getOrNilEventID(data), serviceID, "stop", tenantID, "")
	handleStatus(status, err, w, r)
	if status != 0 {
		return
	}
	//save event
	eventID := sEvent.EventID
	logger := event.GetManager().GetLogger(eventID)
	defer event.CloseManager()
	startStopStruct := &api_model.StartStopStruct{
		TenantID:  tenantID,
		ServiceID: serviceID,
		EventID:   eventID,
		TaskType:  "stop",
	}
	if err := handler.GetServiceManager().StartStopService(startStopStruct); err != nil {
		logger.Error("应用停止任务发送失败 "+err.Error(), map[string]string{"step": "callback", "status": "failure"})
		httputil.ReturnError(r, w, 500, "get service info error.")
		return
	}
	logger.Info("应用停止任务发送成功 ", map[string]string{"step": "stop-service", "status": "starting"})
	httputil.ReturnSuccess(r, w, sEvent)
}

//RestartService RestartService
// swagger:operation POST /v2/tenants/{tenant_name}/services/{service_alias}/restart v2 restartService
//
// 重启应用
//
// restart service
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
func (t *TenantStruct) RestartService(w http.ResponseWriter, r *http.Request) {
	rules := validator.MapData{
		"event_id": []string{},
	}
	data, ok := httputil.ValidatorRequestMapAndErrorResponse(r, w, rules, nil)
	if !ok {
		return
	}
	tenantID := r.Context().Value(middleware.ContextKey("tenant_id")).(string)
	serviceID := r.Context().Value(middleware.ContextKey("service_id")).(string)
	sEvent, status, err := createEvent(getOrNilEventID(data), serviceID, "restart", tenantID, "")
	handleStatus(status, err, w, r)
	if status != 0 {
		return
	}
	//save event
	eventID := sEvent.EventID
	logger := event.GetManager().GetLogger(eventID)
	defer event.CloseManager()
	startStopStruct := &api_model.StartStopStruct{
		TenantID:  tenantID,
		ServiceID: serviceID,
		EventID:   eventID,
		TaskType:  "restart",
	}

	curStatus := t.StatusCli.GetStatus(serviceID)
	if curStatus == "closed" {
		startStopStruct.TaskType = "start"
	}
	if err := handler.GetServiceManager().StartStopService(startStopStruct); err != nil {
		logger.Error("应用重启任务发送失败 "+err.Error(), map[string]string{"step": "callback", "status": "failure"})
		httputil.ReturnError(r, w, 500, "get service info error.")
		return
	}
	logger.Info("应用重启任务发送成功 ", map[string]string{"step": "restart-service", "status": "starting"})
	httputil.ReturnSuccess(r, w, sEvent)
	return
}

//VerticalService VerticalService
// swagger:operation PUT /v2/tenants/{tenant_name}/services/{service_alias}/vertical v2 verticalService
//
// 应用垂直伸缩
//
// service vertical
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
func (t *TenantStruct) VerticalService(w http.ResponseWriter, r *http.Request) {
	rules := validator.MapData{
		"event_id":         []string{},
		"container_cpu":    []string{"required"},
		"container_memory": []string{"required"},
	}
	data, ok := httputil.ValidatorRequestMapAndErrorResponse(r, w, rules, nil)
	if !ok {
		return
	}
	tenantID := r.Context().Value(middleware.ContextKey("tenant_id")).(string)
	serviceID := r.Context().Value(middleware.ContextKey("service_id")).(string)
	sEvent, status, err := createEvent(getOrNilEventID(data), serviceID, "update", tenantID, "")
	handleStatus(status, err, w, r)
	if status != 0 {
		return
	}
	eventID := sEvent.EventID
	logger := event.GetManager().GetLogger(eventID)
	defer event.CloseManager()
	cpu := int(data["container_cpu"].(float64))
	mem := int(data["container_memory"].(float64))
	verticalTask := &model.VerticalScalingTaskBody{
		TenantID:        tenantID,
		ServiceID:       serviceID,
		EventID:         eventID,
		ContainerCPU:    cpu,
		ContainerMemory: mem,
	}
	if err := handler.GetServiceManager().ServiceVertical(verticalTask); err != nil {
		logger.Error("应用垂直升级任务发送失败 "+err.Error(), map[string]string{"step": "callback", "status": "failure"})
		httputil.ReturnError(r, w, 500, fmt.Sprintf("service vertical error. %v", err))
		return
	}
	logger.Info("应用垂直升级任务发送成功 ", map[string]string{"step": "vertical-service", "status": "starting"})
	httputil.ReturnSuccess(r, w, sEvent)
}

//HorizontalService HorizontalService
// swagger:operation PUT /v2/tenants/{tenant_name}/services/{service_alias}/horizontal v2 horizontalService
//
// 应用水平伸缩
//
// service horizontal
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
func (t *TenantStruct) HorizontalService(w http.ResponseWriter, r *http.Request) {
	rules := validator.MapData{
		"node_num": []string{"required"},
	}
	data, ok := httputil.ValidatorRequestMapAndErrorResponse(r, w, rules, nil)
	if !ok {
		return
	}
	tenantID := r.Context().Value(middleware.ContextKey("tenant_id")).(string)
	serviceID := r.Context().Value(middleware.ContextKey("service_id")).(string)
	sEvent, status, err := createEvent(getOrNilEventID(data), serviceID, "update", tenantID, "")
	handleStatus(status, err, w, r)
	if status != 0 {
		return
	}
	//save event
	eventID := sEvent.EventID
	logger := event.GetManager().GetLogger(eventID)
	defer event.CloseManager()
	replicas := int32(data["node_num"].(float64))
	horizontalTask := &model.HorizontalScalingTaskBody{
		TenantID:  tenantID,
		ServiceID: serviceID,
		EventID:   eventID,
		Replicas:  replicas,
	}
	if err := handler.GetServiceManager().ServiceHorizontal(horizontalTask); err != nil {
		logger.Error("应用水平升级任务发送失败 "+err.Error(), map[string]string{"step": "callback", "status": "failure"})
		httputil.ReturnError(r, w, 500, fmt.Sprintf("service horizontal error. %v", err))
		return
	}
	logger.Info("应用水平升级任务发送成功 ", map[string]string{"step": "horizontal-service", "status": "starting"})
	httputil.ReturnSuccess(r, w, sEvent)
}

//BuildService BuildService
// swagger:operation POST /v2/tenants/{tenant_name}/services/{service_alias}/build v2 serviceBuild
//
// 应用构建
//
// service build
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
func (t *TenantStruct) BuildService(w http.ResponseWriter, r *http.Request) {

	var build api_model.BuildServiceStruct
	ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &build.Body, nil)
	if !ok {
		return
	}
	if len(build.Body.DeployVersion) == 0 {
		httputil.ReturnError(r, w, 400, "deploy version can not be empty.")
		return
	}
	tenantID := r.Context().Value(middleware.ContextKey("tenant_id")).(string)
	serviceID := r.Context().Value(middleware.ContextKey("service_id")).(string)
	tenantName := r.Context().Value(middleware.ContextKey("tenant_name")).(string)
	serviceAlias := r.Context().Value(middleware.ContextKey("service_alias")).(string)
	build.Body.TenantName = tenantName
	build.Body.ServiceAlias = serviceAlias

	sEvent, status, err := createEvent(build.Body.EventID, serviceID, "build", tenantID, build.Body.DeployVersion)
	handleStatus(status, err, w, r)
	if status != 0 {
		return
	}
	version := dbmodel.VersionInfo{
		EventID:      sEvent.EventID,
		ServiceID:    serviceID,
		RepoURL:      build.Body.RepoURL,
		Kind:         build.Body.Kind,
		BuildVersion: build.Body.DeployVersion,
	}
	err = db.GetManager().VersionInfoDao().AddModel(&version)
	if err != nil {
		logrus.Infof("error add version %v ,details %s", version, err.Error())
		httputil.ReturnError(r, w, 500, "create service version error.")
		return
	}
	build.Body.EventID = sEvent.EventID
	if err := handler.GetServiceManager().ServiceBuild(tenantID, serviceID, &build); err != nil {
		logrus.Error("build service error", err.Error())
		httputil.ReturnError(r, w, 500, fmt.Sprintf("build service error, %v", err))
		return
	}
	httputil.ReturnSuccess(r, w, sEvent)
}

//BuildList BuildList
func (t *TenantStruct) BuildList(w http.ResponseWriter, r *http.Request) {
	serviceID := r.Context().Value(middleware.ContextKey("service_id")).(string)
	versionInfoList, err := db.GetManager().VersionInfoDao().GetVersionByServiceID(serviceID)
	if err != nil {
		logrus.Error("get version info error", err.Error())
		httputil.ReturnError(r, w, 500, fmt.Sprintf("get version info erro, %v", err))
		return
	}
	httputil.ReturnSuccess(r, w, versionInfoList)
}

func (t *TenantStruct) BuildVersionIsExist(w http.ResponseWriter, r *http.Request) {
	statusMap := make(map[string]bool)
	serviceID := r.Context().Value(middleware.ContextKey("service_id")).(string)
	buildVersion := chi.URLParam(r, "build_version")
	_, err := db.GetManager().VersionInfoDao().GetVersionByDeployVersion(buildVersion, serviceID)
	if err != nil && err != gorm.ErrRecordNotFound {
		httputil.ReturnError(r, w, 500, fmt.Sprintf("get build version status erro, %v", err))
		return
	}
	if err == gorm.ErrRecordNotFound {
		statusMap["status"] = false
	} else {
		statusMap["status"] = true
	}
	httputil.ReturnSuccess(r, w, statusMap)

}

func (t *TenantStruct) DeleteBuildVersion(w http.ResponseWriter, r *http.Request) {
	serviceID := r.Context().Value(middleware.ContextKey("service_id")).(string)
	buildVersion := chi.URLParam(r, "build_version")
	val, err := db.GetManager().VersionInfoDao().GetVersionByDeployVersion(buildVersion, serviceID)
	if err != nil && err != gorm.ErrRecordNotFound {
		httputil.ReturnError(r, w, 500, fmt.Sprintf("delete build version erro, %v", err))
		return
	}
	if err == gorm.ErrRecordNotFound {

	} else {
		if val.DeliveredType == "slug" && val.FinalStatus == "success" {
			if err := os.Remove(val.DeliveredPath); err != nil {
				httputil.ReturnError(r, w, 500, fmt.Sprintf("delete build version erro, %v", err))
				return

			}
			if err := db.GetManager().VersionInfoDao().DeleteVersionInfo(val); err != nil {
				httputil.ReturnError(r, w, 500, fmt.Sprintf("delete build version erro, %v", err))
				return

			}
		}
		if val.DeliveredType == "slug" && val.FinalStatus == "failure" {
			if err := db.GetManager().VersionInfoDao().DeleteVersionInfo(val); err != nil {
				httputil.ReturnError(r, w, 500, fmt.Sprintf("delete build version erro, %v", err))
				return
			}
		}
		if val.DeliveredType == "image" {
			if err := db.GetManager().VersionInfoDao().DeleteVersionInfo(val); err != nil {
				httputil.ReturnError(r, w, 500, fmt.Sprintf("delete build version erro, %v", err))
				return
			}
		}
	}
	httputil.ReturnSuccess(r, w, nil)

}

func (t *TenantStruct) BuildVersionInfo(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "DELETE":
		t.DeleteBuildVersion(w, r)
	case "GET":
		t.BuildVersionIsExist(w, r)
	}

}

//GetDeployVersion GetDeployVersion by service
func (t *TenantStruct) GetDeployVersion(w http.ResponseWriter, r *http.Request) {
	service := r.Context().Value(middleware.ContextKey("service")).(*dbmodel.TenantServices)
	version, err := db.GetManager().VersionInfoDao().GetVersionByDeployVersion(service.DeployVersion, service.ServiceID)
	if err != nil && err != gorm.ErrRecordNotFound {
		httputil.ReturnError(r, w, 500, fmt.Sprintf("get build version status erro, %v", err))
		return
	}
	if err == gorm.ErrRecordNotFound {
		httputil.ReturnError(r, w, 404, fmt.Sprintf("build version do not exist"))
		return
	}
	httputil.ReturnSuccess(r, w, version)
}

//GetManyDeployVersion GetDeployVersion by some service id
func (t *TenantStruct) GetManyDeployVersion(w http.ResponseWriter, r *http.Request) {
	rules := validator.MapData{
		"service_ids": []string{"required"},
	}
	data, ok := httputil.ValidatorRequestMapAndErrorResponse(r, w, rules, nil)
	if !ok {
		return
	}
	serviceIDs, ok := data["service_ids"].([]interface{})
	if !ok {
		httputil.ReturnError(r, w, 400, fmt.Sprintf("service ids must be a array"))
		return
	}
	var list []string
	for _, s := range serviceIDs {
		list = append(list, s.(string))
	}
	services, err := db.GetManager().TenantServiceDao().GetServiceByIDs(list)
	if err != nil {
		httputil.ReturnError(r, w, 500, fmt.Sprintf(err.Error()))
		return
	}
	var versionList []*dbmodel.VersionInfo
	for _, service := range services {
		version, err := db.GetManager().VersionInfoDao().GetVersionByDeployVersion(service.DeployVersion, service.ServiceID)
		if err != nil && err != gorm.ErrRecordNotFound {
			httputil.ReturnError(r, w, 500, fmt.Sprintf("get build version status erro, %v", err))
			return
		}
		versionList = append(versionList, version)
	}
	httputil.ReturnSuccess(r, w, versionList)
}

//DeployService DeployService
func (t *TenantStruct) DeployService(w http.ResponseWriter, r *http.Request) {
	logrus.Debugf("trans deploy service")
	w.Write([]byte("deploy service"))
}

//UpgradeService UpgradeService
// swagger:operation POST /v2/tenants/{tenant_name}/services/{service_alias}/upgrade v2 upgradeService
//
// 升级应用
//
// upgrade service
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
func (t *TenantStruct) UpgradeService(w http.ResponseWriter, r *http.Request) {
	rules := validator.MapData{
		"deploy_version": []string{"required"},
	}
	data, ok := httputil.ValidatorRequestMapAndErrorResponse(r, w, rules, nil)
	if !ok {
		return
	}
	tenantID := r.Context().Value(middleware.ContextKey("tenant_id")).(string)
	serviceID := r.Context().Value(middleware.ContextKey("service_id")).(string)

	sEvent, status, err := createEvent(getOrNilEventID(data), serviceID, "update", tenantID, data["deploy_version"].(string))
	handleStatus(status, err, w, r)
	if status != 0 {
		return
	}

	eventID := sEvent.EventID
	logger := event.GetManager().GetLogger(eventID)
	defer event.CloseManager()
	newDeployVersion := data["deploy_version"].(string)
	//两个deploy version
	upgradeTask := &model.RollingUpgradeTaskBody{
		TenantID:         tenantID,
		ServiceID:        serviceID,
		NewDeployVersion: newDeployVersion,
		EventID:          eventID,
	}
	if err := handler.GetServiceManager().ServiceUpgrade(upgradeTask); err != nil {
		logger.Error("应用升级任务发送失败 "+err.Error(), map[string]string{"step": "callback", "status": "failure"})
		httputil.ReturnError(r, w, 500, fmt.Sprintf("service upgrade error, %v", err))
		return
	}
	logger.Info("应用升级任务发送成功 ", map[string]string{"step": "upgrade-service", "status": "starting"})
	httputil.ReturnSuccess(r, w, sEvent)
}

//CheckCode CheckCode
// swagger:operation POST /v2/tenants/{tenant_name}/code-check v2 checkCode
//
// 应用代码检测
//
// check  code
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
func (t *TenantStruct) CheckCode(w http.ResponseWriter, r *http.Request) {

	var ccs api_model.CheckCodeStruct
	ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &ccs.Body, nil)
	if !ok {
		return
	}
	if ccs.Body.TenantID == "" {
		tenantID := r.Context().Value(middleware.ContextKey("tenant_id")).(string)
		ccs.Body.TenantID = tenantID
	}
	ccs.Body.Action = "code_check"
	if err := handler.GetServiceManager().CodeCheck(&ccs); err != nil {
		httputil.ReturnError(r, w, 500, fmt.Sprintf("task code check error,%v", err))
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}

//ShareCloud 云市分享
func (t *TenantStruct) ShareCloud(w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /v2/tenants/{tenant_name}/cloud-share v2 sharecloud
	//
	// 云市分享 （v3.5弃用）
	//
	// share cloud
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
	logrus.Debugf("trans cloud share service")
	var css api_model.CloudShareStruct
	ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &css.Body, nil)
	if !ok {
		return
	}
	if err := handler.GetServiceManager().ShareCloud(&css); err != nil {
		if err.Error() == "need share kind" {
			httputil.ReturnError(r, w, 400, err.Error())
		}
		httputil.ReturnError(r, w, 500, fmt.Sprintf("task code check error, %v", err))
		return
	}
	httputil.ReturnSuccess(r, w, nil)
	return
}

//RollBack RollBack
// swagger:operation Post /v2/tenants/{tenant_name}/services/{service_alias}/rollback v2 rollback
//
// 应用版本回滚
//
// service rollback
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
func (t *TenantStruct) RollBack(w http.ResponseWriter, r *http.Request) {
	rules := validator.MapData{
		"deploy_version": []string{"required"},
		"operator":       []string{},
	}
	data, ok := httputil.ValidatorRequestMapAndErrorResponse(r, w, rules, nil)
	if !ok {
		return
	}
	tenantID := r.Context().Value(middleware.ContextKey("tenant_id")).(string)
	serviceID := r.Context().Value(middleware.ContextKey("service_id")).(string)
	sEvent, status, err := createEvent(getOrNilEventID(data), serviceID, "rollback", tenantID, data["deploy_version"].(string))
	handleStatus(status, err, w, r)
	if status != 0 {
		return
	}
	eventID := sEvent.EventID
	logger := event.GetManager().GetLogger(eventID)
	defer event.CloseManager()
	rs := &api_model.RollbackStruct{
		TenantID:  tenantID,
		ServiceID: serviceID,
		EventID:   eventID,
		//todo
		DeployVersion: data["deploy_version"].(string),
	}
	if _, ok := data["operator"]; ok {
		rs.Operator = data["operator"].(string)
	}

	if err := handler.GetServiceManager().RollBack(rs); err != nil {
		logger.Error("应用回滚任务发送失败 "+err.Error(), map[string]string{"step": "callback", "status": "failure"})
		httputil.ReturnError(r, w, 500, fmt.Sprintf("check deploy version error, %v", err))
		return
	}
	logger.Info("应用回滚任务发送成功 ", map[string]string{"step": "rollback-service", "status": "starting"})
	httputil.ReturnSuccess(r, w, sEvent)
	return
}
