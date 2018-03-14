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

package status

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/goodrain/rainbond/cmd/worker/option"
	"github.com/goodrain/rainbond/pkg/db"
	"github.com/goodrain/rainbond/pkg/db/model"

	"github.com/jinzhu/gorm"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/Sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// These are the available operation types.
const (
	RUNNING  string = "running"
	CLOSED          = "closed"
	STARTING        = "starting"
	STOPPING        = "stopping"
	CHECKING        = "checking"
	//运行异常
	ABNORMAL = "abnormal"
	//升级中
	UPGRADE  = "upgrade"
	UNDEPLOY = "undeploy"
	//构建中
	DEPLOYING = "deploying"
)

//ServiceStatusManager 应用运行状态控制器
type ServiceStatusManager interface {
	SetStatus(serviceID, status string) error
	GetStatus(serviceID string) (string, error)
	CheckStatus(serviceID string)
	Start() error
	Stop() error
	SyncStatus()
	IgnoreDelete(name string)
	RmIgnoreDelete(name string)
}

type statusManager struct {
	c                     option.Config
	stopChan              chan struct{}
	Ctx                   context.Context
	Cancel                context.CancelFunc
	ClientSet             *kubernetes.Clientset
	StatefulSetUpdateChan chan StatefulSetUpdate
	RCUpdateChan          chan RCUpdate
	DeploymentUpdateChan  chan DeploymentUpdate
	checkChan             chan string
	ignoreDelete          map[string]string
	ignoreLock            sync.Mutex
	status                map[string]string
}

//NewManager 创建一个应用运行状态控制器
func NewManager(conf option.Config) ServiceStatusManager {
	ctx, cancel := context.WithCancel(context.Background())
	kubeconfig := conf.KubeConfig
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		logrus.Error(err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		logrus.Error(err)
	}
	logrus.Info("Kube client api create success.")
	return &statusManager{
		c:                     conf,
		Ctx:                   ctx,
		stopChan:              make(chan struct{}),
		Cancel:                cancel,
		ClientSet:             clientset,
		RCUpdateChan:          make(chan RCUpdate, 10),
		DeploymentUpdateChan:  make(chan DeploymentUpdate, 10),
		StatefulSetUpdateChan: make(chan StatefulSetUpdate, 10),
		checkChan:             make(chan string, 20),
		ignoreDelete:          make(map[string]string),
		status:                make(map[string]string),
	}
}

func (s *statusManager) SetStatus(serviceID, status string) error {
	if err := db.GetManager().TenantServiceStatusDao().SetTenantServiceStatus(serviceID, status); err != nil {
		logrus.Error("set application status error.", err.Error())
		return err
	}
	if err := db.GetManager().TenantServiceDao().SetTenantServiceStatus(serviceID, status); err != nil {
		logrus.Error("set application service status error.", err.Error())
		return err
	}
	//本地缓存
	//s.status[serviceID] = status
	return nil
}

func (s *statusManager) GetStatus(serviceID string) (string, error) {

	// 本地缓存应用状态
	// if status, ok := s.status[serviceID]; ok {
	// 	return status, nil
	// }
	status, err := db.GetManager().TenantServiceStatusDao().GetTenantServiceStatus(serviceID)
	if err != nil {
		return "", err
	}
	if status != nil {
		return status.Status, nil
	}
	//历史数据兼容
	service, err := db.GetManager().TenantServiceDao().GetServiceByID(serviceID)
	if err != nil {
		return "", err
	}
	return service.CurStatus, nil
}

func (s *statusManager) Start() error {
	logrus.Info("status manager starting...")
	go s.checkStatus()
	go s.handleUpdate()
	NewSourceAPI(s.ClientSet.Core().RESTClient(),
		s.ClientSet.AppsV1beta1().RESTClient(),
		15*time.Minute,
		s.RCUpdateChan,
		s.DeploymentUpdateChan,
		s.StatefulSetUpdateChan,
		s.stopChan,
	)
	logrus.Info("status manager started")
	return nil
}
func (s *statusManager) handleUpdate() {
	for {
		select {
		case <-s.Ctx.Done():
			return
		case update := <-s.RCUpdateChan:
			s.handleRCUpdate(update)
		case update := <-s.DeploymentUpdateChan:
			s.handleDeploymentUpdate(update)
		case update := <-s.StatefulSetUpdateChan:
			s.handleStatefulUpdate(update)
		}
	}
}

func (s *statusManager) checkStatus() {
	for {
		select {
		case <-s.Ctx.Done():
			return
		case serviceID := <-s.checkChan:
			deployInfo, err := db.GetManager().K8sDeployReplicationDao().GetK8sDeployReplicationByService(serviceID)
			if err != nil {
				if err == gorm.ErrRecordNotFound {
					s.SetStatus(serviceID, CLOSED)
					continue
				}
				logrus.Error("get deploy info error where check application status.", err.Error())
				continue
			}
			if deployInfo == nil || len(deployInfo) == 0 {
				logrus.Info("deployInfo is nil or length is 0.")
				s.SetStatus(serviceID, CLOSED)
				continue
			}
			switch deployInfo[0].ReplicationType {
			case model.TypeDeployment:
				for i := 0; i < 3; i++ {
					d, err := s.ClientSet.AppsV1beta1().Deployments(deployInfo[0].TenantID).Get(deployInfo[0].ReplicationID, metav1.GetOptions{})
					if err != nil {
						if strings.HasSuffix(err.Error(), "not found") {
							s.SetStatus(serviceID, CLOSED)
							break
						} else {
							logrus.Error("get Deployment info from k8s error when check app status.", err.Error())
							time.Sleep(time.Second * 2)
							continue
						}
					} else {
						if d.Status.ReadyReplicas >= d.Status.Replicas && d.Status.Replicas != 0 {
							s.SetStatus(serviceID, RUNNING)
							break
						} else {
							s.SetStatus(serviceID, ABNORMAL)
							break
						}
					}
				}

			case model.TypeReplicationController:
				for i := 0; i < 3; i++ {
					d, err := s.ClientSet.Core().ReplicationControllers(deployInfo[0].TenantID).Get(deployInfo[0].ReplicationID, metav1.GetOptions{})
					if err != nil {
						if strings.HasSuffix(err.Error(), "not found") {
							s.SetStatus(serviceID, CLOSED)
							break
						} else {
							logrus.Error("get ReplicationControllers info from k8s error when check app status.", err.Error())
							time.Sleep(time.Second * 2)
							continue
						}
					} else {
						if d.Status.ReadyReplicas >= d.Status.Replicas && d.Status.Replicas != 0 {
							s.SetStatus(serviceID, RUNNING)
							break
						} else {
							s.SetStatus(serviceID, ABNORMAL)
							break
						}
					}
				}
			case model.TypeStatefulSet:
				for i := 0; i < 3; i++ {
					d, err := s.ClientSet.AppsV1beta1().StatefulSets(deployInfo[0].TenantID).Get(deployInfo[0].ReplicationID, metav1.GetOptions{})
					if err != nil {
						if strings.HasSuffix(err.Error(), "not found") {
							s.SetStatus(serviceID, CLOSED)
							break
						} else {
							logrus.Error("get StatefulSets info from k8s error when check app status.", err.Error())
							time.Sleep(time.Second * 2)
							continue
						}
					} else {
						readycount := s.getReadyCount(d.Namespace,
							d.Labels["name"], d.Labels["version"])
						if readycount >= d.Status.Replicas && d.Status.Replicas != 0 {
							s.SetStatus(serviceID, RUNNING)
							break
						} else {
							s.SetStatus(serviceID, ABNORMAL)
						}
					}
					break
				}
			default:
				for i := 0; i < 3; i++ {
					d, err := s.ClientSet.Core().ReplicationControllers(deployInfo[0].TenantID).Get(deployInfo[0].ReplicationID, metav1.GetOptions{})
					if err != nil {
						if strings.HasSuffix(err.Error(), "not found") {
							s.SetStatus(serviceID, CLOSED)
							break
						} else {
							logrus.Error("get ReplicationControllers info from k8s error when check app status.", err.Error())
							time.Sleep(time.Second * 2)
							continue
						}
					} else {
						if d.Status.ReadyReplicas >= d.Status.Replicas && d.Status.Replicas != 0 {
							s.SetStatus(serviceID, RUNNING)
							break
						}
					}
					break
				}
			}

		}
	}
}

func (s *statusManager) CheckStatus(serviceID string) {
	select {
	case s.checkChan <- serviceID:
	default:
	}
}

//Stop 停止
func (s *statusManager) Stop() error {
	logrus.Info("Source manager is stoping.")
	close(s.stopChan)
	s.Cancel()
	return nil
}

func (s *statusManager) SaveDeployInfo(serviceID, tenantID, deployVersion, replicationID, replicationType string) (*model.K8sDeployReplication, error) {
	deploy := &model.K8sDeployReplication{
		TenantID:        tenantID,
		ServiceID:       serviceID,
		ReplicationID:   replicationID,
		ReplicationType: replicationType,
		DeployVersion:   deployVersion,
	}
	err := db.GetManager().K8sDeployReplicationDao().AddModel(deploy)
	if err != nil {
		logrus.Error("Try to save deploy information failed.", err.Error())
		return nil, err
	}
	return deploy, nil
}

func (s *statusManager) SyncStatus() {
	all, err := db.GetManager().TenantServiceStatusDao().GetRunningService()
	if err != nil {
		logrus.Error("get all running and starting service error")
		return
	}
	if len(all) > 0 {
		for _, sta := range all {
			s.CheckStatus(sta.ServiceID)
		}
	}
}

func (s *statusManager) IgnoreDelete(name string) {
	s.ignoreLock.Lock()
	defer s.ignoreLock.Unlock()
	s.ignoreDelete[name] = name
}
func (s *statusManager) isIgnoreDelete(name string) bool {
	s.ignoreLock.Lock()
	defer s.ignoreLock.Unlock()
	if _, ok := s.ignoreDelete[name]; ok {
		return true
	}
	return false
}

func (s *statusManager) RmIgnoreDelete(name string) {
	s.ignoreLock.Lock()
	defer s.ignoreLock.Unlock()
	if _, ok := s.ignoreDelete[name]; ok {
		delete(s.ignoreDelete, name)
	}
}
