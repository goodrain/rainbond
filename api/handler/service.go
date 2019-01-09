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

package handler

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/goodrain/rainbond/cmd/api/option"
	"github.com/goodrain/rainbond/worker/server/pb"

	"github.com/coreos/etcd/clientv3"
	"github.com/goodrain/rainbond/db"
	core_model "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/event"
	"github.com/twinj/uuid"

	"github.com/jinzhu/gorm"

	"github.com/pquerna/ffjson/ffjson"

	api_model "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/api/util"
	dbmodel "github.com/goodrain/rainbond/db/model"
	gclient "github.com/goodrain/rainbond/mq/client"
	core_util "github.com/goodrain/rainbond/util"
	"github.com/goodrain/rainbond/worker/client"
	"github.com/goodrain/rainbond/worker/discover/model"

	"encoding/json"
	"net/http"

	"github.com/Sirupsen/logrus"
)

//ServiceAction service act
type ServiceAction struct {
	MQClient  gclient.MQClient
	EtcdCli   *clientv3.Client
	statusCli *client.AppRuntimeSyncClient
	conf      option.Config
}

//CreateManager create Manger
func CreateManager(conf option.Config, mqClient gclient.MQClient,
	etcdCli *clientv3.Client, statusCli *client.AppRuntimeSyncClient) *ServiceAction {
	return &ServiceAction{
		MQClient:  mqClient,
		EtcdCli:   etcdCli,
		statusCli: statusCli,
		conf:      conf,
	}
}

//ServiceBuild service build
func (s *ServiceAction) ServiceBuild(tenantID, serviceID string, r *api_model.BuildServiceStruct) error {
	eventID := r.Body.EventID
	logger := event.GetManager().GetLogger(eventID)
	defer event.CloseManager()
	service, err := db.GetManager().TenantServiceDao().GetServiceByID(serviceID)
	db.GetManager().TenantServiceDao().UpdateModel(service)
	if err != nil {
		return err
	}
	if r.Body.Kind == "" {
		r.Body.Kind = "source"
	}
	switch r.Body.Kind {
	case "build_from_image":
		if err := s.buildFromImage(r, service); err != nil {
			logger.Error("The image build application task failed to send: "+err.Error(), map[string]string{"step": "callback", "status": "failure"})
			return err
		}
		logger.Info("The mirror build application task successed to send ", map[string]string{"step": "image-service", "status": "starting"})
		return nil
	case "build_from_source_code":
		if err := s.buildFromSourceCode(r, service); err != nil {
			logger.Error("The source code build application task failed to send "+err.Error(), map[string]string{"step": "callback", "status": "failure"})
			return err
		}
		logger.Info("The source code build application task successed to send ", map[string]string{"step": "source-service", "status": "starting"})
		return nil
	case "build_from_market_image":
		if err := s.buildFromImage(r, service); err != nil {
			logger.Error("The cloud image build application task failed to send "+err.Error(), map[string]string{"step": "callback", "status": "failure"})
			return err
		}
		logger.Info("The cloud image build application task successed to send ", map[string]string{"step": "image-service", "status": "starting"})
		return nil
	case "build_from_market_slug":
		if err := s.buildFromMarketSlug(r, service); err != nil {
			logger.Error("The cloud slug build application task failed to send "+err.Error(), map[string]string{"step": "callback", "status": "failure"})
			return err
		}
		logger.Info("The cloud slug build application task successed to send ", map[string]string{"step": "image-service", "status": "starting"})
		return nil
	default:
		return fmt.Errorf("unexpect kind")
	}
}
func (s *ServiceAction) buildFromMarketSlug(r *api_model.BuildServiceStruct, service *dbmodel.TenantServices) error {
	body := make(map[string]interface{})
	if r.Body.Operator == "" {
		body["operator"] = "define"
	} else {
		body["operator"] = r.Body.Operator
	}
	body["deploy_version"] = r.Body.DeployVersion
	body["event_id"] = r.Body.EventID
	body["action"] = r.Body.Action
	body["tenant_name"] = r.Body.TenantName
	body["tenant_id"] = service.TenantID
	body["service_id"] = service.ServiceID
	body["service_alias"] = r.Body.ServiceAlias
	body["slug_info"] = r.Body.SlugInfo

	topic := gclient.BuilderTopic
	if s.isWindowsService(service.ServiceID) {
		topic = gclient.WindowsBuilderTopic
	}
	return s.MQClient.SendBuilderTopic(gclient.TaskStruct{
		Topic:    topic,
		TaskType: "build_from_market_slug",
		TaskBody: body,
	})
}

func (s *ServiceAction) buildFromImage(r *api_model.BuildServiceStruct, service *dbmodel.TenantServices) error {
	dependIds, err := db.GetManager().TenantServiceRelationDao().GetTenantServiceRelations(service.ServiceID)
	if err != nil {
		return err
	}
	body := make(map[string]interface{})
	if r.Body.Operator == "" {
		body["operator"] = "define"
	} else {
		body["operator"] = r.Body.Operator
	}
	body["image"] = r.Body.ImageURL
	body["service_id"] = service.ServiceID
	body["deploy_version"] = r.Body.DeployVersion
	body["namespace"] = service.Namespace
	body["operator"] = r.Body.Operator
	body["event_id"] = r.Body.EventID
	body["tenant_name"] = r.Body.TenantName
	body["service_alias"] = r.Body.ServiceAlias
	body["action"] = r.Body.Action
	body["dep_sids"] = dependIds
	body["code_from"] = "image_manual"
	if r.Body.User != "" && r.Body.Password != "" {
		body["user"] = r.Body.User
		body["password"] = r.Body.Password
	}
	topic := gclient.BuilderTopic
	if s.isWindowsService(service.ServiceID) {
		topic = gclient.WindowsBuilderTopic
	}
	return s.MQClient.SendBuilderTopic(gclient.TaskStruct{
		Topic:    topic,
		TaskType: "build_from_image",
		TaskBody: body,
	})
}

func (s *ServiceAction) buildFromSourceCode(r *api_model.BuildServiceStruct, service *dbmodel.TenantServices) error {
	logrus.Debugf("build_from_source_code")
	if r.Body.RepoURL == "" || r.Body.Branch == "" || r.Body.DeployVersion == "" || r.Body.EventID == "" {
		return fmt.Errorf("args error")
	}
	body := make(map[string]interface{})
	if r.Body.Operator == "" {
		body["operator"] = "define"
	} else {
		body["operator"] = r.Body.Operator
	}
	body["tenant_id"] = service.TenantID
	body["service_id"] = service.ServiceID
	body["repo_url"] = r.Body.RepoURL
	body["action"] = r.Body.Action
	body["lang"] = r.Body.Lang
	body["runtime"] = r.Body.Runtime
	body["deploy_version"] = r.Body.DeployVersion
	body["event_id"] = r.Body.EventID
	body["envs"] = r.Body.ENVS
	body["tenant_name"] = r.Body.TenantName
	body["branch"] = r.Body.Branch
	body["server_type"] = r.Body.ServerType
	body["service_alias"] = r.Body.ServiceAlias
	if r.Body.User != "" && r.Body.Password != "" {
		body["user"] = r.Body.User
		body["password"] = r.Body.Password
	}
	body["expire"] = 180
	topic := gclient.BuilderTopic
	if s.isWindowsService(service.ServiceID) {
		topic = gclient.WindowsBuilderTopic
	}
	return s.MQClient.SendBuilderTopic(gclient.TaskStruct{
		Topic:    topic,
		TaskType: "build_from_source_code",
		TaskBody: body,
	})
}

func (s *ServiceAction) isWindowsService(serviceID string) bool {
	label, err := db.GetManager().TenantServiceLabelDao().GetLabelByNodeSelectorKey(serviceID, "windows")
	if label == nil || err != nil {
		return false
	}
	return true
}

//AddLabel add labels
func (s *ServiceAction) AddLabel(l *api_model.LabelsStruct, serviceID string) error {
	tx := db.GetManager().Begin()
	for _, label := range l.Labels {
		var labelModel dbmodel.TenantServiceLable
		switch label.LabelKey {
		case core_model.LabelKeyServiceType:
			labelModel.ServiceID = serviceID
			labelModel.LabelKey = core_model.LabelKeyServiceType
			labelModel.LabelValue = chekeServiceLabel(label.LabelValue)
		default:
			labelModel.ServiceID = serviceID
			labelModel.LabelKey = label.LabelKey
			labelModel.LabelValue = label.LabelValue
		}
		if err := db.GetManager().TenantServiceLabelDaoTransactions(tx).AddModel(&labelModel); err != nil {
			tx.Rollback()
			return err
		}
	}
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return err
	}
	return nil
}

//UpdateLabel updates labels
func (s *ServiceAction) UpdateLabel(l *api_model.LabelsStruct, serviceID string) error {
	tx := db.GetManager().Begin()
	for _, label := range l.Labels {
		// delete old labels
		err := db.GetManager().TenantServiceLabelDaoTransactions(tx).
			DelTenantServiceLabelsByServiceIDKey(serviceID, label.LabelKey)
		if err != nil {
			logrus.Errorf("error deleting old labels: %v", err)
			tx.Rollback()
			return err
		}
		// add new labels
		var labelModel dbmodel.TenantServiceLable
		switch label.LabelKey {
		case core_model.LabelKeyServiceType:
			labelModel.ServiceID = serviceID
			labelModel.LabelKey = core_model.LabelKeyServiceType
			labelModel.LabelValue = chekeServiceLabel(label.LabelValue)
		default:
			labelModel.ServiceID = serviceID
			labelModel.LabelKey = label.LabelKey
			labelModel.LabelValue = label.LabelValue
		}
		if err := db.GetManager().TenantServiceLabelDaoTransactions(tx).AddModel(&labelModel); err != nil {
			logrus.Errorf("error adding new labels: %v", err)
			tx.Rollback()
			return err

		}
	}
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return err
	}
	return nil
}

//DeleteLabel deletes label
func (s *ServiceAction) DeleteLabel(l *api_model.LabelsStruct, serviceID string) error {
	tx := db.GetManager().Begin()
	for _, label := range l.Labels {
		err := db.GetManager().TenantServiceLabelDaoTransactions(tx).
			DelTenantServiceLabelsByServiceIDKeyValue(serviceID, label.LabelKey, label.LabelValue)
		if err != nil {
			logrus.Errorf("error deleting label: %v", err)
			tx.Rollback()
			return err
		}
	}
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return err
	}
	return nil
}

//UpdateServiceLabel UpdateLabel
func (s *ServiceAction) UpdateServiceLabel(serviceID, value string) error {
	sls, err := db.GetManager().TenantServiceLabelDao().GetTenantServiceLabel(serviceID)
	if err != nil {
		return err
	}
	if len(sls) > 0 {
		for _, sl := range sls {
			sl.ServiceID = serviceID
			sl.LabelKey = core_model.LabelKeyServiceType
			value = chekeServiceLabel(value)
			sl.LabelValue = value
			return db.GetManager().TenantServiceLabelDao().UpdateModel(sl)
		}
	}
	return fmt.Errorf("Get tenant service label error")
}

//StartStopService start service
func (s *ServiceAction) StartStopService(sss *api_model.StartStopStruct) error {
	services, err := db.GetManager().TenantServiceDao().GetServiceByID(sss.ServiceID)
	if err != nil {
		logrus.Errorf("get service by id error, %v", err)
		return err
	}
	TaskBody := model.StopTaskBody{
		TenantID:      sss.TenantID,
		ServiceID:     sss.ServiceID,
		DeployVersion: services.DeployVersion,
		EventID:       sss.EventID,
	}
	err = s.MQClient.SendBuilderTopic(gclient.TaskStruct{
		TaskType: sss.TaskType,
		TaskBody: TaskBody,
		Topic:    gclient.WorkerTopic,
	})
	if err != nil {
		logrus.Errorf("equque mq error, %v", err)
		return err
	}
	logrus.Debugf("equeue mq startstop task success")
	return nil
}

//ServiceVertical vertical service
func (s *ServiceAction) ServiceVertical(vs *model.VerticalScalingTaskBody) error {
	service, err := db.GetManager().TenantServiceDao().GetServiceByID(vs.ServiceID)
	if err != nil {
		logrus.Errorf("get service by id %s error, %s", service.ServiceID, err)
		return err
	}
	if service.ContainerMemory == vs.ContainerMemory && service.ContainerCPU == vs.ContainerCPU {
		return nil
	}
	service.ContainerMemory = vs.ContainerMemory
	service.ContainerCPU = vs.ContainerCPU
	err = db.GetManager().TenantServiceDao().UpdateModel(service)
	if err != nil {
		logrus.Errorf("update service memory and cpu failure. %v", err)
		return fmt.Errorf("Vertical service faliure:%s", err.Error())
	}
	err = s.MQClient.SendBuilderTopic(gclient.TaskStruct{
		TaskType: "vertical_scaling",
		TaskBody: vs,
		Topic:    gclient.WorkerTopic,
	})
	if err != nil {
		logrus.Errorf("equque mq error, %v", err)
		return err
	}
	logrus.Debugf("equeue mq vertical task success")
	return nil
}

//ServiceHorizontal Service Horizontal
func (s *ServiceAction) ServiceHorizontal(hs *model.HorizontalScalingTaskBody) error {
	service, err := db.GetManager().TenantServiceDao().GetServiceByID(hs.ServiceID)
	if err != nil {
		logrus.Errorf("get service by id %s error, %s", service.ServiceID, err)
		return err
	}
	if int32(service.Replicas) == hs.Replicas {
		return nil
	}
	service.Replicas = int(hs.Replicas)
	err = db.GetManager().TenantServiceDao().UpdateModel(service)
	if err != nil {
		logrus.Errorf("updtae service replicas failure. %v", err)
		return fmt.Errorf("horizontal service faliure:%s", err.Error())
	}
	err = s.MQClient.SendBuilderTopic(gclient.TaskStruct{
		TaskType: "horizontal_scaling",
		TaskBody: hs,
		Topic:    gclient.WorkerTopic,
	})
	if err != nil {
		logrus.Errorf("equque mq error, %v", err)
		return err
	}
	logrus.Debugf("equeue mq horizontal task success")
	return nil
}

//ServiceUpgrade service upgrade
func (s *ServiceAction) ServiceUpgrade(ru *model.RollingUpgradeTaskBody) error {
	services, err := db.GetManager().TenantServiceDao().GetServiceByID(ru.ServiceID)
	if err != nil {
		logrus.Errorf("get service by id %s error %s", ru.ServiceID, err.Error())
		return err
	}
	version, err := db.GetManager().VersionInfoDao().GetVersionByDeployVersion(ru.NewDeployVersion, ru.ServiceID)
	if err != nil {
		logrus.Errorf("get service version by id %s version %s error, %s", ru.ServiceID, ru.NewDeployVersion, err.Error())
		return err
	}
	if version.FinalStatus != "success" {
		return fmt.Errorf("deploy version is not build success,do not upgrade")
	}
	services.DeployVersion = ru.NewDeployVersion
	err = db.GetManager().TenantServiceDao().UpdateModel(services)
	if err != nil {
		logrus.Errorf("update service deploy version error. %v", err)
		return fmt.Errorf("horizontal service faliure:%s", err.Error())
	}
	err = s.MQClient.SendBuilderTopic(gclient.TaskStruct{
		TaskBody: ru,
		TaskType: "rolling_upgrade",
		Topic:    gclient.WorkerTopic,
	})
	if err != nil {
		logrus.Errorf("equque upgrade message error, %v", err)
		return err
	}
	return nil
}

//ServiceCreate create service
func (s *ServiceAction) ServiceCreate(sc *api_model.ServiceStruct) error {
	jsonSC, err := ffjson.Marshal(sc)
	if err != nil {
		logrus.Errorf("trans service struct to json failed. %v", err)
		return err
	}
	var ts dbmodel.TenantServices
	if err := ffjson.Unmarshal(jsonSC, &ts); err != nil {
		logrus.Errorf("trans json to tenant service error, %v", err)
		return err
	}
	if ts.ServiceName == "" {
		ts.ServiceName = ts.ServiceAlias
	}
	ts.UpdateTime = time.Now()
	ports := sc.PortsInfo
	envs := sc.EnvsInfo
	volumns := sc.VolumesInfo
	dependVolumes := sc.DepVolumesInfo
	dependIds := sc.DependIDs
	ts.DeployVersion = ""
	tx := db.GetManager().Begin()
	//create app
	if err := db.GetManager().TenantServiceDaoTransactions(tx).AddModel(&ts); err != nil {
		logrus.Errorf("add service error, %v", err)
		tx.Rollback()
		return err
	}
	//set app envs
	if len(envs) > 0 {
		for _, env := range envs {
			env.ServiceID = ts.ServiceID
			env.TenantID = ts.TenantID
			if err := db.GetManager().TenantServiceEnvVarDaoTransactions(tx).AddModel(&env); err != nil {
				logrus.Errorf("add env %v error, %v", env.AttrName, err)
				tx.Rollback()
				return err
			}
		}
	}
	//set app port
	if len(ports) > 0 {
		for _, port := range ports {
			port.ServiceID = ts.ServiceID
			port.TenantID = ts.TenantID
			if err := db.GetManager().TenantServicesPortDaoTransactions(tx).AddModel(&port); err != nil {
				logrus.Errorf("add port %v error, %v", port.ContainerPort, err)
				tx.Rollback()
				return err
			}
		}
	}
	//set app volumns
	if len(volumns) > 0 {
		localPath := os.Getenv("LOCAL_DATA_PATH")
		sharePath := os.Getenv("SHARE_DATA_PATH")
		if localPath == "" {
			localPath = "/grlocaldata"
		}
		if sharePath == "" {
			sharePath = "/grdata"
		}

		for _, volumn := range volumns {
			volumn.ServiceID = ts.ServiceID
			if volumn.VolumeType == "" {
				volumn.VolumeType = dbmodel.ShareFileVolumeType.String()
			}
			if volumn.HostPath == "" {
				//step 1 设置主机目录
				switch volumn.VolumeType {
				//共享文件存储
				case dbmodel.ShareFileVolumeType.String():
					volumn.HostPath = fmt.Sprintf("%s/tenant/%s/service/%s%s", sharePath, sc.TenantID, volumn.ServiceID, volumn.VolumePath)
				//本地文件存储
				case dbmodel.LocalVolumeType.String():
					if sc.ExtendMethod != "state" {
						tx.Rollback()
						return util.CreateAPIHandleError(400, fmt.Errorf("应用类型不为有状态应用.不支持本地存储"))
					}
					volumn.HostPath = fmt.Sprintf("%s/tenant/%s/service/%s%s", localPath, sc.TenantID, volumn.ServiceID, volumn.VolumePath)
				}
			}
			if volumn.VolumeName == "" {
				volumn.VolumeName = uuid.NewV4().String()
			}
			if err := db.GetManager().TenantServiceVolumeDaoTransactions(tx).AddModel(&volumn); err != nil {
				logrus.Errorf("add volumn %v error, %v", volumn.HostPath, err)
				tx.Rollback()
				return err
			}
		}
	}
	//set app dependVolumes
	if len(dependVolumes) > 0 {
		for _, depVolume := range dependVolumes {
			depVolume.ServiceID = ts.ServiceID
			depVolume.TenantID = ts.TenantID
			volume, err := db.GetManager().TenantServiceVolumeDao().GetVolumeByServiceIDAndName(depVolume.DependServiceID, depVolume.VolumeName)
			if err != nil {
				tx.Rollback()
				return fmt.Errorf("find volume %s error %s", depVolume.VolumeName, err.Error())
			}
			depVolume.HostPath = volume.HostPath
			if err := db.GetManager().TenantServiceMountRelationDaoTransactions(tx).AddModel(&depVolume); err != nil {
				tx.Rollback()
				return fmt.Errorf("add dep volume %s error %s", depVolume.VolumeName, err.Error())
			}
		}
	}
	//set app depends
	if len(dependIds) > 0 {
		for _, id := range dependIds {
			if err := db.GetManager().TenantServiceRelationDaoTransactions(tx).AddModel(&id); err != nil {
				logrus.Errorf("add depend_id %v error, %v", id.DependServiceID, err)
				tx.Rollback()
				return err
			}
		}
	}
	//set app label
	if err := db.GetManager().TenantServiceLabelDaoTransactions(tx).AddModel(&dbmodel.TenantServiceLable{
		ServiceID:  ts.ServiceID,
		LabelKey:   core_model.LabelKeyServiceType,
		LabelValue: sc.ServiceLabel,
	}); err != nil {
		logrus.Errorf("add label %v error, %v", ts.ServiceID, err)
		tx.Rollback()
		return err
	}
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return err
	}
	logrus.Debugf("create a new app %s success", ts.ServiceAlias)
	return nil
}

//ServiceUpdate update service
func (s *ServiceAction) ServiceUpdate(sc map[string]interface{}) error {
	ts, err := db.GetManager().TenantServiceDao().GetServiceByID(sc["service_id"].(string))
	if err != nil {
		return err
	}
	version, err := db.GetManager().VersionInfoDao().GetVersionByDeployVersion(ts.DeployVersion, ts.ServiceID)
	if sc["container_memory"] != nil {
		ts.ContainerMemory = sc["container_memory"].(int)
	}
	if sc["container_cmd"] != nil {
		version.Cmd = sc["container_cmd"].(string)
	}
	if sc["service_name"] != nil {
		ts.ServiceName = sc["service_name"].(string)
	}
	//update service
	if err := db.GetManager().TenantServiceDao().UpdateModel(ts); err != nil {
		logrus.Errorf("update service error, %v", err)
		return err
	}
	//update service version
	if err := db.GetManager().VersionInfoDao().UpdateModel(version); err != nil {
		logrus.Errorf("update version error, %v", err)
		return err
	}
	return nil
}

//LanguageSet language set
func (s *ServiceAction) LanguageSet(langS *api_model.LanguageSet) error {
	logrus.Debugf("service id is %s, language is %s", langS.ServiceID, langS.Language)
	services, err := db.GetManager().TenantServiceDao().GetServiceByID(langS.ServiceID)
	if err != nil {
		logrus.Errorf("get service by id error, %v, %v", services, err)
		return err
	}
	if langS.Language == "java" {
		services.ContainerMemory = 512
		if err := db.GetManager().TenantServiceDao().UpdateModel(services); err != nil {
			logrus.Errorf("update tenant service error %v", err)
			return err
		}
	}
	return nil
}

//GetService get service(s)
func (s *ServiceAction) GetService(tenantID string) ([]*dbmodel.TenantServices, error) {
	services, err := db.GetManager().TenantServiceDao().GetServicesAllInfoByTenantID(tenantID)
	if err != nil {
		logrus.Errorf("get service by id error, %v, %v", services, err)
		return nil, err
	}
	var serviceIDs []string
	for _, s := range services {
		serviceIDs = append(serviceIDs, s.ServiceID)
	}
	status := s.statusCli.GetStatuss(strings.Join(serviceIDs, ","))
	for _, s := range services {
		if status, ok := status[s.ServiceID]; ok {
			s.CurStatus = status
		}
	}
	return services, nil
}

//GetPagedTenantRes get pagedTenantServiceRes(s)
func (s *ServiceAction) GetPagedTenantRes(offset, len int) ([]*api_model.TenantResource, int, error) {
	allstatus := s.statusCli.GetAllStatus()
	var serviceIDs []string
	for k, v := range allstatus {
		if !s.statusCli.IsClosedStatus(v) {
			serviceIDs = append(serviceIDs, k)
		}
	}
	services, count, err := db.GetManager().TenantServiceDao().GetPagedTenantService(offset, len, serviceIDs)
	if err != nil {
		logrus.Errorf("get service by id error, %v, %v", services, err)
		return nil, count, err
	}
	var result []*api_model.TenantResource
	for _, v := range services {
		var res api_model.TenantResource
		res.UUID, _ = v["tenant"].(string)
		res.Name, _ = v["tenant_name"].(string)
		res.EID, _ = v["eid"].(string)
		res.AllocatedCPU, _ = v["capcpu"].(int)
		res.AllocatedMEM, _ = v["capmem"].(int)
		res.UsedCPU, _ = v["usecpu"].(int)
		res.UsedMEM, _ = v["usemem"].(int)
		result = append(result, &res)
	}
	return result, count, nil
}

//GetTenantRes get pagedTenantServiceRes(s)
func (s *ServiceAction) GetTenantRes(uuid string) (*api_model.TenantResource, error) {
	services, err := db.GetManager().TenantServiceDao().GetServicesByTenantID(uuid)
	if err != nil {
		logrus.Errorf("get service by id error, %v, %v", services, err)
		return nil, err
	}
	var serviceIDs string
	var AllocatedCPU, AllocatedMEM int
	var serMap = make(map[string]*dbmodel.TenantServices, len(services))
	for _, ser := range services {
		if serviceIDs == "" {
			serviceIDs += ser.ServiceID
		} else {
			serviceIDs += "," + ser.ServiceID
		}
		AllocatedCPU += ser.ContainerCPU
		AllocatedMEM += ser.ContainerMemory
		serMap[ser.ServiceID] = ser
	}
	status := s.statusCli.GetStatuss(serviceIDs)
	var UsedCPU, UsedMEM int
	for k, v := range status {
		if !s.statusCli.IsClosedStatus(v) {
			UsedCPU += serMap[k].ContainerCPU
			UsedMEM += serMap[k].ContainerMemory
		}
	}
	disks := s.statusCli.GetAppsDisk(serviceIDs)
	var value float64
	for _, v := range disks {
		value += v
	}
	var res api_model.TenantResource
	res.UUID = uuid
	res.Name = ""
	res.EID = ""
	res.AllocatedCPU = AllocatedCPU
	res.AllocatedMEM = AllocatedMEM
	res.UsedCPU = UsedCPU
	res.UsedMEM = UsedMEM
	res.UsedDisk = value
	return &res, nil
}

//CodeCheck code check
func (s *ServiceAction) CodeCheck(c *api_model.CheckCodeStruct) error {
	err := s.MQClient.SendBuilderTopic(gclient.TaskStruct{
		TaskType: "code_check",
		TaskBody: c.Body,
		Topic:    gclient.BuilderTopic,
	})
	if err != nil {
		logrus.Errorf("equque mq error, %v", err)
		return err
	}
	return nil
}

//ServiceDepend service depend
func (s *ServiceAction) ServiceDepend(action string, ds *api_model.DependService) error {
	switch action {
	case "add":
		tsr := &dbmodel.TenantServiceRelation{
			TenantID:          ds.TenantID,
			ServiceID:         ds.ServiceID,
			DependServiceID:   ds.DepServiceID,
			DependServiceType: ds.DepServiceType,
			DependOrder:       1,
		}
		if err := db.GetManager().TenantServiceRelationDao().AddModel(tsr); err != nil {
			logrus.Errorf("add depend error, %v", err)
			return err
		}
	case "delete":
		logrus.Debugf("serviceid is %v, depid is %v", ds.ServiceID, ds.DepServiceID)
		if err := db.GetManager().TenantServiceRelationDao().DeleteRelationByDepID(ds.ServiceID, ds.DepServiceID); err != nil {
			logrus.Errorf("delete depend error, %v", err)
			return err
		}
	}
	return nil
}

//EnvAttr env attr
func (s *ServiceAction) EnvAttr(action string, at *dbmodel.TenantServiceEnvVar) error {
	switch action {
	case "add":
		if err := db.GetManager().TenantServiceEnvVarDao().AddModel(at); err != nil {
			logrus.Errorf("add env %v error, %v", at.AttrName, err)
			return err
		}
	case "delete":
		if err := db.GetManager().TenantServiceEnvVarDao().DeleteModel(at.ServiceID, at.AttrName); err != nil {
			logrus.Errorf("delete env %v error, %v", at.AttrName, err)
			return err
		}
	case "update":
		if err := db.GetManager().TenantServiceEnvVarDao().UpdateModel(at); err != nil {
			logrus.Errorf("update env %v error,%v", at.AttrName, err)
			return err
		}
	}
	return nil
}

//PortVar port var
func (s *ServiceAction) PortVar(action, tenantID, serviceID string, vps *api_model.ServicePorts, oldPort int) error {
	crt, err := db.GetManager().TenantServicePluginRelationDao().CheckSomeModelPluginByServiceID(
		serviceID,
		dbmodel.UpNetPlugin,
	)
	if err != nil {
		return err
	}
	switch action {
	case "add":
		for _, vp := range vps.Port {
			var vpD dbmodel.TenantServicesPort
			vpD.ServiceID = serviceID
			vpD.TenantID = tenantID
			//默认不打开
			vpD.IsInnerService = false
			vpD.IsOuterService = false
			vpD.ContainerPort = vp.ContainerPort
			vpD.MappingPort = vp.MappingPort
			vpD.Protocol = vp.Protocol
			vpD.PortAlias = vp.PortAlias
			if err := db.GetManager().TenantServicesPortDao().AddModel(&vpD); err != nil {
				logrus.Errorf("add port var error, %v", err)
				return err
			}
		}
	case "delete":
		tx := db.GetManager().Begin()
		for _, vp := range vps.Port {
			if err := db.GetManager().TenantServicesPortDaoTransactions(tx).DeleteModel(serviceID, vp.ContainerPort); err != nil {
				logrus.Errorf("delete port var error, %v", err)
				tx.Rollback()
				return err
			}
		}
		if err := tx.Commit().Error; err != nil {
			tx.Rollback()
			logrus.Debugf("commit delete port error, %v", err)
			return err
		}
	case "update":
		tx := db.GetManager().Begin()
		for _, vp := range vps.Port {
			//port更新单个请求
			if oldPort == 0 {
				oldPort = vp.ContainerPort
			}
			vpD, err := db.GetManager().TenantServicesPortDao().GetPort(serviceID, oldPort)
			if err != nil {
				tx.Rollback()
				return err
			}
			vpD.ServiceID = serviceID
			vpD.TenantID = tenantID
			vpD.IsInnerService = vp.IsInnerService
			vpD.IsOuterService = vp.IsOuterService
			vpD.ContainerPort = vp.ContainerPort
			vpD.MappingPort = vp.MappingPort
			vpD.Protocol = vp.Protocol
			vpD.PortAlias = vp.PortAlias
			if err := db.GetManager().TenantServicesPortDaoTransactions(tx).UpdateModel(vpD); err != nil {
				logrus.Errorf("update port var error, %v", err)
				tx.Rollback()
				return err
			}
			if crt {
				pluginPort, err := db.GetManager().TenantServicesStreamPluginPortDao().GetPluginMappingPortByServiceIDAndContainerPort(
					serviceID,
					dbmodel.UpNetPlugin,
					oldPort,
				)
				goon := true
				if err != nil {
					if strings.Contains(err.Error(), "record not found") {
						goon = false
					} else {
						logrus.Errorf("get plugin mapping port error:(%s)", err)
						tx.Rollback()
						return err
					}
				}
				if goon {
					pluginPort.ContainerPort = vp.ContainerPort
					if err := db.GetManager().TenantServicesStreamPluginPortDaoTransactions(tx).UpdateModel(pluginPort); err != nil {
						logrus.Errorf("update plugin mapping port error:(%s)", err)
						tx.Rollback()
						return err
					}
				}
			}
		}
		if err := tx.Commit().Error; err != nil {
			tx.Rollback()
			logrus.Debugf("commit update port error, %v", err)
			return err
		}
	}
	return nil
}

//PortOuter 端口对外服务操作
func (s *ServiceAction) PortOuter(tenantName, serviceID string, containerPort int,
	servicePort *api_model.ServicePortInnerOrOuter) (*dbmodel.TenantServiceLBMappingPort, string, error) {
	p, err := db.GetManager().TenantServicesPortDao().GetPort(serviceID, containerPort)
	if err != nil {
		return nil, "", fmt.Errorf("find service port error:%s", err.Error())
	}
	_, err = db.GetManager().TenantServiceDao().GetServiceByID(serviceID)
	if err != nil {
		return nil, "", fmt.Errorf("find service error:%s", err.Error())
	}
	hasUpStream, err := db.GetManager().TenantServicePluginRelationDao().CheckSomeModelPluginByServiceID(
		serviceID,
		dbmodel.UpNetPlugin,
	)
	if err != nil {
		return nil, "", fmt.Errorf("get plugin relations error: %s", err.Error())
	}
	//if stream 创建vs端口
	vsPort := &dbmodel.TenantServiceLBMappingPort{}
	switch servicePort.Body.Operation {
	case "close":
		if p.IsOuterService { //如果端口已经开了对外
			p.IsOuterService = false
			tx := db.GetManager().Begin()
			if err = db.GetManager().TenantServicesPortDaoTransactions(tx).UpdateModel(p); err != nil {
				tx.Rollback()
				return nil, "", err
			}

			if hasUpStream {
				pluginPort, err := db.GetManager().TenantServicesStreamPluginPortDao().GetPluginMappingPortByServiceIDAndContainerPort(
					serviceID,
					dbmodel.UpNetPlugin,
					containerPort,
				)
				if err != nil {
					if err.Error() == gorm.ErrRecordNotFound.Error() {
						logrus.Debugf("outer, plugin port (%d) is not exist, do not need delete", containerPort)
						goto OUTERCLOSEPASS
					}
					tx.Rollback()
					return nil, "", fmt.Errorf("outer, get plugin mapping port error:(%s)", err)
				}
				if p.IsInnerService {
					//发现内网未关闭则不删除该映射
					logrus.Debugf("outer, close outer, but plugin inner port (%d) is exist, do not need delete", containerPort)
					goto OUTERCLOSEPASS
				}
				if err := db.GetManager().TenantServicesStreamPluginPortDaoTransactions(tx).DeletePluginMappingPortByContainerPort(
					serviceID,
					dbmodel.UpNetPlugin,
					containerPort,
				); err != nil {
					tx.Rollback()
					return nil, "", fmt.Errorf("outer, delete plugin mapping port %d error:(%s)", containerPort, err)
				}
				logrus.Debugf(fmt.Sprintf("outer, delete plugin port %d->%d", containerPort, pluginPort.PluginPort))
			OUTERCLOSEPASS:
			}
			if err := tx.Commit().Error; err != nil {
				tx.Rollback()
				return nil, "", err
			}
		} else {
			return nil, "", nil
		}

	case "open":
		if p.IsOuterService {
			if p.Protocol != "http" && p.Protocol != "https" && servicePort.Body.IfCreateExPort {
				vsPort, err = s.createVSPort(serviceID, p.ContainerPort)
				if vsPort == nil {
					return nil, "", fmt.Errorf("port already open but can not get lb mapping port,%s", err.Error())
				}
				return vsPort, p.Protocol, nil
			}
		}
		p.IsOuterService = true
		tx := db.GetManager().Begin()
		if err = db.GetManager().TenantServicesPortDaoTransactions(tx).UpdateModel(p); err != nil {
			tx.Rollback()
			return nil, "", err
		}
		if p.Protocol != "http" && p.Protocol != "https" && servicePort.Body.IfCreateExPort {
			vsPort, err = s.createVSPort(serviceID, p.ContainerPort)
			if vsPort == nil {
				tx.Rollback()
				return nil, "", fmt.Errorf("create or get vs map port for service error,%s", err.Error())
			}
		}
		if hasUpStream {
			pluginPort, err := db.GetManager().TenantServicesStreamPluginPortDao().GetPluginMappingPortByServiceIDAndContainerPort(
				serviceID,
				dbmodel.UpNetPlugin,
				containerPort,
			)
			var pPort int
			if err != nil {
				if err.Error() == gorm.ErrRecordNotFound.Error() {
					ppPort, err := db.GetManager().TenantServicesStreamPluginPortDaoTransactions(tx).SetPluginMappingPort(
						p.TenantID,
						serviceID,
						dbmodel.UpNetPlugin,
						containerPort,
					)
					if err != nil {
						tx.Rollback()
						logrus.Errorf("outer, set plugin mapping port error:(%s)", err)
						return nil, "", fmt.Errorf("outer, set plugin mapping port error:(%s)", err)
					}
					pPort = ppPort
					goto OUTEROPENPASS
				}
				tx.Rollback()
				return nil, "", fmt.Errorf("outer, in setting plugin mapping port, get plugin mapping port error:(%s)", err)
			}
			logrus.Debugf("outer, plugin mapping port is already exist, %d->%d", pluginPort.ContainerPort, pluginPort.PluginPort)
		OUTEROPENPASS:
			logrus.Debugf("outer, set plugin mapping port %d->%d", containerPort, pPort)
		}
		if err := tx.Commit().Error; err != nil {
			tx.Rollback()
			return nil, "", err
		}
	}
	return vsPort, p.Protocol, nil
}
func (s *ServiceAction) createVSPort(serviceID string, containerPort int) (*dbmodel.TenantServiceLBMappingPort, error) {
	vsPort, err := db.GetManager().TenantServiceLBMappingPortDao().CreateTenantServiceLBMappingPort(serviceID, containerPort)
	if err != nil {
		return nil, fmt.Errorf("create vs map port for service error,%s", err.Error())
	}
	return vsPort, nil
}

//PortInner 端口对内服务操作
//TODO: send task to worker
func (s *ServiceAction) PortInner(tenantName, serviceID, operation string, port int) error {
	p, err := db.GetManager().TenantServicesPortDao().GetPort(serviceID, port)
	if err != nil {
		return err
	}
	_, err = db.GetManager().TenantServiceDao().GetServiceByID(serviceID)
	if err != nil {
		return fmt.Errorf("get service error:%s", err.Error())
	}
	hasUpStream, err := db.GetManager().TenantServicePluginRelationDao().CheckSomeModelPluginByServiceID(
		serviceID,
		dbmodel.UpNetPlugin,
	)
	if err != nil {
		return fmt.Errorf("get plugin relations error: %s", err.Error())
	}
	tx := db.GetManager().Begin()
	switch operation {
	case "close":
		if p.IsInnerService { //如果端口已经开了对内
			p.IsInnerService = false
			if err = db.GetManager().TenantServicesPortDaoTransactions(tx).UpdateModel(p); err != nil {
				tx.Rollback()
				return fmt.Errorf("update service port error: %s", err.Error())
			}
			if hasUpStream {
				pluginPort, err := db.GetManager().TenantServicesStreamPluginPortDao().GetPluginMappingPortByServiceIDAndContainerPort(
					serviceID,
					dbmodel.UpNetPlugin,
					port,
				)
				if err != nil {
					if err.Error() == gorm.ErrRecordNotFound.Error() {
						logrus.Debugf("inner, plugin port (%d) is not exist, do not need delete", port)
						goto INNERCLOSEPASS
					}
					tx.Rollback()
					return fmt.Errorf("inner, get plugin mapping port error:(%s)", err)
				}
				if p.IsOuterService {
					logrus.Debugf("inner, close inner, but plugin outerport (%d) is exist, do not need delete", port)
					goto INNERCLOSEPASS
				}
				if err := db.GetManager().TenantServicesStreamPluginPortDaoTransactions(tx).DeletePluginMappingPortByContainerPort(
					serviceID,
					dbmodel.UpNetPlugin,
					port,
				); err != nil {
					tx.Rollback()
					return fmt.Errorf("inner, delete plugin mapping port %d error:(%s)", port, err)
				}
				logrus.Debugf(fmt.Sprintf("inner, delete plugin port %d->%d", port, pluginPort.PluginPort))
			INNERCLOSEPASS:
			}
		} else {
			tx.Rollback()
			return fmt.Errorf("already close")
		}
	case "open":
		if p.IsInnerService {
			tx.Rollback()
			return fmt.Errorf("already open")
		}
		p.IsInnerService = true
		if err = db.GetManager().TenantServicesPortDaoTransactions(tx).UpdateModel(p); err != nil {
			tx.Rollback()
			return err
		}
		if hasUpStream {
			pluginPort, err := db.GetManager().TenantServicesStreamPluginPortDao().GetPluginMappingPortByServiceIDAndContainerPort(
				serviceID,
				dbmodel.UpNetPlugin,
				port,
			)
			var pPort int
			if err != nil {
				if err.Error() == gorm.ErrRecordNotFound.Error() {
					ppPort, err := db.GetManager().TenantServicesStreamPluginPortDaoTransactions(tx).SetPluginMappingPort(
						p.TenantID,
						serviceID,
						dbmodel.UpNetPlugin,
						port,
					)
					if err != nil {
						tx.Rollback()
						logrus.Errorf("inner, set plugin mapping port error:(%s)", err)
						return fmt.Errorf("inner, set plugin mapping port error:(%s)", err)
					}
					pPort = ppPort
					goto INNEROPENPASS
				}
				tx.Rollback()
				return fmt.Errorf("inner, in setting plugin mapping port, get plugin mapping port error:(%s)", err)
			}
			logrus.Debugf("inner, plugin mapping port is already exist, %d->%d", pluginPort.ContainerPort, pluginPort.PluginPort)
		INNEROPENPASS:
			logrus.Debugf("inner, set plugin mapping port %d->%d", port, pPort)
		}
	}
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return err
	}
	return nil
}

//ChangeLBPort change lb mapping port
//only support change to existing port in this tenants
func (s *ServiceAction) ChangeLBPort(tenantID, serviceID string, containerPort, changelbPort int) (*dbmodel.TenantServiceLBMappingPort, *util.APIHandleError) {
	oldmapport, err := db.GetManager().TenantServiceLBMappingPortDao().GetLBPortByTenantAndPort(tenantID, changelbPort)
	if err != nil {
		logrus.Errorf("change lb port check error, %s", err.Error())
		return nil, util.CreateAPIHandleErrorFromDBError("change lb port", err)
	}
	mapport, err := db.GetManager().TenantServiceLBMappingPortDao().GetTenantServiceLBMappingPort(serviceID, containerPort)
	if err != nil {
		logrus.Errorf("change lb port get error, %s", err.Error())
		return nil, util.CreateAPIHandleErrorFromDBError("change lb port", err)
	}
	port := oldmapport.Port
	oldmapport.Port = mapport.Port
	mapport.Port = port
	tx := db.GetManager().Begin()
	if err := db.GetManager().TenantServiceLBMappingPortDaoTransactions(tx).DELServiceLBMappingPortByServiceIDAndPort(oldmapport.ServiceID, port); err != nil {
		tx.Rollback()
		return nil, util.CreateAPIHandleErrorFromDBError("change lb port", err)
	}
	if err := db.GetManager().TenantServiceLBMappingPortDaoTransactions(tx).UpdateModel(mapport); err != nil {
		tx.Rollback()
		return nil, util.CreateAPIHandleErrorFromDBError("change lb port", err)
	}
	if err := db.GetManager().TenantServiceLBMappingPortDaoTransactions(tx).AddModel(oldmapport); err != nil {
		tx.Rollback()
		return nil, util.CreateAPIHandleErrorFromDBError("change lb port", err)
	}
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return nil, util.CreateAPIHandleErrorFromDBError("change lb port", err)
	}
	return mapport, nil
}

//VolumnVar var volumn
func (s *ServiceAction) VolumnVar(tsv *dbmodel.TenantServiceVolume, tenantID, fileContent, action string) *util.APIHandleError {
	localPath := os.Getenv("LOCAL_DATA_PATH")
	sharePath := os.Getenv("SHARE_DATA_PATH")
	if localPath == "" {
		localPath = "/grlocaldata"
	}
	if sharePath == "" {
		sharePath = "/grdata"
	}

	switch action {
	case "add":
		if tsv.HostPath == "" {
			//step 1 设置主机目录
			switch tsv.VolumeType {
			//共享文件存储
			case dbmodel.ShareFileVolumeType.String():
				tsv.HostPath = fmt.Sprintf("%s/tenant/%s/service/%s%s", sharePath, tenantID, tsv.ServiceID, tsv.VolumePath)
				//本地文件存储
			case dbmodel.LocalVolumeType.String():
				serviceType, err := db.GetManager().TenantServiceLabelDao().GetTenantServiceTypeLabel(tsv.ServiceID)
				if err != nil {
					return util.CreateAPIHandleErrorFromDBError("service type", err)
				}
				if serviceType == nil || serviceType.LabelValue != core_util.StatefulServiceType {
					return util.CreateAPIHandleError(400, fmt.Errorf("应用类型不为有状态应用.不支持本地存储"))
				}
				tsv.HostPath = fmt.Sprintf("%s/tenant/%s/service/%s%s", localPath, tenantID, tsv.ServiceID, tsv.VolumePath)
			}
		}
		if tsv.VolumeName == "" {
			tsv.VolumeName = uuid.NewV4().String()
		}
		// begin transaction
		tx := db.GetManager().Begin()
		if err := db.GetManager().TenantServiceVolumeDaoTransactions(tx).AddModel(tsv); err != nil {
			tx.Rollback()
			return util.CreateAPIHandleErrorFromDBError("add volume", err)
		}
		if fileContent != "" {
			cf := &dbmodel.TenantServiceConfigFile{
				UUID:        uuid.NewV4().String(),
				VolumeName:  tsv.VolumeName,
				FileContent: fileContent,
			}
			if err := db.GetManager().TenantServiceConfigFileDaoTransactions(tx).AddModel(cf); err != nil {
				tx.Rollback()
				return util.CreateAPIHandleErrorFromDBError("error creating config file", err)
			}
		}
		// end transaction
		if err := tx.Commit().Error; err != nil {
			tx.Rollback()
			return util.CreateAPIHandleErrorFromDBError("error ending transaction", err)
		}
	case "delete":
		// begin transaction
		tx := db.GetManager().Begin()
		if tsv.VolumeName != "" {
			err := db.GetManager().TenantServiceVolumeDaoTransactions(tx).DeleteModel(tsv.ServiceID, tsv.VolumeName)
			if err != nil && err.Error() != gorm.ErrRecordNotFound.Error() {
				tx.Rollback()
				return util.CreateAPIHandleErrorFromDBError("delete volume", err)
			}
		} else {
			if err := db.GetManager().TenantServiceVolumeDaoTransactions(tx).DeleteByServiceIDAndVolumePath(tsv.ServiceID, tsv.VolumePath);
				err != nil && err.Error() != gorm.ErrRecordNotFound.Error() {
				tx.Rollback()
				return util.CreateAPIHandleErrorFromDBError("delete volume", err)
			}
		}
		if err := db.GetManager().TenantServiceConfigFileDaoTransactions(tx).DelByVolumeID(tsv.VolumeName); err != nil {
			tx.Rollback()
			return util.CreateAPIHandleErrorFromDBError("error deleting config files", err)
		}
		// end transaction
		if err := tx.Commit().Error; err != nil {
			tx.Rollback()
			return util.CreateAPIHandleErrorFromDBError("error ending transaction", err)
		}
	}
	return nil
}

//GetVolumes 获取应用全部存储
func (s *ServiceAction) GetVolumes(serviceID string) ([]*dbmodel.TenantServiceVolume, *util.APIHandleError) {
	dbManager := db.GetManager()
	vs, err := dbManager.TenantServiceVolumeDao().GetTenantServiceVolumesByServiceID(serviceID)
	if err != nil && err.Error() != gorm.ErrRecordNotFound.Error() {
		return nil, util.CreateAPIHandleErrorFromDBError("get volumes", err)
	}
	return vs, nil
}

//VolumeDependency VolumeDependency
func (s *ServiceAction) VolumeDependency(tsr *dbmodel.TenantServiceMountRelation, action string) *util.APIHandleError {
	switch action {
	case "add":
		if tsr.VolumeName != "" {
			vm, err := db.GetManager().TenantServiceVolumeDao().GetVolumeByServiceIDAndName(tsr.DependServiceID, tsr.VolumeName)
			if err != nil {
				return util.CreateAPIHandleErrorFromDBError("get volume", err)
			}
			tsr.HostPath = vm.HostPath
			if err := db.GetManager().TenantServiceMountRelationDao().AddModel(tsr); err != nil {
				return util.CreateAPIHandleErrorFromDBError("add volume mount relation", err)
			}
		} else {
			if tsr.HostPath == "" {
				return util.CreateAPIHandleError(400, fmt.Errorf("host path can not be empty when create volume dependency in api v2"))
			}
			if err := db.GetManager().TenantServiceMountRelationDao().AddModel(tsr); err != nil {
				return util.CreateAPIHandleErrorFromDBError("add volume mount relation", err)
			}
		}
	case "delete":
		if tsr.VolumeName != "" {
			if err := db.GetManager().TenantServiceMountRelationDao().DElTenantServiceMountRelationByServiceAndName(tsr.ServiceID, tsr.VolumeName); err != nil {
				return util.CreateAPIHandleErrorFromDBError("delete mount relation", err)
			}
		} else {
			if err := db.GetManager().TenantServiceMountRelationDao().DElTenantServiceMountRelationByDepService(tsr.ServiceID, tsr.DependServiceID); err != nil {
				return util.CreateAPIHandleErrorFromDBError("delete mount relation", err)
			}
		}
	}
	return nil
}

//GetDepVolumes 获取依赖存储
func (s *ServiceAction) GetDepVolumes(serviceID string) ([]*dbmodel.TenantServiceMountRelation, *util.APIHandleError) {
	dbManager := db.GetManager()
	mounts, err := dbManager.TenantServiceMountRelationDao().GetTenantServiceMountRelationsByService(serviceID)
	if err != nil {
		return nil, util.CreateAPIHandleErrorFromDBError("get dep volume", err)
	}
	return mounts, nil
}

//ServiceProbe ServiceProbe
func (s *ServiceAction) ServiceProbe(tsp *dbmodel.TenantServiceProbe, action string) error {
	switch action {
	case "add":
		if err := db.GetManager().ServiceProbeDao().AddModel(tsp); err != nil {
			return err
		}
	case "update":
		if err := db.GetManager().ServiceProbeDao().UpdateModel(tsp); err != nil {
			return err
		}
	case "delete":
		if err := db.GetManager().ServiceProbeDao().DeleteModel(tsp.ServiceID, tsp.ProbeID); err != nil {
			return err
		}
	}
	return nil
}

//RollBack RollBack
func (s *ServiceAction) RollBack(rs *api_model.RollbackStruct) error {
	tx := db.GetManager().Begin()
	service, err := db.GetManager().TenantServiceDaoTransactions(tx).GetServiceByID(rs.ServiceID)
	if err != nil {
		tx.Rollback()
		return err
	}
	if service.DeployVersion == rs.DeployVersion {
		tx.Rollback()
		return fmt.Errorf("current version is %v, don't need rollback", rs.DeployVersion)
	}
	service.DeployVersion = rs.DeployVersion
	if err := db.GetManager().TenantServiceDaoTransactions(tx).UpdateModel(service); err != nil {
		tx.Rollback()
		return err
	}
	//发送重启消息到MQ
	startStopStruct := &api_model.StartStopStruct{
		TenantID:  rs.TenantID,
		ServiceID: rs.ServiceID,
		EventID:   rs.EventID,
		TaskType:  "rolling_upgrade",
	}
	if err := GetServiceManager().StartStopService(startStopStruct); err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Commit().Error; err != nil {
		return err
	}
	return nil
}

//GetStatus GetStatus
func (s *ServiceAction) GetStatus(serviceID string) (*api_model.StatusList, error) {
	services, errS := db.GetManager().TenantServiceDao().GetServiceByID(serviceID)
	if errS != nil {
		return nil, errS
	}
	sl := &api_model.StatusList{
		TenantID:      services.TenantID,
		ServiceID:     serviceID,
		ServiceAlias:  services.ServiceAlias,
		DeployVersion: services.DeployVersion,
		Replicas:      services.Replicas,
		ContainerMem:  services.ContainerMemory,
		ContainerCPU:  services.ContainerCPU,
		CurStatus:     services.CurStatus,
		StatusCN:      TransStatus(services.CurStatus),
	}
	status := s.statusCli.GetStatus(serviceID)
	if status != "" {
		sl.CurStatus = status
		sl.StatusCN = TransStatus(status)
	}
	return sl, nil
}

//GetServicesStatus  获取一组应用状态，若 serviceIDs为空,获取租户所有应用状态
func (s *ServiceAction) GetServicesStatus(tenantID string, serviceIDs []string) map[string]string {
	if serviceIDs == nil || len(serviceIDs) == 0 {
		services, _ := db.GetManager().TenantServiceDao().GetServicesByTenantID(tenantID)
		for _, s := range services {
			serviceIDs = append(serviceIDs, s.ServiceID)
		}
	}
	if len(serviceIDs) > 0 {
		status := s.statusCli.GetStatuss(strings.Join(serviceIDs, ","))
		return status
	}
	return nil
}

//CreateTenant create tenant
func (s *ServiceAction) CreateTenant(t *dbmodel.Tenants) error {
	if ten, _ := db.GetManager().TenantDao().GetTenantIDByName(t.Name); ten != nil {
		return fmt.Errorf("tenant name %s is exist", t.Name)
	}
	tx := db.GetManager().Begin()
	if err := db.GetManager().TenantDaoTransactions(tx).AddModel(t); err != nil {
		if !strings.HasSuffix(err.Error(), "is exist") {
			tx.Rollback()
			return err
		}
	}
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return err
	}
	return nil
}

//CreateTenandIDAndName create tenant_id and tenant_name
func (s *ServiceAction) CreateTenandIDAndName(eid string) (string, string, error) {
	id := fmt.Sprintf("%s", uuid.NewV4())
	uid := strings.Replace(id, "-", "", -1)
	name := strings.Split(id, "-")[0]
	logrus.Debugf("uuid is %v, name is %v", uid, name)
	return uid, name, nil
}

//K8sPodInfo for api
type K8sPodInfo struct {
	ServiceID  string                       `json:"service_id"`
	DeployID   string                       `json:"deploy_id"`
	DeployType string                       `json:"deploy_type"`
	PodName    string                       `json:"pod_name"`
	PodIP      string                       `json:"pod_ip"`
	PodStatus  string                       `json:"pod_status"`
	Container  map[string]map[string]string `json:"container"`
}

//GetPods get pods
func (s *ServiceAction) GetPods(serviceID string) ([]*K8sPodInfo, error) {
	var podsInfoList []*K8sPodInfo
	pods, err := s.statusCli.GetServicePods(serviceID)
	if err != nil {
		logrus.Error("GetPodByService Error:", err)
		return nil, err
	}
	var podNames []string
	for _, v := range pods.Pods {
		var podInfo K8sPodInfo
		containerInfos := make(map[string]map[string]string, 10)
		podInfo.ServiceID = v.ServiceId
		podInfo.DeployID = v.DeployId
		podInfo.DeployType = v.DeployType
		podInfo.PodName = v.PodName
		podInfo.PodIP = v.PodIp
		podInfo.PodStatus = v.PodStatus
		for _, container := range v.Containers {
			containerInfos[container.ContainerName] = map[string]string{
				"memory_limit": fmt.Sprintf("%d", container.MemoryLimit),
				"memory_usage": "0",
			}
		}
		podInfo.Container = containerInfos
		podNames = append(podNames, v.PodName)
		podsInfoList = append(podsInfoList, &podInfo)
	}
	containerMemInfo, _ := s.GetPodContainerMemory(podNames)
	fmt.Println(containerMemInfo)
	for _, c := range podsInfoList {
		for k := range c.Container {
			if info, exist := containerMemInfo[c.PodName][k]; exist {
				c.Container[k]["memory_usage"] = info
			}
		}
	}
	return podsInfoList, nil
}

//GetPodContainerMemory Use Prometheus to query memory resources
func (s *ServiceAction) GetPodContainerMemory(podNames []string) (map[string]map[string]string, error) {
	memoryUsageMap := make(map[string]map[string]string, 10)
	proxy := GetPrometheusProxy()
	queryName := strings.Join(podNames, "|")
	query := fmt.Sprintf(`container_memory_usage_bytes{pod_name=~"%s"}`, queryName)
	proQuery := strings.Replace(query, " ", "%20", -1)
	req, err := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:9999/api/v1/query?query=%s", proQuery), nil)
	if err != nil {
		logrus.Error("create request prometheus api error ", err.Error())
		return memoryUsageMap, nil
	}
	presult, err := proxy.Do(req)
	if err != nil {
		logrus.Error("do proxy request prometheus api error ", err.Error())
		return memoryUsageMap, nil
	}
	if presult.Body != nil {
		defer presult.Body.Close()
		if presult.StatusCode != 200 {
			logrus.Error("StatusCode:", presult.StatusCode, err)
			return memoryUsageMap, nil
		}
		var qres QueryResult
		err = json.NewDecoder(presult.Body).Decode(&qres)
		if err == nil {
			for _, re := range qres.Data.Result {
				var containerName, podName string
				var valuesBytes string
				if cname, ok := re["metric"].(map[string]interface{}); ok {
					if containerName, ok = cname["container_name"].(string); !ok {
						continue
					}
					if podName, ok = cname["pod_name"].(string); !ok {
						continue
					}
				} else {
					logrus.Info("metric decode error")
				}
				if val, ok := (re["value"]).([]interface{}); ok && len(val) == 2 {
					valuesBytes = val[1].(string)
				} else {
					logrus.Info("value decode error")
				}
				if _, ok := memoryUsageMap[podName]; ok {
					memoryUsageMap[podName][containerName] = valuesBytes
				} else {
					memoryUsageMap[podName] = map[string]string{
						containerName: valuesBytes,
					}
				}
			}
			return memoryUsageMap, nil
		}
	} else {
		logrus.Error("Body Is empty")
	}
	return memoryUsageMap, nil
}

//TransServieToDelete trans service info to delete table
func (s *ServiceAction) TransServieToDelete(serviceID string) error {
	service, err := db.GetManager().TenantServiceDao().GetServiceByID(serviceID)
	if err != nil {
		return err
	}
	status := s.statusCli.GetStatus(serviceID)
	if !s.statusCli.IsClosedStatus(status) {
		return fmt.Errorf("unclosed")
	}
	tx := db.GetManager().Begin()
	delService := service.ChangeDelete()
	delService.ID = 0
	if err := db.GetManager().TenantServiceDeleteDaoTransactions(tx).AddModel(delService); err != nil {
		tx.Rollback()
		return err
	}
	if err := db.GetManager().TenantServiceDaoTransactions(tx).DeleteServiceByServiceID(serviceID); err != nil {
		tx.Rollback()
		return err
	}
	//删除domain
	//删除pause
	//删除tenant_system_pause
	//删除tenant_service_relation
	if err := db.GetManager().TenantServiceMountRelationDaoTransactions(tx).DELTenantServiceMountRelationByServiceID(serviceID); err != nil {
		if err.Error() != gorm.ErrRecordNotFound.Error() {
			tx.Rollback()
			return err
		}
	}
	//删除tenant_service_evn_var
	if err := db.GetManager().TenantServiceEnvVarDaoTransactions(tx).DELServiceEnvsByServiceID(serviceID); err != nil {
		if err.Error() != gorm.ErrRecordNotFound.Error() {
			tx.Rollback()
			return err
		}
	}
	//删除tenant_services_port
	if err := db.GetManager().TenantServicesPortDaoTransactions(tx).DELPortsByServiceID(serviceID); err != nil {
		if err.Error() != gorm.ErrRecordNotFound.Error() {
			tx.Rollback()
			return err
		}
	}
	//删除tenant_service_mnt_relation
	if err := db.GetManager().TenantServiceRelationDaoTransactions(tx).DELRelationsByServiceID(serviceID); err != nil {
		if err.Error() != gorm.ErrRecordNotFound.Error() {
			tx.Rollback()
			return err
		}
	}
	//删除tenant_lb_mapping_port
	if err := db.GetManager().TenantServiceLBMappingPortDaoTransactions(tx).DELServiceLBMappingPortByServiceID(serviceID); err != nil {
		if err.Error() != gorm.ErrRecordNotFound.Error() {
			tx.Rollback()
			return err
		}
	}
	//删除tenant_service_volume
	if err := db.GetManager().TenantServiceVolumeDaoTransactions(tx).DeleteTenantServiceVolumesByServiceID(serviceID); err != nil {
		if err.Error() != gorm.ErrRecordNotFound.Error() {
			tx.Rollback()
			return err
		}
	}
	//删除service_probe
	if err := db.GetManager().ServiceProbeDaoTransactions(tx).DELServiceProbesByServiceID(serviceID); err != nil {
		if err.Error() != gorm.ErrRecordNotFound.Error() {
			tx.Rollback()
			return err
		}
	}
	//TODO: 如果有关联过插件，需要删除该插件相关配置及资源
	if err := db.GetManager().TenantServicePluginRelationDaoTransactions(tx).DeleteALLRelationByServiceID(serviceID); err != nil {
		if err.Error() != gorm.ErrRecordNotFound.Error() {
			tx.Rollback()
			return err
		}
	}
	if err := db.GetManager().TenantServicesStreamPluginPortDaoTransactions(tx).DeleteAllPluginMappingPortByServiceID(serviceID); err != nil {
		if err.Error() != gorm.ErrRecordNotFound.Error() {
			tx.Rollback()
			return err
		}
	}
	if err := db.GetManager().TenantPluginVersionENVDaoTransactions(tx).DeleteEnvByServiceID(serviceID); err != nil {
		if err.Error() != gorm.ErrRecordNotFound.Error() {
			tx.Rollback()
			return err
		}
	}
	if err := db.GetManager().TenantServiceLabelDaoTransactions(tx).DeleteLabelByServiceID(serviceID); err != nil {
		if err.Error() != gorm.ErrRecordNotFound.Error() {
			tx.Rollback()
			return err
		}
	}
	// delete gateway related resources
	httpRules, err := db.GetManager().HttpRuleDaoTransactions(tx).ListByServiceID(serviceID)
	if err != nil {
		tx.Rollback()
		return err
	}
	for _, r := range httpRules {
		if err := db.GetManager().HttpRuleDaoTransactions(tx).DeleteHttpRuleByID(r.UUID); err != nil {
			tx.Rollback()
			return err
		}
	}
	tcpRules, err := db.GetManager().TcpRuleDaoTransactions(tx).ListByServiceID(serviceID)
	if err != nil {
		tx.Rollback()
		return err
	}
	for _, r := range tcpRules {
		if err := db.GetManager().TcpRuleDaoTransactions(tx).DeleteTcpRule(r); err != nil {
			tx.Rollback()
			return err
		}
	}
	//删除plugin etcd资源
	prefixK := fmt.Sprintf("/resources/define/%s/%s", service.TenantID, service.ServiceAlias)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, err = s.EtcdCli.Delete(ctx, prefixK, clientv3.WithPrefix())
	if err != nil {
		logrus.Errorf("delete prefix %s from etcd error, %v", prefixK, err)
		tx.Rollback()
		return err
	}
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return err
	}
	return nil
}

//GetServiceDeployInfo get service deploy info
func (s *ServiceAction) GetServiceDeployInfo(tenantID, serviceID string) (*pb.DeployInfo, *util.APIHandleError) {
	info, err := s.statusCli.GetServiceDeployInfo(serviceID)
	if err != nil {
		return nil, util.CreateAPIHandleError(500, err)
	}
	return info, nil
}

//TransStatus trans service status
func TransStatus(eStatus string) string {
	switch eStatus {
	case "starting":
		return "启动中"
	case "abnormal":
		return "运行异常"
	case "upgrade":
		return "升级中"
	case "closed":
		return "已关闭"
	case "stopping":
		return "关闭中"
	case "checking":
		return "检测中"
	case "unusual":
		return "运行异常"
	case "running":
		return "运行中"
	case "failure":
		return "未知"
	case "undeploy":
		return "未部署"
	case "deployed":
		return "已部署"
	}
	return ""
}

//CheckLabel check label
func CheckLabel(serviceID string) bool {
	//true for v2, false for v1
	serviceLabel, err := db.GetManager().TenantServiceLabelDao().GetTenantServiceLabel(serviceID)
	if err != nil {
		return false
	}
	if serviceLabel != nil && len(serviceLabel) > 0 {
		return true
	}
	return false
}

//CheckMapKey CheckMapKey
func CheckMapKey(rebody map[string]interface{}, key string, defaultValue interface{}) map[string]interface{} {
	if _, ok := rebody[key]; ok {
		return rebody
	}
	rebody[key] = defaultValue
	return rebody
}

func chekeServiceLabel(v string) string {
	if strings.Contains(v, "有状态") {
		return core_util.StatefulServiceType
	}
	if strings.Contains(v, "无状态") {
		return core_util.StatelessServiceType
	}
	return v
}
