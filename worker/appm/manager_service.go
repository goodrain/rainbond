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

	"github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/event"

	"github.com/jinzhu/gorm"

	"github.com/Sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/pkg/api/v1"
)

func (m *manager) StartService(serviceID string, logger event.Logger, ReplicationID, ReplicationType string) error {
	logger.Info("创建K8sService资源开始", map[string]string{"step": "worker-appm", "status": "starting"})
	builder, err := K8sServiceBuilder(serviceID, ReplicationType, logger)
	if err != nil {
		logger.Error("创建K8sService构建器失败", map[string]string{"step": "worker-appm", "status": "failure"})
		logrus.Error("create k8s service builder error.", err.Error())
		return err
	}
	services, err := builder.Build()
	if err != nil {
		logger.Error("构建K8sService资源失败", map[string]string{"step": "worker-appm", "status": "failure"})
		logrus.Error("build k8s services error.", err.Error())
		return err
	}
	if services != nil && len(services) > 0 {
		for i := range services {
			service := services[i]
			m.createService(serviceID, builder.GetTenantID(), service, logger, ReplicationID, ReplicationType)
		}
	}
	return nil
}
func (m *manager) UpdateService(serviceID string, logger event.Logger, ReplicationID, ReplicationType string) error {
	logger.Info("创建K8sService资源开始", map[string]string{"step": "worker-appm", "status": "starting"})
	builder, err := K8sServiceBuilder(serviceID, ReplicationType, logger)
	if err != nil {
		logger.Error("创建K8sService构建器失败", map[string]string{"step": "worker-appm", "status": "failure"})
		logrus.Error("create k8s service builder error.", err.Error())
		return err
	}
	services, err := builder.Build()
	if err != nil {
		logger.Error("构建K8sService资源失败", map[string]string{"step": "worker-appm", "status": "failure"})
		logrus.Error("build k8s services error.", err.Error())
		return err
	}
	if services != nil && len(services) > 0 {
		for i := range services {
			service := services[i]
			_, err := m.kubeclient.CoreV1().Services(builder.GetTenantID()).Get(service.Name, metav1.GetOptions{})
			if err != nil {
				if err := checkNotFoundError(err); err == nil {
					m.createService(serviceID, builder.GetTenantID(), service, logger, ReplicationID, ReplicationType)
					continue
				} else {
					logrus.Error("get k8s  service info error ,", err.Error())
					continue
				}
			}
			// service.Spec.ClusterIP = re.Spec.ClusterIP
			// service.ResourceVersion = re.ResourceVersion
			// m.updateService(serviceID, builder.GetTenantID(), service, logger, ReplicationID, ReplicationType)
		}
	}
	return nil
}
func (m *manager) updateService(serviceID, tenantID string, service *v1.Service, logger event.Logger, ReplicationID, ReplicationType string) {
	_, err := m.kubeclient.CoreV1().Services(tenantID).Update(service)
	if err != nil {
		if err := checkNotFoundError(err); err == nil {
			m.createService(serviceID, tenantID, service, logger, ReplicationID, ReplicationType)
			return
		}
		logger.Error(fmt.Sprintf("集群更新K8sService(%s)失败", service.Name), map[string]string{"step": "worker-appm", "status": "failure"})
		logrus.Errorf("update k8s service %s error %s", service.Name, err.Error())
	}
	k8sService := &model.K8sService{
		TenantID:        tenantID,
		ServiceID:       serviceID,
		ReplicationID:   ReplicationID,
		ReplicationType: ReplicationType,
		K8sServiceID:    service.Name,
	}
	if len(service.Spec.Ports) == 1 {
		k8sService.ContainerPort = int(service.Spec.Ports[0].Port)
	}
	//有状态service不存储port,避免存储失败
	if service.Labels["service_type"] == "stateful" {
		k8sService.ContainerPort = 0
	}
	if strings.HasSuffix(service.Name, "out") {
		k8sService.IsOut = true
	}
	serviceOld, err := m.dbmanager.K8sServiceDao().GetK8sService(serviceID, k8sService.ContainerPort, k8sService.IsOut)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			err = m.dbmanager.K8sServiceDao().AddModel(k8sService)
		} else {
			logrus.Errorf("get k8s service from db error %s", err.Error())
		}
	} else {
		k8sService.ID = serviceOld.ID
		k8sService.CreatedAt = serviceOld.CreatedAt
		err = m.dbmanager.K8sServiceDao().UpdateModel(k8sService)
		if err != nil {
			logger.Error(fmt.Sprintf("更新K8sService(%s)信息到数据库失败", service.Name), map[string]string{"step": "worker-appm", "status": "failure"})
			logrus.Errorf("update k8s service %s error %s", service.Name, err.Error())
		} else {
			logger.Info(fmt.Sprintf("更新K8sService(%s)成功", service.Name), map[string]string{"step": "worker-appm", "status": "success"})
		}
	}
}
func (m *manager) createService(serviceID, tenantID string, service *v1.Service, logger event.Logger, ReplicationID, ReplicationType string) {
	_, err := m.kubeclient.CoreV1().Services(tenantID).Create(service)
	if err != nil && !strings.HasSuffix(err.Error(), "already exists") {
		logger.Error(fmt.Sprintf("集群创建K8sService(%s)失败", service.Name), map[string]string{"step": "worker-appm", "status": "failure"})
		logrus.Errorf("create k8s service %s error %s", service.Name, err.Error())
		return
	}
	k8sService := &model.K8sService{
		TenantID:        tenantID,
		ServiceID:       serviceID,
		ReplicationID:   ReplicationID,
		ReplicationType: ReplicationType,
		K8sServiceID:    service.Name,
	}
	if strings.HasSuffix(service.Name, "out") {
		k8sService.IsOut = true
	}
	if len(service.Spec.Ports) > 0 {
		k8sService.ContainerPort = int(service.Spec.Ports[0].TargetPort.IntVal)
	}
	//有状态service不存储port,避免存储失败
	if service.Labels["service_type"] == "stateful" {
		k8sService.ContainerPort = 0
	}
	err = m.dbmanager.K8sServiceDao().AddModel(k8sService)
	if err != nil {
		logger.Error(fmt.Sprintf("存储K8sService(%s)信息到数据库失败", service.Name), map[string]string{"step": "worker-appm", "status": "failure"})
		logrus.Errorf("save k8s service(%s) to db  error %s", service.Name, err.Error())
	} else {
		logger.Info(fmt.Sprintf("创建K8sService(%s)成功", service.Name), map[string]string{"step": "worker-appm", "status": "success"})
	}
}
func (m *manager) StopService(serviceID string, logger event.Logger) error {
	logger.Info("删除K8sService资源开始", map[string]string{"step": "worker-appm", "status": "starting"})
	k8sServices, err := m.dbmanager.K8sServiceDao().GetK8sServiceByTenantServiceID(serviceID)
	if err != nil {
		logger.Error("从数据库获取K8sService资源失败", map[string]string{"step": "worker-appm", "status": "failure"})
		logrus.Error("get k8s services from db error.", err.Error())
		return err
	}
	if k8sServices != nil && len(k8sServices) > 0 {
		service, err := m.dbmanager.TenantServiceDao().GetServiceByID(serviceID)
		if err != nil {
			logger.Error("从数据库获取应用信息失败", map[string]string{"step": "worker-appm", "status": "failure"})
			logrus.Error("get tenant service from db error.", err.Error())
			return err
		}
		for i := range k8sServices {
			k8sService := k8sServices[i]
			err = m.kubeclient.CoreV1().Services(service.TenantID).Delete(k8sService.K8sServiceID, &metav1.DeleteOptions{})
			if err != nil {
				logger.Error(fmt.Sprintf("删除K8sService(%s)错误", k8sService.K8sServiceID), map[string]string{"step": "worker-appm", "status": "failure"})
				logrus.Error("delete k8s service from kube api error.", err.Error())
				if err = checkNotFoundError(err); err != nil {
					//TODO:未知错误，暂时不删除数据库资源
					continue
				}
			}
			logger.Info(fmt.Sprintf("删除K8sService(%s)成功", k8sService.K8sServiceID), map[string]string{"step": "worker-appm", "status": "success"})
			err = m.dbmanager.K8sServiceDao().DeleteK8sServiceByName(k8sService.K8sServiceID)
			if err != nil {
				logrus.Error("delete k8s service info from db error.", err.Error())
			}
		}
	}
	return nil
}

func (m *manager) StartServiceByPort(serviceID string, port int, isOut bool, logger event.Logger) error {
	// logger.Info("创建K8sService资源开始", map[string]string{"step": "worker-appm", "status": "starting"})

	// builder, err := K8sServiceBuilder(serviceID, logger)
	// if err != nil {
	// 	logger.Error("创建K8sService构建器失败", map[string]string{"step": "worker-appm", "status": "failure"})
	// 	logrus.Error("create k8s service builder error.", err.Error())
	// 	return err
	// }
	// service, err := builder.BuildOnPort(port, isOut)
	// if err != nil {
	// 	logger.Error("构建K8sService资源失败", map[string]string{"step": "worker-appm", "status": "failure"})
	// 	logrus.Error("build k8s services error.", err.Error())
	// 	return err
	// }
	// //TODO:
	// //查询出ReplicationID 和ReplicationType
	// m.createService(serviceID, builder.GetTenantID(), service, logger, "", "")
	return nil
}

func (m *manager) StopServiceByPort(serviceID string, port int, isOut bool, logger event.Logger) error {
	logger.Info("删除K8sService资源开始", map[string]string{"step": "worker-appm", "status": "starting"})
	k8sService, err := m.dbmanager.K8sServiceDao().GetK8sService(serviceID, port, isOut)
	if err != nil {
		logger.Error("从数据库获取K8sService资源失败", map[string]string{"step": "worker-appm", "status": "failure"})
		logrus.Error("get k8s service from db error.", err.Error())
		return err
	}
	if k8sService != nil {
		service, err := m.dbmanager.TenantServiceDao().GetServiceByID(serviceID)
		if err != nil {
			logger.Error("从数据库获取应用信息失败", map[string]string{"step": "worker-appm", "status": "failure"})
			logrus.Error("get tenant service from db error.", err.Error())
			return err
		}
		err = m.kubeclient.CoreV1().Services(service.TenantID).Delete(k8sService.K8sServiceID, &metav1.DeleteOptions{})
		if err != nil {
			logger.Error(fmt.Sprintf("删除K8sService(%s)错误", k8sService.K8sServiceID), map[string]string{"step": "worker-appm", "status": "failure"})
			logrus.Error("delete k8s service from kube api error.", err.Error())
		} else {
			logger.Info(fmt.Sprintf("删除K8sService(%s)成功", k8sService.K8sServiceID), map[string]string{"step": "worker-appm", "status": "success"})
			err = m.dbmanager.K8sServiceDao().DeleteK8sServiceByName(k8sService.K8sServiceID)
			if err != nil {
				logrus.Error("delete k8s service info from db error.", err.Error())
			}
		}
	}
	return nil
}
