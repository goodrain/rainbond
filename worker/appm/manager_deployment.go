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
	"github.com/goodrain/rainbond/util"

	"github.com/Sirupsen/logrus"
	"github.com/jinzhu/gorm"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/apis/apps/v1beta1"
)

//StartDeployment 部署StartDeployment
//返回部署结果
func (m *manager) StartDeployment(serviceID string, logger event.Logger) (*v1beta1.Deployment, error) {
	logger.Info("创建Deployment资源开始", map[string]string{"step": "worker-appm", "status": "starting"})
	builder, err := DeploymentBuilder(serviceID, logger, m.conf.NodeAPI)
	if err != nil {
		logrus.Error("create Deployment builder error.", err.Error())
		logger.Error("创建Deployment Builder失败", map[string]string{"step": "worker-appm", "status": "error"})
		return nil, err
	}
	//判断应用镜像名称是否合法，非法镜像名进制启动
	imageName := builder.service.ImageName
	deployVersion, err := m.dbmanager.VersionInfoDao().GetVersionByDeployVersion(builder.service.DeployVersion, serviceID)
	if err != nil {
		logrus.Warnf("error get version info by deployversion %s,details %s", builder.service.DeployVersion, err.Error())
	} else {
		if CheckVersionInfo(deployVersion) {
			imageName = deployVersion.ImageName
		}
	}
	if !strings.HasPrefix(imageName, "goodrain.me/") {
		logger.Error(fmt.Sprintf("启动应用失败,镜像名(%s)非法，请重新构建应用", builder.service.ImageName), map[string]string{"step": "callback", "status": "error"})
		return nil, fmt.Errorf("service image name invoid, it only can with prefix goodrain.me/")
	}
	deployment, err := builder.Build(util.NewUUID())
	if err != nil {
		logrus.Error("build Deployment error.", err.Error())
		logger.Error("创建Deployment失败", map[string]string{"step": "worker-appm", "status": "error"})
		return nil, err
	}
	result, err := m.kubeclient.AppsV1beta1().Deployments(builder.GetTenant()).Create(deployment)
	if err != nil {
		logrus.Error("deploy Deployment to apiserver error.", err.Error())
		logger.Error("部署Deployment到集群失败", map[string]string{"step": "worker-appm", "status": "error"})
		return nil, err
	}
	err = m.dbmanager.K8sDeployReplicationDao().AddModel(&model.K8sDeployReplication{
		TenantID:        builder.GetTenant(),
		ServiceID:       serviceID,
		ReplicationID:   deployment.Name,
		ReplicationType: model.TypeDeployment,
		DeployVersion:   builder.service.DeployVersion,
	})
	if err != nil {
		logrus.Error("save Deployment info to db error.", err.Error())
		logger.Error("存储Deployment信息到数据库错误", map[string]string{"step": "worker-appm", "status": "error"})
	}
	err = m.waitDeploymentReplicasReady(*deployment.Spec.Replicas, serviceID, logger, result)
	if err != nil {
		if err == ErrTimeOut {
			return result, err
		}
		logrus.Error("deploy Deployment to apiserver then watch error.", err.Error())
		logger.Error("Deployment实例启动情况检测失败", map[string]string{"step": "worker-appm", "status": "error"})
		return result, err
	}
	return result, nil
}

//StopDeployment 停止
func (m *manager) StopDeployment(serviceID string, logger event.Logger) error {
	logger.Info("停止删除Deployment资源开始", map[string]string{"step": "worker-appm", "status": "starting"})
	service, err := m.dbmanager.TenantServiceDao().GetServiceByID(serviceID)
	if err != nil {
		logrus.Error("delete Deployment error. find service from db error", err.Error())
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
		//更新deployment pod数量为0
		deployment, err := m.kubeclient.AppsV1beta1().Deployments(service.TenantID).Patch(deploy.ReplicationID, types.StrategicMergePatchType, Replicas0)
		if err != nil {
			if err = checkNotFoundError(err); err != nil {
				logrus.Error("patch Deployment info error.", err.Error())
				logger.Error("更改Deployment Pod数量为0失败", map[string]string{"step": "worker-appm", "status": "error"})
				return err
			}
			err = m.dbmanager.K8sDeployReplicationDao().DeleteK8sDeployReplicationByService(serviceID)
			if err != nil {
				if err != gorm.ErrRecordNotFound {
					logrus.Error("delete deploy info from db error.", err.Error())
				}
			}
			return nil
		}
		//判断pod数量为0
		err = m.waitDeploymentReplicas(0, logger, deployment)
		if err != nil {
			if err != ErrTimeOut {
				logger.Error("更改Deployment Pod数量为0结果检测错误", map[string]string{"step": "worker-appm", "status": "error"})
				logrus.Error("patch Deployment replicas to 0 and watch error.", err.Error())
				return err
			}
			logger.Error("更改Deployment Pod数量为0结果检测超时,继续删除Deployment", map[string]string{"step": "worker-appm", "status": "error"})
		}
		//删除deployment
		err = m.kubeclient.AppsV1beta1().Deployments(service.TenantID).Delete(service.ServiceAlias, &metav1.DeleteOptions{})
		if err != nil {
			if err = checkNotFoundError(err); err != nil {
				logrus.Error("delete Deployment error.", err.Error())
				logger.Error("从集群中删除Deployment失败", map[string]string{"step": "worker-appm", "status": "error"})
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

	//清理集群内可能遗留的资源
	deletePodsErr := DeletePods(m, service, logger);
	if deletePodsErr != nil {
		return deletePodsErr
	}
	rcList, err := m.kubeclient.AppsV1beta1().Deployments(service.ServiceID).List(metav1.ListOptions{LabelSelector: fmt.Sprintf("name=%s,creator=%s,version=%s", service.ServiceAlias, "RainBond", service.DeployVersion)})
	if err != nil {
		if err = checkNotFoundError(err); err != nil {
			logrus.Error("get service Deployments error.", err.Error())
			logger.Error("从集群中查询该应用的Deployments失败", map[string]string{"step": "worker-appm", "status": "error"})
			return err
		}
	}
	for _, v := range rcList.Items {
		err := m.kubeclient.AppsV1beta1().Deployments(service.ServiceID).Delete(v.Name, &metav1.DeleteOptions{});
		if err != nil {
			if err = checkNotFoundError(err); err != nil {
				logrus.Error("delete service Deployments error.", err.Error())
				logger.Error("从集群中删除应用的Deployments失败", map[string]string{"step": "worker-appm", "status": "error"})
				return err
			}
		}
	}

	logger.Info("根据资源标签移除残留的Deployments资源完成", map[string]string{"step": "worker-appm", "status": "starting"})

	return nil
}

func (m *manager) waitDeployment(mode, serviceID string, n int32, logger event.Logger, deployment *v1beta1.Deployment) error {
	if mode == "up" {
		logger.Info("扩容结果监听开始", map[string]string{"step": "worker-appm", "status": "starting"})
		return m.waitDeploymentReplicasReady(n, serviceID, logger, deployment)
	}
	if mode == "down" {
		logger.Info("缩容结果监听开始", map[string]string{"step": "worker-appm", "status": "starting"})
		return m.waitDeploymentReplicas(n, logger, deployment)
	}
	return nil
}

//移除实例检测
func (m *manager) waitDeploymentReplicas(n int32, logger event.Logger, deployment *v1beta1.Deployment) error {
	if deployment.Status.Replicas <= n {
		return nil
	}
	second := int32(40)
	var deleteCount int32
	if deployment.Status.Replicas-n > 0 {
		deleteCount = deployment.Status.Replicas - n
		second = second * deleteCount
	}
	logger.Info(fmt.Sprintf("实例开始关闭，需要关闭实例数 %d, 超时时间:%d秒 ", deployment.Status.Replicas-n, second), map[string]string{"step": "worker-appm"})
	timeout := time.Tick(time.Duration(second) * time.Second)
	watch, err := m.kubeclient.AppsV1beta1().Deployments(deployment.Namespace).Watch(metav1.ListOptions{
		LabelSelector:   fmt.Sprintf("name=%s,version=%s", deployment.Labels["name"], deployment.Labels["version"]),
		ResourceVersion: deployment.ResourceVersion,
	})
	if err != nil {
		return err
	}
	defer watch.Stop()
	podWatch, err := m.kubeclient.CoreV1().Pods(deployment.Namespace).Watch(metav1.ListOptions{
		LabelSelector: fmt.Sprintf("name=%s,version=%s", deployment.Labels["name"], deployment.Labels["version"]),
	})
	if err != nil {
		return err
	}
	defer podWatch.Stop()
	for {
		select {
		case <-timeout:
			logger.Error("实例关闭超时，请重试！", map[string]string{"step": "worker-appm", "status": "error"})
			return ErrTimeOut
		case event := <-watch.ResultChan():
			state := event.Object.(*v1beta1.Deployment)
			logger.Info(fmt.Sprintf("实例正在关闭，当前应用实例数 %d", state.Status.Replicas), map[string]string{"step": "worker-appm"})
		case event := <-podWatch.ResultChan():
			if event.Type == "DELETED" {
				deleteCount--
				pod := event.Object.(*v1.Pod)
				m.statusCache.RemovePod(pod.Name)
				logger.Info(fmt.Sprintf("实例(%s)已停止并移除", pod.Name), map[string]string{"step": "worker-appm"})
				if deleteCount <= 0 {
					return nil
				}
			}
		}
	}
}

//增加实例检测
func (m *manager) waitDeploymentReplicasReady(n int32, serviceID string, logger event.Logger, deployment *v1beta1.Deployment) error {
	if deployment.Status.Replicas >= n {
		logger.Info(fmt.Sprintf("启动实例数 %d,已完成", deployment.Status.Replicas), map[string]string{"step": "worker-appm"})
		return nil
	}
	second := int32(60)
	if deployment != nil && len(deployment.Spec.Template.Spec.Containers) > 0 {
		for _, c := range deployment.Spec.Template.Spec.Containers {
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
	watch, err := m.kubeclient.AppsV1beta1().Deployments(deployment.Namespace).Watch(metav1.ListOptions{
		LabelSelector:   fmt.Sprintf("name=%s,version=%s", deployment.Labels["name"], deployment.Labels["version"]),
		ResourceVersion: deployment.ResourceVersion,
	})
	if err != nil {
		return err
	}
	defer watch.Stop()
	podWatch, err := m.kubeclient.CoreV1().Pods(deployment.Namespace).Watch(metav1.ListOptions{
		LabelSelector: fmt.Sprintf("name=%s,version=%s", deployment.Labels["name"], deployment.Labels["version"]),
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
		case event := <-watch.ResultChan():
			state := event.Object.(*v1beta1.Deployment)
			logger.Info(fmt.Sprintf("实例正在启动，当前启动实例数 %d,未启动实例数 %d ", state.Status.Replicas, n-state.Status.Replicas), map[string]string{"step": "worker-appm"})
		case event := <-podWatch.ResultChan():
			if event.Type == "ADDED" || event.Type == "MODIFIED" {
				pod := event.Object.(*v1.Pod)
				status := m.statusCache.AddPod(pod.Name, logger)
				if ok, err := status.AddStatus(pod.Status); ok {
					readyPodCount++
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

func (m *manager) RollingUpgradeDeployment(serviceID string, logger event.Logger) (*v1beta1.Deployment, error) {
	deploys, err := m.dbmanager.K8sDeployReplicationDao().GetK8sDeployReplicationByService(serviceID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			logger.Info("应用未部署，开始启动应用", map[string]string{"step": "worker-appm", "status": "success"})
			return m.StartDeployment(serviceID, logger)
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
		return m.StartDeployment(serviceID, logger)
	}

	logger.Info("Deployment滚动更新创建资源开始", map[string]string{"step": "worker-appm", "status": "starting"})
	builder, err := DeploymentBuilder(serviceID, logger, m.conf.NodeAPI)
	if err != nil {
		logrus.Error("create Deployment builder error.", err.Error())
		logger.Error("创建Deployment Builder失败", map[string]string{"step": "worker-appm", "status": "error"})
		return nil, err
	}
	deployment, err := builder.Build(util.NewUUID())
	if err != nil {
		logrus.Error("build Deployment error.", err.Error())
		logger.Error("创建Deployment失败", map[string]string{"step": "worker-appm", "status": "error"})
		return nil, err
	}
	m.kubeclient.AppsV1beta1().Deployments(deploy.TenantID).Update(deployment)
	return nil, nil
}
