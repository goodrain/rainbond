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
	"encoding/json"
	"fmt"
	"github.com/goodrain/rainbond/util/constants"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/goodrain/rainbond/api/client/prometheus"
	api_model "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/api/util"
	"github.com/goodrain/rainbond/api/util/bcode"
	"github.com/goodrain/rainbond/api/util/license"
	"github.com/goodrain/rainbond/builder/parser"
	"github.com/goodrain/rainbond/cmd/api/option"
	"github.com/goodrain/rainbond/db"
	dberr "github.com/goodrain/rainbond/db/errors"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/event"
	gclient "github.com/goodrain/rainbond/mq/client"
	"github.com/goodrain/rainbond/pkg/generated/clientset/versioned"
	core_util "github.com/goodrain/rainbond/util"
	"github.com/goodrain/rainbond/worker/client"
	"github.com/goodrain/rainbond/worker/discover/model"
	"github.com/goodrain/rainbond/worker/server"
	"github.com/goodrain/rainbond/worker/server/pb"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"
	"github.com/pquerna/ffjson/ffjson"
	"github.com/sirupsen/logrus"
	"github.com/twinj/uuid"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/util/flushwriter"
	"k8s.io/client-go/kubernetes"
)

// ErrServiceNotClosed -
var ErrServiceNotClosed = errors.New("Service has not been closed")

//ServiceAction service act
type ServiceAction struct {
	MQClient       gclient.MQClient
	EtcdCli        *clientv3.Client
	statusCli      *client.AppRuntimeSyncClient
	prometheusCli  prometheus.Interface
	conf           option.Config
	rainbondClient versioned.Interface
	kubeClient     kubernetes.Interface
}

type dCfg struct {
	Type     string   `json:"type"`
	Servers  []string `json:"servers"`
	Key      string   `json:"key"`
	Username string   `json:"username"`
	Password string   `json:"password"`
}

//CreateManager create Manger
func CreateManager(conf option.Config,
	mqClient gclient.MQClient,
	etcdCli *clientv3.Client,
	statusCli *client.AppRuntimeSyncClient,
	prometheusCli prometheus.Interface,
	rainbondClient versioned.Interface,
	kubeClient kubernetes.Interface) *ServiceAction {
	return &ServiceAction{
		MQClient:       mqClient,
		EtcdCli:        etcdCli,
		statusCli:      statusCli,
		conf:           conf,
		prometheusCli:  prometheusCli,
		rainbondClient: rainbondClient,
		kubeClient:     kubeClient,
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
	defer func() {
		if r := recover(); r != nil {
			logrus.Errorf("Unexpected panic occurred, rollback transaction: %v", r)
			tx.Rollback()
		}
	}()
	//V5.2: do not support service type label
	for _, label := range l.Labels {
		labelModel := dbmodel.TenantServiceLable{
			ServiceID:  serviceID,
			LabelKey:   label.LabelKey,
			LabelValue: label.LabelValue,
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
	defer func() {
		if r := recover(); r != nil {
			logrus.Errorf("Unexpected panic occurred, rollback transaction: %v", r)
			tx.Rollback()
		}
	}()
	for _, label := range l.Labels {
		// delete old labels
		err := db.GetManager().TenantServiceLabelDaoTransactions(tx).
			DelTenantServiceLabelsByServiceIDKey(serviceID, label.LabelKey)
		if err != nil {
			logrus.Errorf("error deleting old labels: %v", err)
			tx.Rollback()
			return err
		}
		// V5.2 do not support service type label
		// add new labels
		labelModel := dbmodel.TenantServiceLable{
			ServiceID:  serviceID,
			LabelKey:   label.LabelKey,
			LabelValue: label.LabelValue,
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
	defer func() {
		if r := recover(); r != nil {
			logrus.Errorf("Unexpected panic occurred, rollback transaction: %v", r)
			tx.Rollback()
		}
	}()
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
func (s *ServiceAction) ServiceVertical(ctx context.Context, vs *model.VerticalScalingTaskBody) error {
	service, err := db.GetManager().TenantServiceDao().GetServiceByID(vs.ServiceID)
	if err != nil {
		logrus.Errorf("get service by id %s error, %s", vs.ServiceID, err)
		db.GetManager().ServiceEventDao().SetEventStatus(ctx, dbmodel.EventStatusFailure)
		return err
	}
	oldMemory := service.ContainerMemory
	oldCPU := service.ContainerCPU
	oldGPU := service.ContainerGPU
	var rollback = func() {
		service.ContainerMemory = oldMemory
		service.ContainerCPU = oldCPU
		service.ContainerGPU = oldGPU
		_ = db.GetManager().TenantServiceDao().UpdateModel(service)
	}
	if vs.ContainerCPU != nil {
		service.ContainerCPU = *vs.ContainerCPU
	}
	if vs.ContainerMemory != nil {
		service.ContainerMemory = *vs.ContainerMemory
	}
	if vs.ContainerGPU != nil {
		service.ContainerGPU = *vs.ContainerGPU
	}
	licenseInfo := license.ReadLicense()
	if licenseInfo == nil || !licenseInfo.HaveFeature("GPU") {
		service.ContainerGPU = 0
	}
	if service.ContainerMemory == oldMemory && service.ContainerCPU == oldCPU && service.ContainerGPU == oldGPU {
		db.GetManager().ServiceEventDao().SetEventStatus(ctx, dbmodel.EventStatusSuccess)
		return nil
	}
	err = db.GetManager().TenantServiceDao().UpdateModel(service)
	if err != nil {
		db.GetManager().ServiceEventDao().SetEventStatus(ctx, dbmodel.EventStatusFailure)
		logrus.Errorf("update service memory and cpu failure. %v", err)
		return fmt.Errorf("vertical service faliure:%s", err.Error())
	}
	err = s.MQClient.SendBuilderTopic(gclient.TaskStruct{
		TaskType: "vertical_scaling",
		TaskBody: vs,
		Topic:    gclient.WorkerTopic,
	})
	if err != nil {
		// roll back service
		rollback()
		logrus.Errorf("equque mq error, %v", err)
		db.GetManager().ServiceEventDao().SetEventStatus(ctx, dbmodel.EventStatusFailure)
		return err
	}
	logrus.Debugf("equeue mq vertical task success")
	return nil
}

//ServiceHorizontal Service Horizontal
func (s *ServiceAction) ServiceHorizontal(hs *model.HorizontalScalingTaskBody) error {
	service, err := db.GetManager().TenantServiceDao().GetServiceByID(hs.ServiceID)
	if err != nil {
		logrus.Errorf("get service by id %s error, %s", hs.ServiceID, err)
		return err
	}

	// for rollback database
	oldReplicas := service.Replicas
	pods, err := s.statusCli.GetServicePods(service.ServiceID)
	if err != nil {
		logrus.Errorf("get service pods error: %v", err)
		return fmt.Errorf("horizontal service faliure:%s", err.Error())
	}
	if int32(len(pods.NewPods)) == hs.Replicas {
		return bcode.ErrHorizontalDueToNoChange
	}

	service.Replicas = int(hs.Replicas)
	err = db.GetManager().TenantServiceDao().UpdateModel(service)
	if err != nil {
		logrus.Errorf("updtae service replicas failure. %v", err)
		return fmt.Errorf("horizontal service faliure:%s", err.Error())
	}

	var rollback = func() {
		service.Replicas = oldReplicas
		_ = db.GetManager().TenantServiceDao().UpdateModel(service)
	}

	err = s.MQClient.SendBuilderTopic(gclient.TaskStruct{
		TaskType: "horizontal_scaling",
		TaskBody: hs,
		Topic:    gclient.WorkerTopic,
	})
	if err != nil {
		// roll back service
		rollback()
		logrus.Errorf("equque mq error, %v", err)
		return err
	}

	// if send task success, return nil
	logrus.Debugf("enqueue mq horizontal task success")
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
	oldDeployVersion := services.DeployVersion
	var rollback = func() {
		services.DeployVersion = oldDeployVersion
		_ = db.GetManager().TenantServiceDao().UpdateModel(services)
	}
	if version.FinalStatus != "success" {
		logrus.Warnf("deploy version %s is not build success,can not change deploy version in this upgrade event", ru.NewDeployVersion)
	} else {
		services.DeployVersion = ru.NewDeployVersion
		err = db.GetManager().TenantServiceDao().UpdateModel(services)
		if err != nil {
			logrus.Errorf("update service deploy version error. %v", err)
			return fmt.Errorf("horizontal service faliure:%s", err.Error())
		}
	}
	err = s.MQClient.SendBuilderTopic(gclient.TaskStruct{
		TaskBody: ru,
		TaskType: "rolling_upgrade",
		Topic:    gclient.WorkerTopic,
	})
	if err != nil {
		// roll back service deploy version
		rollback()
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
	if ts.ContainerCPU < 0 {
		ts.ContainerCPU = 0
	}
	if ts.ContainerMemory < 0 {
		ts.ContainerMemory = 0
	}
	if ts.ContainerGPU < 0 {
		ts.ContainerGPU = 0
	}
	if ts.K8sComponentName != "" {
		if db.GetManager().TenantServiceDao().IsK8sComponentNameDuplicate(ts.AppID, ts.ServiceID, ts.K8sComponentName) {
			return bcode.ErrK8sComponentNameExists
		}
	}
	ts.UpdateTime = time.Now()
	var (
		ports         = sc.PortsInfo
		envs          = sc.EnvsInfo
		volumns       = sc.VolumesInfo
		dependVolumes = sc.DepVolumesInfo
		dependIds     = sc.DependIDs
		probes        = sc.ComponentProbes
		monitors      = sc.ComponentMonitors
		httpRules     = sc.HTTPRules
		tcpRules      = sc.TCPRules
	)
	ts.AppID = sc.AppID
	ts.DeployVersion = ""
	tx := db.GetManager().Begin()
	defer func() {
		if r := recover(); r != nil {
			logrus.Errorf("Unexpected panic occurred, rollback transaction: %v", r)
			tx.Rollback()
		}
	}()
	//create app
	if err := db.GetManager().TenantServiceDaoTransactions(tx).AddModel(&ts); err != nil {
		logrus.Errorf("add service error, %v", err)
		tx.Rollback()
		return err
	}
	//set app envs
	if len(envs) > 0 {
		var batchEnvs []*dbmodel.TenantServiceEnvVar
		for _, env := range envs {
			env := env
			env.ServiceID = ts.ServiceID
			env.TenantID = ts.TenantID
			batchEnvs = append(batchEnvs, &env)
		}
		if err := db.GetManager().TenantServiceEnvVarDaoTransactions(tx).CreateOrUpdateEnvsInBatch(batchEnvs); err != nil {
			logrus.Errorf("batch add env error, %v", err)
			tx.Rollback()
			return err
		}
	}
	//set app port
	if len(ports) > 0 {
		var batchPorts []*dbmodel.TenantServicesPort
		for _, port := range ports {
			port := port
			port.ServiceID = ts.ServiceID
			port.TenantID = ts.TenantID
			batchPorts = append(batchPorts, &port)
		}
		if err := db.GetManager().TenantServicesPortDaoTransactions(tx).CreateOrUpdatePortsInBatch(batchPorts); err != nil {
			logrus.Errorf("batch add port error, %v", err)
			tx.Rollback()
			return err
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
			v := dbmodel.TenantServiceVolume{
				ServiceID:      ts.ServiceID,
				Category:       volumn.Category,
				VolumeType:     volumn.VolumeType,
				VolumeName:     volumn.VolumeName,
				HostPath:       volumn.HostPath,
				VolumePath:     volumn.VolumePath,
				IsReadOnly:     volumn.IsReadOnly,
				VolumeCapacity: volumn.VolumeCapacity,
				// AccessMode 读写模式（Important! A volume can only be mounted using one access mode at a time, even if it supports many. For example, a GCEPersistentDisk can be mounted as ReadWriteOnce by a single node or ReadOnlyMany by many nodes, but not at the same time. #https://kubernetes.io/docs/concepts/storage/persistent-volumes/#access-modes）
				AccessMode: volumn.AccessMode,
				// SharePolicy 共享模式
				SharePolicy: volumn.SharePolicy,
				// BackupPolicy 备份策略
				BackupPolicy: volumn.BackupPolicy,
				// ReclaimPolicy 回收策略
				ReclaimPolicy: volumn.ReclaimPolicy,
				// AllowExpansion 是否支持扩展
				AllowExpansion: volumn.AllowExpansion,
				// VolumeProviderName 使用的存储驱动别名
				VolumeProviderName: volumn.VolumeProviderName,
			}
			v.ServiceID = ts.ServiceID
			if volumn.VolumeType == "" {
				v.VolumeType = dbmodel.ShareFileVolumeType.String()
			}
			if volumn.HostPath == "" {
				//step 1 设置主机目录
				switch volumn.VolumeType {
				//共享文件存储
				case dbmodel.ShareFileVolumeType.String():
					v.HostPath = fmt.Sprintf("%s/tenant/%s/service/%s%s", sharePath, sc.TenantID, ts.ServiceID, volumn.VolumePath)
				//本地文件存储
				case dbmodel.LocalVolumeType.String():
					if !dbmodel.ServiceType(sc.ExtendMethod).IsState() {
						tx.Rollback()
						return util.CreateAPIHandleError(400, fmt.Errorf("local volume type only support state component"))
					}
					v.HostPath = fmt.Sprintf("%s/tenant/%s/service/%s%s", localPath, sc.TenantID, ts.ServiceID, volumn.VolumePath)
				case dbmodel.ConfigFileVolumeType.String(), dbmodel.MemoryFSVolumeType.String():
					logrus.Debug("simple volume type : ", volumn.VolumeType)
				default:
					if !dbmodel.ServiceType(sc.ExtendMethod).IsState() {
						tx.Rollback()
						return util.CreateAPIHandleError(400, fmt.Errorf("custom volume type only support state component"))
					}
				}
			}
			if volumn.VolumeName == "" {
				v.VolumeName = uuid.NewV4().String()
			}
			if err := db.GetManager().TenantServiceVolumeDaoTransactions(tx).AddModel(&v); err != nil {
				logrus.Errorf("add volumn %v error, %v", volumn.HostPath, err)
				tx.Rollback()
				return err
			}
			if volumn.FileContent != "" {
				cf := &dbmodel.TenantServiceConfigFile{
					ServiceID:   sc.ServiceID,
					VolumeName:  volumn.VolumeName,
					FileContent: volumn.FileContent,
				}
				if err := db.GetManager().TenantServiceConfigFileDaoTransactions(tx).AddModel(cf); err != nil {
					tx.Rollback()
					return util.CreateAPIHandleErrorFromDBError("error creating config file", err)
				}
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
			depVolume.VolumeType = volume.VolumeType
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
	if sc.OSType == "windows" {
		if err := db.GetManager().TenantServiceLabelDaoTransactions(tx).AddModel(&dbmodel.TenantServiceLable{
			ServiceID:  ts.ServiceID,
			LabelKey:   dbmodel.LabelKeyNodeSelector,
			LabelValue: sc.OSType,
		}); err != nil {
			logrus.Errorf("add label %s=%s  %v error, %v", dbmodel.LabelKeyNodeSelector, sc.OSType, ts.ServiceID, err)
			tx.Rollback()
			return err
		}
	}
	// sc.Endpoints can't be nil
	// sc.Endpoints.Discovery or sc.Endpoints.Static can't be nil
	if sc.Kind == dbmodel.ServiceKindThirdParty.String() { // TODO: validate request data
		if sc.Endpoints == nil {
			tx.Rollback()
			return fmt.Errorf("endpoints can not be empty for third-party service")
		}
		if sc.Endpoints.Kubernetes != nil {
			c := &dbmodel.ThirdPartySvcDiscoveryCfg{
				ServiceID:   sc.ServiceID,
				Type:        string(dbmodel.DiscorveryTypeKubernetes),
				Namespace:   sc.Endpoints.Kubernetes.Namespace,
				ServiceName: sc.Endpoints.Kubernetes.ServiceName,
			}
			if err := db.GetManager().ThirdPartySvcDiscoveryCfgDaoTransactions(tx).
				AddModel(c); err != nil {
				logrus.Errorf("error saving discover center configuration: %v", err)
				tx.Rollback()
				return err
			}
		}
		if sc.Endpoints.Static != nil {
			for _, o := range sc.Endpoints.Static {
				ep := &dbmodel.Endpoint{
					ServiceID: sc.ServiceID,
					UUID:      core_util.NewUUID(),
				}
				address := o
				port := 0
				prefix := ""
				if strings.HasPrefix(address, "https://") {
					address = strings.Split(address, "https://")[1]
					prefix = "https://"
				}
				if strings.HasPrefix(address, "http://") {
					address = strings.Split(address, "http://")[1]
					prefix = "http://"
				}
				if strings.Contains(address, ":") {
					addressL := strings.Split(address, ":")
					address = addressL[0]
					port, _ = strconv.Atoi(addressL[1])
				}
				ep.IP = prefix + address
				ep.Port = port

				logrus.Debugf("add new endpoint: %v", ep)

				if err := db.GetManager().EndpointsDaoTransactions(tx).AddModel(ep); err != nil {
					tx.Rollback()
					logrus.Errorf("error saving o endpoint: %v", err)
					return err
				}
			}
		}
	}
	if len(probes) > 0 {
		for _, pb := range probes {
			probe := s.convertProbeModel(&pb, ts.ServiceID)
			if err := db.GetManager().ServiceProbeDaoTransactions(tx).AddModel(probe); err != nil {
				logrus.Errorf("add probe %v error, %v", probe.ProbeID, err)
				tx.Rollback()
				return err
			}
		}
	}
	if len(monitors) > 0 {
		for _, m := range monitors {
			monitor := dbmodel.TenantServiceMonitor{
				Name:            m.Name,
				TenantID:        ts.TenantID,
				ServiceID:       ts.ServiceID,
				ServiceShowName: m.ServiceShowName,
				Port:            m.Port,
				Path:            m.Path,
				Interval:        m.Interval,
			}
			if err := db.GetManager().TenantServiceMonitorDaoTransactions(tx).AddModel(&monitor); err != nil {
				logrus.Errorf("add monitor %v error, %v", monitor.Name, err)
				tx.Rollback()
				return err
			}
		}
	}
	if len(httpRules) > 0 {
		for _, httpRule := range httpRules {
			if err := GetGatewayHandler().CreateHTTPRule(tx, &httpRule); err != nil {
				logrus.Errorf("add service http rule error %v", err)
				tx.Rollback()
				return err
			}
		}
	}
	if len(tcpRules) > 0 {
		for _, tcpRule := range tcpRules {
			if GetGatewayHandler().TCPIPPortExists(tcpRule.IP, tcpRule.Port) {
				logrus.Debugf("tcp rule %v:%v exists", tcpRule.IP, tcpRule.Port)
				continue
			}
			if err := GetGatewayHandler().CreateTCPRule(tx, &tcpRule); err != nil {
				logrus.Errorf("add service tcp rule error %v", err)
				tx.Rollback()
				return err
			}
		}
	}
	labelModel := dbmodel.TenantServiceLable{
		ServiceID:  ts.ServiceID,
		LabelKey:   dbmodel.LabelKeyServiceType,
		LabelValue: core_util.StatelessServiceType,
	}
	if ts.IsState() {
		labelModel.LabelValue = core_util.StatefulServiceType
	}
	if err := db.GetManager().TenantServiceLabelDaoTransactions(tx).AddModel(&labelModel); err != nil {
		tx.Rollback()
		return err
	}

	// TODO: create default probe for third-party service.
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return err
	}
	logrus.Debugf("create a new app %s success", ts.ServiceAlias)

	return nil
}

func (s *ServiceAction) convertProbeModel(req *api_model.ServiceProbe, serviceID string) *dbmodel.TenantServiceProbe {
	return &dbmodel.TenantServiceProbe{
		ServiceID:          serviceID,
		Cmd:                req.Cmd,
		FailureThreshold:   req.FailureThreshold,
		HTTPHeader:         req.HTTPHeader,
		InitialDelaySecond: req.InitialDelaySecond,
		IsUsed:             &req.IsUsed,
		Mode:               req.Mode,
		Path:               req.Path,
		PeriodSecond:       req.PeriodSecond,
		Port:               req.Port,
		ProbeID:            req.ProbeID,
		Scheme:             req.Scheme,
		SuccessThreshold:   req.SuccessThreshold,
		TimeoutSecond:      req.TimeoutSecond,
		FailureAction:      req.FailureAction,
	}
}

//ServiceUpdate update service
func (s *ServiceAction) ServiceUpdate(sc map[string]interface{}) error {
	ts, err := db.GetManager().TenantServiceDao().GetServiceByID(sc["service_id"].(string))
	if err != nil {
		return err
	}
	if memory, ok := sc["container_memory"].(int); ok && memory >= 0 {
		ts.ContainerMemory = memory
	}
	if cpu, ok := sc["container_cpu"].(int); ok && cpu >= 0 {
		ts.ContainerCPU = cpu
	}
	if gpu, ok := sc["container_gpu"].(int); ok {
		ts.ContainerCPU = gpu
	}
	if name, ok := sc["service_name"].(string); ok && name != "" {
		ts.ServiceName = name
	}
	if appID, ok := sc["app_id"].(string); ok && appID != "" {
		ts.AppID = appID
	}
	if k8sComponentName, ok := sc["k8s_component_name"].(string); ok && k8sComponentName != "" {
		if db.GetManager().TenantServiceDao().IsK8sComponentNameDuplicate(ts.AppID, ts.ServiceID, k8sComponentName) {
			return bcode.ErrK8sComponentNameExists
		}
		ts.K8sComponentName = k8sComponentName
	}
	if sc["extend_method"] != nil {
		extendMethod := sc["extend_method"].(string)
		ts.ExtendMethod = extendMethod
		// if component replicas is more than 1, so can't change service type to singleton
		if ts.Replicas > 1 && ts.IsSingleton() {
			err := fmt.Errorf("service[%s] replicas > 1, can't change service typ to stateless_singleton", ts.ServiceAlias)
			return err
		}
		volumes, err := db.GetManager().TenantServiceVolumeDao().GetTenantServiceVolumesByServiceID(ts.ServiceID)
		if err != nil {
			return err
		}
		for _, vo := range volumes {
			if vo.VolumeType == dbmodel.ShareFileVolumeType.String() || vo.VolumeType == dbmodel.MemoryFSVolumeType.String() {
				continue
			}
			if vo.VolumeType == dbmodel.LocalVolumeType.String() && !ts.IsState() {
				err := fmt.Errorf("service[%s] has local volume type, can't change type to stateless", ts.ServiceAlias)
				return err
			}
			// if component use volume, what it accessMode is rwo, can't change volume type to stateless
			if vo.AccessMode == "RWO" && !ts.IsState() {
				err := fmt.Errorf("service[%s] volume[%s] access_mode is RWO, can't change type to stateless", ts.ServiceAlias, vo.VolumeName)
				return err
			}
		}
		ts.ExtendMethod = extendMethod
		ts.ServiceType = extendMethod
	}
	if js, ok := sc["job_strategy"].(string); ok {
		ts.JobStrategy = js
	}
	//update component
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

//GetServicesByAppID get service(s) by appID
func (s *ServiceAction) GetServicesByAppID(appID string, page, pageSize int) (*api_model.ListServiceResponse, error) {
	var resp api_model.ListServiceResponse
	services, total, err := db.GetManager().TenantServiceDao().GetServicesInfoByAppID(appID, page, pageSize)
	if err != nil {
		logrus.Errorf("get service by application id error, %v, %v", services, err)
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
	if services != nil {
		resp.Services = services
	} else {
		resp.Services = make([]*dbmodel.TenantServices, 0)
	}

	resp.Page = page
	resp.Total = total
	resp.PageSize = pageSize
	return &resp, nil
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
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		defer core_util.Elapsed("[ServiceAction] get tenant resource")()
	}

	tenant, err := db.GetManager().TenantDao().GetTenantByUUID(uuid)
	if err != nil {
		logrus.Errorf("get tenant %s info failure %v", uuid, err.Error())
		return nil, err
	}
	services, err := db.GetManager().TenantServiceDao().GetServicesByTenantID(uuid)
	if err != nil {
		logrus.Errorf("get service by id error, %v, %v", services, err.Error())
		return nil, err
	}
	var serviceIDs string
	var AllocatedCPU, AllocatedMEM int
	for _, ser := range services {
		if serviceIDs == "" {
			serviceIDs += ser.ServiceID
		} else {
			serviceIDs += "," + ser.ServiceID
		}
		AllocatedCPU += ser.ContainerCPU * ser.Replicas
		AllocatedMEM += ser.ContainerMemory * ser.Replicas
	}
	tenantResUesd, err := s.statusCli.GetTenantResource(uuid)
	if err != nil {
		logrus.Errorf("get tenant %s resource failure %s", uuid, err.Error())
	}
	disks := GetServicesDiskDeprecated(strings.Split(serviceIDs, ","), s.prometheusCli)
	var value float64
	for _, v := range disks {
		value += v
	}
	var res api_model.TenantResource
	res.UUID = uuid
	res.Name = tenant.Name
	res.EID = tenant.EID
	res.AllocatedCPU = AllocatedCPU
	res.AllocatedMEM = AllocatedMEM
	if tenantResUesd != nil {
		res.UsedCPU = int(tenantResUesd.CpuRequest)
		res.UsedMEM = int(tenantResUesd.MemoryRequest)
	}
	res.UsedDisk = value
	return &res, nil
}

// // GetTenantMemoryCPU get pagedTenantServiceRes(s)
// func (s *ServiceAction) GetAllocableResources(tenantID string) (*api_model.TenantResource, error) {
// 	if logrus.IsLevelEnabled(logrus.DebugLevel) {
// 		defer core_util.Elapsed("[ServiceAction] get allocable resources")()
// 	}

// 	tenant, err := db.GetManager().TenantDao().GetTenantByUUID(tenantID)
// 	if err != nil {
// 		return nil, err
// 	}

// 	services, err := db.GetManager().TenantServiceDao().GetServicesByTenantID(tenantID)
// 	if err != nil {
// 		logrus.Errorf("get service by id error, %v, %v", services, err.Error())
// 		return nil, err
// 	}

// 	var serviceIDs string
// 	var allocatedCPU, allocatedMEM int
// 	for _, svc := range services {
// 		allocatedCPU += svc.ContainerCPU * svc.Replicas
// 		allocatedMEM += svc.ContainerMemory * svc.Replicas
// 	}
// 	usedResource, err := s.statusCli.GetTenantResource(tenantID)
// 	if err != nil {
// 		return nil, err
// 	}

// 	return &res, nil
// }

//GetServicesDiskDeprecated get service disk
//
// Deprecated
func GetServicesDiskDeprecated(ids []string, prometheusCli prometheus.Interface) map[string]float64 {
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		defer core_util.Elapsed("[GetServicesDiskDeprecated] get tenant resource")()
	}

	result := make(map[string]float64)
	//query disk used in prometheus
	query := fmt.Sprintf(`max(app_resource_appfs{service_id=~"%s"}) by(service_id)`, strings.Join(ids, "|"))
	metric := prometheusCli.GetMetric(query, time.Now())
	for _, re := range metric.MetricData.MetricValues {
		var serviceID = re.Metadata["service_id"]
		if re.Sample != nil {
			result[serviceID] = re.Sample.Value()
		}
	}
	return result
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
			if err == dberr.ErrRecordAlreadyExist {
				return nil
			}
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

// CreatePorts -
func (s *ServiceAction) CreatePorts(tenantID, serviceID string, vps *api_model.ServicePorts) error {
	tx := db.GetManager().Begin()
	defer func() {
		if r := recover(); r != nil {
			logrus.Errorf("Unexpected panic occurred, rollback transaction: %v", r)
			tx.Rollback()
		}
	}()

	for _, vp := range vps.Port {
		// make sure K8sServiceName is unique
		if vp.K8sServiceName != "" {
			port, err := db.GetManager().TenantServicesPortDao().GetByTenantAndName(tenantID, vp.K8sServiceName)
			if err != nil && err != gorm.ErrRecordNotFound {
				tx.Rollback()
				return err
			}
			if port != nil && port.ServiceID != serviceID {
				tx.Rollback()
				return bcode.ErrK8sServiceNameExists
			}
		}

		var vpD dbmodel.TenantServicesPort
		vpD.ServiceID = serviceID
		vpD.TenantID = tenantID
		vpD.IsInnerService = &vp.IsInnerService
		vpD.IsOuterService = &vp.IsOuterService
		vpD.ContainerPort = vp.ContainerPort
		vpD.MappingPort = vp.MappingPort
		vpD.Protocol = vp.Protocol
		vpD.PortAlias = vp.PortAlias
		vpD.K8sServiceName = vp.K8sServiceName
		if err := db.GetManager().TenantServicesPortDaoTransactions(tx).AddModel(&vpD); err != nil {
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

func (s *ServiceAction) deletePorts(componentID string, ports *api_model.ServicePorts) error {
	return db.GetManager().DB().Transaction(func(tx *gorm.DB) error {
		for _, port := range ports.Port {
			if err := db.GetManager().TenantServicesPortDaoTransactions(tx).DeleteModel(componentID, port.ContainerPort); err != nil {
				return err
			}

			// delete related ingress rules
			if err := GetGatewayHandler().DeleteIngressRulesByComponentPort(tx, componentID, port.ContainerPort); err != nil {
				return err
			}
		}

		return nil
	})
}

// SyncComponentPorts -
func (s *ServiceAction) SyncComponentPorts(tx *gorm.DB, app *dbmodel.Application, components []*api_model.Component) error {
	var (
		componentIDs []string
		ports        []*dbmodel.TenantServicesPort
	)
	for _, component := range components {
		if component.Ports == nil {
			continue
		}
		componentIDs = append(componentIDs, component.ComponentBase.ComponentID)
		for _, port := range component.Ports {
			ports = append(ports, port.DbModel(app.TenantID, component.ComponentBase.ComponentID))
		}
	}
	if err := db.GetManager().TenantServicesPortDaoTransactions(tx).DeleteByComponentIDs(componentIDs); err != nil {
		return err
	}
	return db.GetManager().TenantServicesPortDaoTransactions(tx).CreateOrUpdatePortsInBatch(ports)
}

//PortVar port var
func (s *ServiceAction) PortVar(action, tenantID, serviceID string, vps *api_model.ServicePorts, oldPort int) error {
	crt, err := db.GetManager().TenantServicePluginRelationDao().CheckSomeModelPluginByServiceID(
		serviceID,
		dbmodel.InBoundNetPlugin,
	)
	if err != nil {
		return err
	}
	switch action {
	case "delete":
		return s.deletePorts(serviceID, vps)
	case "update":
		tx := db.GetManager().Begin()
		defer func() {
			if r := recover(); r != nil {
				logrus.Errorf("Unexpected panic occurred, rollback transaction: %v", r)
				tx.Rollback()
			}
		}()
		for _, vp := range vps.Port {
			//port更新单个请求
			oldPort = vp.ContainerPort
			vpD, err := db.GetManager().TenantServicesPortDao().GetPort(serviceID, oldPort)
			if err != nil {
				tx.Rollback()
				return err
			}
			// make sure K8sServiceName is unique
			if vp.K8sServiceName != "" {
				port, err := db.GetManager().TenantServicesPortDao().GetByTenantAndName(tenantID, vp.K8sServiceName)
				if err != nil && err != gorm.ErrRecordNotFound {
					tx.Rollback()
					return err
				}
				if port != nil && vpD.K8sServiceName != vp.K8sServiceName && port.ServiceID != serviceID {
					tx.Rollback()
					return bcode.ErrK8sServiceNameExists
				}
			}

			vpD.ServiceID = serviceID
			vpD.TenantID = tenantID
			vpD.IsInnerService = &vp.IsInnerService
			vpD.IsOuterService = &vp.IsOuterService
			vpD.ContainerPort = vp.ContainerPort
			vpD.MappingPort = vp.MappingPort
			vpD.Protocol = vp.Protocol
			vpD.PortAlias = vp.PortAlias
			vpD.K8sServiceName = vp.K8sServiceName
			if err := db.GetManager().TenantServicesPortDao().UpdateModel(vpD); err != nil {
				logrus.Errorf("update port var error, %v", err)
				tx.Rollback()
				return err
			}
			if crt {
				pluginPort, err := db.GetManager().TenantServicesStreamPluginPortDao().GetPluginMappingPortByServiceIDAndContainerPort(
					serviceID,
					dbmodel.InBoundNetPlugin,
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
					if err := db.GetManager().TenantServicesStreamPluginPortDao().UpdateModel(pluginPort); err != nil {
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
		dbmodel.InBoundNetPlugin,
	)
	if err != nil {
		return nil, "", fmt.Errorf("get plugin relations error: %s", err.Error())
	}
	//if stream 创建vs端口
	vsPort := &dbmodel.TenantServiceLBMappingPort{}
	switch servicePort.Body.Operation {
	case "close":
		if *p.IsOuterService { //如果端口已经开了对外
			falsev := false
			p.IsOuterService = &falsev
			tx := db.GetManager().Begin()
			defer func() {
				if r := recover(); r != nil {
					logrus.Errorf("Unexpected panic occurred, rollback transaction: %v", r)
					tx.Rollback()
				}
			}()
			if err = db.GetManager().TenantServicesPortDaoTransactions(tx).UpdateModel(p); err != nil {
				tx.Rollback()
				return nil, "", err
			}

			if hasUpStream {
				pluginPort, err := db.GetManager().TenantServicesStreamPluginPortDao().GetPluginMappingPortByServiceIDAndContainerPort(
					serviceID,
					dbmodel.InBoundNetPlugin,
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
				if *p.IsInnerService {
					//发现内网未关闭则不删除该映射
					logrus.Debugf("outer, close outer, but plugin inner port (%d) is exist, do not need delete", containerPort)
					goto OUTERCLOSEPASS
				}
				if err := db.GetManager().TenantServicesStreamPluginPortDaoTransactions(tx).DeletePluginMappingPortByContainerPort(
					serviceID,
					dbmodel.InBoundNetPlugin,
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
		truev := true
		p.IsOuterService = &truev
		tx := db.GetManager().Begin()
		defer func() {
			if r := recover(); r != nil {
				logrus.Errorf("Unexpected panic occurred, rollback transaction: %v", r)
				tx.Rollback()
			}
		}()
		if err = db.GetManager().TenantServicesPortDaoTransactions(tx).UpdateModel(p); err != nil {
			tx.Rollback()
			return nil, "", err
		}
		if hasUpStream {
			pluginPort, err := db.GetManager().TenantServicesStreamPluginPortDao().GetPluginMappingPortByServiceIDAndContainerPort(
				serviceID,
				dbmodel.InBoundNetPlugin,
				containerPort,
			)
			var pPort int
			if err != nil {
				if err.Error() == gorm.ErrRecordNotFound.Error() {
					ppPort, err := db.GetManager().TenantServicesStreamPluginPortDaoTransactions(tx).SetPluginMappingPort(
						p.TenantID,
						serviceID,
						dbmodel.InBoundNetPlugin,
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
		dbmodel.InBoundNetPlugin,
	)
	if err != nil {
		return fmt.Errorf("get plugin relations error: %s", err.Error())
	}
	tx := db.GetManager().Begin()
	defer func() {
		if r := recover(); r != nil {
			logrus.Errorf("Unexpected panic occurred, rollback transaction: %v", r)
			tx.Rollback()
		}
	}()
	switch operation {
	case "close":
		if *p.IsInnerService { //如果端口已经开了对内
			falsev := false
			p.IsInnerService = &falsev
			if err = db.GetManager().TenantServicesPortDaoTransactions(tx).UpdateModel(p); err != nil {
				tx.Rollback()
				return fmt.Errorf("update service port error: %s", err.Error())
			}
			if hasUpStream {
				pluginPort, err := db.GetManager().TenantServicesStreamPluginPortDao().GetPluginMappingPortByServiceIDAndContainerPort(
					serviceID,
					dbmodel.InBoundNetPlugin,
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
				if *p.IsOuterService {
					logrus.Debugf("inner, close inner, but plugin outerport (%d) is exist, do not need delete", port)
					goto INNERCLOSEPASS
				}
				if err := db.GetManager().TenantServicesStreamPluginPortDaoTransactions(tx).DeletePluginMappingPortByContainerPort(
					serviceID,
					dbmodel.InBoundNetPlugin,
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
		if *p.IsInnerService {
			tx.Rollback()
			return fmt.Errorf("already open")
		}
		truv := true
		p.IsInnerService = &truv
		if err = db.GetManager().TenantServicesPortDaoTransactions(tx).UpdateModel(p); err != nil {
			tx.Rollback()
			return err
		}
		if hasUpStream {
			pluginPort, err := db.GetManager().TenantServicesStreamPluginPortDao().GetPluginMappingPortByServiceIDAndContainerPort(
				serviceID,
				dbmodel.InBoundNetPlugin,
				port,
			)
			var pPort int
			if err != nil {
				if err.Error() == gorm.ErrRecordNotFound.Error() {
					ppPort, err := db.GetManager().TenantServicesStreamPluginPortDaoTransactions(tx).SetPluginMappingPort(
						p.TenantID,
						serviceID,
						dbmodel.InBoundNetPlugin,
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
				serviceInfo, err := db.GetManager().TenantServiceDao().GetServiceTypeByID(tsv.ServiceID)
				if err != nil {
					return util.CreateAPIHandleErrorFromDBError("service type", err)
				}
				// local volume just only support state component
				if serviceInfo == nil || !serviceInfo.IsState() {
					return util.CreateAPIHandleError(400, fmt.Errorf("应用类型为'无状态'.不支持本地存储"))
				}
				tsv.HostPath = fmt.Sprintf("%s/tenant/%s/service/%s%s", localPath, tenantID, tsv.ServiceID, tsv.VolumePath)
			}
		}
		util.SetVolumeDefaultValue(tsv)
		// begin transaction
		tx := db.GetManager().Begin()
		defer func() {
			if r := recover(); r != nil {
				logrus.Errorf("Unexpected panic occurred, rollback transaction: %v", r)
				tx.Rollback()
			}
		}()
		if err := db.GetManager().TenantServiceVolumeDaoTransactions(tx).AddModel(tsv); err != nil {
			tx.Rollback()
			return util.CreateAPIHandleErrorFromDBError("add volume", err)
		}
		if fileContent != "" {
			cf := &dbmodel.TenantServiceConfigFile{
				ServiceID:   tsv.ServiceID,
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
		defer func() {
			if r := recover(); r != nil {
				logrus.Errorf("Unexpected panic occurred, rollback transaction: %v", r)
				tx.Rollback()
			}
		}()
		if tsv.VolumeName != "" {
			volume, err := db.GetManager().TenantServiceVolumeDaoTransactions(tx).GetVolumeByServiceIDAndName(tsv.ServiceID, tsv.VolumeName)
			if err != nil {
				tx.Rollback()
				return util.CreateAPIHandleErrorFromDBError("find volume", err)
			}

			if err := db.GetManager().TenantServiceVolumeDaoTransactions(tx).DeleteModel(tsv.ServiceID, tsv.VolumeName); err != nil && err.Error() != gorm.ErrRecordNotFound.Error() {
				tx.Rollback()
				return util.CreateAPIHandleErrorFromDBError("delete volume", err)
			}

			err = s.MQClient.SendBuilderTopic(gclient.TaskStruct{
				Topic:    gclient.WorkerTopic,
				TaskType: "volume_gc",
				TaskBody: map[string]interface{}{
					"tenant_id":   tenantID,
					"service_id":  volume.ServiceID,
					"volume_id":   volume.ID,
					"volume_path": volume.VolumePath,
				},
			})
			if err != nil {
				logrus.Errorf("send 'volume_gc' task: %v", err)
				tx.Rollback()
				return util.CreateAPIHandleErrorFromDBError("send 'volume_gc' task", err)
			}
		} else {
			if err := db.GetManager().TenantServiceVolumeDaoTransactions(tx).DeleteByServiceIDAndVolumePath(tsv.ServiceID, tsv.VolumePath); err != nil && err.Error() != gorm.ErrRecordNotFound.Error() {
				tx.Rollback()
				return util.CreateAPIHandleErrorFromDBError("delete volume", err)
			}
		}
		if err := db.GetManager().TenantServiceConfigFileDaoTransactions(tx).DelByVolumeID(tsv.ServiceID, tsv.VolumeName); err != nil {
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

// UpdVolume updates service volume.
func (s *ServiceAction) UpdVolume(sid string, req *api_model.UpdVolumeReq) error {
	tx := db.GetManager().Begin()
	defer func() {
		if r := recover(); r != nil {
			logrus.Errorf("Unexpected panic occurred, rollback transaction: %v", r)
			tx.Rollback()
		}
	}()
	v, err := db.GetManager().TenantServiceVolumeDaoTransactions(tx).GetVolumeByServiceIDAndName(sid, req.VolumeName)
	if err != nil {
		tx.Rollback()
		return err
	}
	v.VolumePath = req.VolumePath
	v.Mode = req.Mode
	if err := db.GetManager().TenantServiceVolumeDaoTransactions(tx).UpdateModel(v); err != nil {
		tx.Rollback()
		return err
	}
	if req.VolumeType == "config-file" {
		configfile, err := db.GetManager().TenantServiceConfigFileDaoTransactions(tx).GetByVolumeName(sid, req.VolumeName)
		if err != nil {
			tx.Rollback()
			return err
		}
		configfile.FileContent = req.FileContent
		if err := db.GetManager().TenantServiceConfigFileDaoTransactions(tx).UpdateModel(configfile); err != nil {
			tx.Rollback()
			return err
		}
	}
	tx.Commit()
	return nil
}

//GetVolumes 获取应用全部存储
func (s *ServiceAction) GetVolumes(serviceID string) ([]*api_model.VolumeWithStatusStruct, *util.APIHandleError) {
	volumeWithStatusList := make([]*api_model.VolumeWithStatusStruct, 0)
	vs, err := db.GetManager().TenantServiceVolumeDao().GetTenantServiceVolumesByServiceID(serviceID)
	if err != nil && err.Error() != gorm.ErrRecordNotFound.Error() {
		return nil, util.CreateAPIHandleErrorFromDBError("get volumes", err)
	}

	volumeStatusList, err := s.statusCli.GetAppVolumeStatus(serviceID)
	if err != nil {
		logrus.Warnf("get volume status error: %s", err.Error())
	}
	volumeStatus := make(map[string]pb.ServiceVolumeStatus)
	if volumeStatusList != nil && volumeStatusList.GetStatus() != nil {
		volumeStatus = volumeStatusList.GetStatus()
	}
	isMountedShareVolume := false
	mountStatus := pb.ServiceVolumeStatus_NOT_READY.String()
	for _, volume := range vs {
		vws := &api_model.VolumeWithStatusStruct{
			ServiceID:          volume.ServiceID,
			Category:           volume.Category,
			VolumeType:         volume.VolumeType,
			VolumeName:         volume.VolumeName,
			HostPath:           volume.HostPath,
			VolumePath:         volume.VolumePath,
			IsReadOnly:         volume.IsReadOnly,
			VolumeCapacity:     volume.VolumeCapacity,
			AccessMode:         volume.AccessMode,
			SharePolicy:        volume.SharePolicy,
			BackupPolicy:       volume.BackupPolicy,
			ReclaimPolicy:      volume.ReclaimPolicy,
			AllowExpansion:     volume.AllowExpansion,
			VolumeProviderName: volume.VolumeProviderName,
		}
		volumeID := strconv.FormatInt(int64(volume.ID), 10)
		if phrase, ok := volumeStatus[volumeID]; ok {
			vws.Status = phrase.String()
			if os.Getenv("ENABLE_SUBPATH") == "true" && vws.VolumeType == "share-file" && strings.HasPrefix(vws.HostPath, "/grdata") {
				isMountedShareVolume = true
				mountStatus = vws.Status
			}
		} else {
			vws.Status = pb.ServiceVolumeStatus_NOT_READY.String()
			if isMountedShareVolume && strings.HasPrefix(vws.HostPath, "/grdata") {
				vws.Status = mountStatus
			}
		}
		volumeWithStatusList = append(volumeWithStatusList, vws)
	}

	return volumeWithStatusList, nil
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
	service, err := db.GetManager().TenantServiceDao().GetServiceByID(rs.ServiceID)
	if err != nil {
		return err
	}
	oldDeployVersion := service.DeployVersion
	if service.DeployVersion == rs.DeployVersion {
		return fmt.Errorf("current version is %v, don't need rollback", rs.DeployVersion)
	}
	service.DeployVersion = rs.DeployVersion
	if err := db.GetManager().TenantServiceDao().UpdateModel(service); err != nil {
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
		// rollback
		service.DeployVersion = oldDeployVersion
		if err := db.GetManager().TenantServiceDao().UpdateModel(service); err != nil {
			logrus.Warningf("error deploy version rollback: %v", err)
		}
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
	di, err := s.statusCli.GetServiceDeployInfo(serviceID)
	if err != nil {
		logrus.Warningf("service id: %s; failed to get deploy info: %v", serviceID, err)
	} else {
		sl.StartTime = di.GetStartTime()
	}
	return sl, nil
}

//GetServicesStatus  获取一组应用状态，若 serviceIDs为空,获取租户所有应用状态
func (s *ServiceAction) GetServicesStatus(tenantID string, serviceIDs []string) []map[string]interface{} {
	if len(serviceIDs) == 0 {
		services, _ := db.GetManager().TenantServiceDao().GetServicesByTenantID(tenantID)
		for _, s := range services {
			serviceIDs = append(serviceIDs, s.ServiceID)
		}
	}
	if len(serviceIDs) == 0 {
		return []map[string]interface{}{}
	}
	statusList := s.statusCli.GetStatuss(strings.Join(serviceIDs, ","))
	var info = make([]map[string]interface{}, 0)
	for k, v := range statusList {
		serviceInfo := map[string]interface{}{"service_id": k, "status": v, "status_cn": TransStatus(v), "used_mem": 0}
		info = append(info, serviceInfo)
	}
	return info
}

// GetEnterpriseServicesStatus get services
func (s *ServiceAction) GetEnterpriseServicesStatus(enterpriseID string) (map[string]string, *util.APIHandleError) {
	var tenantIDs []string
	tenants, err := db.GetManager().EnterpriseDao().GetEnterpriseTenants(enterpriseID)
	if err != nil {
		logrus.Errorf("list tenant failed: %s", err.Error())
		return nil, util.CreateAPIHandleErrorFromDBError(fmt.Sprintf("enterprise[%s] get tenant failed", enterpriseID), err)
	}
	if len(tenants) == 0 {
		return nil, util.CreateAPIHandleErrorf(400, "enterprise[%s] has not tenants", enterpriseID)
	}
	for _, tenant := range tenants {
		tenantIDs = append(tenantIDs, tenant.UUID)
	}
	services, err := db.GetManager().TenantServiceDao().GetServicesByTenantIDs(tenantIDs)
	if err != nil {
		logrus.Errorf("list tenants service failed: %s", err.Error())
		return nil, util.CreateAPIHandleErrorf(500, "get enterprise[%s] service failed: %s", enterpriseID, err.Error())
	}
	var serviceIDs []string
	for _, svc := range services {
		serviceIDs = append(serviceIDs, svc.ServiceID)
	}
	statusList := s.statusCli.GetStatuss(strings.Join(serviceIDs, ","))
	return statusList, nil
}

//CreateTenant create tenant
func (s *ServiceAction) CreateTenant(t *dbmodel.Tenants) error {
	tenant, _ := db.GetManager().TenantDao().GetTenantIDByName(t.Name)
	if tenant != nil {
		return fmt.Errorf("tenant name %s is exist", t.Name)
	}
	labels := map[string]string{
		constants.ResourceManagedByLabel: constants.Rainbond,
	}
	return db.GetManager().DB().Transaction(func(tx *gorm.DB) error {
		if err := db.GetManager().TenantDaoTransactions(tx).AddModel(t); err != nil {
			if !strings.HasSuffix(err.Error(), "is exist") {
				return err
			}
		}
		if _, err := s.kubeClient.CoreV1().Namespaces().Create(context.Background(), &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:   t.Namespace,
				Labels: labels,
			},
		}, metav1.CreateOptions{}); err != nil {
			if k8sErrors.IsAlreadyExists(err) {
				return bcode.ErrNamespaceExists
			}
			return err
		}
		return nil
	})
}

//CreateTenandIDAndName create tenant_id and tenant_name
func (s *ServiceAction) CreateTenandIDAndName(eid string) (string, string, error) {
	id := uuid.NewV4().String()
	uid := strings.Replace(id, "-", "", -1)
	name := strings.Split(id, "-")[0]
	logrus.Debugf("uuid is %v, name is %v", uid, name)
	return uid, name, nil
}

//K8sPodInfos -
type K8sPodInfos struct {
	NewPods []*K8sPodInfo `json:"new_pods"`
	OldPods []*K8sPodInfo `json:"old_pods"`
}

//K8sPodInfo for api
type K8sPodInfo struct {
	PodName   string                       `json:"pod_name"`
	PodIP     string                       `json:"pod_ip"`
	PodStatus string                       `json:"pod_status"`
	ServiceID string                       `json:"service_id"`
	Container map[string]map[string]string `json:"container"`
}

//GetPods get pods
func (s *ServiceAction) GetPods(serviceID string) (*K8sPodInfos, error) {
	pods, err := s.statusCli.GetServicePods(serviceID)
	if err != nil && !strings.Contains(err.Error(), server.ErrAppServiceNotFound.Error()) &&
		!strings.Contains(err.Error(), server.ErrPodNotFound.Error()) {
		logrus.Error("GetPodByService Error:", err)
		return nil, err
	}
	if pods == nil {
		return nil, nil
	}
	convpod := func(pods []*pb.ServiceAppPod) []*K8sPodInfo {
		var podsInfoList []*K8sPodInfo
		var podNames []string
		for _, v := range pods {
			var podInfo K8sPodInfo
			podInfo.PodName = v.PodName
			podInfo.PodIP = v.PodIp
			podInfo.PodStatus = v.PodStatus
			podInfo.ServiceID = serviceID
			containerInfos := make(map[string]map[string]string, 10)
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
		for _, c := range podsInfoList {
			for k := range c.Container {
				if info, exist := containerMemInfo[c.PodName][k]; exist {
					c.Container[k]["memory_usage"] = info
				}
			}
		}
		return podsInfoList
	}
	newpods := convpod(pods.NewPods)
	oldpods := convpod(pods.OldPods)
	return &K8sPodInfos{
		NewPods: newpods,
		OldPods: oldpods,
	}, nil
}

//GetMultiServicePods get pods
func (s *ServiceAction) GetMultiServicePods(serviceIDs []string) (*K8sPodInfos, error) {
	mpods, err := s.statusCli.GetMultiServicePods(serviceIDs)
	if err != nil && !strings.Contains(err.Error(), server.ErrAppServiceNotFound.Error()) &&
		!strings.Contains(err.Error(), server.ErrPodNotFound.Error()) {
		logrus.Error("GetPodByService Error:", err)
		return nil, err
	}
	if mpods == nil {
		return nil, nil
	}
	convpod := func(serviceID string, pods []*pb.ServiceAppPod) []*K8sPodInfo {
		var podsInfoList []*K8sPodInfo
		for _, v := range pods {
			var podInfo K8sPodInfo
			podInfo.PodName = v.PodName
			podInfo.PodIP = v.PodIp
			podInfo.PodStatus = v.PodStatus
			podInfo.ServiceID = serviceID
			podsInfoList = append(podsInfoList, &podInfo)
		}
		return podsInfoList
	}
	var re K8sPodInfos
	for serviceID, pods := range mpods.ServicePods {
		if pods != nil {
			re.NewPods = append(re.NewPods, convpod(serviceID, pods.NewPods)...)
			re.OldPods = append(re.OldPods, convpod(serviceID, pods.OldPods)...)
		}
	}
	return &re, nil
}

// GetComponentPodNums get pods
func (s *ServiceAction) GetComponentPodNums(ctx context.Context, componentIDs []string) (map[string]int32, error) {
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		defer core_util.Elapsed(fmt.Sprintf("[AppRuntimeSyncClient] [GetComponentPodNums] component nums: %d", len(componentIDs)))()
	}

	podNums, err := s.statusCli.GetComponentPodNums(ctx, componentIDs)
	if err != nil {
		return nil, errors.Wrap(err, "get component nums")
	}

	return podNums, nil
}

//GetPodContainerMemory Use Prometheus to query memory resources
func (s *ServiceAction) GetPodContainerMemory(podNames []string) (map[string]map[string]string, error) {
	memoryUsageMap := make(map[string]map[string]string, 10)
	queryName := strings.Join(podNames, "|")
	query := fmt.Sprintf(`container_memory_rss{pod=~"%s"}`, queryName)
	metric := s.prometheusCli.GetMetric(query, time.Now())

	for _, re := range metric.MetricData.MetricValues {
		var containerName = re.Metadata["container"]
		var podName = re.Metadata["pod"]
		var valuesBytes string
		if re.Sample != nil {
			valuesBytes = fmt.Sprintf("%d", int(re.Sample.Value()))
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

//TransServieToDelete trans service info to delete table
func (s *ServiceAction) TransServieToDelete(ctx context.Context, tenantID, serviceID string) error {
	_, err := db.GetManager().TenantServiceDao().GetServiceByID(serviceID)
	if err != nil && gorm.ErrRecordNotFound == err {
		logrus.Infof("service[%s] of tenant[%s] do not exist, ignore it", serviceID, tenantID)
		return nil
	}
	if err := s.isServiceClosed(serviceID); err != nil {
		return err
	}

	body, err := s.gcTaskBody(tenantID, serviceID)
	if err != nil {
		return fmt.Errorf("GC task body: %v", err)
	}

	if err := s.delServiceMetadata(ctx, serviceID); err != nil {
		return fmt.Errorf("delete service-related metadata: %v", err)
	}

	// let rbd-chaos remove related persistent data
	logrus.Info("let rbd-chaos remove related persistent data")
	topic := gclient.WorkerTopic
	if err := s.MQClient.SendBuilderTopic(gclient.TaskStruct{
		Topic:    topic,
		TaskType: "service_gc",
		TaskBody: body,
	}); err != nil {
		logrus.Warningf("send gc task: %v", err)
	}

	return nil
}

// isServiceClosed checks if the service has been closed according to the serviceID.
func (s *ServiceAction) isServiceClosed(serviceID string) error {
	service, err := db.GetManager().TenantServiceDao().GetServiceByID(serviceID)
	if err != nil {
		return err
	}
	status := s.statusCli.GetStatus(serviceID)
	if service.Kind != dbmodel.ServiceKindThirdParty.String() {
		if !s.statusCli.IsClosedStatus(status) {
			return ErrServiceNotClosed
		}
	}
	return nil
}

func (s *ServiceAction) deleteComponent(tx *gorm.DB, service *dbmodel.TenantServices) error {
	delService := service.ChangeDelete()
	delService.ID = 0
	if err := db.GetManager().TenantServiceDeleteDaoTransactions(tx).AddModel(delService); err != nil {
		return err
	}
	var deleteServicePropertyFunc = []func(serviceID string) error{
		db.GetManager().CodeCheckResultDaoTransactions(tx).DeleteByServiceID,
		db.GetManager().TenantServiceEnvVarDaoTransactions(tx).DELServiceEnvsByServiceID,
		db.GetManager().TenantPluginVersionConfigDaoTransactions(tx).DeletePluginConfigByServiceID,
		db.GetManager().TenantServicePluginRelationDaoTransactions(tx).DeleteALLRelationByServiceID,
		db.GetManager().TenantServicesStreamPluginPortDaoTransactions(tx).DeleteAllPluginMappingPortByServiceID,
		db.GetManager().TenantServiceDaoTransactions(tx).DeleteServiceByServiceID,
		db.GetManager().TenantServicesPortDaoTransactions(tx).DELPortsByServiceID,
		db.GetManager().TenantServiceRelationDaoTransactions(tx).DELRelationsByServiceID,
		db.GetManager().TenantServiceMountRelationDaoTransactions(tx).DELTenantServiceMountRelationByServiceID,
		db.GetManager().TenantServiceVolumeDaoTransactions(tx).DeleteTenantServiceVolumesByServiceID,
		db.GetManager().TenantServiceConfigFileDaoTransactions(tx).DelByServiceID,
		db.GetManager().EndpointsDaoTransactions(tx).DeleteByServiceID,
		db.GetManager().ThirdPartySvcDiscoveryCfgDaoTransactions(tx).DeleteByServiceID,
		db.GetManager().TenantServiceLabelDaoTransactions(tx).DeleteLabelByServiceID,
		db.GetManager().VersionInfoDaoTransactions(tx).DeleteVersionByServiceID,
		db.GetManager().TenantPluginVersionENVDaoTransactions(tx).DeleteEnvByServiceID,
		db.GetManager().ServiceProbeDaoTransactions(tx).DELServiceProbesByServiceID,
		db.GetManager().ServiceEventDaoTransactions(tx).DelEventByServiceID,
		db.GetManager().TenantServiceMonitorDaoTransactions(tx).DeleteServiceMonitorByServiceID,
		db.GetManager().AppConfigGroupServiceDaoTransactions(tx).DeleteEffectiveServiceByServiceID,
	}
	if err := GetGatewayHandler().DeleteTCPRuleByServiceIDWithTransaction(service.ServiceID, tx); err != nil {
		return err
	}
	if err := GetGatewayHandler().DeleteHTTPRuleByServiceIDWithTransaction(service.ServiceID, tx); err != nil {
		return err
	}
	for _, del := range deleteServicePropertyFunc {
		if err := del(service.ServiceID); err != nil {
			if err != gorm.ErrRecordNotFound {
				return err
			}
		}
	}
	return nil
}

// delServiceMetadata deletes service-related metadata in the database.
func (s *ServiceAction) delServiceMetadata(ctx context.Context, serviceID string) error {
	service, err := db.GetManager().TenantServiceDao().GetServiceByID(serviceID)
	if err != nil {
		return err
	}
	if db.GetManager().DB().Dialect().GetName() == "sqlite3" {
		if err := s.deleteThirdComponent(ctx, service); err != nil {
			return err
		}
		return s.deleteComponent(db.GetManager().DB(), service)
	}
	logrus.Infof("delete service %s %s", serviceID, service.ServiceAlias)
	return db.GetManager().DB().Transaction(func(tx *gorm.DB) error {
		if err := s.deleteThirdComponent(ctx, service); err != nil {
			return err
		}
		return s.deleteComponent(tx, service)
	})
}

func (s *ServiceAction) deleteThirdComponent(ctx context.Context, component *dbmodel.TenantServices) error {
	if component.Kind != "third_party" {
		return nil
	}
	tenant, err := db.GetManager().TenantDao().GetTenantByUUID(component.TenantID)
	if err != nil {
		return err
	}
	thirdPartySvcDiscoveryCfg, err := db.GetManager().ThirdPartySvcDiscoveryCfgDao().GetByServiceID(component.ServiceID)
	if err != nil {
		return err
	}
	if thirdPartySvcDiscoveryCfg == nil {
		return nil
	}
	if thirdPartySvcDiscoveryCfg.Type != string(dbmodel.DiscorveryTypeKubernetes) {
		return nil
	}

	newCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	err = s.rainbondClient.RainbondV1alpha1().ThirdComponents(tenant.Namespace).Delete(newCtx, component.ServiceID, metav1.DeleteOptions{})
	if err != nil && !k8sErrors.IsNotFound(err) {
		return err
	}
	return nil
}

func (s *ServiceAction) gcTaskBody(tenantID, serviceID string) (map[string]interface{}, error) {
	events, err := db.GetManager().ServiceEventDao().ListByTargetID(serviceID)
	if err != nil {
		logrus.Errorf("list events based on serviceID: %v", err)
	}
	var eventIDs []string
	for _, event := range events {
		eventIDs = append(eventIDs, event.EventID)
	}

	return map[string]interface{}{
		"tenant_id":  tenantID,
		"service_id": serviceID,
		"event_ids":  eventIDs,
	}, nil
}

//GetServiceDeployInfo get service deploy info
func (s *ServiceAction) GetServiceDeployInfo(tenantID, serviceID string) (*pb.DeployInfo, *util.APIHandleError) {
	info, err := s.statusCli.GetServiceDeployInfo(serviceID)
	if err != nil {
		return nil, util.CreateAPIHandleError(500, err)
	}
	return info, nil
}

// ListVersionInfo lists version info
func (s *ServiceAction) ListVersionInfo(serviceID string) (*api_model.BuildListRespVO, error) {
	versionInfos, err := db.GetManager().VersionInfoDao().GetAllVersionByServiceID(serviceID)
	if err != nil && err != gorm.ErrRecordNotFound {
		logrus.Errorf("error getting all version by service id: %v", err)
		return nil, fmt.Errorf("error getting all version by service id: %v", err)
	}
	svc, err := db.GetManager().TenantServiceDao().GetServiceByID(serviceID)
	if err != nil {
		logrus.Errorf("error getting service by uuid: %v", err)
		return nil, fmt.Errorf("error getting service by uuid: %v", err)
	}
	b, err := json.Marshal(versionInfos)
	if err != nil {
		return nil, fmt.Errorf("error marshaling version infos: %v", err)
	}
	var bversions []*api_model.BuildVersion
	if err := json.Unmarshal(b, &bversions); err != nil {
		return nil, fmt.Errorf("error unmarshaling version infos: %v", err)
	}
	for idx := range bversions {
		bv := bversions[idx]
		if bv.Kind == "build_from_image" || bv.Kind == "build_from_market_image" {
			image := parser.ParseImageName(bv.RepoURL)
			bv.ImageDomain = image.GetDomain()
			bv.ImageRepo = image.GetRepostory()
			bv.ImageTag = image.GetTag()
		}
	}
	result := &api_model.BuildListRespVO{
		DeployVersion: svc.DeployVersion,
		List:          bversions,
	}
	return result, nil
}

// EventBuildVersion -
func (s *ServiceAction) EventBuildVersion(serviceID, buildVersion string) (*api_model.BuildListRespVO, error) {
	versionInfo, err := db.GetManager().VersionInfoDao().GetVersionByDeployVersion(buildVersion, serviceID)
	if err != nil && err != gorm.ErrRecordNotFound {
		logrus.Errorf("error getting all version by service id: %v", err)
		return nil, fmt.Errorf("error getting all version by service id: %v", err)
	}
	b, err := json.Marshal(versionInfo)
	if err != nil {
		return nil, fmt.Errorf("error marshaling version infos: %v", err)
	}
	var bversion *api_model.BuildVersion
	if err := json.Unmarshal(b, &bversion); err != nil {
		return nil, fmt.Errorf("error unmarshaling version infos: %v", err)
	}

	result := &api_model.BuildListRespVO{
		DeployVersion: buildVersion,
		List:          bversion,
	}
	return result, nil
}

// AddAutoscalerRule -
func (s *ServiceAction) AddAutoscalerRule(req *api_model.AutoscalerRuleReq) error {
	tx := db.GetManager().Begin()
	defer db.GetManager().EnsureEndTransactionFunc()

	r := &dbmodel.TenantServiceAutoscalerRules{
		RuleID:      req.RuleID,
		ServiceID:   req.ServiceID,
		Enable:      req.Enable,
		XPAType:     req.XPAType,
		MinReplicas: req.MinReplicas,
		MaxReplicas: req.MaxReplicas,
	}
	if err := db.GetManager().TenantServceAutoscalerRulesDaoTransactions(tx).AddModel(r); err != nil {
		tx.Rollback()
		return err
	}

	for _, metric := range req.Metrics {
		m := &dbmodel.TenantServiceAutoscalerRuleMetrics{
			RuleID:            req.RuleID,
			MetricsType:       metric.MetricsType,
			MetricsName:       metric.MetricsName,
			MetricTargetType:  metric.MetricTargetType,
			MetricTargetValue: metric.MetricTargetValue,
		}
		if err := db.GetManager().TenantServceAutoscalerRuleMetricsDaoTransactions(tx).AddModel(m); err != nil {
			tx.Rollback()
			return err
		}
	}

	taskbody := map[string]interface{}{
		"service_id": r.ServiceID,
		"rule_id":    r.RuleID,
	}
	if err := s.MQClient.SendBuilderTopic(gclient.TaskStruct{
		TaskType: "refreshhpa",
		TaskBody: taskbody,
		Topic:    gclient.WorkerTopic,
	}); err != nil {
		logrus.Errorf("send 'refreshhpa' task: %v", err)
		return err
	}
	logrus.Infof("rule id: %s; successfully send 'refreshhpa' task.", r.RuleID)

	return tx.Commit().Error
}

// UpdAutoscalerRule -
func (s *ServiceAction) UpdAutoscalerRule(req *api_model.AutoscalerRuleReq) error {
	rule, err := db.GetManager().TenantServceAutoscalerRulesDao().GetByRuleID(req.RuleID)
	if err != nil {
		return err
	}

	rule.Enable = req.Enable
	rule.XPAType = req.XPAType
	rule.MinReplicas = req.MinReplicas
	rule.MaxReplicas = req.MaxReplicas

	tx := db.GetManager().Begin()
	defer db.GetManager().EnsureEndTransactionFunc()

	if err := db.GetManager().TenantServceAutoscalerRulesDaoTransactions(tx).UpdateModel(rule); err != nil {
		tx.Rollback()
		return err
	}

	// delete metrics
	if err := db.GetManager().TenantServceAutoscalerRuleMetricsDaoTransactions(tx).DeleteByRuleID(req.RuleID); err != nil {
		tx.Rollback()
		return err
	}

	for _, metric := range req.Metrics {
		m := &dbmodel.TenantServiceAutoscalerRuleMetrics{
			RuleID:            req.RuleID,
			MetricsType:       metric.MetricsType,
			MetricsName:       metric.MetricsName,
			MetricTargetType:  metric.MetricTargetType,
			MetricTargetValue: metric.MetricTargetValue,
		}
		if err := db.GetManager().TenantServceAutoscalerRuleMetricsDaoTransactions(tx).AddModel(m); err != nil {
			tx.Rollback()
			return err
		}
	}

	taskbody := map[string]interface{}{
		"service_id": rule.ServiceID,
		"rule_id":    rule.RuleID,
	}
	if err := s.MQClient.SendBuilderTopic(gclient.TaskStruct{
		TaskType: "refreshhpa",
		TaskBody: taskbody,
		Topic:    gclient.WorkerTopic,
	}); err != nil {
		logrus.Errorf("send 'refreshhpa' task: %v", err)
		return err
	}
	logrus.Infof("rule id: %s; successfully send 'refreshhpa' task.", rule.RuleID)

	return tx.Commit().Error
}

// ListScalingRecords -
func (s *ServiceAction) ListScalingRecords(serviceID string, page, pageSize int) ([]*dbmodel.TenantServiceScalingRecords, int, error) {
	records, err := db.GetManager().TenantServiceScalingRecordsDao().ListByServiceID(serviceID, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, 0, err
	}

	count, err := db.GetManager().TenantServiceScalingRecordsDao().CountByServiceID(serviceID)
	if err != nil {
		return nil, 0, err
	}

	return records, count, nil
}

// SyncComponentBase -
func (s *ServiceAction) SyncComponentBase(tx *gorm.DB, app *dbmodel.Application, components []*api_model.Component) error {
	var (
		componentIDs []string
		dbComponents []*dbmodel.TenantServices
	)
	for _, component := range components {
		componentIDs = append(componentIDs, component.ComponentBase.ComponentID)
	}
	oldComponents, err := db.GetManager().TenantServiceDao().GetServiceByIDs(componentIDs)
	if err != nil {
		return err
	}
	existComponents := make(map[string]*dbmodel.TenantServices)
	for _, oc := range oldComponents {
		existComponents[oc.ServiceID] = oc
	}
	for _, component := range components {
		var deployVersion string
		if oldComponent, ok := existComponents[component.ComponentBase.ComponentID]; ok {
			deployVersion = oldComponent.DeployVersion
		}
		dbComponents = append(dbComponents, component.ComponentBase.DbModel(app.TenantID, app.AppID, deployVersion))
	}
	if err := db.GetManager().TenantServiceDaoTransactions(tx).DeleteByComponentIDs(app.TenantID, app.AppID, componentIDs); err != nil {
		return err
	}
	return db.GetManager().TenantServiceDaoTransactions(tx).CreateOrUpdateComponentsInBatch(dbComponents)
}

// SyncComponentRelations -
func (s *ServiceAction) SyncComponentRelations(tx *gorm.DB, app *dbmodel.Application, components []*api_model.Component) error {
	var (
		componentIDs []string
		relations    []*dbmodel.TenantServiceRelation
	)
	for _, component := range components {
		if component.Relations == nil {
			continue
		}
		componentIDs = append(componentIDs, component.ComponentBase.ComponentID)
		for _, relation := range component.Relations {
			relations = append(relations, relation.DbModel(app.TenantID, component.ComponentBase.ComponentID))
		}
	}
	if err := db.GetManager().TenantServiceRelationDaoTransactions(tx).DeleteByComponentIDs(componentIDs); err != nil {
		return err
	}
	return db.GetManager().TenantServiceRelationDaoTransactions(tx).CreateOrUpdateRelationsInBatch(relations)
}

// SyncComponentEnvs -
func (s *ServiceAction) SyncComponentEnvs(tx *gorm.DB, app *dbmodel.Application, components []*api_model.Component) error {
	var (
		componentIDs []string
		envs         []*dbmodel.TenantServiceEnvVar
	)
	for _, component := range components {
		if component.Envs == nil {
			continue
		}
		componentIDs = append(componentIDs, component.ComponentBase.ComponentID)
		for _, env := range component.Envs {
			envs = append(envs, env.DbModel(app.TenantID, component.ComponentBase.ComponentID))
		}
	}
	if err := db.GetManager().TenantServiceEnvVarDaoTransactions(tx).DeleteByComponentIDs(componentIDs); err != nil {
		return err
	}
	return db.GetManager().TenantServiceEnvVarDaoTransactions(tx).CreateOrUpdateEnvsInBatch(envs)
}

// SyncComponentVolumeRels -
func (s *ServiceAction) SyncComponentVolumeRels(tx *gorm.DB, app *dbmodel.Application, components []*api_model.Component) error {
	var (
		componentIDs []string
		volRels      []*dbmodel.TenantServiceMountRelation
	)
	// Get the storage of all components under the application
	appComponents, err := db.GetManager().TenantServiceDao().ListByAppID(app.AppID)
	if err != nil {
		return err
	}
	var appComponentIDs []string
	for _, ac := range appComponents {
		appComponentIDs = append(appComponentIDs, ac.ServiceID)
	}
	existVolume, err := s.getExistVolumes(appComponentIDs)
	if err != nil {
		return err
	}
	// Get the storage that needs to be newly created
	for _, component := range components {
		componentID := component.ComponentBase.ComponentID
		if component.Volumes == nil {
			continue
		}
		for _, vol := range component.Volumes {
			if _, ok := existVolume[vol.Key(componentID)]; !ok {
				existVolume[vol.Key(componentID)] = vol.DbModel(componentID)
			}
		}
	}

	for _, component := range components {
		if component.VolumeRelations == nil {
			continue
		}
		componentIDs = append(componentIDs, component.ComponentBase.ComponentID)
		//The hostpath attribute should not be recorded in the mount relationship table,
		//and should be processed when the worker takes effect
		for _, volumeRelation := range component.VolumeRelations {
			if vol, ok := existVolume[volumeRelation.Key()]; ok {
				volRels = append(volRels, volumeRelation.DbModel(app.TenantID, component.ComponentBase.ComponentID, vol.HostPath, vol.VolumeType))
			}
		}
	}
	if err := db.GetManager().TenantServiceMountRelationDaoTransactions(tx).DeleteByComponentIDs(componentIDs); err != nil {
		return err
	}
	return db.GetManager().TenantServiceMountRelationDaoTransactions(tx).CreateOrUpdateVolumeRelsInBatch(volRels)
}

// SyncComponentVolumes -
func (s *ServiceAction) SyncComponentVolumes(tx *gorm.DB, components []*api_model.Component) error {
	var (
		componentIDs []string
		volumes      []*dbmodel.TenantServiceVolume
	)
	for _, component := range components {
		if component.Volumes == nil {
			continue
		}
		componentIDs = append(componentIDs, component.ComponentBase.ComponentID)
		for _, volume := range component.Volumes {
			volumes = append(volumes, volume.DbModel(component.ComponentBase.ComponentID))
		}
	}
	existVolumes, err := s.getExistVolumes(componentIDs)
	if err != nil {
		return err
	}
	deleteVolumeIDs := s.getDeleteVolumeIDs(existVolumes, volumes)
	createOrUpdates := s.getCreateOrUpdateVolumes(existVolumes, volumes)
	if err := db.GetManager().TenantServiceVolumeDaoTransactions(tx).DeleteByVolumeIDs(deleteVolumeIDs); err != nil {
		return err
	}
	return db.GetManager().TenantServiceVolumeDaoTransactions(tx).CreateOrUpdateVolumesInBatch(createOrUpdates)
}

func (s *ServiceAction) getExistVolumes(componentIDs []string) (existVolumes map[string]*dbmodel.TenantServiceVolume, err error) {
	existVolumes = make(map[string]*dbmodel.TenantServiceVolume)
	volumes, err := db.GetManager().TenantServiceVolumeDao().ListVolumesByComponentIDs(componentIDs)
	if err != nil {
		return nil, err
	}
	for _, volume := range volumes {
		existVolumes[volume.Key()] = volume
	}
	return existVolumes, nil
}

func (s *ServiceAction) getCreateOrUpdateVolumes(existVolumes map[string]*dbmodel.TenantServiceVolume, incomeVolumes []*dbmodel.TenantServiceVolume) (volumes []*dbmodel.TenantServiceVolume) {
	for _, incomeVolume := range incomeVolumes {
		if _, ok := existVolumes[incomeVolume.Key()]; ok {
			incomeVolume.ID = existVolumes[incomeVolume.Key()].ID
		}
		volumes = append(volumes, incomeVolume)
	}
	return volumes
}

func (s *ServiceAction) getDeleteVolumeIDs(existVolumes map[string]*dbmodel.TenantServiceVolume, incomeVolumes []*dbmodel.TenantServiceVolume) (deleteVolumeIDs []uint) {
	newVolumes := make(map[string]struct{})
	for _, volume := range incomeVolumes {
		newVolumes[volume.Key()] = struct{}{}
	}
	for existKey, existVolume := range existVolumes {
		if _, ok := newVolumes[existKey]; !ok {
			deleteVolumeIDs = append(deleteVolumeIDs, existVolume.ID)
		}
	}
	return deleteVolumeIDs
}

// SyncComponentConfigFiles -
func (s *ServiceAction) SyncComponentConfigFiles(tx *gorm.DB, components []*api_model.Component) error {
	var (
		componentIDs []string
		configFiles  []*dbmodel.TenantServiceConfigFile
	)
	for _, component := range components {
		if component.ConfigFiles == nil {
			continue
		}
		componentIDs = append(componentIDs, component.ComponentBase.ComponentID)
		for _, configFile := range component.ConfigFiles {
			configFiles = append(configFiles, configFile.DbModel(component.ComponentBase.ComponentID))
		}
	}
	if err := db.GetManager().TenantServiceConfigFileDaoTransactions(tx).DeleteByComponentIDs(componentIDs); err != nil {
		return err
	}
	return db.GetManager().TenantServiceConfigFileDaoTransactions(tx).CreateOrUpdateConfigFilesInBatch(configFiles)
}

// SyncComponentProbes -
func (s *ServiceAction) SyncComponentProbes(tx *gorm.DB, components []*api_model.Component) error {
	var (
		componentIDs []string
		probes       []*dbmodel.TenantServiceProbe
	)
	for _, component := range components {
		componentIDs = append(componentIDs, component.ComponentBase.ComponentID)
		modes := make(map[string]struct{})
		for _, probe := range component.Probes {
			_, ok := modes[probe.Mode]
			if ok {
				continue
			}
			probes = append(probes, probe.DbModel(component.ComponentBase.ComponentID))
			modes[probe.Mode] = struct{}{}
		}
	}
	if err := db.GetManager().ServiceProbeDaoTransactions(tx).DeleteByComponentIDs(componentIDs); err != nil {
		return err
	}
	return db.GetManager().ServiceProbeDaoTransactions(tx).CreateOrUpdateProbesInBatch(probes)
}

// SyncComponentLabels -
func (s *ServiceAction) SyncComponentLabels(tx *gorm.DB, components []*api_model.Component) error {
	var (
		componentIDs []string
		labels       []*dbmodel.TenantServiceLable
	)
	for _, component := range components {
		if component.Labels == nil {
			continue
		}
		componentIDs = append(componentIDs, component.ComponentBase.ComponentID)
		for _, label := range component.Labels {
			labels = append(labels, label.DbModel(component.ComponentBase.ComponentID))
		}
	}
	if err := db.GetManager().TenantServiceLabelDaoTransactions(tx).DeleteByComponentIDs(componentIDs); err != nil {
		return err
	}
	return db.GetManager().TenantServiceLabelDaoTransactions(tx).CreateOrUpdateLabelsInBatch(labels)
}

// SyncComponentPlugins -
func (s *ServiceAction) SyncComponentPlugins(tx *gorm.DB, app *dbmodel.Application, components []*api_model.Component) error {
	var (
		componentIDs           []string
		portConfigComponentIDs []string
		envComponentIDs        []string
		pluginRelations        []*dbmodel.TenantServicePluginRelation
		pluginVersionEnvs      []*dbmodel.TenantPluginVersionEnv
		pluginVersionConfigs   []*dbmodel.TenantPluginVersionDiscoverConfig
		pluginStreamPorts      []*dbmodel.TenantServicesStreamPluginPort
	)
	for _, component := range components {
		if component.Plugins == nil {
			continue
		}
		componentIDs = append(componentIDs, component.ComponentBase.ComponentID)
		for _, plugin := range component.Plugins {
			pluginRelations = append(pluginRelations, plugin.DbModel(component.ComponentBase.ComponentID))
			if plugin.ConfigEnvs.NormalEnvs != nil {
				envComponentIDs = append(envComponentIDs, component.ComponentBase.ComponentID)
				for _, versionEnv := range plugin.ConfigEnvs.NormalEnvs {
					pluginVersionEnvs = append(pluginVersionEnvs, versionEnv.DbModel(component.ComponentBase.ComponentID, plugin.PluginID))
				}
			}

			if configs := plugin.ConfigEnvs.ComplexEnvs; configs != nil {
				portConfigComponentIDs = append(portConfigComponentIDs, component.ComponentBase.ComponentID)
				if configs.BasePorts != nil && checkPluginHaveInbound(plugin.PluginModel) {
					psPorts := s.handlePluginMappingPort(app.TenantID, component.ComponentBase.ComponentID, plugin.PluginModel, configs.BasePorts)
					pluginStreamPorts = append(pluginStreamPorts, psPorts...)
				}
				config, err := ffjson.Marshal(configs)
				if err != nil {
					return err
				}
				pluginVersionConfigs = append(pluginVersionConfigs, &dbmodel.TenantPluginVersionDiscoverConfig{
					PluginID:  plugin.PluginID,
					ServiceID: component.ComponentBase.ComponentID,
					ConfigStr: string(config),
				})
			}
		}
	}

	if err := db.GetManager().TenantServicesStreamPluginPortDaoTransactions(tx).DeleteByComponentIDs(portConfigComponentIDs); err != nil {
		return err
	}
	if err := db.GetManager().TenantPluginVersionConfigDaoTransactions(tx).DeleteByComponentIDs(portConfigComponentIDs); err != nil {
		return err
	}
	if err := db.GetManager().TenantServicePluginRelationDaoTransactions(tx).DeleteByComponentIDs(componentIDs); err != nil {
		return err
	}
	if err := db.GetManager().TenantPluginVersionENVDaoTransactions(tx).DeleteByComponentIDs(envComponentIDs); err != nil {
		return err
	}

	if err := db.GetManager().TenantServicePluginRelationDaoTransactions(tx).CreateOrUpdatePluginRelsInBatch(pluginRelations); err != nil {
		return err
	}
	if err := db.GetManager().TenantPluginVersionENVDaoTransactions(tx).CreateOrUpdatePluginVersionEnvsInBatch(pluginVersionEnvs); err != nil {
		return err
	}
	if err := db.GetManager().TenantServicesStreamPluginPortDaoTransactions(tx).CreateOrUpdateStreamPluginPortsInBatch(pluginStreamPorts); err != nil {
		return err
	}
	return db.GetManager().TenantPluginVersionConfigDaoTransactions(tx).CreateOrUpdatePluginVersionConfigsInBatch(pluginVersionConfigs)
}

//handlePluginMappingPort -
func (s *ServiceAction) handlePluginMappingPort(tenantID, componentID, pluginModel string, ports []*api_model.BasePort) []*dbmodel.TenantServicesStreamPluginPort {
	existPorts := make(map[int]struct{})
	for _, port := range ports {
		existPorts[port.Port] = struct{}{}
	}

	minPort := 65301
	var newPorts []*dbmodel.TenantServicesStreamPluginPort
	for _, port := range ports {
		newPort := &dbmodel.TenantServicesStreamPluginPort{
			TenantID:      tenantID,
			ServiceID:     componentID,
			PluginModel:   pluginModel,
			ContainerPort: port.Port,
		}
		if _, ok := existPorts[minPort]; ok {
			minPort = minPort + 1
		}
		newPluginPort := minPort
		if _, ok := existPorts[newPluginPort]; ok {
			minPort = minPort + 1
			newPluginPort = minPort
		}

		existPorts[newPluginPort] = struct{}{}
		port.ListenPort = newPluginPort
		newPort.PluginPort = newPluginPort
		newPorts = append(newPorts, newPort)
	}
	return newPorts
}

// SyncComponentScaleRules -
func (s *ServiceAction) SyncComponentScaleRules(tx *gorm.DB, components []*api_model.Component) error {
	var (
		componentIDs         []string
		autoScaleRuleIDs     []string
		autoScaleRules       []*dbmodel.TenantServiceAutoscalerRules
		autoScaleRuleMetrics []*dbmodel.TenantServiceAutoscalerRuleMetrics
	)
	for _, component := range components {
		componentIDs = append(componentIDs, component.ComponentBase.ComponentID)
		autoScaleRuleIDs = append(autoScaleRuleIDs, component.AutoScaleRule.RuleID)
		autoScaleRules = append(autoScaleRules, component.AutoScaleRule.DbModel(component.ComponentBase.ComponentID))

		for _, metric := range component.AutoScaleRule.RuleMetrics {
			autoScaleRuleMetrics = append(autoScaleRuleMetrics, metric.DbModel(component.AutoScaleRule.RuleID))
		}
	}
	if err := db.GetManager().TenantServceAutoscalerRulesDaoTransactions(tx).DeleteByComponentIDs(componentIDs); err != nil {
		return err
	}
	if err := db.GetManager().TenantServceAutoscalerRuleMetricsDaoTransactions(tx).DeleteByRuleIDs(autoScaleRuleIDs); err != nil {
		return err
	}
	if err := db.GetManager().TenantServceAutoscalerRulesDaoTransactions(tx).CreateOrUpdateScaleRulesInBatch(autoScaleRules); err != nil {
		return err
	}
	return db.GetManager().TenantServceAutoscalerRuleMetricsDaoTransactions(tx).CreateOrUpdateScaleRuleMetricsInBatch(autoScaleRuleMetrics)
}

// SyncComponentEndpoints -
func (s *ServiceAction) SyncComponentEndpoints(tx *gorm.DB, components []*api_model.Component) error {
	var (
		componentIDs               []string
		thirdPartySvcDiscoveryCfgs []*dbmodel.ThirdPartySvcDiscoveryCfg
	)
	for _, component := range components {
		if component.Endpoint == nil {
			continue
		}
		componentIDs = append(componentIDs, component.ComponentBase.ComponentID)
		if component.Endpoint.Kubernetes != nil {
			thirdPartySvcDiscoveryCfgs = append(thirdPartySvcDiscoveryCfgs, component.Endpoint.DbModel(component.ComponentBase.ComponentID))
		}
	}

	if err := db.GetManager().ThirdPartySvcDiscoveryCfgDaoTransactions(tx).DeleteByComponentIDs(componentIDs); err != nil {
		return err
	}
	return db.GetManager().ThirdPartySvcDiscoveryCfgDaoTransactions(tx).CreateOrUpdate3rdSvcDiscoveryCfgInBatch(thirdPartySvcDiscoveryCfgs)
}

// SyncComponentK8sAttributes -
func (s *ServiceAction) SyncComponentK8sAttributes(tx *gorm.DB, app *dbmodel.Application, components []*api_model.Component) error {
	var (
		componentIDs  []string
		k8sAttributes []*dbmodel.ComponentK8sAttributes
	)
	for _, component := range components {
		if component.ComponentK8sAttributes == nil || len(component.ComponentK8sAttributes) == 0 {
			continue
		}
		componentIDs = append(componentIDs, component.ComponentBase.ComponentID)
		for _, k8sAttribute := range component.ComponentK8sAttributes {
			k8sAttributes = append(k8sAttributes, k8sAttribute.DbModel(app.TenantID, component.ComponentBase.ComponentID))
		}
	}

	if err := db.GetManager().ComponentK8sAttributeDaoTransactions(tx).DeleteByComponentIDs(componentIDs); err != nil {
		return err
	}
	return db.GetManager().ComponentK8sAttributeDaoTransactions(tx).CreateOrUpdateAttributesInBatch(k8sAttributes)
}

// Log returns the logs reader for a container in a pod, a pod or a component.
func (s *ServiceAction) Log(w http.ResponseWriter, r *http.Request, component *dbmodel.TenantServices, podName, containerName string, follow bool) error {
	// If podName and containerName is missing, return the logs reader for the component
	// If containerName is missing, return the logs reader for the pod.
	if podName == "" || containerName == "" {
		// Only support return the logs reader for a container now.
		return errors.WithStack(bcode.NewBadRequest("the field 'podName' and 'containerName' is required"))
	}
	tenant, err := db.GetManager().TenantDao().GetTenantByUUID(component.TenantID)
	if err != nil {
		return fmt.Errorf("get tenant info failure %s", err.Error())
	}
	request := s.kubeClient.CoreV1().Pods(tenant.Namespace).GetLogs(podName, &corev1.PodLogOptions{
		Container: containerName,
		Follow:    follow,
	})

	out, err := request.Stream(context.TODO())
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			return errors.Wrap(bcode.ErrPodNotFound, "get pod log")
		}
		return errors.Wrap(err, "get stream from request")
	}
	defer out.Close()

	w.Header().Set("Transfer-Encoding", "chunked")
	w.WriteHeader(http.StatusOK)

	// Flush headers, if possible
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}

	writer := flushwriter.Wrap(w)

	_, err = io.Copy(writer, out)
	if err != nil {
		if strings.HasSuffix(err.Error(), "write: broken pipe") {
			return nil
		}
		logrus.Warningf("write stream to response: %v", err)
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
	case "succeeded":
		return "已完成"
	}
	return ""
}
