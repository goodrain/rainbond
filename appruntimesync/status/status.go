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

	"github.com/goodrain/rainbond/appruntimesync/source"
	"github.com/jinzhu/gorm"
	"k8s.io/client-go/kubernetes"

	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/cmd/worker/option"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/model"
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

//Manager app status manager
type Manager struct {
	c                     option.Config
	ctx                   context.Context
	StatefulSetUpdateChan chan source.StatefulSetUpdate
	RCUpdateChan          chan source.RCUpdate
	DeploymentUpdateChan  chan source.DeploymentUpdate
	checkChan             chan string
	ignoreDelete          map[string]string
	ignoreLock            sync.Mutex
	status                map[string]string
	statusLock            sync.RWMutex
	ClientSet             *kubernetes.Clientset
}

//NewManager create app runtime status manager
func NewManager(ctx context.Context, clientset *kubernetes.Clientset) *Manager {
	return &Manager{
		ctx:                   ctx,
		ClientSet:             clientset,
		RCUpdateChan:          make(chan source.RCUpdate, 10),
		DeploymentUpdateChan:  make(chan source.DeploymentUpdate, 10),
		StatefulSetUpdateChan: make(chan source.StatefulSetUpdate, 10),
		checkChan:             make(chan string, 20),
		ignoreDelete:          make(map[string]string),
		status:                make(map[string]string),
	}
}

//Start start
func (s *Manager) Start() error {
	logrus.Info("status manager starting...")
	s.cacheAllAPPStatus()
	go s.checkStatus()
	go s.handleUpdate()
	go s.SyncStatus()
	logrus.Info("status manager started")
	return nil
}

//handleUpdate
func (s *Manager) handleUpdate() {
	for {
		select {
		case <-s.ctx.Done():
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

func (s *Manager) checkStatus() {
	for {
		select {
		case <-s.ctx.Done():
			return
		case serviceID := <-s.checkChan:
			deployInfo, err := db.GetManager().K8sDeployReplicationDao().GetK8sDeployReplicationByService(serviceID)
			if err != nil {
				if err == gorm.ErrRecordNotFound {
					if s.GetStatus(serviceID) != UNDEPLOY && s.GetStatus(serviceID) != DEPLOYING {
						s.SetStatus(serviceID, CLOSED)
					}
					continue
				}
				logrus.Error("get deploy info error where check application status.", err.Error())
				continue
			}
			if deployInfo == nil || len(deployInfo) == 0 {
				if s.GetStatus(serviceID) != UNDEPLOY && s.GetStatus(serviceID) != DEPLOYING {
					s.SetStatus(serviceID, CLOSED)
				}
				continue
			}
			switch deployInfo[0].ReplicationType {
			case model.TypeDeployment:
				for i := 0; i < 3; i++ {
					d, err := s.ClientSet.AppsV1beta1().Deployments(deployInfo[0].TenantID).Get(deployInfo[0].ReplicationID, metav1.GetOptions{})
					if err != nil {
						if strings.HasSuffix(err.Error(), "not found") {
							if s.GetStatus(serviceID) != UNDEPLOY && s.GetStatus(serviceID) != DEPLOYING {
								s.SetStatus(serviceID, CLOSED)
							}
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
							if s.GetStatus(serviceID) != UNDEPLOY && s.GetStatus(serviceID) != DEPLOYING {
								s.SetStatus(serviceID, CLOSED)
							}
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
							if s.GetStatus(serviceID) != UNDEPLOY && s.GetStatus(serviceID) != DEPLOYING {
								s.SetStatus(serviceID, CLOSED)
							}
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
							if s.GetStatus(serviceID) != UNDEPLOY && s.GetStatus(serviceID) != DEPLOYING {
								s.SetStatus(serviceID, CLOSED)
							}
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

//cacheAllAPPStatus get all app status cache
func (s *Manager) cacheAllAPPStatus() {
	all, err := db.GetManager().TenantServiceStatusDao().GetAll()
	if err != nil {
		logrus.Error("get all running and starting service error")
		return
	}
	if len(all) > 0 {
		for _, sta := range all {
			s.cacheStatus(sta.ServiceID, sta.Status)
		}
	}

}

//SetStatus set app status
func (s *Manager) SetStatus(serviceID, status string) error {
	s.cacheStatus(serviceID, status)
	if err := db.GetManager().TenantServiceStatusDao().SetTenantServiceStatus(serviceID, status); err != nil {
		logrus.Error("set application status error.", err.Error())
		return err
	}
	return nil
}

func (s *Manager) cacheStatus(serviceID, status string) {
	s.statusLock.Lock()
	defer s.statusLock.Unlock()
	s.status[serviceID] = status
}

//GetStatus get app status
func (s *Manager) GetStatus(serviceID string) string {
	s.statusLock.RLock()
	defer s.statusLock.RUnlock()
	if status, ok := s.status[serviceID]; ok {
		return status
	}
	s.CheckStatus(serviceID)
	return "unknow"
}

//GetAllStatus get all app status
func (s *Manager) GetAllStatus() map[string]string {
	s.statusLock.RLock()
	defer s.statusLock.RUnlock()
	var re = make(map[string]string)
	for k, v := range s.status {
		re[k] = v
	}
	return re
}

//CheckStatus check app status
func (s *Manager) CheckStatus(serviceID string) {
	select {
	case s.checkChan <- serviceID:
	default:
	}
}

//SaveDeployInfo save app deploy info
func (s *Manager) SaveDeployInfo(serviceID, tenantID, deployVersion, replicationID, replicationType string) (*model.K8sDeployReplication, error) {
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

//SyncStatus sync app status
func (s *Manager) SyncStatus() {
	allServic, err := db.GetManager().TenantServiceDao().GetAllServices()
	if err != nil {
		logrus.Error("get all  service error")
		return
	}
	if len(allServic) == len(s.status) {
		for k := range s.status {
			s.checkChan <- k
		}
		return
	}
	for i, ser := range allServic {
		logrus.Debugf("(%d)check service %s status", i, ser.ServiceID)
		s.checkChan <- ser.ServiceID
	}
}
func (s *Manager) isIgnoreDelete(name string) bool {
	s.ignoreLock.Lock()
	defer s.ignoreLock.Unlock()
	if _, ok := s.ignoreDelete[name]; ok {
		return true
	}
	return false
}

//RmIgnoreDelete remove ignore delete info
func (s *Manager) RmIgnoreDelete(name string) {
	s.ignoreLock.Lock()
	defer s.ignoreLock.Unlock()
	if _, ok := s.ignoreDelete[name]; ok {
		delete(s.ignoreDelete, name)
	}
}

//IgnoreDelete  add ignore delete info
func (s *Manager) IgnoreDelete(name string) {
	s.ignoreLock.Lock()
	defer s.ignoreLock.Unlock()
	s.ignoreDelete[name] = name
}
