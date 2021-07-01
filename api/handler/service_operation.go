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
	"github.com/pkg/errors"
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
func (o *OperationHandler) Build(batchOpReq model.ComponentOpReq) (*model.ComponentOpResult, error) {
	res := batchOpReq.BatchOpFailureItem()
	if err := o.build(batchOpReq); err != nil {
		res.ErrMsg = err.Error()
	} else {
		res.Success()
	}
	return res, nil
}

func (o *OperationHandler) build(batchOpReq model.ComponentOpReq) error {
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		util.Elapsed(fmt.Sprintf("build component(%s)", batchOpReq.GetComponentID()))()
	}

	service, err := db.GetManager().TenantServiceDao().GetServiceByID(batchOpReq.GetComponentID())
	if err != nil {
		return err
	}
	if dbmodel.ServiceKind(service.Kind) == dbmodel.ServiceKindThirdParty {
		return nil
	}

	buildReq := batchOpReq.(*model.ComponentBuildReq)
	buildReq.DeployVersion = util.CreateVersionByTime()

	version := dbmodel.VersionInfo{
		EventID:      buildReq.GetEventID(),
		ServiceID:    buildReq.ServiceID,
		RepoURL:      buildReq.CodeInfo.RepoURL,
		Kind:         buildReq.Kind,
		BuildVersion: buildReq.DeployVersion,
		Cmd:          buildReq.ImageInfo.Cmd,
		Author:       buildReq.Operator,
		FinishTime:   time.Now(),
		PlanVersion:  buildReq.PlanVersion,
	}
	if buildReq.CodeInfo.Cmd != "" {
		version.Cmd = buildReq.CodeInfo.Cmd
	}
	if err = db.GetManager().VersionInfoDao().AddModel(&version); err != nil {
		return err
	}

	switch buildReq.Kind {
	case model.FromImageBuildKing:
		if err := o.buildFromImage(buildReq, service); err != nil {
			return err
		}
	case model.FromCodeBuildKing:
		if err := o.buildFromSourceCode(buildReq, service); err != nil {
			return err
		}
	case model.FromMarketImageBuildKing:
		if err := o.buildFromImage(buildReq, service); err != nil {
			return err
		}
	case model.FromMarketSlugBuildKing:
		if err := o.buildFromMarketSlug(buildReq, service); err != nil {
			return err
		}
	default:
		return errors.New("unsupported build kind: " + buildReq.Kind)
	}
	return nil
}

//Stop service stop
func (o *OperationHandler) Stop(batchOpReq model.ComponentOpReq) error {
	service, err := db.GetManager().TenantServiceDao().GetServiceByID(batchOpReq.GetComponentID())
	if err != nil {
		return err
	}
	body := batchOpReq.TaskBody(service)
	err = o.mqCli.SendBuilderTopic(gclient.TaskStruct{
		TaskType: "stop",
		TaskBody: body,
		Topic:    gclient.WorkerTopic,
	})
	if err != nil {
		return err
	}
	return nil
}

//Start service start
func (o *OperationHandler) Start(batchOpReq model.ComponentOpReq) error {
	service, err := db.GetManager().TenantServiceDao().GetServiceByID(batchOpReq.GetComponentID())
	if err != nil {
		return err
	}

	body := batchOpReq.TaskBody(service)
	err = o.mqCli.SendBuilderTopic(gclient.TaskStruct{
		TaskType: "start",
		TaskBody: body,
		Topic:    gclient.WorkerTopic,
	})
	if err != nil {
		return err
	}
	return nil
}

//Upgrade service upgrade
func (o *OperationHandler) Upgrade(batchOpReq model.ComponentOpReq) (*model.ComponentOpResult, error) {
	res := batchOpReq.BatchOpFailureItem()
	if err := o.upgrade(batchOpReq); err != nil {
		res.ErrMsg = err.Error()
	} else {
		res.Success()
	}
	return res, nil
}
func (o *OperationHandler) upgrade(batchOpReq model.ComponentOpReq) error {
	component, err := db.GetManager().TenantServiceDao().GetServiceByID(batchOpReq.GetComponentID())
	if err != nil {
		return err
	}
	if dbmodel.ServiceKind(component.Kind) == dbmodel.ServiceKindThirdParty {
		return err
	}

	batchOpReq.SetVersion(component.DeployVersion)

	version, err := db.GetManager().VersionInfoDao().GetVersionByDeployVersion(batchOpReq.GetVersion(), batchOpReq.GetComponentID())
	if err != nil {
		return err
	}
	oldDeployVersion := component.DeployVersion
	var rollback = func() {
		component.DeployVersion = oldDeployVersion
		_ = db.GetManager().TenantServiceDao().UpdateModel(component)
	}

	if version.FinalStatus != "success" {
		logrus.Warnf("deploy version %s is not build success,can not change deploy version in this upgrade event", batchOpReq.GetVersion())
	} else {
		component.DeployVersion = batchOpReq.GetVersion()
		err = db.GetManager().TenantServiceDao().UpdateModel(component)
		if err != nil {
			return err
		}
	}

	body := batchOpReq.TaskBody(component)
	err = o.mqCli.SendBuilderTopic(gclient.TaskStruct{
		TaskBody: body,
		TaskType: "rolling_upgrade",
		Topic:    gclient.WorkerTopic,
	})
	if err != nil {
		rollback()
		return err
	}
	return nil
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

func (o *OperationHandler) buildFromMarketSlug(r *model.ComponentBuildReq, service *dbmodel.TenantServices) error {
	body := make(map[string]interface{})
	body["deploy_version"] = r.DeployVersion
	body["event_id"] = r.GetEventID()
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

func (o *OperationHandler) buildFromImage(r *model.ComponentBuildReq, service *dbmodel.TenantServices) error {
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		util.Elapsed(fmt.Sprintf("[buildFromImage] build component(%s)", r.GetComponentID()))()
	}

	if r.ImageInfo.ImageURL == "" || r.DeployVersion == "" {
		return fmt.Errorf("build from image failure, args error")
	}
	body := make(map[string]interface{})
	body["image"] = r.ImageInfo.ImageURL
	body["service_id"] = service.ServiceID
	body["deploy_version"] = r.DeployVersion
	body["namespace"] = service.Namespace
	body["event_id"] = r.GetEventID()
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

func (o *OperationHandler) buildFromSourceCode(r *model.ComponentBuildReq, service *dbmodel.TenantServices) error {
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
	body["event_id"] = r.GetEventID()
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
