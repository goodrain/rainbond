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
	"k8s.io/client-go/pkg/apis/apps/v1beta1"
)

//StartStatefulSet 部署StartStatefulSet
//返回部署结果
func (m *manager) StartStatefulSet(serviceID string, logger event.Logger) (*v1beta1.StatefulSet, error) {
	logger.Info("创建StatefulSet资源开始", map[string]string{"step": "worker-appm", "status": "starting"})
	builder, err := StatefulSetBuilder(serviceID, logger, m.conf.NodeAPI)
	if err != nil {
		logrus.Error("create statefulset builder error.", err.Error())
		logger.Error("创建StatefulSet Builder失败", map[string]string{"step": "worker-appm", "status": "error"})
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
	statefull, err := builder.Build()
	if err != nil {
		logrus.Error("build statefulset error.", err.Error())
		logger.Error("创建StatefulSet失败", map[string]string{"step": "worker-appm", "status": "error"})
		return nil, err
	}
	//有状态服务先创建service
	if statefull != nil {
		err := m.StartService(serviceID, logger, statefull.Name, model.TypeStatefulSet)
		if err != nil {
			logger.Error("Service创建执行失败。"+err.Error(), map[string]string{"step": "callback", "status": "failure"})
			return nil, err
		}
	}
	result, err := m.kubeclient.AppsV1beta1().StatefulSets(builder.GetTenant()).Create(statefull)
	if err != nil {
		logrus.Error("deploy statefulset to apiserver error.", err.Error())
		logger.Error("部署StatefulSet到集群失败", map[string]string{"step": "worker-appm", "status": "error"})
		return nil, err
	}
	err = m.dbmanager.K8sDeployReplicationDao().AddModel(&model.K8sDeployReplication{
		TenantID:        builder.GetTenant(),
		ServiceID:       serviceID,
		ReplicationID:   statefull.Name,
		ReplicationType: model.TypeStatefulSet,
		DeployVersion:   builder.service.DeployVersion,
	})
	if err != nil {
		logrus.Error("save statefulset info to db error.", err.Error())
		logger.Error("存储StatefulSet信息到数据库错误", map[string]string{"step": "worker-appm", "status": "error"})
	}
	err = m.waitStatefulReplicasReady(*statefull.Spec.Replicas, serviceID, logger, result)
	if err != nil {
		if err == ErrTimeOut {
			return result, err
		}
		logrus.Error("deploy statefulset to apiserver then watch error.", err.Error())
		logger.Error("StatefulSet实例启动情况检测失败", map[string]string{"step": "worker-appm", "status": "error"})
		return result, err
	}
	return result, nil
}

//StopStatefulSet 停止
func (m *manager) StopStatefulSet(serviceID string, logger event.Logger) error {
	logger.Info("停止删除StatefulSet资源开始", map[string]string{"step": "worker-appm", "status": "starting"})
	service, err := m.dbmanager.TenantServiceDao().GetServiceByID(serviceID)
	if err != nil {
		logrus.Error("delete statefulset error. find service from db error", err.Error())
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
		logger.Error("应用未部署", map[string]string{"step": "worker-appm", "status": "success"})
		return ErrNotDeploy
	}
	for _, deploy := range deploys {
		//更新stateful pod数量为0
		stateful, err := m.kubeclient.AppsV1beta1().StatefulSets(service.TenantID).Patch(deploy.ReplicationID, types.StrategicMergePatchType, Replicas0)
		if err != nil {
			if err = checkNotFoundError(err); err != nil {
				logrus.Error("patch statefulset info error.", err.Error())
				logger.Error("更改StatefulSet Pod数量为0失败", map[string]string{"step": "worker-appm", "status": "error"})
				return err
			}
			logger.Info("集群中StatefulSet已不存在", map[string]string{"step": "worker-appm", "status": "error"})
			err = m.dbmanager.K8sDeployReplicationDao().DeleteK8sDeployReplicationByService(serviceID)
			if err != nil {
				if err != gorm.ErrRecordNotFound {
					logrus.Error("delete deploy info from db error.", err.Error())
				}
			}
			return nil
		}
		//判断pod数量为0
		err = m.waitStatefulReplicas(0, logger, stateful)
		if err != nil {
			if err != ErrTimeOut {
				logger.Error("更改StatefulSet Pod数量为0结果检测错误", map[string]string{"step": "worker-appm", "status": "error"})
				logrus.Error("patch StatefulSet replicas to 0 and watch error.", err.Error())
				return err
			}
			logger.Error("更改StatefulSet Pod数量为0结果检测超时,继续删除RC", map[string]string{"step": "worker-appm", "status": "error"})
		}
		//删除stateful
		err = m.kubeclient.AppsV1beta1().StatefulSets(service.TenantID).Delete(service.ServiceAlias, &metav1.DeleteOptions{})
		if err != nil {
			if err = checkNotFoundError(err); err != nil {
				logrus.Error("delete statefulset error.", err.Error())
				logger.Error("从集群中删除StatefulSet失败", map[string]string{"step": "worker-appm", "status": "error"})
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
	return nil
}

func (m *manager) waitStateful(mode, serviceID string, n int32, logger event.Logger, stateful *v1beta1.StatefulSet) error {
	if mode == "up" {
		logger.Info("扩容结果监听开始", map[string]string{"step": "worker-appm", "status": "starting"})
		return m.waitStatefulReplicasReady(n, serviceID, logger, stateful)
	}
	if mode == "down" {
		logger.Info("缩容结果监听开始", map[string]string{"step": "worker-appm", "status": "starting"})
		return m.waitStatefulReplicas(n, logger, stateful)
	}
	return nil
}

//移除实例检测
func (m *manager) waitStatefulReplicas(n int32, logger event.Logger, stateful *v1beta1.StatefulSet) error {
	if stateful.Status.Replicas <= n {
		return nil
	}
	second := int32(40)
	var needdeleteCount int32
	if stateful.Status.Replicas-n > 0 {
		needdeleteCount = stateful.Status.Replicas - n
		second = second * needdeleteCount
	}
	logger.Info(fmt.Sprintf("实例开始顺序关闭，需要关闭实例数 %d, 超时时间:%d秒 ", stateful.Status.Replicas-n, second), map[string]string{"step": "worker-appm"})
	timeout := time.Tick(time.Duration(second) * time.Second)
	podWatch, err := m.kubeclient.CoreV1().Pods(stateful.Namespace).Watch(metav1.ListOptions{
		LabelSelector: fmt.Sprintf("name=%s,version=%s", stateful.Labels["name"], stateful.Labels["version"]),
	})
	if err != nil {
		return err
	}
	defer podWatch.Stop()
	total := stateful.Status.Replicas
	var deleteCount int32
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
				logger.Info(fmt.Sprintf("实例(%s)已停止并移除,当前剩余实例数 %d", pod.Name, total-deleteCount), map[string]string{"step": "worker-appm"})
				if deleteCount >= needdeleteCount {
					return nil
				}
			}
		}
	}
}

//增加实例检测
func (m *manager) waitStatefulReplicasReady(n int32, serviceID string, logger event.Logger, stateful *v1beta1.StatefulSet) error {
	if stateful.Status.Replicas >= n {
		logger.Info(fmt.Sprintf("启动实例数 %d,已完成", stateful.Status.Replicas), map[string]string{"step": "worker-appm"})
		return nil
	}
	second := int32(60)
	if stateful != nil && len(stateful.Spec.Template.Spec.Containers) > 0 {
		for _, c := range stateful.Spec.Template.Spec.Containers {
			if c.ReadinessProbe != nil {
				second += c.ReadinessProbe.InitialDelaySeconds + c.ReadinessProbe.SuccessThreshold*c.ReadinessProbe.PeriodSeconds
			}
		}
	}
	if n > 0 {
		second = second * n
	}
	logger.Info(fmt.Sprintf("实例开始顺序启动，需要启动实例数 %d, 超时时间:%d秒 ", n, second), map[string]string{"step": "worker-appm"})
	timeout := time.Tick(time.Duration(second) * time.Second)
	podWatch, err := m.kubeclient.CoreV1().Pods(stateful.Namespace).Watch(metav1.ListOptions{
		LabelSelector: fmt.Sprintf("name=%s,version=%s", stateful.Labels["name"], stateful.Labels["version"]),
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
					logger.Info(fmt.Sprintf("实例正在顺序启动，当前就绪实例数 %d,未启动实例数 %d ", readyPodCount, n-readyPodCount), map[string]string{"step": "worker-appm"})
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

//RollingUpgradeStatefulSet 临时实现有状态服务的升级，采用重启操作
func (m *manager) RollingUpgradeStatefulSet(serviceID string, logger event.Logger) (*v1beta1.StatefulSet, error) {
	service, err := m.dbmanager.TenantServiceDao().GetServiceByID(serviceID)
	if err != nil {
		logrus.Error("delete statefulset error. find service from db error", err.Error())
		logger.Error("查询应用信息失败", map[string]string{"step": "worker-appm", "status": "error"})
		return nil, err
	}
	deploys, err := m.dbmanager.K8sDeployReplicationDao().GetK8sDeployReplicationByService(serviceID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			logger.Info("应用未部署，开始启动应用", map[string]string{"step": "worker-appm", "status": "success"})
			return m.StartStatefulSet(serviceID, logger)
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
		return m.StartStatefulSet(serviceID, logger)
	}
	logger.Info("有状态服务重启操作开始", map[string]string{"step": "worker-appm", "status": "success"})
	for _, deploy := range deploys {
		//更新stateful pod数量为0
		stateful, err := m.kubeclient.AppsV1beta1().StatefulSets(service.TenantID).Patch(deploy.ReplicationID, types.StrategicMergePatchType, Replicas0)
		if err != nil {
			if err = checkNotFoundError(err); err != nil {
				logrus.Error("patch statefulset info error.", err.Error())
				logger.Error("更改StatefulSet Pod数量为0失败", map[string]string{"step": "worker-appm", "status": "error"})
				return nil, err
			}
			logger.Info("集群中StatefulSet已不存在", map[string]string{"step": "worker-appm", "status": "error"})
			err = m.dbmanager.K8sDeployReplicationDao().DeleteK8sDeployReplicationByService(serviceID)
			if err != nil {
				if err != gorm.ErrRecordNotFound {
					logrus.Error("delete deploy info from db error.", err.Error())
				}
			}
			return nil, nil
		}
		//判断pod数量为0
		err = m.waitStatefulReplicas(0, logger, stateful)
		if err != nil {
			if err != ErrTimeOut {
				logger.Error("更改StatefulSet Pod数量为0结果检测错误", map[string]string{"step": "worker-appm", "status": "error"})
				logrus.Error("patch StatefulSet replicas to 0 and watch error.", err.Error())
				return nil, err
			}
			logger.Error("更改StatefulSet Pod数量为0结果检测超时,继续删除RC", map[string]string{"step": "worker-appm", "status": "error"})
		}
		//删除stateful
		err = m.kubeclient.AppsV1beta1().StatefulSets(service.TenantID).Delete(service.ServiceAlias, &metav1.DeleteOptions{})
		if err != nil {
			if err = checkNotFoundError(err); err != nil {
				logrus.Error("delete statefulset error.", err.Error())
				logger.Error("从集群中删除StatefulSet失败", map[string]string{"step": "worker-appm", "status": "error"})
				return nil, err
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
		//如果滚动升级时，需要删除以下代码
		err = m.dbmanager.K8sPodDao().DeleteK8sPod(serviceID)
		if err != nil {
			if err != gorm.ErrRecordNotFound {
				logrus.Error("delete pods by service id error.", err.Error())
			}
		}
	}
	if err := m.StopService(serviceID, logger); err != nil {
		return nil, err
	}
	logger.Info("开始启动有状态应用实例", map[string]string{"step": "worker-appm", "status": "starting"})
	return m.StartStatefulSet(serviceID, logger)
}
