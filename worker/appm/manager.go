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

package appm

import (
	"errors"
	"fmt"
	"time"

	"github.com/goodrain/rainbond/appruntimesync/client"
	"github.com/goodrain/rainbond/cmd/worker/option"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/event"

	"github.com/Sirupsen/logrus"
	"github.com/jinzhu/gorm"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/apis/apps/v1beta1"
	"k8s.io/client-go/tools/clientcmd"
)

//Replicas0 petch replicas to 0
var Replicas0 = []byte(`{"spec":{"replicas":0}}`)

//ErrTimeOut 超时
var ErrTimeOut = errors.New("time out")

//ErrNotDeploy 未部署错误
var ErrNotDeploy = errors.New("not deploy")

//Replicas petch replicas to n
func Replicas(n int) []byte {
	return []byte(fmt.Sprintf(`{"spec":{"replicas":%d}}`, n))
}

//Manager kubeapi
type Manager interface {
	StartStatefulSet(serviceID string, logger event.Logger) (*v1beta1.StatefulSet, error)
	StopStatefulSet(serviceID string, logger event.Logger) error
	//TODO: 实现滚动升级，目前无滚动升级，采用重启操作
	RollingUpgradeStatefulSet(serviceID string, logger event.Logger) (*v1beta1.StatefulSet, error)
	StartReplicationController(serviceID string, logger event.Logger) (*v1.ReplicationController, error)
	StopReplicationController(serviceID string, logger event.Logger) error
	RollingUpgradeReplicationController(serviceID string, stopChan chan struct{}, logger event.Logger) (*v1.ReplicationController, error)
	RollingUpgradeReplicationControllerCompatible(serviceID string, stopChan chan struct{}, logger event.Logger) (*v1.ReplicationController, error)
	StartDeployment(serviceID string, logger event.Logger) (*v1beta1.Deployment, error)
	StopDeployment(serviceID string, logger event.Logger) error
	//TODO:
	RollingUpgradeDeployment(serviceID string, logger event.Logger) (*v1beta1.Deployment, error)
	HorizontalScaling(serviceID string, oldReplicas int32, logger event.Logger) error
	StartService(serviceID string, logger event.Logger, ReplicationID, ReplicationType string) error
	UpdateService(serviceID string, logger event.Logger, ReplicationID, ReplicationType string) error
	StopService(serviceID string, logger event.Logger) error
	//StartServiceByPort(serviceID string, port int, isOut bool, logger event.Logger) error
	StopServiceByPort(serviceID string, port int, isOut bool, logger event.Logger) error
	SyncData()
	Stop()
}

type manager struct {
	kubeclient      *kubernetes.Clientset
	conf            option.Config
	dbmanager       db.Manager
	statusCache     *CacheManager
	statusManager   *client.AppRuntimeSyncClient
	informerFactory informers.SharedInformerFactory
	stop            chan struct{}
}

//NewManager 创建manager
func NewManager(conf option.Config, statusManager *client.AppRuntimeSyncClient) (Manager, error) {
	c, err := clientcmd.BuildConfigFromFlags("", conf.KubeConfig)
	if err != nil {
		logrus.Error("read kube config file error.", err)
		return nil, err
	}
	clientset, err := kubernetes.NewForConfig(c)
	if err != nil {
		logrus.Error("create kube api client error", err)
		return nil, err
	}
	cacheManager := NewCacheManager()
	return &manager{kubeclient: clientset, conf: conf,
		dbmanager:     db.GetManager(),
		statusCache:   cacheManager,
		statusManager: statusManager,
	}, nil
}
func (m *manager) Stop() {
}

//HorizontalScaling 水平伸缩
func (m *manager) HorizontalScaling(serviceID string, oldReplicas int32, logger event.Logger) error {
	service, err := m.dbmanager.TenantServiceDao().GetServiceByID(serviceID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			logger.Error("服务在数据中心不存在", map[string]string{"step": "worker-appm", "status": "failure"})
		}
		return err
	}
	logrus.Infof("service %s current replicas %d , scaling replicas %d ", serviceID, oldReplicas, service.Replicas)
	logger.Info(fmt.Sprintf("应用当前节点数%d,伸缩目地节点数%d", oldReplicas, service.Replicas), map[string]string{"step": "worker-appm"})
	var mode string
	if int32(service.Replicas) > oldReplicas {
		logger.Info("实例扩容操作开始", map[string]string{"step": "worker-appm", "status": "starting"})
		mode = "up"
	}
	if int32(service.Replicas) < oldReplicas {
		logger.Info("实例缩容操作开始", map[string]string{"step": "worker-appm", "status": "starting"})
		mode = "down"
	}
	if int32(service.Replicas) == oldReplicas {
		logger.Info("实例数无变化，无需操作", map[string]string{"step": "worker-appm", "status": "success"})
		return nil
	}
	deploys, err := m.dbmanager.K8sDeployReplicationDao().GetK8sDeployReplicationByService(serviceID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			logger.Info("应用未部署，操作完成", map[string]string{"step": "worker-appm", "status": "success"})
			return ErrNotDeploy
		}
		logrus.Error("get tenant service deploy info error.", err.Error())
		logger.Error("获取应用部署信息失败，水平伸缩停止。", map[string]string{"step": "worker-appm", "status": "failure"})
		return err
	}
	var deploy *model.K8sDeployReplication
	if deploys != nil || len(deploys) > 0 {
		for _, d := range deploys {
			if !d.IsDelete {
				deploy = d
			}
		}
	}
	if deploy == nil {
		logger.Info("应用未部署，操作完成", map[string]string{"step": "worker-appm", "status": "success"})
		return ErrNotDeploy
	}
	//水平升级过程 状态管理器会任务应用运行异常，此时忽略异常
	m.statusManager.IgnoreDelete(deploy.ReplicationID)
	defer m.statusManager.RmIgnoreDelete(deploy.ReplicationID)

	replicas := int32(service.Replicas)
	switch deploy.ReplicationType {
	case model.TypeStatefulSet:
		//更新stateful pod数量为replicas
		logger.Info("水平伸缩资源类型为TypeStatefulSet", map[string]string{"step": "worker-appm"})
		stateful, err := m.kubeclient.AppsV1beta1().StatefulSets(service.TenantID).Patch(service.ServiceAlias, types.StrategicMergePatchType, Replicas(int(replicas)))
		if err != nil {
			logrus.Error("patch statefulset info error.", err.Error())
			logger.Error(fmt.Sprintf("更改StatefulSet Pod数量为%d失败", replicas), map[string]string{"step": "worker-appm", "status": "error"})
			return err
		}
		err = m.waitStateful(mode, serviceID, replicas, logger, stateful)
		if err != nil {
			logrus.Errorf("patch statefulset replicas to %d and watch error.%v", replicas, err.Error())
			logger.Error(fmt.Sprintf("更改StatefulSet Pod数量为%d结果检测失败", replicas), map[string]string{"step": "worker-appm", "status": "error"})
			return err
		}
	case model.TypeDeployment:
		//更新deployment pod数量为replicas
		logger.Info("水平伸缩资源类型为TypeDeployment", map[string]string{"step": "worker-appm"})
		deployment, err := m.kubeclient.AppsV1beta1().Deployments(service.TenantID).Patch(deploy.ReplicationID, types.StrategicMergePatchType, Replicas(int(replicas)))
		if err != nil {
			logrus.Error("patch Deployment info error.", err.Error())
			logger.Error(fmt.Sprintf("更改Deployment Pod数量为%d失败", replicas), map[string]string{"step": "worker-appm", "status": "error"})
			return err
		}
		err = m.waitDeployment(mode, serviceID, replicas, logger, deployment)
		if err != nil {
			logrus.Errorf("patch Deployment replicas to %d and watch error.%v", replicas, err.Error())
			logger.Error(fmt.Sprintf("更改Deployment Pod数量为%d结果检测失败", replicas), map[string]string{"step": "worker-appm", "status": "error"})
			return err
		}
	case model.TypeReplicationController:
		//更新ReplicationController pod数量为replicas
		logger.Info("水平伸缩资源类型为TypeReplicationController", map[string]string{"step": "worker-appm"})
		rc, err := m.kubeclient.Core().ReplicationControllers(service.TenantID).Patch(deploy.ReplicationID, types.StrategicMergePatchType, Replicas(int(replicas)))
		if err != nil {
			logrus.Error("patch ReplicationController info error.", err.Error())
			logger.Error(fmt.Sprintf("更改ReplicationController Pod数量为%d失败", replicas), map[string]string{"step": "worker-appm", "status": "error"})
			return err
		}
		err = m.waitReplicationController(mode, serviceID, replicas, logger, rc)
		if err != nil {
			logrus.Errorf("patch ReplicationController replicas to %d and watch error.%v", replicas, err.Error())
			logger.Error(fmt.Sprintf("更改ReplicationController Pod数量为%d结果检测失败", replicas), map[string]string{"step": "worker-appm", "status": "error"})
			return err
		}
	}
	return nil
}

//SyncData 同步数据库数据
func (m *manager) SyncData() {
	//step1 :同步service_deploy_record
	//正向同步
	deploys, err := m.dbmanager.K8sDeployReplicationDao().GetReplications()
	if err != nil {
		logrus.Error("get deploy info error when sync data.", err.Error())
		return
	}
	var deletelist []uint
	if len(deploys) > 0 {
		for i := range deploys {
			deploy := deploys[i]
			if deploy.IsDelete {
				deletelist = append(deletelist, deploy.ID)
				continue
			}
			if deploy.ReplicationType == model.TypeReplicationController || deploy.ReplicationType == "" {
				_, err := m.kubeclient.Core().ReplicationControllers(deploy.TenantID).Get(deploy.ReplicationID, metav1.GetOptions{})
				if err != nil {
					if err := checkNotFoundError(err); err == nil {
						err = m.dbmanager.K8sDeployReplicationDao().DeleteK8sDeployReplication(deploy.ReplicationID)
						if err != nil {
							logrus.Errorf("delete deploy %s info error when sync data.%v", deploy.ReplicationID, err.Error())
						} else {
							logrus.Infof("delete old deploy info of service %s", deploy.ServiceID)
							deletelist = append(deletelist, deploy.ID)
						}
					} else {
						logrus.Errorf("get deploy info from kube api error when sync data.%v", err.Error())
					}
				} else {
					deploy.ReplicationType = model.TypeReplicationController
					deploy.CreatedAt = time.Now()
					err = m.dbmanager.K8sDeployReplicationDao().UpdateModel(deploy)
					if err != nil {
						logrus.Errorf("update deploy %s info error when sync data.%v", deploy.ReplicationID, err.Error())
					}
				}
			}
		}
	}
	if len(deletelist) > 0 {
		m.dbmanager.K8sDeployReplicationDao().BeachDelete(deletelist)
	}
	//反向同步
	//TODO:
	// res, err := m.kubeclient.Core().ReplicationControllers(v1.NamespaceAll).List(v1.ListOptions{})
	// if err == nil && res.Size() > 0 {
	// 	for _, rc := range res.Items {
	// 	}
	// }
	//step2 :同步tenant_service_pod
	//TODO:
}
