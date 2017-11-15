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

package handler

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/goodrain/rainbond/cmd/api/option"
	api_db "github.com/goodrain/rainbond/pkg/api/db"
	api_model "github.com/goodrain/rainbond/pkg/api/model"
	"github.com/goodrain/rainbond/pkg/api/util"
	"github.com/goodrain/rainbond/pkg/db"
	dbmodel "github.com/goodrain/rainbond/pkg/db/model"
	"github.com/goodrain/rainbond/pkg/event"
	"github.com/goodrain/rainbond/pkg/mq/api/grpc/pb"

	"github.com/pquerna/ffjson/ffjson"

	builder_model "github.com/goodrain/rainbond/pkg/builder/model"

	"github.com/Sirupsen/logrus"
)

//PluginAction  plugin action struct
type PluginAction struct {
	MQClient pb.TaskQueueClient
}

//CreatePluginManager get plugin manager
func CreatePluginManager(conf option.Config) (*PluginAction, error) {
	mq := api_db.MQManager{
		Endpoint: conf.MQAPI,
	}
	mqClient, errMQ := mq.NewMQManager()
	if errMQ != nil {
		logrus.Errorf("new MQ manager failed, %v", errMQ)
		return nil, errMQ
	}
	logrus.Debugf("mqclient is %v", mqClient)
	return &PluginAction{
		MQClient: mqClient,
	}, nil
}

//CreatePluginAct PluginAct
func (p *PluginAction) CreatePluginAct(cps *api_model.CreatePluginStruct) *util.APIHandleError {
	//TODO:事务
	tx := db.GetManager().Begin()
	tp := &dbmodel.TenantPlugin{
		TenantID:    cps.Body.TenantID,
		PluginCMD:   cps.Body.PluginCMD,
		PluginID:    cps.Body.PluginID,
		PluginInfo:  cps.Body.PluginInfo,
		PluginModel: cps.Body.PluginModel,
		PluginName:  cps.Body.PluginName,
		ImageLocal:  cps.Body.ImageLocal,
		ImageURL:    cps.Body.ImageURL,
		GitURL:      cps.Body.GitURL,
		Repo:        cps.Body.Repo,
		BuildModel:  cps.Body.BuildModel,
	}
	err := db.GetManager().TenantPluginDaoTransactions(tx).AddModel(tp)
	if err != nil {
		tx.Rollback()
		return util.CreateAPIHandleErrorFromDBError("create plugin", err)
	}
	for _, env := range cps.Body.EVNInfo {
		vis := &dbmodel.TenantPluginDefaultENV{
			PluginID: cps.Body.PluginID,
			ENVName:  env.ENVName,
			ENVValue: env.ENVValue,
			IsChange: env.IsChange,
		}
		err := db.GetManager().TenantPluginDefaultENVDaoTransactions(tx).AddModel(vis)
		if err != nil {
			tx.Rollback()
			return util.CreateAPIHandleErrorFromDBError(fmt.Sprintf("add default env %s", env.ENVName), err)
		}
	}
	//添加默认plugin model env
	vis := &dbmodel.TenantPluginDefaultENV{
		PluginID: cps.Body.PluginID,
		ENVName:  "PLUGIN_MOEL",
		ENVValue: cps.Body.PluginModel,
		IsChange: false,
	}
	err = db.GetManager().TenantPluginDefaultENVDaoTransactions(tx).AddModel(vis)
	if err != nil {
		tx.Rollback()
		return util.CreateAPIHandleErrorFromDBError("add default env PLUGIN_MOEL", err)
	}
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return util.CreateAPIHandleErrorFromDBError("commit create plugin transactions", err)
	}
	return nil
}

//UpdatePluginAct UpdatePluginAct
func (p *PluginAction) UpdatePluginAct(pluginID string, cps *api_model.UpdatePluginStruct) *util.APIHandleError {
	tp, err := db.GetManager().TenantPluginDao().GetPluginByID(pluginID)
	if err != nil {
		return util.CreateAPIHandleErrorFromDBError("get old plugin info", err)
	}
	//全量更新，但pluginID和所在租户不提供修改
	tp.PluginCMD = cps.Body.PluginCMD
	tp.PluginInfo = cps.Body.PluginInfo
	tp.PluginModel = cps.Body.PluginModel
	tp.PluginName = cps.Body.PluginName
	tp.ImageLocal = cps.Body.ImageLocal
	tp.ImageURL = cps.Body.ImageURL
	tp.GitURL = cps.Body.GitURL
	tp.Repo = cps.Body.Repo
	tp.BuildModel = cps.Body.BuildModel
	err = db.GetManager().TenantPluginDao().UpdateModel(tp)
	if err != nil {
		return util.CreateAPIHandleErrorFromDBError("update plugin", err)
	}
	return nil
}

//DeletePluginAct DeletePluginAct
func (p *PluginAction) DeletePluginAct(pluginID string) *util.APIHandleError {
	//TODO: 事务
	tx := db.GetManager().Begin()
	err := db.GetManager().TenantPluginDaoTransactions(tx).DeletePluginByID(pluginID)
	if err != nil {
		tx.Rollback()
		return util.CreateAPIHandleErrorFromDBError("delete plugin", err)
	}
	err = db.GetManager().TenantPluginDefaultENVDaoTransactions(tx).DeleteAllDefaultENVByPluginID(pluginID)
	if err != nil {
		tx.Rollback()
		return util.CreateAPIHandleErrorFromDBError("delete default env", err)
	}
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return util.CreateAPIHandleErrorFromDBError("commit delete plugin transactions", err)
	}
	return nil
}

//GetPlugins 获取当前租户下所有的plugins
func (p *PluginAction) GetPlugins(tenantID string) ([]*dbmodel.TenantPlugin, *util.APIHandleError) {
	plugins, err := db.GetManager().TenantPluginDao().GetPluginsByTenantID(tenantID)
	if err != nil {
		return nil, util.CreateAPIHandleErrorFromDBError("get plugins by tenant id", err)
	}
	return plugins, nil
}

//AddDefaultEnv AddDefaultEnv
func (p *PluginAction) AddDefaultEnv(est *api_model.ENVStruct) *util.APIHandleError {
	tx := db.GetManager().Begin()
	for _, env := range est.Body.EVNInfo {
		vis := &dbmodel.TenantPluginDefaultENV{
			PluginID: est.PluginID,
			ENVName:  env.ENVName,
			ENVValue: env.ENVValue,
			IsChange: env.IsChange,
		}
		err := db.GetManager().TenantPluginDefaultENVDaoTransactions(tx).AddModel(vis)
		if err != nil {
			tx.Rollback()
			return util.CreateAPIHandleErrorFromDBError(fmt.Sprintf("add default env %s", env.ENVName), err)
		}
	}
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return util.CreateAPIHandleErrorFromDBError("commit add default env transactions", err)
	}
	return nil
}

//UpdateDefaultEnv UpdateDefaultEnv
func (p *PluginAction) UpdateDefaultEnv(est *api_model.ENVStruct) *util.APIHandleError {
	for _, env := range est.Body.EVNInfo {
		vis := &dbmodel.TenantPluginDefaultENV{
			ENVName:  env.ENVName,
			ENVValue: env.ENVValue,
			IsChange: env.IsChange,
		}
		err := db.GetManager().TenantPluginDefaultENVDao().UpdateModel(vis)
		if err != nil {
			return util.CreateAPIHandleErrorFromDBError(fmt.Sprintf("update default env %s", env.ENVName), err)
		}
	}
	return nil
}

//DeleteDefaultEnv DeleteDefaultEnv
func (p *PluginAction) DeleteDefaultEnv(pluginID, name string) *util.APIHandleError {
	if err := db.GetManager().TenantPluginDefaultENVDao().DeleteDefaultENVByName(pluginID, name); err != nil {
		return util.CreateAPIHandleErrorFromDBError(fmt.Sprintf("delete default env %s", name), err)
	}
	return nil
}

//GetDefaultEnv GetDefaultEnv
func (p *PluginAction) GetDefaultEnv(pluginID string) ([]*dbmodel.TenantPluginDefaultENV, *util.APIHandleError) {
	envs, err := db.GetManager().TenantPluginDefaultENVDao().GetDefaultENVSByPluginID(pluginID)
	if err != nil {
		return nil, util.CreateAPIHandleErrorFromDBError("get default env", err)
	}
	return envs, nil
}

//GetEnvsWhichCanBeSet GetEnvsWhichCanBeSet
func (p *PluginAction) GetEnvsWhichCanBeSet(serviceID, pluginID string) (interface{}, *util.APIHandleError) {
	envs, err := db.GetManager().TenantPluginVersionENVDao().GetVersionEnvByServiceID(serviceID, pluginID)
	if err != nil {
		return nil, util.CreateAPIHandleErrorFromDBError("get envs which can be set", err)
	}
	if len(envs) > 0 {
		return envs, nil
	}
	envD, errD := db.GetManager().TenantPluginDefaultENVDao().GetDefaultENVSByPluginIDCantBeSet(pluginID)
	if errD != nil {
		return nil, util.CreateAPIHandleErrorFromDBError("get envs which can be set", errD)
	}
	return envD, nil
}

//BuildPluginManual BuildPluginManual
func (p *PluginAction) BuildPluginManual(bps *api_model.BuildPluginStruct) (string, *util.APIHandleError) {
	eventID := bps.Body.EventID
	logger := event.GetManager().GetLogger(eventID)
	defer event.CloseManager()
	plugin, err := db.GetManager().TenantPluginDao().GetPluginByID(bps.PluginID)
	if err != nil {
		return "", util.CreateAPIHandleErrorFromDBError(fmt.Sprintf("get plugin by %v", bps.PluginID), err)
	}
	switch bps.Body.Kind {
	case "image":
		buildVersion, err := p.ImageBuildPlugin(bps, plugin)
		if err != nil {
			logger.Error("从镜像构建插件任务发送失败 "+err.Error(), map[string]string{"step": "callback", "status": "failure"})
			return "", util.CreateAPIHandleError(500, fmt.Errorf("build plugin from image error"))
		}
		logger.Info("从镜像构建插件任务发送成功 ", map[string]string{"step": "image-plugin", "status": "starting"})
		return buildVersion, nil
	case "dockerfile":
		buildVersion, err := p.DockerfileBuildPlugin(bps, plugin)
		if err != nil {
			logger.Error("从dockerfile构建插件任务发送失败 "+err.Error(), map[string]string{"step": "callback", "status": "failure"})
			return "", util.CreateAPIHandleError(500, fmt.Errorf("build plugin from dockerfile error"))
		}
		logger.Info("从dockerfile构建插件任务发送成功 ", map[string]string{"step": "dockerfile-plugin", "status": "starting"})
		return buildVersion, nil
	default:
		return "", util.CreateAPIHandleError(400, fmt.Errorf("unexpect kind"))
	}
}
func createVersionID(s []byte) string {
	h := md5.New()
	h.Write(s)
	return hex.EncodeToString(h.Sum(nil))
}

//ImageBuildPlugin ImageBuildPlugin
func (p *PluginAction) ImageBuildPlugin(b *api_model.BuildPluginStruct, plugin *dbmodel.TenantPlugin) (string, error) {
	if b.Body.ImageURL == "" {
		return "", fmt.Errorf("need image url")
	}
	if b.Body.Operator == "" {
		b.Body.Operator = "define"
	}
	diffStr := fmt.Sprintf("%s%s%s%s", b.TenantName, b.Body.ImageURL, b.PluginID, time.Now().Format(time.RFC3339))
	buildVersion := createVersionID([]byte(diffStr))
	pbv := &dbmodel.TenantPluginBuildVersion{
		VersionID: buildVersion,
		PluginID:  b.PluginID,
		Kind:      b.Body.Kind,
		BaseImage: b.Body.ImageURL,
		BuildTime: time.Now().Format(time.RFC3339),
		Info:      b.Body.Info,
		Status:    "building",
	}
	tx := db.GetManager().Begin()
	if err := db.GetManager().TenantPluginBuildVersionDaoTransactions(tx).AddModel(pbv); err != nil {
		tx.Rollback()
		return "", err
	}
	taskBody := &builder_model.BuildPluginTaskBody{
		TenantID:  b.Body.TenantID,
		PluginID:  b.PluginID,
		Operator:  b.Body.Operator,
		ImageURL:  b.Body.ImageURL,
		EventID:   b.Body.EventID,
		Kind:      b.Body.Kind,
		VersionID: buildVersion,
	}
	jtask, errJ := ffjson.Marshal(taskBody)
	if errJ != nil {
		tx.Rollback()
		logrus.Debugf("unmarshall jtask error, %v", errJ)
		return "", errJ
	}
	ts := &api_db.BuildTaskStruct{
		TaskType: "plugin_image_build",
		TaskBody: jtask,
		User:     b.Body.Operator,
	}
	eq, err := api_db.BuildTaskBuild(ts)
	if err != nil {
		logrus.Errorf("build equeue build plugin from image error, %v", err)
		tx.Rollback()
		return "", err
	}
	if _, err := p.MQClient.Enqueue(context.Background(), eq); err != nil {
		logrus.Errorf("equque mq error, %v", err)
		tx.Rollback()
		return "", err
	}
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		logrus.Debugf("commit mysql error, %v", err)
		return "", nil
	}
	logrus.Debugf("equeue mq build plugin from image success")
	return buildVersion, nil
}

//DockerfileBuildPlugin DockerfileBuildPlugin
func (p *PluginAction) DockerfileBuildPlugin(b *api_model.BuildPluginStruct, plugin *dbmodel.TenantPlugin) (string, error) {
	if b.Body.GitURL == "" || b.Body.RepoURL == "" {
		return "", fmt.Errorf("need repo url or git url")
	}
	if b.Body.Operator == "" {
		b.Body.Operator = "define"
	}
	diffStr := fmt.Sprintf("%s%s%s%s", b.TenantName, b.Body.RepoURL, b.PluginID, time.Now().Format(time.RFC3339))
	buildVersion := createVersionID([]byte(diffStr))
	pbv := &dbmodel.TenantPluginBuildVersion{
		VersionID: buildVersion,
		PluginID:  b.PluginID,
		Kind:      b.Body.Kind,
		Repo:      b.Body.RepoURL,
		GitURL:    b.Body.GitURL,
		Info:      b.Body.Info,
		BuildTime: time.Now().Format(time.RFC3339),
		Status:    "building",
	}
	tx := db.GetManager().Begin()
	if err := db.GetManager().TenantPluginBuildVersionDaoTransactions(tx).AddModel(pbv); err != nil {
		tx.Rollback()
		return "", err
	}
	taskBody := &builder_model.BuildPluginTaskBody{
		TenantID:  b.Body.TenantID,
		PluginID:  b.PluginID,
		Operator:  b.Body.Operator,
		EventID:   b.Body.EventID,
		Repo:      b.Body.RepoURL,
		GitURL:    b.Body.GitURL,
		Kind:      b.Body.Kind,
		VersionID: buildVersion,
	}
	jtask, errJ := ffjson.Marshal(taskBody)
	if errJ != nil {
		tx.Rollback()
		logrus.Debugf("unmarshall jtask error, %v", errJ)
		return "", errJ
	}
	ts := &api_db.BuildTaskStruct{
		TaskType: "plugin_dockerfile_build",
		TaskBody: jtask,
		User:     b.Body.Operator,
	}
	eq, err := api_db.BuildTaskBuild(ts)
	if err != nil {
		logrus.Errorf("build equeue build plugin from dockerfile error, %v", err)
		tx.Rollback()
		return "", err
	}
	if _, err := p.MQClient.Enqueue(context.Background(), eq); err != nil {
		logrus.Errorf("equque mq error, %v", err)
		tx.Rollback()
		return "", err
	}
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		logrus.Debugf("commit mysql error, %v", err)
		return "", nil
	}
	logrus.Debugf("equeue mq build plugin from dockerfile success")
	return buildVersion, nil
}

//GetAllPluginBuildVersions GetAllPluginBuildVersions
func (p *PluginAction) GetAllPluginBuildVersions(pluginID string) ([]*dbmodel.TenantPluginBuildVersion, *util.APIHandleError) {
	versions, err := db.GetManager().TenantPluginBuildVersionDao().GetBuildVersionByPluginID(pluginID)
	if err != nil {
		return nil, util.CreateAPIHandleErrorFromDBError("get all plugin build version", err)
	}
	return versions, nil
}

//GetPluginBuildVersion GetPluginBuildVersion
func (p *PluginAction) GetPluginBuildVersion(pluginID, versionID string) (*dbmodel.TenantPluginBuildVersion, *util.APIHandleError) {
	version, err := db.GetManager().TenantPluginBuildVersionDao().GetBuildVersionByVersionID(pluginID, versionID)
	if err != nil {
		return nil, util.CreateAPIHandleErrorFromDBError(fmt.Sprintf("get plugin build version by id %v", versionID), err)
	}
	return version, nil
}

//DeletePluginBuildVersion DeletePluginBuildVersion
func (p *PluginAction) DeletePluginBuildVersion(pluginID, versionID string) *util.APIHandleError {
	err := db.GetManager().TenantPluginBuildVersionDao().DeleteBuildVersionByVersionID(versionID)
	if err != nil {
		return util.CreateAPIHandleErrorFromDBError(fmt.Sprintf("delete plugin build version by id %v", versionID), err)
	}
	return nil
}
