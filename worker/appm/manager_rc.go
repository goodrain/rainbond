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
	"fmt"
	"strings"
	"time"

	"github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/event"

	"github.com/Sirupsen/logrus"
	"github.com/jinzhu/gorm"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/pkg/api/v1"
)

//StartReplicationController 部署StartReplicationController
//返回部署结果
func (m *manager) StartReplicationController(serviceID string, logger event.Logger) (*v1.ReplicationController, error) {
	logger.Info("创建ReplicationController资源开始", map[string]string{"step": "worker-appm", "status": "starting"})
	builder, err := ReplicationControllerBuilder(serviceID, logger, m.conf.NodeAPI)
	if err != nil {
		logrus.Error("create ReplicationController builder error.", err.Error())
		logger.Error("创建ReplicationController Builder失败", map[string]string{"step": "worker-appm", "status": "error"})
		return nil, err
	}
	//判断应用镜像名称是否合法，非法镜像名进制启动
	deployVersion, err := m.dbmanager.VersionInfoDao().GetVersionByDeployVersion(builder.service.DeployVersion, serviceID)
	imageName := builder.service.ImageName
	if err != nil {
		logrus.Warnf("error get version info by deployversion %s,details %s", builder.service.DeployVersion, err.Error())
	} else {
		if CheckVersionInfo(deployVersion) {
			imageName = deployVersion.ImageName
		}
	}
	if !strings.HasPrefix(imageName, "goodrain.me/") {
		logger.Error("启动应用失败,镜像名(%s)非法，请重新构建应用", map[string]string{"step": "callback", "status": "error"})
		return nil, fmt.Errorf("service image name invoid, it only can with prefix goodrain.me/")
	}
	rc, err := builder.Build()
	if err != nil {
		logrus.Error("build ReplicationController error.", err.Error())
		logger.Error("创建ReplicationController失败", map[string]string{"step": "worker-appm", "status": "error"})
		return nil, err
	}
	result, err := m.kubeclient.Core().ReplicationControllers(builder.GetTenant()).Create(rc)
	if err != nil {
		logrus.Error("deploy ReplicationController to apiserver error.", err.Error())
		logger.Error("部署ReplicationController到集群失败", map[string]string{"step": "worker-appm", "status": "error"})
		return nil, err
	}
	err = m.dbmanager.K8sDeployReplicationDao().AddModel(&model.K8sDeployReplication{
		TenantID:        builder.GetTenant(),
		ServiceID:       serviceID,
		ReplicationID:   rc.Name,
		ReplicationType: model.TypeReplicationController,
		DeployVersion:   builder.service.DeployVersion,
	})
	if err != nil {
		logrus.Error("save ReplicationController info to db error.", err.Error())
		logger.Error("存储ReplicationController信息到数据库错误", map[string]string{"step": "worker-appm", "status": "error"})
	}
	err = m.waitRCReplicasReady(*rc.Spec.Replicas, serviceID, logger, result)
	if err != nil {
		if err == ErrTimeOut {
			return result, err
		}
		logrus.Error("deploy ReplicationController to apiserver then watch error.", err.Error())
		logger.Error("ReplicationController实例启动情况检测失败", map[string]string{"step": "worker-appm", "status": "error"})
		return result, err
	}
	return result, nil
}

//CheckVersionInfo CheckVersionInfo
func CheckVersionInfo(version *model.VersionInfo) bool {
	if !strings.Contains(strings.ToLower(version.FinalStatus), "success") {
		return false
	}
	if len(version.ImageName) == 0 || !strings.Contains(version.ImageName, "goodrain.me/") {
		return false
	}
	return true
}

//StopReplicationController 停止
func (m *manager) StopReplicationController(serviceID string, logger event.Logger) error {
	logger.Info("停止删除ReplicationController资源开始", map[string]string{"step": "worker-appm", "status": "starting"})
	service, err := m.dbmanager.TenantServiceDao().GetServiceByID(serviceID)
	if err != nil {
		logrus.Errorf("delete ReplicationController of service(%s) error. find service from db error %v", serviceID, err.Error())
		logger.Error("查询应用信息失败", map[string]string{"step": "worker-appm", "status": "error"})
		return err
	}
	deploys, err := m.dbmanager.K8sDeployReplicationDao().GetK8sDeployReplicationByService(serviceID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			logger.Error("应用未部署", map[string]string{"step": "worker-appm", "status": "error"})
			return ErrNotDeploy
		}
		logrus.Error("find service deploy info from db error", err.Error())
		logger.Error("查询应用部署信息失败", map[string]string{"step": "worker-appm", "status": "error"})
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
		logger.Error("应用未部署", map[string]string{"step": "worker-appm", "status": "error"})
		return ErrNotDeploy
	}
	for _, deploy := range deploys {
		//更新rc pod数量为0
		rc, err := m.kubeclient.Core().ReplicationControllers(service.TenantID).Patch(deploy.ReplicationID, types.StrategicMergePatchType, Replicas0)
		if err != nil {
			if err = checkNotFoundError(err); err != nil {
				logrus.Error("patch ReplicationController info error.", err.Error())
				logger.Error("更改ReplicationController Pod数量为0失败", map[string]string{"step": "worker-appm", "status": "error"})
				return err
			}
			logger.Info("集群中ReplicationController已不存在", map[string]string{"step": "worker-appm", "status": "error"})
			err = m.dbmanager.K8sDeployReplicationDao().DeleteK8sDeployReplicationByService(serviceID)
			if err != nil {
				if err != gorm.ErrRecordNotFound {
					logrus.Error("delete deploy info from db error.", err.Error())
				}
			}
			return nil
		}
		//判断pod数量为0
		err = m.waitRCReplicas(0, logger, rc)
		if err != nil {
			if err != ErrTimeOut {
				logger.Error("更改RC Pod数量为0结果检测错误", map[string]string{"step": "worker-appm", "status": "error"})
				logrus.Error("patch ReplicationController replicas to 0 and watch error.", err.Error())
				return err
			}
			logger.Error("更改RC Pod数量为0结果检测超时,继续删除RC", map[string]string{"step": "worker-appm", "status": "error"})
		}
		//删除rc
		err = m.kubeclient.Core().ReplicationControllers(service.TenantID).Delete(deploy.ReplicationID, &metav1.DeleteOptions{})
		if err != nil {
			if err = checkNotFoundError(err); err != nil {
				logrus.Error("delete ReplicationController error.", err.Error())
				logger.Error("从集群中删除ReplicationController失败", map[string]string{"step": "worker-appm", "status": "error"})
				return err
			}
		}
		err = m.dbmanager.K8sDeployReplicationDao().DeleteK8sDeployReplicationByService(serviceID)
		if err != nil {
			if err != gorm.ErrRecordNotFound {
				logrus.Error("delete deploy info from db error.", err.Error())
			}
		}
	}
	//删除未移除成功的pod
	logger.Info("开始移除残留的Pod实例", map[string]string{"step": "worker-appm", "status": "starting"})
	pods, err := m.dbmanager.K8sPodDao().GetPodByService(serviceID)
	if err != nil {
		logrus.Error("get more than need by deleted pod from db error.", err.Error())
		logger.Error("查询更过需要被移除的Pod失败", map[string]string{"step": "worker-appm", "status": "error"})
	}
	if pods != nil && len(pods) > 0 {
		for i := range pods {
			pod := pods[i]
			err = m.kubeclient.CoreV1().Pods(service.TenantID).Delete(pod.PodName, &metav1.DeleteOptions{})
			if err != nil {
				if err = checkNotFoundError(err); err != nil {
					logrus.Errorf("delete pod (%s) from k8s api error %s", pod.PodName, err.Error())
				}
			} else {
				logger.Info(fmt.Sprintf("实例(%s)已停止并移除", pod.PodName), map[string]string{"step": "worker-appm"})
			}

		}
		err = m.dbmanager.K8sPodDao().DeleteK8sPod(serviceID)
		if err != nil {
			if err != gorm.ErrRecordNotFound {
				logrus.Error("delete pods by service id error.", err.Error())
			}
		}
	}
	logger.Info("移除残留的Pod实例完成", map[string]string{"step": "worker-appm", "status": "starting"})
	return nil
}

func (m *manager) waitReplicationController(mode, serviceID string, n int32, logger event.Logger, rc *v1.ReplicationController) error {
	if mode == "up" {
		logger.Info("扩容结果监听开始", map[string]string{"step": "worker-appm", "status": "starting"})
		return m.waitRCReplicasReady(n, serviceID, logger, rc)
	}
	if mode == "down" {
		logger.Info("缩容结果监听开始", map[string]string{"step": "worker-appm", "status": "starting"})
		return m.waitRCReplicas(n, logger, rc)
	}
	return nil
}

//RollingUpgradeReplicationController 滚动升级RC
func (m *manager) RollingUpgradeReplicationController(serviceID string, stopChan chan struct{}, logger event.Logger) (*v1.ReplicationController, error) {
	deploys, err := m.dbmanager.K8sDeployReplicationDao().GetK8sDeployReplicationByService(serviceID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			logger.Info("应用未部署，开始启动应用", map[string]string{"step": "worker-appm", "status": "success"})
			return m.StartReplicationController(serviceID, logger)
		}
		logrus.Error("get old deploy info error.", err.Error())
		logger.Error("获取当前应用部署信息失败", map[string]string{"step": "worker-appm", "status": "error"})
		return nil, err
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
		logger.Info("应用未部署，开始启动应用", map[string]string{"step": "worker-appm", "status": "success"})
		return m.StartReplicationController(serviceID, logger)
	}
	logger.Info("创建ReplicationController资源开始", map[string]string{"step": "worker-appm", "status": "starting"})
	builder, err := ReplicationControllerBuilder(serviceID, logger, m.conf.NodeAPI)
	if err != nil {
		logrus.Error("create ReplicationController builder error.", err.Error())
		logger.Error("创建ReplicationController Builder失败", map[string]string{"step": "worker-appm", "status": "error"})
		return nil, err
	}
	rc, err := builder.Build()
	if err != nil {
		logrus.Error("build ReplicationController error.", err.Error())
		logger.Error("创建ReplicationController失败", map[string]string{"step": "worker-appm", "status": "error"})
		return nil, err
	}
	var replicas = rc.Spec.Replicas
	rc.Spec.Replicas = int32Ptr(0)
	result, err := m.kubeclient.Core().ReplicationControllers(builder.GetTenant()).Create(rc)
	if err != nil {
		logrus.Error("deploy ReplicationController to apiserver error.", err.Error())
		logger.Error("部署ReplicationController到集群失败", map[string]string{"step": "worker-appm", "status": "error"})
		return nil, err
	}
	var oldRCName = deploy.ReplicationID
	newDeploy := &model.K8sDeployReplication{
		ReplicationID:   rc.Name,
		DeployVersion:   rc.Labels["version"],
		ReplicationType: model.TypeReplicationController,
		TenantID:        deploy.TenantID,
		ServiceID:       serviceID,
	}
	err = m.dbmanager.K8sDeployReplicationDao().AddModel(newDeploy)
	if err != nil {
		m.kubeclient.Core().ReplicationControllers(builder.GetTenant()).Delete(rc.Name, &metav1.DeleteOptions{})
		logrus.Error("save new deploy info to db error.", err.Error())
		logger.Error("添加部署信息失败", map[string]string{"step": "worker-appm", "status": "error"})
		return nil, err
	}
	//保证删除旧RC
	defer func() {
		//step3 delete old rc
		//注入status模块，忽略本RC的感知
		m.statusManager.IgnoreDelete(oldRCName)
		err = m.kubeclient.Core().ReplicationControllers(builder.GetTenant()).Delete(oldRCName, &metav1.DeleteOptions{})
		if err != nil {
			if err = checkNotFoundError(err); err != nil {
				logrus.Error("delete ReplicationController error.", err.Error())
				logger.Error("从集群中删除ReplicationController失败", map[string]string{"step": "worker-appm", "status": "error"})
			}
		}
		m.dbmanager.K8sDeployReplicationDao().DeleteK8sDeployReplicationByServiceAndVersion(deploy.ServiceID, deploy.DeployVersion)
	}()
	//step2 sync pod number
	logger.Info("开始滚动替换实例", map[string]string{"step": "worker-appm", "status": "starting"})
	var i int32
	for i = 1; i <= *replicas; i++ {
		if err := m.rollingUpgrade(builder.GetTenant(), serviceID, oldRCName, result.Name, (*replicas)-i, i, logger); err != nil {
			return nil, err
		}
		select {
		case <-stopChan:
			logger.Info("应用滚动升级停止。", map[string]string{"step": "worker-appm"})
			return nil, nil
		default:
			//延时1s
			//TODO: 外层传入时间间隔
			if i < *replicas {
				time.Sleep(time.Second * 1)
			}
		}
	}
	//删除未移除成功的pod
	logger.Info("开始移除残留的Pod实例", map[string]string{"step": "worker-appm", "status": "starting"})
	pods, err := m.dbmanager.K8sPodDao().GetPodByReplicationID(oldRCName)
	if err != nil {
		logrus.Error("get more than need by deleted pod from db error.", err.Error())
		logger.Error("查询更过需要被移除的Pod失败", map[string]string{"step": "worker-appm", "status": "error"})
	}
	if pods != nil && len(pods) > 0 {
		for i := range pods {
			pod := pods[i]
			err = m.kubeclient.CoreV1().Pods(builder.GetTenant()).Delete(pod.PodName, &metav1.DeleteOptions{})
			if err != nil {
				if err = checkNotFoundError(err); err != nil {
					logrus.Errorf("delete pod (%s) from k8s api error %s", pod.PodName, err.Error())
				}
			} else {
				logger.Info(fmt.Sprintf("实例(%s)已停止并移除", pod.PodName), map[string]string{"step": "worker-appm"})
			}
		}
	}
	logger.Info("移除残留的Pod实例完成", map[string]string{"step": "worker-appm", "status": "success"})
	return rc, nil
}

//RollingUpgradeReplicationControllerCompatible 滚动升级RC
//该方法的存在是为了兼容旧应用，如旧版MySQL被设为无状态类型，所以要先删除实例再创建新实例
func (m *manager) RollingUpgradeReplicationControllerCompatible(serviceID string, stopChan chan struct{}, logger event.Logger) (*v1.ReplicationController, error) {
	deploys, err := m.dbmanager.K8sDeployReplicationDao().GetK8sDeployReplicationByService(serviceID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			logger.Info("应用未部署，开始启动应用", map[string]string{"step": "worker-appm", "status": "success"})
			return m.StartReplicationController(serviceID, logger)
		}
		logrus.Error("get old deploy info error.", err.Error())
		logger.Error("获取当前应用部署信息失败", map[string]string{"step": "worker-appm", "status": "error"})
		return nil, err
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
		logger.Info("应用未部署，开始启动应用", map[string]string{"step": "worker-appm", "status": "success"})
		return m.StartReplicationController(serviceID, logger)
	}
	logger.Info("创建ReplicationController资源开始", map[string]string{"step": "worker-appm", "status": "starting"})
	builder, err := ReplicationControllerBuilder(serviceID, logger, m.conf.NodeAPI)
	if err != nil {
		logrus.Error("create ReplicationController builder error.", err.Error())
		logger.Error("创建ReplicationController Builder失败", map[string]string{"step": "worker-appm", "status": "error"})
		return nil, err
	}
	rc, err := builder.Build()
	if err != nil {
		logrus.Error("build ReplicationController error.", err.Error())
		logger.Error("创建ReplicationController失败", map[string]string{"step": "worker-appm", "status": "error"})
		return nil, err
	}

	// create new empty rc in k8s
	var replicas = rc.Spec.Replicas
	rc.Spec.Replicas = int32Ptr(0)
	result, err := m.kubeclient.Core().ReplicationControllers(builder.GetTenant()).Create(rc)
	if err != nil {
		logrus.Error("deploy ReplicationController to apiserver error.", err.Error())
		logger.Error("部署ReplicationController到集群失败", map[string]string{"step": "worker-appm", "status": "error"})
		return nil, err
	}

	// mark for ready to delete the old rc in db
	deploy.IsDelete = true
	err = m.dbmanager.K8sDeployReplicationDao().UpdateModel(deploy)
	if err != nil {
		logrus.Error("Failed to mark for ready to delete the rc in db: ", err)
	}

	// create new rc in db
	var oldRCName = deploy.ReplicationID
	newDeploy := &model.K8sDeployReplication{
		ReplicationID:   rc.Name,
		DeployVersion:   rc.Labels["version"],
		ReplicationType: model.TypeReplicationController,
		TenantID:        deploy.TenantID,
		ServiceID:       serviceID,
	}
	err = m.dbmanager.K8sDeployReplicationDao().AddModel(newDeploy)
	if err != nil {
		m.kubeclient.Core().ReplicationControllers(builder.GetTenant()).Delete(rc.Name, &metav1.DeleteOptions{})
		logrus.Error("save new deploy info to db error.", err.Error())
		logger.Error("添加部署信息失败", map[string]string{"step": "worker-appm", "status": "error"})
		return nil, err
	}

	defer func() {
		//step3 delete old rc
		//注入status模块，忽略本RC的感知
		m.statusManager.IgnoreDelete(oldRCName)
		err = m.kubeclient.Core().ReplicationControllers(builder.GetTenant()).Delete(oldRCName, &metav1.DeleteOptions{})
		if err != nil {
			if err = checkNotFoundError(err); err != nil {
				logrus.Error("delete ReplicationController error.", err.Error())
				logger.Error("从集群中删除ReplicationController失败", map[string]string{"step": "worker-appm", "status": "error"})
			}
		}
		m.dbmanager.K8sDeployReplicationDao().DeleteK8sDeployReplicationByServiceAndMarked(deploy.ServiceID)
	}()
	//step2 sync pod number
	logger.Info("开始滚动替换实例", map[string]string{"step": "worker-appm", "status": "starting"})
	var i int32
	for i = 1; i <= *replicas; i++ {
		if err := m.rollingUpgradeCompatible(builder.GetTenant(), serviceID, oldRCName, result.Name, (*replicas)-i, i, logger); err != nil {
			return nil, err
		}
		select {
		case <-stopChan:
			logger.Info("应用滚动升级停止。", map[string]string{"step": "worker-appm"})
			return nil, nil
		default:
			//延时1s
			//TODO: 外层传入时间间隔
			if i < *replicas {
				time.Sleep(time.Second * 1)
			}
		}
	}
	//删除未移除成功的pod
	logger.Info("开始移除残留的Pod实例", map[string]string{"step": "worker-appm", "status": "starting"})
	pods, err := m.dbmanager.K8sPodDao().GetPodByReplicationID(oldRCName)
	if err != nil {
		logrus.Error("get more than need by deleted pod from db error.", err.Error())
		logger.Error("查询更过需要被移除的Pod失败", map[string]string{"step": "worker-appm", "status": "error"})
	}
	if pods != nil && len(pods) > 0 {
		for i := range pods {
			pod := pods[i]
			err = m.kubeclient.CoreV1().Pods(builder.GetTenant()).Delete(pod.PodName, &metav1.DeleteOptions{})
			if err != nil {
				if err = checkNotFoundError(err); err != nil {
					logrus.Errorf("delete pod (%s) from k8s api error %s", pod.PodName, err.Error())
				}
			} else {
				logger.Info(fmt.Sprintf("实例(%s)已停止并移除", pod.PodName), map[string]string{"step": "worker-appm"})
			}
		}
	}
	logger.Info("移除残留的Pod实例完成", map[string]string{"step": "worker-appm", "status": "success"})
	return rc, nil
}

//该方法的存在是为了兼容旧应用，如旧版MySQL被设为无状态类型，所以要先删除实例再创建新实例
func (m *manager) rollingUpgradeCompatible(namespace, serviceID, oldRC, newRC string, oldReplicase, newReplicas int32, logger event.Logger) error {
	logger.Info(fmt.Sprintf("新版实例数:%d,旧版实例数:%d,替换开始", newReplicas, oldReplicase), map[string]string{"step": "worker-appm", "status": "starting"})
	// first delete old pods
	rc, err := m.kubeclient.Core().ReplicationControllers(namespace).Patch(oldRC, types.StrategicMergePatchType, Replicas(int(oldReplicase)))
	if err != nil {
		logrus.Error("patch ReplicationController info error.", err.Error())
		logger.Error(fmt.Sprintf("更改ReplicationController Pod数量为%d失败", oldReplicase), map[string]string{"step": "worker-appm", "status": "error"})
		return err
	}
	err = m.waitReplicationController("down", serviceID, oldReplicase, logger, rc)
	if err != nil && err.Error() != ErrTimeOut.Error() {
		logrus.Errorf("patch ReplicationController replicas to %d and watch error.%v", oldReplicase, err.Error())
		logger.Error(fmt.Sprintf("更改ReplicationController Pod数量为%d结果检测失败", oldReplicase), map[string]string{"step": "worker-appm", "status": "error"})
		return err
	}
	// create new pods
	rc, err = m.kubeclient.Core().ReplicationControllers(namespace).Patch(newRC, types.StrategicMergePatchType, Replicas(int(newReplicas)))
	if err != nil {
		logrus.Error("patch ReplicationController info error.", err.Error())
		logger.Error(fmt.Sprintf("更改ReplicationController Pod数量为%d失败", newReplicas), map[string]string{"step": "worker-appm", "status": "error"})
		return err
	}
	err = m.waitReplicationController("up", serviceID, newReplicas, logger, rc)
	if err != nil && err.Error() != ErrTimeOut.Error() {
		logrus.Errorf("patch ReplicationController replicas to %d and watch error.%v", newReplicas, err.Error())
		logger.Error(fmt.Sprintf("更改ReplicationController Pod数量为%d结果检测失败", newReplicas), map[string]string{"step": "worker-appm", "status": "error"})
		return err
	}
	logger.Info(fmt.Sprintf("新版实例数:%d,旧版实例数:%d,替换完成", newReplicas, oldReplicase), map[string]string{"step": "worker-appm", "status": "starting"})
	return nil
}

func (m *manager) rollingUpgrade(namespace, serviceID, oldRC, newRC string, oldReplicase, newReplicas int32, logger event.Logger) error {
	logger.Info(fmt.Sprintf("新版实例数:%d,旧版实例数:%d,替换开始", newReplicas, oldReplicase), map[string]string{"step": "worker-appm", "status": "starting"})
	rc, err := m.kubeclient.Core().ReplicationControllers(namespace).Patch(newRC, types.StrategicMergePatchType, Replicas(int(newReplicas)))
	if err != nil {
		logrus.Error("patch ReplicationController info error.", err.Error())
		logger.Error(fmt.Sprintf("更改ReplicationController Pod数量为%d失败", newReplicas), map[string]string{"step": "worker-appm", "status": "error"})
		return err
	}
	err = m.waitReplicationController("up", serviceID, newReplicas, logger, rc)
	if err != nil && err.Error() != ErrTimeOut.Error() {
		logrus.Errorf("patch ReplicationController replicas to %d and watch error.%v", newReplicas, err.Error())
		logger.Error(fmt.Sprintf("更改ReplicationController Pod数量为%d结果检测失败", newReplicas), map[string]string{"step": "worker-appm", "status": "error"})
		return err
	}
	rc, err = m.kubeclient.Core().ReplicationControllers(namespace).Patch(oldRC, types.StrategicMergePatchType, Replicas(int(oldReplicase)))
	if err != nil {
		logrus.Error("patch ReplicationController info error.", err.Error())
		logger.Error(fmt.Sprintf("更改ReplicationController Pod数量为%d失败", oldReplicase), map[string]string{"step": "worker-appm", "status": "error"})
		return err
	}
	err = m.waitReplicationController("down", serviceID, oldReplicase, logger, rc)
	if err != nil && err.Error() != ErrTimeOut.Error() {
		logrus.Errorf("patch ReplicationController replicas to %d and watch error.%v", oldReplicase, err.Error())
		logger.Error(fmt.Sprintf("更改ReplicationController Pod数量为%d结果检测失败", oldReplicase), map[string]string{"step": "worker-appm", "status": "error"})
		return err
	}
	logger.Info(fmt.Sprintf("新版实例数:%d,旧版实例数:%d,替换完成", newReplicas, oldReplicase), map[string]string{"step": "worker-appm", "status": "starting"})
	return nil
}

//移除实例检测
func (m *manager) waitRCReplicas(n int32, logger event.Logger, rc *v1.ReplicationController) error {
	if rc.Status.Replicas <= n {
		return nil
	}
	second := int32(40)
	var needdeleteCount int32
	if rc.Status.Replicas-n > 0 {
		needdeleteCount = rc.Status.Replicas - n
		second = second * needdeleteCount
	}
	logger.Info(fmt.Sprintf("实例开始关闭，需要关闭实例数 %d, 超时时间:%d秒 ", rc.Status.Replicas-n, second), map[string]string{"step": "worker-appm"})
	timeout := time.Tick(time.Duration(second) * time.Second)
	podWatch, err := m.kubeclient.CoreV1().Pods(rc.Namespace).Watch(metav1.ListOptions{
		LabelSelector: fmt.Sprintf("name=%s,version=%s", rc.Labels["name"], rc.Labels["version"]),
	})
	if err != nil {
		return err
	}
	defer podWatch.Stop()
	var deleteCount int32
	var total = rc.Status.Replicas
	for {
		select {
		case <-timeout:
			logger.Error("实例关闭超时，请重试！", map[string]string{"step": "worker-appm", "status": "error"})
			return ErrTimeOut
		case event := <-podWatch.ResultChan():
			if event.Type == "DELETED" {
				deleteCount++
				pod := event.Object.(*v1.Pod)
				m.statusCache.RemovePod(pod.Name)
				logger.Info(fmt.Sprintf("实例(%s)已停止并移除,剩余实例数 %d", pod.Name, total-deleteCount), map[string]string{"step": "worker-appm"})
				if deleteCount >= needdeleteCount {
					return nil
				}
			}
		}
	}
}

//增加实例检测
func (m *manager) waitRCReplicasReady(n int32, serviceID string, logger event.Logger, rc *v1.ReplicationController) error {
	if rc.Status.Replicas >= n {
		logger.Info(fmt.Sprintf("启动实例数 %d,已完成", rc.Status.Replicas), map[string]string{"step": "worker-appm"})
		return nil
	}
	second := int32(60)
	if rc.Spec.Template != nil && len(rc.Spec.Template.Spec.Containers) > 0 {
		for _, c := range rc.Spec.Template.Spec.Containers {
			if c.ReadinessProbe != nil {
				second += c.ReadinessProbe.InitialDelaySeconds + c.ReadinessProbe.SuccessThreshold*c.ReadinessProbe.PeriodSeconds
			}
		}
	}
	if n > 0 {
		second = second * n
	}
	logger.Info(fmt.Sprintf("实例开始启动，需要启动实例数 %d, 超时时间:%d秒 ", n, second), map[string]string{"step": "worker-appm"})
	timeout := time.Tick(time.Duration(second) * time.Second)
	podWatch, err := m.kubeclient.CoreV1().Pods(rc.Namespace).Watch(metav1.ListOptions{
		LabelSelector: fmt.Sprintf("name=%s,version=%s", rc.Labels["name"], rc.Labels["version"]),
	})
	if err != nil {
		return err
	}
	defer podWatch.Stop()
	var readyPodCount int32
	for {
		select {
		case <-timeout:
			logger.Error("实例启动超时，置于后台启动，请留意应用状态", map[string]string{"step": "worker-appm", "status": "error"})
			return ErrTimeOut
		case event := <-podWatch.ResultChan():
			if event.Type == "ADDED" || event.Type == "MODIFIED" {
				pod := event.Object.(*v1.Pod)
				status := m.statusCache.AddPod(pod.Name, logger)
				if ok, err := status.AddStatus(pod.Status); ok {
					readyPodCount++
					logger.Info(fmt.Sprintf("实例正在启动，当前就绪实例数 %d, 未启动实例数 %d ", readyPodCount, n-readyPodCount), map[string]string{"step": "worker-appm"})
					if readyPodCount >= n {
						return nil
					}
				} else if err != nil {
					return err
				}
			}
		}
	}
}
