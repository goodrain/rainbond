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

package dao

import (
	"fmt"
	"strings"
	"time"

	"github.com/goodrain/rainbond/db/model"

	"github.com/jinzhu/gorm"
)

type K8sServiceDaoImpl struct {
	DB *gorm.DB
}

//AddModel 添加应用Service
func (t *K8sServiceDaoImpl) AddModel(mo model.Interface) error {
	service := mo.(*model.K8sService)
	var oldService model.K8sService
	if ok := t.DB.Where("service_id=? and container_port=? and is_out=?", service.ServiceID, service.ContainerPort, service.IsOut).Find(&oldService).RecordNotFound(); ok {
		if err := t.DB.Create(service).Error; err != nil {
			return err
		}
	} else {
		return fmt.Errorf("k8s service %s of service %s container %d is exist", service.K8sServiceID, service.ServiceID, service.ContainerPort)
	}
	return nil
}

//UpdateModel 更新应用Pod
func (t *K8sServiceDaoImpl) UpdateModel(mo model.Interface) error {
	service := mo.(*model.K8sService)
	if service.ID == 0 {
		return fmt.Errorf("k8s service id can not be empty when update ")
	}
	if err := t.DB.Save(service).Error; err != nil {
		return err
	}
	return nil
}

//GetK8sService 获取k8s service
func (t *K8sServiceDaoImpl) GetK8sService(serviceID string, containerPort int, isOut bool) (*model.K8sService, error) {
	var service model.K8sService
	if err := t.DB.Where("service_id=? and container_port=? and is_out=?", serviceID, containerPort, isOut).Find(&service).Error; err != nil {
		return nil, err
	}
	return &service, nil
}

func (t *K8sServiceDaoImpl) GetK8sServiceByReplicationID(replicationID string) (*model.K8sService, error) {
	var service model.K8sService
	if err := t.DB.Where("rc_id=?", replicationID).Find(&service).Error; err != nil {
		return nil, err
	}
	return &service, nil
}
func (t *K8sServiceDaoImpl) GetK8sServiceByTenantServiceID(tenantServiceID string) ([]*model.K8sService, error) {
	var services []*model.K8sService
	if err := t.DB.Where("service_id=?", tenantServiceID).Find(&services).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return services, nil
		}
		return nil, err
	}
	return services, nil
}
func (t *K8sServiceDaoImpl) DeleteK8sServiceByReplicationID(replicationID string) error {
	var service model.K8sService
	if err := t.DB.Where("rc_id=?", replicationID).Delete(&service).Error; err != nil {
		return err
	}
	return nil
}
func (t *K8sServiceDaoImpl) GetK8sServiceByReplicationIDAndPort(replicationID string, port int, isOut bool) (*model.K8sService, error) {
	var service model.K8sService
	if err := t.DB.Where("rc_id=? and port=? and is_out=?", replicationID, port, isOut).Find(&service).Error; err != nil {
		return nil, err
	}
	return &service, nil
}
func (t *K8sServiceDaoImpl) DeleteK8sServiceByReplicationIDAndPort(replicationID string, port int, isOut bool) error {
	var service model.K8sService
	if err := t.DB.Where("rc_id=? and port=? and is_out=?", replicationID, port, isOut).Delete(&service).Error; err != nil {
		return err
	}
	return nil
}
func (t *K8sServiceDaoImpl) DeleteK8sServiceByName(k8sServiceName string) error {
	var service model.K8sService
	if err := t.DB.Where("inner_service_id=?", k8sServiceName).Delete(&service).Error; err != nil {
		return err
	}
	return nil
}

func (t *K8sServiceDaoImpl) GetAllK8sService() ([]*model.K8sService, error) {
	var services []*model.K8sService
	if err := t.DB.Find(&services).Error; err != nil {
		return nil, err
	} else {
		return services, err
	}

}

func (t *K8sServiceDaoImpl) K8sServiceIsExist(tenantId string, K8sServiceID string) bool {
	var services model.K8sService
	isExist := t.DB.Where("tenant_id=? AND inner_service_id=?", tenantId, K8sServiceID).First(&services).RecordNotFound()
	return isExist
}

type K8sDeployReplicationDaoImpl struct {
	DB *gorm.DB
}

//AddModel 添加应用Service
//一个应用同时只能有一个部署信息
func (t *K8sDeployReplicationDaoImpl) AddModel(mo model.Interface) error {
	deploy := mo.(*model.K8sDeployReplication)
	var oldDeploy model.K8sDeployReplication
	if ok := t.DB.Where("service_id=? and rc_id=?", deploy.ServiceID, deploy.ReplicationID).Find(&oldDeploy).RecordNotFound(); ok {
		if err := t.DB.Create(deploy).Error; err != nil {
			return err
		}
	} else {
		if oldDeploy.IsDelete {
			deploy.ID = oldDeploy.ID
			deploy.CreatedAt = time.Now()
			if err := t.DB.Save(deploy).Error; err != nil {
				return err
			}
		} else {
			return fmt.Errorf("k8s deploy of service %s is exist", deploy.ServiceID)
		}
	}
	return nil
}

//UpdateModel 更新应用Pod
func (t *K8sDeployReplicationDaoImpl) UpdateModel(mo model.Interface) error {
	deploy := mo.(*model.K8sDeployReplication)
	if deploy.ID == 0 {
		return fmt.Errorf("k8s deploy id can not be empty when update ")
	}
	if err := t.DB.Save(deploy).Error; err != nil {
		return err
	}
	return nil
}
func (t *K8sDeployReplicationDaoImpl) GetReplications() ([]*model.K8sDeployReplication, error) {
	var deploys []*model.K8sDeployReplication
	if err := t.DB.Find(&deploys).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return deploys, nil
		}
		return nil, err
	}
	return deploys, nil
}
func (t *K8sDeployReplicationDaoImpl) GetK8sDeployReplication(replicationID string) (*model.K8sDeployReplication, error) {
	var deploy model.K8sDeployReplication
	if err := t.DB.Where("rc_id=?", replicationID).Find(&deploy).Error; err != nil {
		return nil, err
	}
	return &deploy, nil
}

//GetK8sCurrentDeployReplicationByService 获取应用当前部署信息
func (t *K8sDeployReplicationDaoImpl) GetK8sCurrentDeployReplicationByService(serviceID string) (*model.K8sDeployReplication, error) {
	var deploy model.K8sDeployReplication
	if err := t.DB.Where("service_id=? and is_delete=?", serviceID, false).Find(&deploy).Error; err != nil {
		return nil, err
	}
	return &deploy, nil
}
func (t *K8sDeployReplicationDaoImpl) DeleteK8sDeployReplication(replicationID string) error {
	var deploy model.K8sDeployReplication
	if err := t.DB.Model(&deploy).Where("rc_id=?", replicationID).Update("is_delete", true).Error; err != nil {
		return err
	}
	return nil
}
func (t *K8sDeployReplicationDaoImpl) DeleteK8sDeployReplicationByServiceAndVersion(serviceID, version string) error {
	var deploy model.K8sDeployReplication
	if err := t.DB.Where("service_id=? and deploy_version=?", serviceID, version).Delete(&deploy).Error; err != nil {
		return err
	}
	return nil
}
func (t *K8sDeployReplicationDaoImpl) DeleteK8sDeployReplicationByServiceAndMarked(serviceID string) error {
	var deploy model.K8sDeployReplication
	if err := t.DB.Where("service_id=? and is_delete=?", serviceID, true).Delete(&deploy).Error; err != nil {
		return err
	}
	return nil
}
func (t *K8sDeployReplicationDaoImpl) BeachDelete(deletelist []uint) error {
	var deploy model.K8sDeployReplication
	if err := t.DB.Where("\"ID\" in (?)", deletelist).Delete(&deploy).Error; err != nil {
		return err
	}
	return nil
}

func (t *K8sDeployReplicationDaoImpl) GetK8sDeployReplicationByService(serviceID string) ([]*model.K8sDeployReplication, error) {
	var deploy []*model.K8sDeployReplication
	if err := t.DB.Where("service_id=? and is_delete=?", serviceID, false).Find(&deploy).Error; err != nil {
		return nil, err
	}
	return deploy, nil
}

//DeleteK8sDeployReplicationByService delete deploy info by service
func (t *K8sDeployReplicationDaoImpl) DeleteK8sDeployReplicationByService(serviceID string) error {
	var deploy model.K8sDeployReplication
	if err := t.DB.Model(&deploy).Where("service_id=?", serviceID).Update("is_delete", true).Error; err != nil {
		return err
	}
	return nil
}

func (t *K8sDeployReplicationDaoImpl) GetK8sDeployReplicationByIsDelete(rcType string, isDelete bool) ([]*model.K8sDeployReplication, error) {
	var deploy []*model.K8sDeployReplication
	if err := t.DB.Model(&deploy).Where("rc_type=? AND is_delete=?", rcType, isDelete).Find(&deploy).Error; err != nil {
		return nil, err
	}
	return deploy, nil
}

func (t *K8sDeployReplicationDaoImpl) GetK8sDeployReplicationIsExist(tenantId string, RcType string, RcId string, isDelete bool) (IsExist bool) {
	var deploy model.K8sDeployReplication
	isExist := t.DB.Model(&deploy).Where("tenant_id=? AND rc_type=? AND rc_id=? AND is_delete=?", tenantId, RcType, RcId, isDelete).First(&deploy).RecordNotFound()
	return isExist
}

//K8sPodDaoImpl k8s pod dao
type K8sPodDaoImpl struct {
	DB *gorm.DB
}

func (t *K8sPodDaoImpl) GetK8sPodByNotInPodNameList(podNameList []string) ([]*model.K8sPod, error) {
	var Pods []*model.K8sPod
	if err := t.DB.Not("pod_name", podNameList).Find(&Pods).Error; err != nil {
		return nil, err
	}
	return Pods, nil
}

//AddModel save or update app pod info
func (t *K8sPodDaoImpl) AddModel(mo model.Interface) error {
	pod := mo.(*model.K8sPod)
	var oldPod model.K8sPod
	if ok := t.DB.Where("pod_name=?", pod.PodName).Find(&oldPod).RecordNotFound(); ok {
		if err := t.DB.Create(pod).Error; err != nil {
			return err
		}
	} else {
		pod.ID = oldPod.ID
		if err := t.DB.Save(pod).Error; err != nil {
			return err
		}
	}
	return nil
}

//UpdateModel update pod info
func (t *K8sPodDaoImpl) UpdateModel(mo model.Interface) error {
	pod := mo.(*model.K8sPod)
	if pod.ID == 0 {
		return fmt.Errorf("pod id can not be empty when update ")
	}
	if err := t.DB.Save(pod).Error; err != nil {
		return err
	}
	return nil
}

//DeleteK8sPod delete pod by service
func (t *K8sPodDaoImpl) DeleteK8sPod(serviceID string) error {
	var pod model.K8sPod
	if err := t.DB.Where("service_id=?", serviceID).Delete(&pod).Error; err != nil {
		return err
	}
	return nil
}

//DeleteK8sPodByName delete pod by name
func (t *K8sPodDaoImpl) DeleteK8sPodByName(podName string) error {
	var pod model.K8sPod
	if err := t.DB.Where("pod_name=?", podName).Delete(&pod).Error; err != nil {
		return err
	}
	return nil
}

//GetPodByService get pod from serviceids
// if serviceID support multiple split from ","
func (t *K8sPodDaoImpl) GetPodByService(serviceID string) ([]*model.K8sPod, error) {
	var pods []*model.K8sPod
	if strings.Contains(serviceID, ",") {
		serviceIDs := strings.Split(serviceID, ",")
		if err := t.DB.Where("service_id in (?)", serviceIDs).Find(&pods).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return pods, nil
			}
			return nil, err
		}
		return pods, nil
	}
	if err := t.DB.Where("service_id=?", serviceID).Find(&pods).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return pods, nil
		}
		return nil, err
	}
	return pods, nil
}

//GetPodByReplicationID get pod by replication
func (t *K8sPodDaoImpl) GetPodByReplicationID(replicationID string) ([]*model.K8sPod, error) {
	var pods []*model.K8sPod
	if err := t.DB.Where("rc_id=?", replicationID).Find(&pods).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return pods, nil
		}
		return nil, err
	}
	return pods, nil
}

type ServiceProbeDaoImpl struct {
	DB *gorm.DB
}

//AddModel 添加应用Probe
func (t *ServiceProbeDaoImpl) AddModel(mo model.Interface) error {
	probe := mo.(*model.ServiceProbe)
	var oldProbe model.ServiceProbe
	if ok := t.DB.Where("service_id=? and mode=?", probe.ServiceID, probe.Mode).Find(&oldProbe).RecordNotFound(); ok {
		if err := t.DB.Create(probe).Error; err != nil {
			return err
		}
	} else {
		return fmt.Errorf("probe mode %s of service %s is exist", probe.Mode, probe.ServiceID)
	}
	return nil
}

//UpdateModel 更新应用Probe
func (t *ServiceProbeDaoImpl) UpdateModel(mo model.Interface) error {
	probe := mo.(*model.ServiceProbe)
	if probe.ID == 0 {
		var oldProbe model.ServiceProbe
		if err := t.DB.Where("service_id = ? and mode = ? and probe_id=?", probe.ServiceID, probe.Mode, probe.ProbeID).Find(&oldProbe).Error; err != nil {
			return err
		}
		if oldProbe.ID == 0 {
			return gorm.ErrRecordNotFound
		}
		probe.ID = oldProbe.ID
	}
	return t.DB.Save(probe).Error
}

//DeleteModel 删除应用探针
func (t *ServiceProbeDaoImpl) DeleteModel(serviceID string, args ...interface{}) error {
	probeID := args[0].(string)
	relation := &model.ServiceProbe{
		ServiceID: serviceID,
		ProbeID:   probeID,
	}
	if err := t.DB.Where("service_id=? and probe_id=?", serviceID, probeID).Delete(relation).Error; err != nil {
		return err
	}
	return nil
}

//GetServiceProbes 获取应用探针
func (t *ServiceProbeDaoImpl) GetServiceProbes(serviceID string) ([]*model.ServiceProbe, error) {
	var probes []*model.ServiceProbe
	if err := t.DB.Where("service_id=?", serviceID).Find(&probes).Error; err != nil {
		return nil, err
	}
	return probes, nil
}

//GetServiceUsedProbe 获取指定模式的可用探针定义
func (t *ServiceProbeDaoImpl) GetServiceUsedProbe(serviceID, mode string) (*model.ServiceProbe, error) {
	var probe model.ServiceProbe
	if err := t.DB.Where("service_id=? and mode=? and is_used=?", serviceID, mode, 1).Find(&probe).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &probe, nil
}

//DELServiceProbesByServiceID DELServiceProbesByServiceID
func (t *ServiceProbeDaoImpl) DELServiceProbesByServiceID(serviceID string) error {
	probes := &model.ServiceProbe{
		ServiceID: serviceID,
	}
	if err := t.DB.Where("service_id=?", serviceID).Delete(probes).Error; err != nil {
		return err
	}
	return nil
}

//LocalSchedulerDaoImpl 本地调度存储mysql实现
type LocalSchedulerDaoImpl struct {
	DB *gorm.DB
}

//AddModel 添加本地调度信息
func (t *LocalSchedulerDaoImpl) AddModel(mo model.Interface) error {
	ls := mo.(*model.LocalScheduler)
	var oldLs model.ServiceProbe
	if ok := t.DB.Where("service_id=? and pod_name=?", ls.ServiceID, ls.PodName).Find(&oldLs).RecordNotFound(); ok {
		if err := t.DB.Create(ls).Error; err != nil {
			return err
		}
	} else {
		return fmt.Errorf("service %s local scheduler of pod  %s is exist", ls.ServiceID, ls.PodName)
	}
	return nil
}

//UpdateModel 更新调度信息
func (t *LocalSchedulerDaoImpl) UpdateModel(mo model.Interface) error {
	ls := mo.(*model.LocalScheduler)
	if ls.ID == 0 {
		return fmt.Errorf("LocalScheduler id can not be empty when update ")
	}
	if err := t.DB.Save(ls).Error; err != nil {
		return err
	}
	return nil
}

//GetLocalScheduler 获取应用本地调度信息
func (t *LocalSchedulerDaoImpl) GetLocalScheduler(serviceID string) ([]*model.LocalScheduler, error) {
	var ls []*model.LocalScheduler
	if err := t.DB.Where("service_id=?", serviceID).Find(&ls).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return ls, nil
}
