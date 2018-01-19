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
	"github.com/jinzhu/gorm"
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"strings"
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
	tp := &dbmodel.TenantPlugin{
		TenantID:    cps.Body.TenantID,
		PluginID:    cps.Body.PluginID,
		PluginInfo:  cps.Body.PluginInfo,
		PluginModel: cps.Body.PluginModel,
		PluginName:  cps.Body.PluginName,
		ImageURL:    cps.Body.ImageURL,
		GitURL:      cps.Body.GitURL,
		BuildModel:  cps.Body.BuildModel,
		Domain:      cps.TenantName,
	}
	if err := db.GetManager().TenantPluginDao().AddModel(tp); err != nil {
		return util.CreateAPIHandleErrorFromDBError("create plugin", err)
	}
	return nil
}

//UpdatePluginAct UpdatePluginAct
func (p *PluginAction) UpdatePluginAct(pluginID, tenantID string, cps *api_model.UpdatePluginStruct) *util.APIHandleError {
	tp, err := db.GetManager().TenantPluginDao().GetPluginByID(pluginID, tenantID)
	if err != nil {
		return util.CreateAPIHandleErrorFromDBError("get old plugin info", err)
	}
	//全量更新，但pluginID和所在租户不提供修改
	//TODO: 是否允许修改pluginModel,会影响该插件的性质
	tp.PluginInfo = cps.Body.PluginInfo
	tp.PluginModel = cps.Body.PluginModel
	tp.PluginName = cps.Body.PluginName
	tp.ImageURL = cps.Body.ImageURL
	tp.GitURL = cps.Body.GitURL
	tp.BuildModel = cps.Body.BuildModel
	err = db.GetManager().TenantPluginDao().UpdateModel(tp)
	if err != nil {
		return util.CreateAPIHandleErrorFromDBError("update plugin", err)
	}
	return nil
}

//DeletePluginAct DeletePluginAct
func (p *PluginAction) DeletePluginAct(pluginID,tenantID string) *util.APIHandleError {
	//TODO: 事务
	tx := db.GetManager().Begin()
	err := db.GetManager().TenantPluginDaoTransactions(tx).DeletePluginByID(pluginID, tenantID)
	if err != nil {
		tx.Rollback()
		return util.CreateAPIHandleErrorFromDBError("delete plugin", err)
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
			PluginID:  est.PluginID,
			ENVName:   env.ENVName,
			ENVValue:  env.ENVValue,
			IsChange:  env.IsChange,
			VersionID: env.VersionID,
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
			ENVName:   env.ENVName,
			ENVValue:  env.ENVValue,
			IsChange:  env.IsChange,
			VersionID: env.VersionID,
		}
		err := db.GetManager().TenantPluginDefaultENVDao().UpdateModel(vis)
		if err != nil {
			return util.CreateAPIHandleErrorFromDBError(fmt.Sprintf("update default env %s", env.ENVName), err)
		}
	}
	return nil
}

//DeleteDefaultEnv DeleteDefaultEnv
func (p *PluginAction) DeleteDefaultEnv(pluginID, versionID, name string) *util.APIHandleError {
	if err := db.GetManager().TenantPluginDefaultENVDao().DeleteDefaultENVByName(pluginID, name, versionID); err != nil {
		return util.CreateAPIHandleErrorFromDBError(fmt.Sprintf("delete default env %s", name), err)
	}
	return nil
}

//GetDefaultEnv GetDefaultEnv
func (p *PluginAction) GetDefaultEnv(pluginID, versionID string) ([]*dbmodel.TenantPluginDefaultENV, *util.APIHandleError) {
	envs, err := db.GetManager().TenantPluginDefaultENVDao().GetDefaultENVSByPluginID(pluginID, versionID)
	if err != nil {
		return nil, util.CreateAPIHandleErrorFromDBError("get default env", err)
	}
	return envs, nil
}

//GetEnvsWhichCanBeSet GetEnvsWhichCanBeSet
func (p *PluginAction) GetEnvsWhichCanBeSet(serviceID, pluginID string) (interface{}, *util.APIHandleError) {
	relation, err := db.GetManager().TenantServicePluginRelationDao().GetRelateionByServiceIDAndPluginID(serviceID, pluginID)
	if err != nil {
		return nil, util.CreateAPIHandleErrorFromDBError("get relation", err)
	}
	envs, err := db.GetManager().TenantPluginVersionENVDao().GetVersionEnvByServiceID(serviceID, pluginID)
	if err != nil {
		return nil, util.CreateAPIHandleErrorFromDBError("get envs which can be set", err)
	}
	if len(envs) > 0 {
		return envs, nil
	}
	envD, errD := db.GetManager().TenantPluginDefaultENVDao().GetDefaultEnvWhichCanBeSetByPluginID(pluginID, relation.VersionID)
	if errD != nil {
		return nil, util.CreateAPIHandleErrorFromDBError("get envs which can be set", errD)
	}
	return envD, nil
}

//BuildPluginManual BuildPluginManual
func (p *PluginAction) BuildPluginManual(bps *api_model.BuildPluginStruct) (*dbmodel.TenantPluginBuildVersion, *util.APIHandleError) {
	//TODO: 生成event_id
	eventID := bps.Body.EventID
	logger := event.GetManager().GetLogger(eventID)
	defer event.CloseManager()
	plugin, err := db.GetManager().TenantPluginDao().GetPluginByID(bps.PluginID, bps.Body.TenantID)
	if err != nil {
		return nil, util.CreateAPIHandleErrorFromDBError(fmt.Sprintf("get plugin by %v", bps.PluginID), err)
	}
	if bps.Body.PluginFrom != "" {
		switch bps.Body.PluginFrom{
		case "yb":
			pbv, err := p.InstallPluginFromYB(bps, plugin)
			if err != nil {
				logrus.Debugf("install plugin from yb error %s", err.Error())
				return nil, util.CreateAPIHandleError(500, fmt.Errorf("install plugin from yb error"))
			}
			return pbv, nil
		case "ys":
		default:
			return nil, util.CreateAPIHandleError(400, fmt.Errorf("unexpect plugin from"))
		}
	}
	switch plugin.BuildModel {
	case "image":
		pbv, err := p.ImageBuildPlugin(bps, plugin)
		if err != nil {
			logger.Error("从镜像构建插件任务发送失败 "+err.Error(), map[string]string{"step": "callback", "status": "failure"})
			return nil, util.CreateAPIHandleError(500, fmt.Errorf("build plugin from image error"))
		}
		logger.Info("从镜像构建插件任务发送成功 ", map[string]string{"step": "image-plugin", "status": "starting"})
		return pbv, nil
	case "dockerfile":
		pbv, err := p.DockerfileBuildPlugin(bps, plugin)
		if err != nil {
			logger.Error("从dockerfile构建插件任务发送失败 "+err.Error(), map[string]string{"step": "callback", "status": "failure"})
			return nil, util.CreateAPIHandleError(500, fmt.Errorf("build plugin from dockerfile error"))
		}
		logger.Info("从dockerfile构建插件任务发送成功 ", map[string]string{"step": "dockerfile-plugin", "status": "starting"})
		return pbv, nil
	default:
		return nil, util.CreateAPIHandleError(400, fmt.Errorf("unexpect kind"))
	}
}
func createVersionID(s []byte) string {
	h := md5.New()
	h.Write(s)
	return hex.EncodeToString(h.Sum(nil))
}

//InstallPluginFromYB InstallPluginFromYB
func (p *PluginAction) InstallPluginFromYB(b *api_model.BuildPluginStruct, plugin *dbmodel.TenantPlugin)(
	*dbmodel.TenantPluginBuildVersion, error) {
	if b.Body.Operator == "" {
		b.Body.Operator = "define"
	}
	pbv := &dbmodel.TenantPluginBuildVersion{
		VersionID:       b.Body.BuildVersion,
		PluginID:        b.PluginID,
		Kind:            plugin.BuildModel,
		BaseImage:       plugin.ImageURL,
		BuildLocalImage: b.Body.BuildImage,
		ContainerCPU:    b.Body.PluginCPU,
		ContainerMemory: b.Body.PluginMemory,
		ContainerCMD:    b.Body.PluginCMD,
		BuildTime:       time.Now().Format(time.RFC3339),
		Info:            b.Body.Info,
		Status:          "complete",
	}	
	if err := db.GetManager().TenantPluginBuildVersionDao().AddModel(pbv); err != nil {
		if !strings.Contains(err.Error(), "exist") {
			logrus.Errorf("build plugin error: %s", err.Error())
			return nil, err
		}
	}	
	return pbv, nil
}

//ImageBuildPlugin ImageBuildPlugin
func (p *PluginAction) ImageBuildPlugin(b *api_model.BuildPluginStruct, plugin *dbmodel.TenantPlugin) (
	*dbmodel.TenantPluginBuildVersion, error) {
	if plugin.ImageURL == "" {
		return nil, fmt.Errorf("need image url")
	}
	if b.Body.Operator == "" {
		b.Body.Operator = "define"
	}
	//TODO: build_version create in console
	//diffStr := fmt.Sprintf("%s%s%s%s", b.TenantName, plugin.ImageURL, b.PluginID, time.Now().Format(time.RFC3339))
	//buildVersion := createVersionID([]byte(diffStr))
	rebuild := false
	tpbv, err := db.GetManager().TenantPluginBuildVersionDao().GetBuildVersionByVersionID(
		b.PluginID, b.Body.BuildVersion)
	if err != nil {
		if err.Error() == gorm.ErrRecordNotFound.Error(){
			rebuild = false
		}else{
			return nil, err
		}
	}else{
		rebuild = true
	}
	tx := db.GetManager().Begin()
	if rebuild {
		tpbv.Info = b.Body.Info
		tpbv.Status = "building"
		tpbv.BuildTime = time.Now().Format(time.RFC3339)
		if b.Body.PluginCPU == 0 {
			tpbv.ContainerCPU = 125
		}
		if b.Body.PluginMemory == 0 {
			tpbv.ContainerMemory = 50
		}	
		if err := db.GetManager().TenantPluginBuildVersionDaoTransactions(tx).UpdateModel(tpbv); err != nil {
			if err != nil {
				tx.Rollback()
				logrus.Errorf("build plugin error: %s", err.Error())
				return nil, err
			}
		}		
	}else {
		pbv := &dbmodel.TenantPluginBuildVersion{
			VersionID:       b.Body.BuildVersion,
			PluginID:        b.PluginID,
			Kind:            plugin.BuildModel,
			BaseImage:       plugin.ImageURL,
			ContainerCPU:    b.Body.PluginCPU,
			ContainerMemory: b.Body.PluginMemory,
			ContainerCMD:    b.Body.PluginCMD,
			BuildTime:       time.Now().Format(time.RFC3339),
			Info:            b.Body.Info,
			Status:          "building",
		}
		if b.Body.PluginCPU == 0 {
			pbv.ContainerCPU = 125
		}
		if b.Body.PluginMemory == 0 {
			pbv.ContainerMemory = 50
		}
		if err := db.GetManager().TenantPluginBuildVersionDaoTransactions(tx).AddModel(pbv); err != nil {
			if !strings.Contains(err.Error(), "exist") {
				tx.Rollback()
				logrus.Errorf("build plugin error: %s", err.Error())
				return nil, err
			}
		}
		tpbv = pbv
	}
	taskBody := &builder_model.BuildPluginTaskBody{
		TenantID:     b.Body.TenantID,
		PluginID:     b.PluginID,
		Operator:     b.Body.Operator,
		ImageURL:     plugin.ImageURL,
		EventID:      b.Body.EventID,
		Kind:         plugin.BuildModel,
		PluginCMD:    b.Body.PluginCMD,
		PluginCPU:    b.Body.PluginCPU,
		PluginMemory: b.Body.PluginMemory,
		VersionID:    b.Body.BuildVersion,
	}
	jtask, errJ := ffjson.Marshal(taskBody)
	if errJ != nil {
		tx.Rollback()
		logrus.Debugf("unmarshall jtask error, %v", errJ)
		return nil, errJ
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
		return nil, err
	}
	if _, err := p.MQClient.Enqueue(context.Background(), eq); err != nil {
		logrus.Errorf("equque mq error, %v", err)
		tx.Rollback()
		return nil, err
	}
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		logrus.Debugf("commit mysql error, %v", err)
		return nil, nil
	}
	logrus.Debugf("equeue mq build plugin from image success")
	return tpbv, nil
}

//DockerfileBuildPlugin DockerfileBuildPlugin
func (p *PluginAction) DockerfileBuildPlugin(b *api_model.BuildPluginStruct, plugin *dbmodel.TenantPlugin) (
	*dbmodel.TenantPluginBuildVersion, error) {
	if plugin.GitURL == "" {
		return nil, fmt.Errorf("need git url")
	}
	if b.Body.RepoURL == "" {
		b.Body.RepoURL = "master"
	}
	if b.Body.Operator == "" {
		b.Body.Operator = "define"
	}
	// TODO: build_version create in console
	// diffStr := fmt.Sprintf("%s%s%s%s", b.TenantName, b.Body.RepoURL, b.PluginID, time.Now().Format(time.RFC3339))
	// buildVersion := createVersionID([]byte(diffStr))
	rebuild := false
	tpbv, err := db.GetManager().TenantPluginBuildVersionDao().GetBuildVersionByVersionID(
		b.PluginID, b.Body.BuildVersion)
	if err != nil {
		if err.Error() == gorm.ErrRecordNotFound.Error(){
			rebuild = false
		}else{
			return nil, err
		}
	}else{
		rebuild = true
	}
	tx := db.GetManager().Begin()
	if rebuild {
		tpbv.Info = b.Body.Info
		tpbv.Status = "building"
		tpbv.BuildTime = time.Now().Format(time.RFC3339)
		if b.Body.PluginCPU == 0 {
			tpbv.ContainerCPU = 125
		}
		if b.Body.PluginMemory == 0 {
			tpbv.ContainerMemory = 50
		}	
		if err := db.GetManager().TenantPluginBuildVersionDaoTransactions(tx).UpdateModel(tpbv); err != nil {
			if err != nil {
				tx.Rollback()
				logrus.Errorf("build plugin error: %s", err.Error())
				return nil, err
			}
		}	
	}else{
		pbv := &dbmodel.TenantPluginBuildVersion{
			VersionID:       b.Body.BuildVersion,
			PluginID:        b.PluginID,
			Kind:            plugin.BuildModel,
			Repo:            b.Body.RepoURL,
			GitURL:          plugin.GitURL,
			Info:            b.Body.Info,
			ContainerCPU:    b.Body.PluginCPU,
			ContainerMemory: b.Body.PluginMemory,
			ContainerCMD:    b.Body.PluginCMD,
			BuildTime:       time.Now().Format(time.RFC3339),
			Status:          "building",
		}
		if b.Body.PluginCPU == 0 {
			pbv.ContainerCPU = 125
		}
		if b.Body.PluginMemory == 0 {
			pbv.ContainerMemory = 50
		}
		if err := db.GetManager().TenantPluginBuildVersionDaoTransactions(tx).AddModel(pbv); err != nil {
			if !strings.Contains(err.Error(), "exist") {
				tx.Rollback()
				logrus.Errorf("build plugin error: %s", err.Error())
				return nil, err
			}
		}
		tpbv = pbv
	}
	taskBody := &builder_model.BuildPluginTaskBody{
		TenantID:     b.Body.TenantID,
		PluginID:     b.PluginID,
		Operator:     b.Body.Operator,
		EventID:      b.Body.EventID,
		Repo:         b.Body.RepoURL,
		GitURL:       plugin.GitURL,
		Kind:         plugin.BuildModel,
		VersionID:    b.Body.BuildVersion,
		PluginCMD:    b.Body.PluginCMD,
		PluginCPU:    b.Body.PluginCPU,
		PluginMemory: b.Body.PluginMemory,
	}
	jtask, errJ := ffjson.Marshal(taskBody)
	if errJ != nil {
		tx.Rollback()
		logrus.Debugf("unmarshall jtask error, %v", errJ)
		return nil, errJ
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
		return nil, err
	}
	if _, err := p.MQClient.Enqueue(context.Background(), eq); err != nil {
		logrus.Errorf("equque mq error, %v", err)
		tx.Rollback()
		return nil, err
	}
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		logrus.Debugf("commit mysql error, %v", err)
		return nil, nil
	}
	logrus.Debugf("equeue mq build plugin from dockerfile success")
	return tpbv, nil
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

	tx := db.GetManager().Begin()
	err := db.GetManager().TenantPluginBuildVersionDaoTransactions(tx).DeleteBuildVersionByVersionID(versionID)
	if err != nil {
		return util.CreateAPIHandleErrorFromDBError(fmt.Sprintf("delete plugin build version by id %v", versionID), err)
	}
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return util.CreateAPIHandleErrorFromDBError("commit delete plugin transactions", err)
	}
	return nil
}
