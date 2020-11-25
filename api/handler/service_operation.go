// RAINBOND, Application Management Platform
// Copyright (C) 2014-2019 Goodrain Co., Ltd.

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

package handler

import (
	"fmt"
	"time"

	"github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	gclient "github.com/goodrain/rainbond/mq/client"
	"github.com/goodrain/rainbond/util"
	dmodel "github.com/goodrain/rainbond/worker/discover/model"
	"github.com/sirupsen/logrus"
)

//OperationHandler operation handler
type OperationHandler struct {
	mqCli gclient.MQClient
}

//OperationResult batch operation result
type OperationResult struct {
	ServiceID     string `json:"service_id"`
	Operation     string `json:"operation"`
	EventID       string `json:"event_id"`
	Status        string `json:"status"`
	ErrMsg        string `json:"err_message"`
	DeployVersion string `json:"deploy_version"`
}

//CreateOperationHandler create  operation handler
func CreateOperationHandler(mqCli gclient.MQClient) *OperationHandler {
	return &OperationHandler{
		mqCli: mqCli,
	}
}

//Build service build,will create new version
//if deploy version not define, will create by time
func (o *OperationHandler) Build(buildInfo model.BuildInfoRequestStruct) (re OperationResult) {
	service, err := db.GetManager().TenantServiceDao().GetServiceByID(buildInfo.ServiceID)
	if err != nil {
		re.ErrMsg = fmt.Sprintf("find service %s failure", buildInfo.ServiceID)
		return
	}
	if dbmodel.ServiceKind(service.Kind) == dbmodel.ServiceKindThirdParty {
		re.ErrMsg = fmt.Sprintf("service %s is thirdpart service", buildInfo.ServiceID)
		return
	}
	buildInfo.DeployVersion = util.CreateVersionByTime()
	re.DeployVersion = buildInfo.DeployVersion
	version := dbmodel.VersionInfo{
		EventID:      buildInfo.EventID,
		ServiceID:    buildInfo.ServiceID,
		RepoURL:      buildInfo.CodeInfo.RepoURL,
		Kind:         buildInfo.Kind,
		BuildVersion: buildInfo.DeployVersion,
		Cmd:          buildInfo.ImageInfo.Cmd,
		Author:       buildInfo.Operator,
		FinishTime:   time.Now(),
	}
	if buildInfo.CodeInfo.Cmd != "" {
		version.Cmd = buildInfo.CodeInfo.Cmd
	}
	serviceID := buildInfo.ServiceID
	err = db.GetManager().VersionInfoDao().AddModel(&version)
	if err != nil {
		logrus.Errorf("error add version %v ,details %s", version, err.Error())
		re.ErrMsg = fmt.Sprintf("create service %s new version %s failure", serviceID, buildInfo.DeployVersion)
		return
	}
	re.EventID = buildInfo.EventID
	re.Operation = "build"
	re.ServiceID = service.ServiceID
	re.Status = "failure"
	switch buildInfo.Kind {
	case model.FromImageBuildKing:
		if err := o.buildFromImage(buildInfo, service); err != nil {
			logrus.Errorf("build service %s failure %s", serviceID, err.Error())
			re.ErrMsg = fmt.Sprintf("build service %s failure", serviceID)
			return
		}
	case model.FromCodeBuildKing:
		if err := o.buildFromSourceCode(buildInfo, service); err != nil {
			logrus.Errorf("build service %s failure %s", serviceID, err.Error())
			re.ErrMsg = fmt.Sprintf("build service %s failure", serviceID)
			return
		}
	case model.FromMarketImageBuildKing:
		if err := o.buildFromImage(buildInfo, service); err != nil {
			logrus.Errorf("build service %s failure %s", serviceID, err.Error())
			re.ErrMsg = fmt.Sprintf("build service %s failure", serviceID)
			return
		}

	case model.FromMarketSlugBuildKing:
		if err := o.buildFromMarketSlug(buildInfo, service); err != nil {
			logrus.Errorf("build service %s failure %s", serviceID, err.Error())
			re.ErrMsg = fmt.Sprintf("build service %s failure", serviceID)
			return
		}
	default:
		re.ErrMsg = fmt.Sprintf("build service %s failure,kind %s is unsupport", serviceID, buildInfo.Kind)
		return
	}
	re.Status = "success"
	return
}

//Stop service stop
func (o *OperationHandler) Stop(stopInfo model.StartOrStopInfoRequestStruct) (re OperationResult) {
	re.EventID = stopInfo.EventID
	re.Operation = "stop"
	re.ServiceID = stopInfo.ServiceID
	re.Status = "failure"
	service, err := db.GetManager().TenantServiceDao().GetServiceByID(stopInfo.ServiceID)
	if err != nil {
		logrus.Errorf("get service by id error, %v", err)
		re.ErrMsg = fmt.Sprintf("get service %s failure", stopInfo.ServiceID)
		return
	}
	if dbmodel.ServiceKind(service.Kind) == dbmodel.ServiceKindThirdParty {
		re.ErrMsg = fmt.Sprintf("service %s is thirdpart service", stopInfo.ServiceID)
		return
	}
	TaskBody := dmodel.StopTaskBody{
		TenantID:      service.TenantID,
		ServiceID:     service.ServiceID,
		DeployVersion: service.DeployVersion,
		EventID:       re.EventID,
		Configs:       stopInfo.Configs,
	}
	err = o.mqCli.SendBuilderTopic(gclient.TaskStruct{
		TaskType: "stop",
		TaskBody: TaskBody,
		Topic:    gclient.WorkerTopic,
	})
	if err != nil {
		logrus.Errorf("equque mq error, %v", err)
		re.ErrMsg = fmt.Sprintf("start service %s failure", stopInfo.ServiceID)
		return
	}
	re.Status = "success"
	return
}

//Start service start
func (o *OperationHandler) Start(startInfo model.StartOrStopInfoRequestStruct) (re OperationResult) {
	re.Operation = "start"
	re.ServiceID = startInfo.ServiceID
	re.Status = "failure"
	service, err := db.GetManager().TenantServiceDao().GetServiceByID(startInfo.ServiceID)
	if err != nil {
		logrus.Errorf("get service by id error, %v", err)
		re.ErrMsg = fmt.Sprintf("get service %s failure", startInfo.ServiceID)
		return
	}
	if dbmodel.ServiceKind(service.Kind) == dbmodel.ServiceKindThirdParty {
		re.ErrMsg = fmt.Sprintf("service %s is thirdpart service", startInfo.ServiceID)
		return
	}
	re.EventID = startInfo.EventID
	TaskBody := dmodel.StartTaskBody{
		TenantID:              service.TenantID,
		ServiceID:             service.ServiceID,
		DeployVersion:         service.DeployVersion,
		EventID:               startInfo.EventID,
		Configs:               startInfo.Configs,
		DepServiceIDInBootSeq: startInfo.DepServiceIDInBootSeq,
	}
	err = o.mqCli.SendBuilderTopic(gclient.TaskStruct{
		TaskType: "start",
		TaskBody: TaskBody,
		Topic:    gclient.WorkerTopic,
	})
	if err != nil {
		logrus.Errorf("equque mq error, %v", err)
		re.ErrMsg = fmt.Sprintf("start service %s failure", startInfo.ServiceID)
		return
	}
	re.Status = "success"
	return
}

//Upgrade service upgrade
func (o *OperationHandler) Upgrade(ru model.UpgradeInfoRequestStruct) (re OperationResult) {
	re.Operation = "upgrade"
	re.ServiceID = ru.ServiceID
	re.EventID = ru.EventID
	re.Status = "failure"
	services, err := db.GetManager().TenantServiceDao().GetServiceByID(ru.ServiceID)
	if err != nil {
		logrus.Errorf("get service by id %s error %s", ru.ServiceID, err.Error())
		re.ErrMsg = fmt.Sprintf("get service %s failure", ru.ServiceID)
		return
	}
	if dbmodel.ServiceKind(services.Kind) == dbmodel.ServiceKindThirdParty {
		re.ErrMsg = fmt.Sprintf("service %s is thirdpart service", ru.ServiceID)
		return
	}

	// By default, the same version is updated
	if ru.UpgradeVersion == "" {
		ru.UpgradeVersion = services.DeployVersion
	}
	version, err := db.GetManager().VersionInfoDao().GetVersionByDeployVersion(ru.UpgradeVersion, ru.ServiceID)
	if err != nil {
		logrus.Errorf("get service version by id %s version %s error, %s", ru.ServiceID, ru.UpgradeVersion, err.Error())
		re.ErrMsg = fmt.Sprintf("get service %s version %s failure", ru.ServiceID, ru.UpgradeVersion)
		return
	}
	oldDeployVersion := services.DeployVersion
	var rollback = func() {
		services.DeployVersion = oldDeployVersion
		_ = db.GetManager().TenantServiceDao().UpdateModel(services)
	}
	if version.FinalStatus != "success" {
		logrus.Warnf("deploy version %s is not build success,can not change deploy version in this upgrade event", ru.UpgradeVersion)
	} else {
		services.DeployVersion = ru.UpgradeVersion
		err = db.GetManager().TenantServiceDao().UpdateModel(services)
		if err != nil {
			logrus.Errorf("update service deploy version error. %v", err)
			re.ErrMsg = fmt.Sprintf("update service %s deploy version failure", ru.ServiceID)
			return
		}
	}
	err = o.mqCli.SendBuilderTopic(gclient.TaskStruct{
		TaskBody: dmodel.RollingUpgradeTaskBody{
			TenantID:         services.TenantID,
			ServiceID:        services.ServiceID,
			NewDeployVersion: ru.UpgradeVersion,
			EventID:          re.EventID,
			Configs:          ru.Configs,
		},
		TaskType: "rolling_upgrade",
		Topic:    gclient.WorkerTopic,
	})
	if err != nil {
		rollback()
		logrus.Errorf("equque upgrade message error, %v", err)
		re.ErrMsg = fmt.Sprintf("send service %s upgrade message failure", ru.ServiceID)
		return
	}
	re.Status = "success"
	return
}

//RollBack service rollback
func (o *OperationHandler) RollBack(rollback model.RollbackInfoRequestStruct) (re OperationResult) {
	re.Operation = "rollback"
	re.ServiceID = rollback.ServiceID
	re.EventID = rollback.EventID
	re.Status = "failure"
	service, err := db.GetManager().TenantServiceDao().GetServiceByID(rollback.ServiceID)
	if err != nil {
		logrus.Errorf("find service %s failure %s", rollback.ServiceID, err.Error())
		re.ErrMsg = fmt.Sprintf("find service %s failure", rollback.ServiceID)
		return
	}
	if dbmodel.ServiceKind(service.Kind) == dbmodel.ServiceKindThirdParty {
		re.ErrMsg = fmt.Sprintf("service %s is thirdpart service", rollback.ServiceID)
		return
	}
	oldDeployVersion := service.DeployVersion
	var rollbackFunc = func() {
		service.DeployVersion = oldDeployVersion
		_ = db.GetManager().TenantServiceDao().UpdateModel(service)
	}

	if service.DeployVersion == rollback.RollBackVersion {
		logrus.Warningf("rollback version is same of current version")
	}
	service.DeployVersion = rollback.RollBackVersion
	if err := db.GetManager().TenantServiceDao().UpdateModel(service); err != nil {
		logrus.Errorf("update service %s version failure %s", rollback.ServiceID, err.Error())
		re.ErrMsg = fmt.Sprintf("update service %s version failure", rollback.ServiceID)
		return
	}
	err = o.mqCli.SendBuilderTopic(gclient.TaskStruct{
		TaskBody: dmodel.RollingUpgradeTaskBody{
			TenantID:         service.TenantID,
			ServiceID:        service.ServiceID,
			NewDeployVersion: rollback.RollBackVersion,
			EventID:          rollback.EventID,
		},
		TaskType: "rolling_upgrade",
		Topic:    gclient.WorkerTopic,
	})
	if err != nil {
		rollbackFunc()
		logrus.Errorf("equque rollback message error, %v", err)
		re.ErrMsg = fmt.Sprintf("send service %s rollback message failure", rollback.ServiceID)
		return
	}
	re.Status = "success"
	return
}

//Restart service restart
//TODO
func (o *OperationHandler) Restart(restartInfo model.StartOrStopInfoRequestStruct) (re OperationResult) {
	return
}

func (o *OperationHandler) buildFromMarketSlug(r model.BuildInfoRequestStruct, service *dbmodel.TenantServices) error {
	body := make(map[string]interface{})
	body["deploy_version"] = r.DeployVersion
	body["event_id"] = r.EventID
	body["action"] = r.Action
	body["tenant_name"] = r.TenantName
	body["tenant_id"] = service.TenantID
	body["service_id"] = service.ServiceID
	body["service_alias"] = service.ServiceAlias
	body["slug_info"] = r.SlugInfo
	body["configs"] = r.Configs
	return o.sendBuildTopic(service.ServiceID, "build_from_market_slug", body)
}
func (o *OperationHandler) sendBuildTopic(serviceID, taskType string, body map[string]interface{}) error {
	topic := gclient.BuilderTopic
	if o.isWindowsService(serviceID) {
		topic = gclient.WindowsBuilderTopic
	}
	return o.mqCli.SendBuilderTopic(gclient.TaskStruct{
		Topic:    topic,
		TaskType: taskType,
		TaskBody: body,
	})
}

func (o *OperationHandler) buildFromImage(r model.BuildInfoRequestStruct, service *dbmodel.TenantServices) error {
	if r.ImageInfo.ImageURL == "" || r.DeployVersion == "" {
		return fmt.Errorf("build from image failure, args error")
	}
	body := make(map[string]interface{})
	body["image"] = r.ImageInfo.ImageURL
	body["service_id"] = service.ServiceID
	body["deploy_version"] = r.DeployVersion
	body["namespace"] = service.Namespace
	body["event_id"] = r.EventID
	body["tenant_name"] = r.TenantName
	body["service_alias"] = service.ServiceAlias
	body["action"] = r.Action
	body["code_from"] = "image_manual"
	if r.ImageInfo.User != "" && r.ImageInfo.Password != "" {
		body["user"] = r.ImageInfo.User
		body["password"] = r.ImageInfo.Password
	}
	body["configs"] = r.Configs
	return o.sendBuildTopic(service.ServiceID, "build_from_image", body)
}

func (o *OperationHandler) buildFromSourceCode(r model.BuildInfoRequestStruct, service *dbmodel.TenantServices) error {
	if r.CodeInfo.RepoURL == "" || r.CodeInfo.Branch == "" || r.DeployVersion == "" {
		return fmt.Errorf("build from code failure, args error")
	}
	body := make(map[string]interface{})
	body["tenant_id"] = service.TenantID
	body["service_id"] = service.ServiceID
	body["repo_url"] = r.CodeInfo.RepoURL
	body["action"] = r.Action
	body["lang"] = r.CodeInfo.Lang
	body["runtime"] = r.CodeInfo.Runtime
	body["deploy_version"] = r.DeployVersion
	body["event_id"] = r.EventID
	body["envs"] = r.BuildENVs
	body["tenant_name"] = r.TenantName
	body["branch"] = r.CodeInfo.Branch
	body["server_type"] = r.CodeInfo.ServerType
	body["service_alias"] = service.ServiceAlias
	if r.CodeInfo.User != "" && r.CodeInfo.Password != "" {
		body["user"] = r.CodeInfo.User
		body["password"] = r.CodeInfo.Password
	}
	body["expire"] = 180
	body["configs"] = r.Configs
	return o.sendBuildTopic(service.ServiceID, "build_from_source_code", body)
}

func (o *OperationHandler) isWindowsService(serviceID string) bool {
	label, err := db.GetManager().TenantServiceLabelDao().GetLabelByNodeSelectorKey(serviceID, "windows")
	if label == nil || err != nil {
		return false
	}
	return true
}
