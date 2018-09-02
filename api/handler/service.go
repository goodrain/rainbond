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

	"github.com/coreos/etcd/clientv3"
	"github.com/goodrain/rainbond/db"
	core_model "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/mq/api/grpc/pb"
	"github.com/twinj/uuid"

	"github.com/jinzhu/gorm"

	"github.com/pquerna/ffjson/ffjson"

	api_db "github.com/goodrain/rainbond/api/db"
	api_model "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/api/util"
	"github.com/goodrain/rainbond/appruntimesync/client"
	dbmodel "github.com/goodrain/rainbond/db/model"
	core_util "github.com/goodrain/rainbond/util"
	"github.com/goodrain/rainbond/worker/discover/model"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/Sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"
	"net/http"
	"encoding/json"
)

//ServiceAction service act
type ServiceAction struct {
	MQClient   pb.TaskQueueClient
	KubeClient *kubernetes.Clientset
	EtcdCli    *clientv3.Client
	statusCli  *client.AppRuntimeSyncClient
}

//CreateManager create Manger
func CreateManager(mqClient pb.TaskQueueClient,
	kubeClient *kubernetes.Clientset,
	etcdCli *clientv3.Client, statusCli *client.AppRuntimeSyncClient) *ServiceAction {
	return &ServiceAction{
		MQClient:   mqClient,
		KubeClient: kubeClient,
		EtcdCli:    etcdCli,
		statusCli:  statusCli,
	}
}

//ServiceBuild service build
func (s *ServiceAction) ServiceBuild(tenantID, serviceID string, r *api_model.BuildServiceStruct) error {
	eventID := r.Body.EventID
	logger := event.GetManager().GetLogger(eventID)
	defer event.CloseManager()
	service, err := db.GetManager().TenantServiceDao().GetServiceByID(serviceID)
	if err != nil {
		return err
	}
	if r.Body.Kind == "" {
		r.Body.Kind = "source"
	}
	switch r.Body.Kind {
	//deprecated
	case "source":
		//源码构建
		if err := s.sourceBuild(r, service); err != nil {
			logger.Error("源码构建应用任务发送失败 "+err.Error(), map[string]string{"step": "callback", "status": "failure"})
			return err
		}
		logger.Info("源码构建应用任务发送成功 ", map[string]string{"step": "source-service", "status": "starting"})
		return nil
		//deprecated
	case "slug":
		//源码构建的分享至云市安装回平台
		if err := s.slugBuild(r, service); err != nil {
			logger.Error("slug构建应用任务发送失败"+err.Error(), map[string]string{"step": "callback", "status": "failure"})
			return err
		}
		logger.Info("slug构建应用任务发送成功 ", map[string]string{"step": "source-service", "status": "starting"})
		return nil
		//deprecated
	case "image":
		//镜像构建
		if err := s.imageBuild(r, service); err != nil {
			logger.Error("镜像构建应用任务发送失败 "+err.Error(), map[string]string{"step": "callback", "status": "failure"})
			return err
		}
		logger.Info("镜像构建应用任务发送成功 ", map[string]string{"step": "image-service", "status": "starting"})
		return nil
		//deprecated
	case "market":
		//镜像构建分享至云市安装回平台
		if err := s.marketBuild(r, service); err != nil {
			logger.Error("云市构建应用任务发送失败 "+err.Error(), map[string]string{"step": "callback", "status": "failure"})
			return err
		}
		logger.Info("云市构建应用任务发送成功 ", map[string]string{"step": "market-service", "status": "starting"})
		return nil
	case "build_from_image":
		if err := s.buildFromImage(r, service); err != nil {
			logger.Error("镜像构建应用任务发送失败 "+err.Error(), map[string]string{"step": "callback", "status": "failure"})
			return err
		}
		logger.Info("镜像构建应用任务发送成功 ", map[string]string{"step": "image-service", "status": "starting"})
		return nil
	case "build_from_source_code":
		if err := s.buildFromSourceCode(r, service); err != nil {
			logger.Error("源码构建应用任务发送失败 "+err.Error(), map[string]string{"step": "callback", "status": "failure"})
			return err
		}
		logger.Info("源码构建应用任务发送成功 ", map[string]string{"step": "source-service", "status": "starting"})
		return nil
	case "build_from_market_image":
		if err := s.buildFromImage(r, service); err != nil {
			logger.Error("镜像构建应用任务发送失败 "+err.Error(), map[string]string{"step": "callback", "status": "failure"})
			return err
		}
		logger.Info("云市镜像构建应用任务发送成功 ", map[string]string{"step": "image-service", "status": "starting"})
		return nil
	case "build_from_market_slug":
		if err := s.buildFromMarketSlug(r, service); err != nil {
			logger.Error("云市源码包构建应用任务发送失败 "+err.Error(), map[string]string{"step": "callback", "status": "failure"})
			return err
		}
		logger.Info("云市源码包构建应用任务发送成功 ", map[string]string{"step": "image-service", "status": "starting"})
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
	body["tenant_name"] = r.Body.TenantName
	body["tenant_id"] = service.TenantID
	body["service_id"] = service.ServiceID
	body["service_alias"] = r.Body.ServiceAlias
	body["slug_info"] = r.Body.SlugInfo
	return s.sendTask(body, "build_from_market_slug")
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
	body["service_id"] = service.ID
	body["deploy_version"] = r.Body.DeployVersion
	body["app_version"] = service.ServiceVersion
	body["namespace"] = service.Namespace
	body["operator"] = r.Body.Operator
	body["event_id"] = r.Body.EventID
	body["tenant_name"] = r.Body.TenantName
	body["service_alias"] = r.Body.ServiceAlias
	body["action"] = "download_and_deploy"
	body["dep_sids"] = dependIds
	body["code_from"] = "image_manual"
	if r.Body.User != "" && r.Body.Password != "" {
		body["user"] = r.Body.User
		body["password"] = r.Body.Password
	}
	return s.sendTask(body, "build_from_image")
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
	body["server_type"] = r.Body.ServiceType
	body["service_alias"] = r.Body.ServiceAlias
	if r.Body.User != "" && r.Body.Password != "" {
		body["user"] = r.Body.User
		body["password"] = r.Body.Password
	}
	body["expire"] = 180
	logrus.Debugf("app_build body is %v", body)
	return s.sendTask(body, "build_from_source_code")
}

func (s *ServiceAction) sourceBuild(r *api_model.BuildServiceStruct, service *dbmodel.TenantServices) error {
	logrus.Debugf("build from source")
	if r.Body.RepoURL == "" || r.Body.DeployVersion == "" || r.Body.EventID == "" {
		return fmt.Errorf("args error")
	}
	body := make(map[string]interface{})
	if r.Body.Operator == "" {
		body["operator"] = "system"
	} else {
		body["operator"] = r.Body.Operator
	}
	body["tenant_id"] = service.TenantID
	body["service_id"] = service.ServiceID
	body["repo_url"] = r.Body.RepoURL
	body["action"] = r.Body.Action
	body["deploy_version"] = r.Body.DeployVersion
	body["event_id"] = r.Body.EventID
	body["envs"] = r.Body.ENVS
	body["tenant_name"] = r.Body.TenantName
	body["service_alias"] = r.Body.ServiceAlias
	body["expire"] = 180
	logrus.Debugf("app_build body is %v", body)
	return s.sendTask(body, "app_build")
}

func (s *ServiceAction) imageBuild(r *api_model.BuildServiceStruct, service *dbmodel.TenantServices) error {
	logrus.Debugf("build from image")
	if r.Body.EventID == "" {
		return fmt.Errorf("args error")
	}
	dependIds, err := db.GetManager().TenantServiceRelationDao().GetTenantServiceRelations(service.ServiceID)
	if err != nil {
		return err
	}
	body := make(map[string]interface{})
	if r.Body.Operator == "" {
		body["operator"] = "system"
	} else {
		body["operator"] = r.Body.Operator
	}
	body["image"] = service.ImageName
	body["service_id"] = service.ID
	body["deploy_version"] = r.Body.DeployVersion
	body["app_version"] = service.ServiceVersion
	body["namespace"] = service.Namespace
	body["operator"] = r.Body.Operator
	body["event_id"] = r.Body.EventID
	body["tenant_name"] = r.Body.TenantName
	body["service_alias"] = r.Body.ServiceAlias
	body["action"] = "download_and_deploy"
	body["dep_sids"] = dependIds
	body["code_from"] = "image_manual"
	logrus.Debugf("image_manual body is %v", body)
	return s.sendTask(body, "image_manual")
}

func (s *ServiceAction) slugBuild(r *api_model.BuildServiceStruct, service *dbmodel.TenantServices) error {
	logrus.Debugf("build from slug")
	if r.Body.EventID == "" {
		return fmt.Errorf("args error")
	}
	dependIds, err := db.GetManager().TenantServiceRelationDao().GetTenantServiceRelations(service.ServiceID)
	if err != nil {
		return err
	}
	body := make(map[string]interface{})
	if r.Body.Operator == "" {
		body["operator"] = "system"
	} else {
		body["operator"] = r.Body.Operator
	}
	body["image"] = service.ImageName
	body["service_id"] = service.ID
	body["deploy_version"] = r.Body.DeployVersion
	body["service_alias"] = service.ServiceAlias
	body["app_version"] = service.ServiceVersion
	body["app_key"] = service.ServiceKey
	body["namespace"] = service.Namespace
	body["operator"] = r.Body.Operator
	body["event_id"] = r.Body.EventID
	body["tenant_name"] = r.Body.TenantName
	body["service_alias"] = r.Body.ServiceAlias
	body["action"] = "download_and_deploy"
	body["dep_sids"] = dependIds
	logrus.Debugf("image_manual body is %v", body)
	return s.sendTask(body, "app_slug")
}

func (s *ServiceAction) marketBuild(r *api_model.BuildServiceStruct, service *dbmodel.TenantServices) error {
	logrus.Debugf("build from cloud market")
	if r.Body.EventID == "" {
		return fmt.Errorf("args error")
	}
	dependIds, err := db.GetManager().TenantServiceRelationDao().GetTenantServiceRelations(service.ServiceID)
	if err != nil {
		return err
	}
	body := make(map[string]interface{})
	if r.Body.Operator == "" {
		body["operator"] = "system"
	} else {
		body["operator"] = r.Body.Operator
	}
	body["image"] = service.ImageName
	body["service_id"] = service.ID
	body["service_alias"] = service.ServiceAlias
	body["deploy_version"] = r.Body.DeployVersion
	body["app_version"] = service.ServiceVersion
	body["namespace"] = service.Namespace
	body["operator"] = r.Body.Operator
	body["event_id"] = r.Body.EventID
	body["tenant_name"] = r.Body.TenantName
	body["service_alias"] = r.Body.ServiceAlias
	body["action"] = "download_and_deploy"
	body["dep_sids"] = dependIds
	logrus.Debugf("app_image body is %v", body)
	return s.sendTask(body, "app_image")
}

func (s *ServiceAction) sendTask(body map[string]interface{}, taskType string) error {
	bodyJ, err := ffjson.Marshal(body)
	if err != nil {
		return err
	}
	bs := &api_db.BuildTaskStruct{
		TaskType: taskType,
		TaskBody: bodyJ,
		User:     "define",
	}
	eq, errEq := api_db.BuildTaskBuild(bs)
	if errEq != nil {
		logrus.Errorf("build equeue stop request error, %v", errEq)
		return errEq
	}
	ctx, cancel := context.WithCancel(context.Background())
	_, err = s.MQClient.Enqueue(ctx, eq)
	cancel()
	if err != nil {
		logrus.Errorf("equque mq error, %v", err)
		return err
	}
	return nil
}

//AddLabel add labels
func (s *ServiceAction) AddLabel(kind, serviceID string, amp []string) error {
	for _, v := range amp {
		var labelModel dbmodel.TenantServiceLable
		switch kind {
		case "service":
			labelModel.ServiceID = serviceID
			labelModel.LabelKey = core_model.LabelKeyServiceType
			v = chekeServiceLabel(v)
			labelModel.LabelValue = v
		case "node":
			labelModel.ServiceID = serviceID
			labelModel.LabelKey = v
			labelModel.LabelValue = core_model.LabelKeyNodeSelector
		}
		if err := db.GetManager().TenantServiceLabelDao().AddModel(&labelModel); err != nil {
			return err
		}
	}
	return nil
}

//DeleteLabel delete label
func (s *ServiceAction) DeleteLabel(kind, serviceID string, amp []string) error {
	switch kind {
	case "node":
		return db.GetManager().TenantServiceLabelDao().DELTenantServiceLabelsByLabelvaluesAndServiceID(serviceID, amp)
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
	ts := &api_db.TaskStruct{
		TaskType: sss.TaskType,
		TaskBody: TaskBody,
		User:     "define",
	}
	eq, errEq := api_db.BuildTask(ts)
	if errEq != nil {
		logrus.Errorf("build equeue startstop request error, %v", errEq)
		return errEq
	}
	ctx, cancel := context.WithCancel(context.Background())
	_, err = s.MQClient.Enqueue(ctx, eq)
	cancel()
	if err != nil {
		logrus.Errorf("equque mq error, %v", err)
		return err
	}
	logrus.Debugf("equeue mq startstop task success")
	return nil
}

//ServiceVertical vertical service
func (s *ServiceAction) ServiceVertical(vs *model.VerticalScalingTaskBody) error {
	ts := &api_db.TaskStruct{
		TaskType: "vertical_scaling",
		TaskBody: vs,
		User:     "define",
	}
	eq, errEq := api_db.BuildTask(ts)
	if errEq != nil {
		logrus.Errorf("build equeue vertical request error, %v", errEq)
		return errEq
	}
	ctx, cancel := context.WithCancel(context.Background())
	_, err := s.MQClient.Enqueue(ctx, eq)
	cancel()
	if err != nil {
		logrus.Errorf("equque mq error, %v", err)
		return err
	}
	logrus.Debugf("equeue mq vertical task success")
	return nil
}

//ServiceHorizontal Service Horizontal
func (s *ServiceAction) ServiceHorizontal(hs *model.HorizontalScalingTaskBody) error {
	ts := &api_db.TaskStruct{
		TaskType: "horizontal_scaling",
		TaskBody: hs,
		User:     "define",
	}
	eq, errEq := api_db.BuildTask(ts)
	if errEq != nil {
		logrus.Errorf("build equeue horizontal request error, %v", errEq)
		return errEq
	}
	ctx, cancel := context.WithCancel(context.Background())
	_, err := s.MQClient.Enqueue(ctx, eq)
	cancel()
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
		logrus.Errorf("get service by id error, %v, %v", services, err)
		return err
	}
	ru.CurrentDeployVersion = services.DeployVersion
	ts := &api_db.TaskStruct{
		TaskType: "rolling_upgrade",
		TaskBody: ru,
		User:     "define",
	}
	eq, errEq := api_db.BuildTask(ts)
	if errEq != nil {
		logrus.Errorf("build equeue upgrade request error, %v", errEq)
		return errEq
	}
	ctx, cancel := context.WithCancel(context.Background())
	_, err = s.MQClient.Enqueue(ctx, eq)
	cancel()
	if err != nil {
		logrus.Errorf("equque mq error, %v", err)
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
					serviceType, err := db.GetManager().TenantServiceLabelDao().GetTenantServiceTypeLabel(volumn.ServiceID)
					if err != nil {
						tx.Rollback()
						return util.CreateAPIHandleErrorFromDBError("service type", err)
					}
					if serviceType.LabelValue != core_util.StatefulServiceType {
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

	//set app status
	if err := s.statusCli.SetStatus(ts.ServiceID, "undeploy"); err != nil {
		tx.Rollback()
		return err
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
	//TODO: 更新单个项方法不给力
	if sc["image_name"] != nil {
		ts.ImageName = sc["image_name"].(string)
	}
	if sc["container_memory"] != nil {
		ts.ContainerMemory = sc["container_memory"].(int)
	}
	if sc["container_cmd"] != nil {
		ts.ContainerCMD = sc["container_cmd"].(string)
	}
	//服务信息表
	if err := db.GetManager().TenantServiceDao().UpdateModel(ts); err != nil {
		logrus.Errorf("update service error, %v", err)
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
	bodyJ, err := ffjson.Marshal(&c.Body)
	if err != nil {
		return err
	}
	bs := &api_db.BuildTaskStruct{
		TaskType: "code_check",
		TaskBody: bodyJ,
		User:     "define",
	}
	eq, errEq := api_db.BuildTaskBuild(bs)
	if errEq != nil {
		logrus.Errorf("build equeue code check error, %v", errEq)
		return errEq
	}
	ctx, cancel := context.WithCancel(context.Background())
	_, err = s.MQClient.Enqueue(ctx, eq)
	cancel()
	if err != nil {
		logrus.Errorf("equque mq error, %v", err)
		return err
	}
	return nil
}

//ShareCloud share cloud
func (s *ServiceAction) ShareCloud(c *api_model.CloudShareStruct) error {
	var bs api_db.BuildTaskStruct
	switch c.Body.Kind {
	case "app_slug":
		bodyJ, err := ffjson.Marshal(&c.Body.Slug)
		if err != nil {
			return err
		}
		bs.User = "define"
		bs.TaskBody = bodyJ
		//bs.TaskType = "app_slug"
		bs.TaskType = "slug_share"
	case "app_image":
		if c.Body.Image.ServiceID != "" {
			service, err := db.GetManager().TenantServiceDao().GetServiceByID(c.Body.Image.ServiceID)
			if err != nil {
				return err
			}
			c.Body.Image.Image = service.ImageName
		}
		bodyJ, err := ffjson.Marshal(&c.Body.Image)
		if err != nil {
			return err
		}
		bs.User = "define"
		bs.TaskBody = bodyJ
		bs.TaskType = "image_share"
	default:
		return fmt.Errorf("need share kind")
	}
	eq, errEq := api_db.BuildTaskBuild(&bs)
	if errEq != nil {
		logrus.Errorf("build equeue share cloud error, %v", errEq)
		return errEq
	}
	ctx, cancel := context.WithCancel(context.Background())
	_, err := s.MQClient.Enqueue(ctx, eq)
	cancel()
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
			//TODO:删除k8s Service
			service, err := db.GetManager().K8sServiceDao().GetK8sService(serviceID, vp.ContainerPort, true)
			if err != nil && err.Error() != gorm.ErrRecordNotFound.Error() {
				logrus.Error("get deploy k8s service info error.")
				tx.Rollback()
				return err
			}
			if service != nil {
				err := s.KubeClient.Core().Services(tenantID).Delete(service.K8sServiceID, &metav1.DeleteOptions{})
				if err != nil {
					logrus.Error("delete deploy k8s service info from kube-api error.")
					tx.Rollback()
					return err
				}
				err = db.GetManager().K8sServiceDaoTransactions(tx).DeleteK8sServiceByName(service.K8sServiceID)
				if err != nil {
					logrus.Error("delete deploy k8s service info from db error.")
					tx.Rollback()
					return err
				}
				if crt {
					if err := db.GetManager().TenantServicesStreamPluginPortDaoTransactions(tx).DeletePluginMappingPortByContainerPort(
						serviceID,
						dbmodel.UpNetPlugin,
						vp.ContainerPort,
					); err != nil {
						logrus.Errorf("delete plugin stream mapping port error: (%s)", err)
						tx.Rollback()
						return err
					}
				}
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
func (s *ServiceAction) PortOuter(tenantName, serviceID, operation string, port int) (*dbmodel.TenantServiceLBMappingPort, string, error) {
	p, err := db.GetManager().TenantServicesPortDao().GetPort(serviceID, port)
	if err != nil {
		return nil, "", fmt.Errorf("find service port error:%s", err.Error())
	}
	service, err := db.GetManager().TenantServiceDao().GetServiceByID(serviceID)
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
	var k8sService *v1.Service
	//if stream 创建vs端口
	vsPort := &dbmodel.TenantServiceLBMappingPort{}
	switch operation {
	case "close":
		if p.IsOuterService { //如果端口已经开了对外
			p.IsOuterService = false
			tx := db.GetManager().Begin()
			if err = db.GetManager().TenantServicesPortDaoTransactions(tx).UpdateModel(p); err != nil {
				tx.Rollback()
				return nil, "", err
			}
			service, err := db.GetManager().K8sServiceDao().GetK8sService(serviceID, port, true)
			if err != nil && err != gorm.ErrRecordNotFound {
				logrus.Error("get deploy k8s service info error.")
			}
			if service != nil {
				err := s.KubeClient.Core().Services(p.TenantID).Delete(service.K8sServiceID, &metav1.DeleteOptions{})
				if err != nil {
					tx.Rollback()
					return nil, "", fmt.Errorf("delete deploy k8s service info from kube-api error.%s", err.Error())
				}
				err = db.GetManager().K8sServiceDaoTransactions(tx).DeleteK8sServiceByName(service.K8sServiceID)
				if err != nil {
					tx.Rollback()
					return nil, "", fmt.Errorf("delete deploy k8s service info from db error")
				}
			}
			if hasUpStream {
				pluginPort, err := db.GetManager().TenantServicesStreamPluginPortDao().GetPluginMappingPortByServiceIDAndContainerPort(
					serviceID,
					dbmodel.UpNetPlugin,
					port,
				)
				if err != nil {
					if err.Error() == gorm.ErrRecordNotFound.Error() {
						logrus.Debugf("outer, plugin port (%d) is not exist, do not need delete", port)
						goto OUTERCLOSEPASS
					}
					tx.Rollback()
					return nil, "", fmt.Errorf("outer, get plugin mapping port error:(%s)", err)
				}
				if p.IsInnerService {
					//发现内网未关闭则不删除该映射
					logrus.Debugf("outer, close outer, but plugin inner port (%d) is exist, do not need delete", port)
					goto OUTERCLOSEPASS
				}
				if err := db.GetManager().TenantServicesStreamPluginPortDaoTransactions(tx).DeletePluginMappingPortByContainerPort(
					serviceID,
					dbmodel.UpNetPlugin,
					port,
				); err != nil {
					tx.Rollback()
					return nil, "", fmt.Errorf("outer, delete plugin mapping port %d error:(%s)", port, err)
				}
				logrus.Debugf(fmt.Sprintf("outer, delete plugin port %d->%d", port, pluginPort.PluginPort))
			OUTERCLOSEPASS:
			}
			if err := tx.Commit().Error; err != nil {
				tx.Rollback()
				//删除已创建的SERVICE
				if k8sService != nil {
					s.KubeClient.Core().Services(k8sService.Namespace).Delete(k8sService.Name, &metav1.DeleteOptions{})
				}
				return nil, "", err
			}
		} else {
			return nil, "", nil
		}

	case "open":
		if p.IsOuterService {
			if p.Protocol != "http" && p.Protocol != "https" {
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
		if p.Protocol != "http" && p.Protocol != "https" {
			vsPort, err = s.createVSPort(serviceID, p.ContainerPort)
			if vsPort == nil {
				tx.Rollback()
				return nil, "", fmt.Errorf("create or get vs map port for service error,%s", err.Error())
			}
		}
		deploy, _ := db.GetManager().K8sDeployReplicationDao().GetK8sCurrentDeployReplicationByService(serviceID)
		if deploy != nil {
			k8sService, err = s.createOuterK8sService(tenantName, vsPort, service, p, deploy)
			if err != nil && !strings.HasSuffix(err.Error(), "is exist") {
				tx.Rollback()
				return nil, "", fmt.Errorf("create k8s service error,%s", err.Error())
			}
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
			logrus.Debugf("outer, set plugin mapping port %d->%d", port, pPort)
		}
		if err := tx.Commit().Error; err != nil {
			tx.Rollback()
			//删除已创建的SERVICE
			if k8sService != nil {
				s.KubeClient.Core().Services(k8sService.Namespace).Delete(k8sService.Name, &metav1.DeleteOptions{})
			}
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
func (s *ServiceAction) createOuterK8sService(tenantName string, mapPort *dbmodel.TenantServiceLBMappingPort, tenantservice *dbmodel.TenantServices, port *dbmodel.TenantServicesPort, deploy *dbmodel.K8sDeployReplication) (*v1.Service, error) {
	var service v1.Service
	service.Name = fmt.Sprintf("service-%d-%dout", port.ID, port.ContainerPort)
	service.Labels = map[string]string{
		"service_type":     "outer",
		"name":             tenantservice.ServiceAlias + "ServiceOUT",
		"tenant_name":      tenantName,
		"services_version": tenantservice.ServiceVersion,
		"domain":           tenantservice.Autodomain(tenantName, port.ContainerPort),
		"protocol":         port.Protocol,
		"ca":               "",
		"key":              "",
		"event_id":         tenantservice.EventID,
	}
	//TODO: "stream" to ! http
	if port.Protocol != "http" && mapPort != nil { //stream 协议获取映射端口
		service.Labels["lbmap_port"] = fmt.Sprintf("%d", mapPort.Port)
	}
	var servicePort v1.ServicePort
	if port.Protocol == "udp" {
		servicePort.Protocol = "UDP"
	} else {
		servicePort.Protocol = "TCP"
	}
	servicePort.TargetPort = intstr.FromInt(port.ContainerPort)
	servicePort.Port = int32(port.ContainerPort)
	var portType v1.ServiceType
	if os.Getenv("CUR_NET") == "midonet" {
		portType = v1.ServiceTypeNodePort
	} else {
		portType = v1.ServiceTypeClusterIP
	}
	spec := v1.ServiceSpec{
		Ports:    []v1.ServicePort{servicePort},
		Selector: map[string]string{"name": tenantservice.ServiceAlias},
		Type:     portType,
	}
	service.Spec = spec
	k8sService, err := s.KubeClient.Core().Services(tenantservice.TenantID).Create(&service)
	if err != nil && !strings.HasSuffix(err.Error(), "already exists") {
		return nil, err
	}
	if err := db.GetManager().K8sServiceDao().AddModel(&dbmodel.K8sService{
		TenantID:        tenantservice.TenantID,
		ServiceID:       tenantservice.ServiceID,
		K8sServiceID:    service.Name,
		ContainerPort:   port.ContainerPort,
		ReplicationID:   deploy.ReplicationID,
		ReplicationType: deploy.ReplicationType,
		IsOut:           true,
	}); err != nil {
		if !strings.HasSuffix(err.Error(), "is exist") {
			s.KubeClient.Core().Services(tenantservice.TenantID).Delete(k8sService.Name, &metav1.DeleteOptions{})
			return nil, err
		}
	}
	return k8sService, nil
}

func (s *ServiceAction) createInnerService(tenantservice *dbmodel.TenantServices, port *dbmodel.TenantServicesPort, deploy *dbmodel.K8sDeployReplication) (*v1.Service, error) {
	var service v1.Service
	service.Name = fmt.Sprintf("service-%d-%d", port.ID, port.ContainerPort)
	service.Labels = map[string]string{
		"service_type": "inner",
		"name":         tenantservice.ServiceAlias + "Service",
	}
	var servicePort v1.ServicePort
	if port.Protocol == "udp" {
		servicePort.Protocol = "UDP"
	} else {
		servicePort.Protocol = "TCP"
	}
	servicePort.TargetPort = intstr.FromInt(port.ContainerPort)
	servicePort.Port = int32(port.MappingPort)
	if servicePort.Port == 0 {
		servicePort.Port = int32(port.ContainerPort)
	}
	spec := v1.ServiceSpec{
		Ports:    []v1.ServicePort{servicePort},
		Selector: map[string]string{"name": tenantservice.ServiceAlias},
	}
	service.Spec = spec
	k8sService, err := s.KubeClient.Core().Services(tenantservice.TenantID).Create(&service)
	if err != nil && !strings.HasSuffix(err.Error(), "already exists") {
		return nil, err
	}
	if err := db.GetManager().K8sServiceDao().AddModel(&dbmodel.K8sService{
		TenantID:        tenantservice.TenantID,
		ServiceID:       tenantservice.ServiceID,
		K8sServiceID:    service.Name,
		ContainerPort:   port.ContainerPort,
		ReplicationID:   deploy.ReplicationID,
		ReplicationType: deploy.ReplicationType,
		IsOut:           false,
	}); err != nil {
		if !strings.HasSuffix(err.Error(), "is exist") {
			s.KubeClient.Core().Services(tenantservice.TenantID).Delete(k8sService.Name, &metav1.DeleteOptions{})
			return nil, err
		}
	}
	return k8sService, nil
}

//PortInner 端口对内服务操作
func (s *ServiceAction) PortInner(tenantName, serviceID, operation string, port int) error {
	p, err := db.GetManager().TenantServicesPortDao().GetPort(serviceID, port)
	if err != nil {
		return err
	}
	service, err := db.GetManager().TenantServiceDao().GetServiceByID(serviceID)
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
	var k8sService *v1.Service
	tx := db.GetManager().Begin()
	switch operation {
	case "close":
		if p.IsInnerService { //如果端口已经开了对内
			p.IsInnerService = false
			if err = db.GetManager().TenantServicesPortDaoTransactions(tx).UpdateModel(p); err != nil {
				tx.Rollback()
				return fmt.Errorf("update service port error: %s", err.Error())
			}
			service, err := db.GetManager().K8sServiceDao().GetK8sService(serviceID, port, false)
			if err != nil && err != gorm.ErrRecordNotFound {
				logrus.Error("get deploy k8s service info error.", err.Error())
			}
			if service != nil {
				err := s.KubeClient.Core().Services(p.TenantID).Delete(service.K8sServiceID, &metav1.DeleteOptions{})
				if err != nil && !strings.HasSuffix(err.Error(), "not found") {
					tx.Rollback()
					return fmt.Errorf("delete deploy k8s service info from kube-api error")
				}
				err = db.GetManager().K8sServiceDao().DeleteK8sServiceByName(service.K8sServiceID)
				if err != nil {
					tx.Rollback()
					return fmt.Errorf("delete deploy k8s service info from db error")
				}
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
		deploy, _ := db.GetManager().K8sDeployReplicationDao().GetK8sCurrentDeployReplicationByService(serviceID)
		if deploy != nil {
			k8sService, err = s.createInnerService(service, p, deploy)
			if err != nil {
				tx.Rollback()
				return fmt.Errorf("create k8s service error,%s", err.Error())
			}

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
		//删除已创建的SERVICE
		if k8sService != nil {
			s.KubeClient.Core().Services(k8sService.Namespace).Delete(k8sService.Name, &metav1.DeleteOptions{})
		}
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
func (s *ServiceAction) VolumnVar(tsv *dbmodel.TenantServiceVolume, tenantID, action string) *util.APIHandleError {
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
		if err := db.GetManager().TenantServiceVolumeDao().AddModel(tsv); err != nil {
			return util.CreateAPIHandleErrorFromDBError("add volume", err)
		}
	case "delete":
		if tsv.VolumeName != "" {
			err := db.GetManager().TenantServiceVolumeDao().DeleteModel(tsv.ServiceID, tsv.VolumeName)
			if err != nil && err.Error() != gorm.ErrRecordNotFound.Error() {
				return util.CreateAPIHandleErrorFromDBError("delete volume", err)
			}
		} else {
			if err := db.GetManager().TenantServiceVolumeDao().DeleteByServiceIDAndVolumePath(tsv.ServiceID, tsv.VolumePath); err != nil && err.Error() != gorm.ErrRecordNotFound.Error() {
				return util.CreateAPIHandleErrorFromDBError("delete volume", err)
			}
		}
	}
	return nil
}

//GetVolumes 获取应用全部存储
func (s *ServiceAction) GetVolumes(serviceID string) ([]*dbmodel.TenantServiceVolume, *util.APIHandleError) {
	dbManager := db.GetManager()
	service, err := dbManager.TenantServiceDao().GetServiceByID(serviceID)
	if err != nil {
		return nil, util.CreateAPIHandleErrorFromDBError("get service", err)
	}
	vs, err := dbManager.TenantServiceVolumeDao().GetTenantServiceVolumesByServiceID(serviceID)
	if err != nil && err.Error() != gorm.ErrRecordNotFound.Error() {
		return nil, util.CreateAPIHandleErrorFromDBError("get volumes", err)
	}
	if service.VolumePath != "" && service.VolumeMountPath != "" {
		vs = append(vs, &dbmodel.TenantServiceVolume{
			ServiceID:  serviceID,
			VolumeType: service.VolumeType,
			//VolumeName: service.VolumePath,
			VolumePath: service.VolumeMountPath,
			HostPath:   service.HostPath,
		})
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
func (s *ServiceAction) ServiceProbe(tsp *dbmodel.ServiceProbe, action string) error {
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
		TaskType:  "restart",
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
	ns := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:         t.UUID,
			GenerateName: t.Name,
		},
	}
	if _, err := s.KubeClient.Core().Namespaces().Create(ns); err != nil {
		if !strings.HasSuffix(err.Error(), "already exists") {
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

type K8sPodInfo struct {
	ServiceID string `json:"service_id"`
	//部署资源的ID ,例如rc ,deploment, statefulset
	ReplicationID   string                       `json:"rc_id"`
	ReplicationType string                       `json:"rc_type"`
	PodName         string                       `json:"pod_name"`
	PodIP           string                       `json:"pod_ip"`
	Container       map[string]map[string]string `json:"container"`
}

//GetPods get pods
func (s *ServiceAction) GetPods(serviceID string) ([]K8sPodInfo, error) {
	var podsInfoList []K8sPodInfo
	pods, err := db.GetManager().K8sPodDao().GetPodByService(serviceID)
	if err != nil {
		logrus.Error("GetPodByService Error:", err)
		return nil, err
	}
	logrus.Info("pods：", pods)
	for _, v := range pods {
		var podInfo K8sPodInfo
		containerMemory := make(map[string]map[string]string, 10)
		podInfo.ServiceID = v.ServiceID
		podInfo.ReplicationID = v.ReplicationID
		podInfo.ReplicationType = v.ReplicationType
		podInfo.PodName = v.PodName
		podInfo.PodIP = v.PodIP
		memoryUsageQuery := fmt.Sprintf(`container_memory_usage_bytes{pod_name="%s"}`, v.PodName)
		memoryUsageMap, _ := s.GetContainerMemory(memoryUsageQuery)
		logrus.Info("memoryUsageMap", memoryUsageMap)
		for k, val := range memoryUsageMap {
			if _,ok := containerMemory[k];!ok{
				containerMemory[k] = map[string]string{"memory_usage": val}
			}
		}
		memorylimitQuery := fmt.Sprintf(`container_spec_memory_limit_bytes{pod_name="%s"}`, v.PodName)
		memoryLimitMap, _ := s.GetContainerMemory(memorylimitQuery)
		logrus.Info("memoryLimitMap", memoryLimitMap)
		for k2, v2 := range memoryLimitMap {
			if val, ok := containerMemory[k2]; ok {
				val["memory_limit"] = v2
			}
		}
		podInfo.Container = containerMemory
		podsInfoList = append(podsInfoList, podInfo)

	}
	return podsInfoList, nil
}

// Use Prometheus to query memory resources
func (s *ServiceAction) GetContainerMemory(query string) (map[string]string, error) {
	memoryUsageMap := make(map[string]string, 10)
	proxy := GetPrometheusProxy()
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
				var containerName string
				var valuesBytes string
				if cname, ok := re["metric"].(map[string]interface{}); ok {
					containerName = cname["container_name"].(string)
				} else {
					logrus.Info("metric decode error")
				}
				if val, ok := (re["value"]).([]interface{}); ok && len(val) == 2 {
					valuesBytes = val[1].(string)
				} else {
					logrus.Info("value decode error")
				}
				memoryUsageMap[containerName] = valuesBytes
			}
			return memoryUsageMap, nil
		} else {
			logrus.Error("Deserialization failed")
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
	//此处的原因，必须使用golang 1.8 以上版本编译
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
	//删除clear net bridge
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
	//删除tenant_service_pod
	if err := db.GetManager().K8sPodDaoTransactions(tx).DeleteK8sPod(serviceID); err != nil {
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
	//删除应用状态
	if db.GetManager().TenantServiceStatusDaoTransactions(tx).DeleteByServiceID(serviceID); err != nil {
		if err.Error() != gorm.ErrRecordNotFound.Error() {
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

//GetPodList Get pod list
func GetPodList(namespace, serviceAlias string, cli *kubernetes.Clientset) (*v1.PodList, error) {
	labelname := fmt.Sprintf("name=%v", serviceAlias)
	pods, err := cli.CoreV1().Pods(namespace).List(metav1.ListOptions{LabelSelector: labelname})
	if err != nil {
		return nil, err
	}
	return pods, err
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
