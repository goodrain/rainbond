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

package dao

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/goodrain/rainbond/pkg/db/model"

	"github.com/Sirupsen/logrus"

	"github.com/jinzhu/gorm"
)

//TenantDaoImpl 租户信息管理
type TenantDaoImpl struct {
	DB *gorm.DB
}

//AddModel 添加租户
func (t *TenantDaoImpl) AddModel(mo model.Interface) error {
	tenant := mo.(*model.Tenants)
	var oldTenant model.Tenants
	if ok := t.DB.Where("uuid = ? or name=?", tenant.UUID, tenant.Name).Find(&oldTenant).RecordNotFound(); ok {
		if err := t.DB.Create(tenant).Error; err != nil {
			return err
		}
	} else {
		return fmt.Errorf("tenant uuid  %s is exist", tenant.UUID)
	}
	return nil
}

//UpdateModel 更新租户
func (t *TenantDaoImpl) UpdateModel(mo model.Interface) error {
	tenant := mo.(*model.Tenants)
	if err := t.DB.Save(tenant).Error; err != nil {
		return err
	}
	return nil
}

//GetTenantByUUID 获取租户
func (t *TenantDaoImpl) GetTenantByUUID(uuid string) (*model.Tenants, error) {
	var tenant model.Tenants
	if err := t.DB.Where("uuid = ?", uuid).Find(&tenant).Error; err != nil {
		return nil, err
	}
	return &tenant, nil
}

//GetTenantIDByName 获取租户
func (t *TenantDaoImpl) GetTenantIDByName(name string) (*model.Tenants, error) {
	var tenant model.Tenants
	if err := t.DB.Where("name = ?", name).Find(&tenant).Error; err != nil {
		return nil, err
	}
	return &tenant, nil
}

//GetALLTenants GetALLTenants
func (t *TenantDaoImpl) GetALLTenants() ([]*model.Tenants, error) {
	var tenants []*model.Tenants
	if err := t.DB.Find(&tenants).Error; err != nil {
		return nil, err
	}
	return tenants, nil
}

//TenantServicesDaoImpl 租户应用dao
type TenantServicesDaoImpl struct {
	DB *gorm.DB
}

//AddModel 添加租户应用
func (t *TenantServicesDaoImpl) AddModel(mo model.Interface) error {
	service := mo.(*model.TenantServices)
	var oldService model.TenantServices
	if ok := t.DB.Where("service_alias = ? and tenant_id=?", service.ServiceAlias, service.TenantID).Find(&oldService).RecordNotFound(); ok {
		if err := t.DB.Create(service).Error; err != nil {
			return err
		}
	} else {
		return fmt.Errorf("service name  %s and  is exist in tenant %s", service.ServiceAlias, service.TenantID)
	}
	return nil
}

//UpdateModel 更新租户应用
func (t *TenantServicesDaoImpl) UpdateModel(mo model.Interface) error {
	service := mo.(*model.TenantServices)
	if err := t.DB.Save(service).Error; err != nil {
		return err
	}
	return nil
}

//GetServiceByID 获取服务通过服务id
func (t *TenantServicesDaoImpl) GetServiceByID(serviceID string) (*model.TenantServices, error) {
	var service model.TenantServices
	if err := t.DB.Where("service_id=?", serviceID).Find(&service).Error; err != nil {
		return nil, err
	}
	return &service, nil
}

//GetCPUAndMEM GetCPUAndMEM
func (t *TenantServicesDaoImpl) GetCPUAndMEM(tenantName []string) ([]*map[string]interface{}, error) {
	if len(tenantName) == 0 {
		rows, err := t.DB.Raw("select sum(container_cpu) as cpu,sum(container_memory * replicas) as memory from tenant_services where service_id in (select service_id from tenant_service_status where status != 'closed' && status != 'undeploy')").Rows()
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var cpu int
		var mem int
		for rows.Next() {
			rows.Scan(&cpu, &mem)
		}
		var rc []*map[string]interface{}
		res := make(map[string]interface{})
		res["cpu"] = cpu
		res["memory"] = mem
		rc = append(rc, &res)
		return rc, nil
	}
	var rc []*map[string]interface{}
	for _, tenant := range tenantName {
		rows, err := t.DB.Raw("select tenant_id, sum(container_cpu) as cpu, sum(container_memory * replicas) as memory from tenant_services where service_id in (select service_id from tenant_service_status where (status != 'closed' && status != 'undeploy') && service_id in (select service_id from tenant_services where domain = (?))) group by tenant_id", tenant).Rows()
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var cpu int
			var mem int
			var id string
			rows.Scan(&id, &cpu, &mem)
			res := make(map[string]interface{})
			res["cpu"] = cpu
			res["memory"] = mem
			res["tenant_id"] = id
			logrus.Infof("res is $v", res)
			rc = append(rc, &res)
		}
	}
	return rc, nil
}

//GetServiceAliasByIDs 获取应用别名
func (t *TenantServicesDaoImpl) GetServiceAliasByIDs(uids []string) ([]*model.TenantServices, error) {
	var services []*model.TenantServices
	if err := t.DB.Where("service_id in (?)", uids).Select("service_alias,service_id").Find(&services).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return services, nil
		}
		return nil, err
	}
	return services, nil
}

//GetServiceByTenantIDAndServiceAlias 根据租户名和服务名
func (t *TenantServicesDaoImpl) GetServiceByTenantIDAndServiceAlias(tenantID, serviceName string) (*model.TenantServices, error) {
	var service model.TenantServices
	if err := t.DB.Where("service_alias = ? and tenant_id=?", serviceName, tenantID).Find(&service).Error; err != nil {
		return nil, err
	}
	return &service, nil
}

//GetServicesByTenantID GetServicesByTenantID
func (t *TenantServicesDaoImpl) GetServicesByTenantID(tenantID string) ([]*model.TenantServices, error) {
	var services []*model.TenantServices
	if err := t.DB.Where("tenant_id= ?", tenantID).Select("tenant_id,service_alias,service_id,replica_id").Find(&services).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return services, nil
		}
		return nil, err
	}
	return services, nil
}

//GetServicesAllInfoByTenantID GetServicesAllInfoByTenantID
func (t *TenantServicesDaoImpl) GetServicesAllInfoByTenantID(tenantID string) ([]*model.TenantServices, error) {
	var services []*model.TenantServices
	if err := t.DB.Where("tenant_id= ?", tenantID).Find(&services).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return services, nil
		}
		return nil, err
	}
	return services, nil
}

//SetTenantServiceStatus SetTenantServiceStatus
func (t *TenantServicesDaoImpl) SetTenantServiceStatus(serviceID, status string) error {
	var service model.TenantServices
	if status == "closed" || status == "undeploy" {
		if err := t.DB.Model(&service).Where("service_id = ?", serviceID).Update(map[string]interface{}{"cur_status": status, "status": 0}).Error; err != nil {
			return err
		}
	} else {
		if err := t.DB.Model(&service).Where("service_id = ?", serviceID).Update(map[string]interface{}{"cur_status": status, "status": 1}).Error; err != nil {
			return err
		}
	}
	return nil
}

//DeleteServiceByServiceID DeleteServiceByServiceID
func (t *TenantServicesDaoImpl) DeleteServiceByServiceID(serviceID string) error {
	ts := &model.TenantServices{
		ServiceID: serviceID,
	}
	if err := t.DB.Where("service_id = ?", serviceID).Delete(ts).Error; err != nil {
		return err
	}
	return nil
}

//TenantServicesDeleteImpl TenantServiceDeleteImpl
type TenantServicesDeleteImpl struct {
	DB *gorm.DB
}

//AddModel 添加已删除的应用
func (t *TenantServicesDeleteImpl) AddModel(mo model.Interface) error {
	service := mo.(*model.TenantServicesDelete)
	var oldService model.TenantServicesDelete
	if ok := t.DB.Where("service_alias = ? and tenant_id=?", service.ServiceAlias, service.TenantID).Find(&oldService).RecordNotFound(); ok {
		if err := t.DB.Create(service).Error; err != nil {
			return err
		}
	} else {
		return fmt.Errorf("service name  %s and  is exist in tenant %s", service.ServiceAlias, service.TenantID)
	}
	return nil
}

//UpdateModel 更新租户应用
func (t *TenantServicesDeleteImpl) UpdateModel(mo model.Interface) error {
	service := mo.(*model.TenantServicesDelete)
	if err := t.DB.Save(service).Error; err != nil {
		return err
	}
	return nil
}

//TenantServicesPortDaoImpl 租户应用端口操作
type TenantServicesPortDaoImpl struct {
	DB *gorm.DB
}

//AddModel 添加应用端口
func (t *TenantServicesPortDaoImpl) AddModel(mo model.Interface) error {
	port := mo.(*model.TenantServicesPort)
	var oldPort model.TenantServicesPort
	if ok := t.DB.Where("service_id = ? and container_port = ?", port.ServiceID, port.ContainerPort).Find(&oldPort).RecordNotFound(); ok {
		if err := t.DB.Create(port).Error; err != nil {
			return err
		}
	} else {
		return fmt.Errorf("service port %d in service %s is exist", port.ContainerPort, port.ServiceID)
	}
	return nil
}

//UpdateModel 更新租户
func (t *TenantServicesPortDaoImpl) UpdateModel(mo model.Interface) error {
	port := mo.(*model.TenantServicesPort)
	if port.ID == 0 {
		return fmt.Errorf("port id can not be empty when update ")
	}
	if err := t.DB.Save(port).Error; err != nil {
		return err
	}
	return nil
}

//DeleteModel 删除端口
func (t *TenantServicesPortDaoImpl) DeleteModel(serviceID string, args ...interface{}) error {
	if len(args) < 1 {
		return fmt.Errorf("can not provide containerPort")
	}
	containerPort := args[0].(int)
	tsp := &model.TenantServicesPort{
		ServiceID:     serviceID,
		ContainerPort: containerPort,
		//Protocol:      protocol,
	}
	if err := t.DB.Where("service_id=? and container_port=?", serviceID, containerPort).Delete(tsp).Error; err != nil {
		return err
	}
	return nil
}

//GetPortsByServiceID 通过服务获取port
func (t *TenantServicesPortDaoImpl) GetPortsByServiceID(serviceID string) ([]*model.TenantServicesPort, error) {
	var oldPort []*model.TenantServicesPort
	if err := t.DB.Where("service_id = ?", serviceID).Find(&oldPort).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return oldPort, nil
		}
		return nil, err
	}
	return oldPort, nil
}

//GetOuterPorts  获取对外端口
func (t *TenantServicesPortDaoImpl) GetOuterPorts(serviceID string) ([]*model.TenantServicesPort, error) {
	var oldPort []*model.TenantServicesPort
	if err := t.DB.Where("service_id = ? and is_outer_service=?", serviceID, true).Find(&oldPort).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return oldPort, nil
		}
		return nil, err
	}
	return oldPort, nil
}

//GetInnerPorts 获取对内端口
func (t *TenantServicesPortDaoImpl) GetInnerPorts(serviceID string) ([]*model.TenantServicesPort, error) {
	var oldPort []*model.TenantServicesPort
	if err := t.DB.Where("service_id = ? and is_inner_service=?", serviceID, true).Find(&oldPort).Error; err != nil {
		return nil, err
	}
	return oldPort, nil
}

//GetPort get port
func (t *TenantServicesPortDaoImpl) GetPort(serviceID string, port int) (*model.TenantServicesPort, error) {
	var oldPort model.TenantServicesPort
	if err := t.DB.Where("service_id = ? and container_port=?", serviceID, port).Find(&oldPort).Error; err != nil {
		return nil, err
	}
	return &oldPort, nil
}

//DELPortsByServiceID DELPortsByServiceID
func (t *TenantServicesPortDaoImpl) DELPortsByServiceID(serviceID string) error {
	ports := &model.TenantServicesPort{
		ServiceID: serviceID,
	}
	if err := t.DB.Where("service_id=?", serviceID).Where(ports).Error; err != nil {
		return err
	}
	return nil
}

//TenantServiceRelationDaoImpl TenantServiceRelationDaoImpl
type TenantServiceRelationDaoImpl struct {
	DB *gorm.DB
}

//AddModel 添加应用依赖关系
func (t *TenantServiceRelationDaoImpl) AddModel(mo model.Interface) error {
	relation := mo.(*model.TenantServiceRelation)
	var oldRelation model.TenantServiceRelation
	if ok := t.DB.Where("service_id = ? and dep_service_id = ?", relation.ServiceID, relation.DependServiceID).Find(&oldRelation).RecordNotFound(); ok {
		if err := t.DB.Create(relation).Error; err != nil {
			return err
		}
	} else {
		return fmt.Errorf("service  %s depend service %s relation is exist", relation.ServiceID, relation.DependServiceID)
	}
	return nil
}

//UpdateModel 更新应用依赖关系
func (t *TenantServiceRelationDaoImpl) UpdateModel(mo model.Interface) error {
	relation := mo.(*model.TenantServiceRelation)
	if relation.ID == 0 {
		return fmt.Errorf("relation id can not be empty when update ")
	}
	if err := t.DB.Save(relation).Error; err != nil {
		return err
	}
	return nil
}

//DeleteModel 删除依赖
func (t *TenantServiceRelationDaoImpl) DeleteModel(serviceID string, args ...interface{}) error {
	depServiceID := args[0].(string)
	relation := &model.TenantServiceRelation{
		ServiceID:       serviceID,
		DependServiceID: depServiceID,
	}
	logrus.Infof("service: %v, depend: %v", serviceID, depServiceID)
	if err := t.DB.Where("service_id=? and dep_service_id=?", serviceID, depServiceID).Delete(relation).Error; err != nil {
		return err
	}
	return nil
}

//DeleteRelationByDepID DeleteRelationByDepID
func (t *TenantServiceRelationDaoImpl) DeleteRelationByDepID(serviceID, depID string) error {
	relation := &model.TenantServiceRelation{
		ServiceID:       serviceID,
		DependServiceID: depID,
	}
	if err := t.DB.Where("service_id=? and dep_service_id=?", serviceID, depID).Delete(relation).Error; err != nil {
		return err
	}
	return nil
}

//GetTenantServiceRelations 获取应用依赖关系
func (t *TenantServiceRelationDaoImpl) GetTenantServiceRelations(serviceID string) ([]*model.TenantServiceRelation, error) {
	var oldRelation []*model.TenantServiceRelation
	if err := t.DB.Where("service_id = ?", serviceID).Find(&oldRelation).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return oldRelation, nil
		}
		return nil, err
	}
	return oldRelation, nil
}

//HaveRelations 是否有依赖
func (t *TenantServiceRelationDaoImpl) HaveRelations(serviceID string) bool {
	var oldRelation []*model.TenantServiceRelation
	if err := t.DB.Where("service_id = ?", serviceID).Find(&oldRelation).Error; err != nil {
		return false
	}
	if len(oldRelation) > 0 {
		return true
	}
	return false
}

//DELRelationsByServiceID DELRelationsByServiceID
func (t *TenantServiceRelationDaoImpl) DELRelationsByServiceID(serviceID string) error {
	relation := &model.TenantServiceRelation{
		ServiceID: serviceID,
	}
	if err := t.DB.Where("service_id=?", serviceID).Delete(relation).Error; err != nil {
		return err
	}
	return nil
}

//GetTenantServiceRelationsByDependServiceID 获取全部依赖当前服务的应用
func (t *TenantServiceRelationDaoImpl) GetTenantServiceRelationsByDependServiceID(dependServiceID string) ([]*model.TenantServiceRelation, error) {
	var oldRelation []*model.TenantServiceRelation
	if err := t.DB.Where("dep_service_id = ?", dependServiceID).Find(&oldRelation).Error; err != nil {
		return nil, err
	}
	return oldRelation, nil
}

//TenantServiceEnvVarDaoImpl TenantServiceEnvVarDaoImpl
type TenantServiceEnvVarDaoImpl struct {
	DB *gorm.DB
}

//AddModel 添加应用环境变量
func (t *TenantServiceEnvVarDaoImpl) AddModel(mo model.Interface) error {
	relation := mo.(*model.TenantServiceEnvVar)
	var oldRelation model.TenantServiceEnvVar
	if ok := t.DB.Where("service_id = ? and attr_name = ?", relation.ServiceID, relation.AttrName).Find(&oldRelation).RecordNotFound(); ok {
		if err := t.DB.Create(relation).Error; err != nil {
			return err
		}
	} else {
		return fmt.Errorf("already exist")
	}
	return nil
}

//UpdateModel 更新应用环境变量
func (t *TenantServiceEnvVarDaoImpl) UpdateModel(mo model.Interface) error {
	relation := mo.(*model.TenantServiceEnvVar)
	if relation.ID == 0 {
		return fmt.Errorf("relation id can not be empty when update")
	}
	if err := t.DB.Save(relation).Error; err != nil {
		return err
	}
	return nil
}

//DeleteModel 删除env
func (t *TenantServiceEnvVarDaoImpl) DeleteModel(serviceID string, args ...interface{}) error {
	envName := args[0].(string)
	relation := &model.TenantServiceEnvVar{
		ServiceID: serviceID,
		AttrName:  envName,
	}
	if err := t.DB.Where("service_id=? and attr_name=?", serviceID, envName).Delete(relation).Error; err != nil {
		return err
	}
	return nil
}

//GetDependServiceEnvs 获取依赖服务的环境变量
func (t *TenantServiceEnvVarDaoImpl) GetDependServiceEnvs(serviceIDs []string, scopes []string) ([]*model.TenantServiceEnvVar, error) {
	var envs []*model.TenantServiceEnvVar
	if err := t.DB.Where("service_id in (?) and scope in (?)", serviceIDs, scopes).Find(&envs).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return envs, nil
		}
		return nil, err
	}
	return envs, nil
}

//GetServiceEnvs 获取服务环境变量
func (t *TenantServiceEnvVarDaoImpl) GetServiceEnvs(serviceID string, scopes []string) ([]*model.TenantServiceEnvVar, error) {
	var envs []*model.TenantServiceEnvVar
	if scopes == nil {
		if err := t.DB.Where("service_id=?", serviceID).Find(&envs).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return envs, nil
			}
			return nil, err
		}
	} else {
		if err := t.DB.Where("service_id=? and scope in (?)", serviceID, scopes).Find(&envs).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return envs, nil
			}
			return nil, err
		}
	}
	return envs, nil
}

//GetEnv 获取某个环境变量
func (t *TenantServiceEnvVarDaoImpl) GetEnv(serviceID, envName string) (*model.TenantServiceEnvVar, error) {
	var env model.TenantServiceEnvVar
	if err := t.DB.Where("service_id=? and attr_name=? ", serviceID, envName).Find(&env).Error; err != nil {
		return nil, err
	}
	return &env, nil
}

//DELServiceEnvsByServiceID 通过serviceID 删除envs
func (t *TenantServiceEnvVarDaoImpl) DELServiceEnvsByServiceID(serviceID string) error {
	env := &model.TenantServiceEnvVar{
		ServiceID: serviceID,
	}
	if err := t.DB.Where("service_id=?", serviceID).Delete(env).Error; err != nil {
		return err
	}
	return nil
}

//TenantServiceMountRelationDaoImpl 依赖存储
type TenantServiceMountRelationDaoImpl struct {
	DB *gorm.DB
}

//AddModel 添加应用依赖挂载
func (t *TenantServiceMountRelationDaoImpl) AddModel(mo model.Interface) error {
	relation := mo.(*model.TenantServiceMountRelation)
	var oldRelation model.TenantServiceMountRelation
	if ok := t.DB.Where("service_id = ? and dep_service_id = ?", relation.ServiceID, relation.DependServiceID).Find(&oldRelation).RecordNotFound(); ok {
		if err := t.DB.Create(relation).Error; err != nil {
			return err
		}
	} else {
		return fmt.Errorf("service  %s depend service %s mount relation is exist", relation.ServiceID, relation.DependServiceID)
	}
	return nil
}

//UpdateModel 更新应用依赖挂载
func (t *TenantServiceMountRelationDaoImpl) UpdateModel(mo model.Interface) error {
	relation := mo.(*model.TenantServiceMountRelation)
	if relation.ID == 0 {
		return fmt.Errorf("mount relation id can not be empty when update ")
	}
	if err := t.DB.Save(relation).Error; err != nil {
		return err
	}
	return nil
}

//DElTenantServiceMountRelationByServiceAndName DElTenantServiceMountRelationByServiceAndName
func (t *TenantServiceMountRelationDaoImpl) DElTenantServiceMountRelationByServiceAndName(serviceID, name string) error {
	var relation model.TenantServiceMountRelation
	if err := t.DB.Where("service_id=? and volume_name=? ", serviceID, name).Find(&relation).Error; err != nil {
		return err
	}
	if err := t.DB.Where("service_id=? and volume_name=? ", serviceID, name).Delete(&relation).Error; err != nil {
		return err
	}
	return nil
}

//DElTenantServiceMountRelationByDepService del mount relation
func (t *TenantServiceMountRelationDaoImpl) DElTenantServiceMountRelationByDepService(serviceID, depServiceID string) error {
	var relation model.TenantServiceMountRelation
	if err := t.DB.Where("service_id=? and dep_service_id=?", serviceID, depServiceID).Find(&relation).Error; err != nil {
		return err
	}
	if err := t.DB.Where("service_id=? and dep_service_id=?", serviceID, depServiceID).Delete(&relation).Error; err != nil {
		return err
	}
	return nil
}

//DELTenantServiceMountRelationByServiceID DELTenantServiceMountRelationByServiceID
func (t *TenantServiceMountRelationDaoImpl) DELTenantServiceMountRelationByServiceID(serviceID string) error {
	var relation model.TenantServiceMountRelation
	if err := t.DB.Where("service_id=?", serviceID).Delete(&relation).Error; err != nil {
		return err
	}
	return nil
}

//GetTenantServiceMountRelationsByService 获取应用的所有挂载依赖
func (t *TenantServiceMountRelationDaoImpl) GetTenantServiceMountRelationsByService(serviceID string) ([]*model.TenantServiceMountRelation, error) {
	var relations []*model.TenantServiceMountRelation
	if err := t.DB.Where("service_id=? ", serviceID).Find(&relations).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return relations, nil
		}
		return nil, err
	}
	return relations, nil
}

//TenantServiceVolumeDaoImpl 应用存储
type TenantServiceVolumeDaoImpl struct {
	DB *gorm.DB
}

//AddModel 添加应用挂载
func (t *TenantServiceVolumeDaoImpl) AddModel(mo model.Interface) error {
	volume := mo.(*model.TenantServiceVolume)
	var oldvolume model.TenantServiceVolume
	if ok := t.DB.Where("(volume_name=? or volume_path = ?) and service_id=?", volume.VolumeName, volume.VolumePath, volume.ServiceID).Find(&oldvolume).RecordNotFound(); ok {
		if err := t.DB.Create(volume).Error; err != nil {
			return err
		}
	} else {
		return fmt.Errorf("service  %s volume name %s  path  %s is exist ", volume.ServiceID, volume.VolumeName, volume.VolumePath)
	}
	return nil
}

//UpdateModel 更新应用挂载
func (t *TenantServiceVolumeDaoImpl) UpdateModel(mo model.Interface) error {
	volume := mo.(*model.TenantServiceVolume)
	if volume.ID == 0 {
		return fmt.Errorf("volume id can not be empty when update ")
	}
	if err := t.DB.Save(volume).Error; err != nil {
		return err
	}
	return nil
}

//GetTenantServiceVolumesByServiceID 获取应用挂载
func (t *TenantServiceVolumeDaoImpl) GetTenantServiceVolumesByServiceID(serviceID string) ([]*model.TenantServiceVolume, error) {
	var volumes []*model.TenantServiceVolume
	if err := t.DB.Where("service_id=? ", serviceID).Find(&volumes).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return volumes, nil
		}
		return nil, err
	}
	return volumes, nil
}

//DeleteModel 删除挂载
func (t *TenantServiceVolumeDaoImpl) DeleteModel(serviceID string, args ...interface{}) error {
	var volume model.TenantServiceVolume
	volumeName := args[0].(string)
	if err := t.DB.Where("volume_name = ? and service_id=?", volumeName, serviceID).Find(&volume).Error; err != nil {
		return err
	}
	if err := t.DB.Where("volume_name = ? and service_id=?", volumeName, serviceID).Delete(&volume).Error; err != nil {
		return err
	}
	return nil
}

//DeleteByServiceIDAndVolumePath 删除挂载通过挂载的目录
func (t *TenantServiceVolumeDaoImpl) DeleteByServiceIDAndVolumePath(serviceID string, volumePath string) error {
	var volume model.TenantServiceVolume
	if err := t.DB.Where("volume_path = ? and service_id=?", volumePath, serviceID).Find(&volume).Error; err != nil {
		return err
	}
	if err := t.DB.Where("volume_path = ? and service_id=?", volumePath, serviceID).Delete(&volume).Error; err != nil {
		return err
	}
	return nil
}

//GetVolumeByServiceIDAndName 获取存储信息
func (t *TenantServiceVolumeDaoImpl) GetVolumeByServiceIDAndName(serviceID, name string) (*model.TenantServiceVolume, error) {
	var volume model.TenantServiceVolume
	if err := t.DB.Where("service_id=? and volume_name=? ", serviceID, name).Find(&volume).Error; err != nil {
		return nil, err
	}
	return &volume, nil
}

//DeleteTenantServiceVolumesByServiceID 删除挂载
func (t *TenantServiceVolumeDaoImpl) DeleteTenantServiceVolumesByServiceID(serviceID string) error {
	var volume model.TenantServiceVolume
	if err := t.DB.Where("service_id=? ", serviceID).Delete(&volume).Error; err != nil {
		return err
	}
	return nil
}

//TenantServiceLBMappingPortDaoImpl stream服务映射
type TenantServiceLBMappingPortDaoImpl struct {
	DB *gorm.DB
}

//AddModel 添加应用端口映射
func (t *TenantServiceLBMappingPortDaoImpl) AddModel(mo model.Interface) error {
	mapPort := mo.(*model.TenantServiceLBMappingPort)
	var oldMapPort model.TenantServiceLBMappingPort
	if ok := t.DB.Where("service_id=? and port=?", mapPort.ServiceID, mapPort.ContainerPort).Find(&oldMapPort).RecordNotFound(); ok {
		if err := t.DB.Create(mapPort).Error; err != nil {
			return err
		}
	} else {
		return fmt.Errorf("service %s mapport %d is exist ", mapPort.ServiceID, mapPort.ContainerPort)
	}
	return nil
}

//UpdateModel 更新应用端口映射
func (t *TenantServiceLBMappingPortDaoImpl) UpdateModel(mo model.Interface) error {
	mapPort := mo.(*model.TenantServiceLBMappingPort)
	if mapPort.ID == 0 {
		return fmt.Errorf("mapport id can not be empty when update ")
	}
	if err := t.DB.Save(mapPort).Error; err != nil {
		return err
	}
	return nil
}

//GetTenantServiceLBMappingPort 获取端口映射
func (t *TenantServiceLBMappingPortDaoImpl) GetTenantServiceLBMappingPort(serviceID string, containerPort int) (*model.TenantServiceLBMappingPort, error) {
	var mapPort model.TenantServiceLBMappingPort
	if err := t.DB.Where("service_id=? and container_port=?", serviceID, containerPort).Find(&mapPort).Error; err != nil {
		return nil, err
	}
	return &mapPort, nil
}

//CreateTenantServiceLBMappingPort 创建负载均衡VS端口,如果端口分配已存在，直接返回
func (t *TenantServiceLBMappingPortDaoImpl) CreateTenantServiceLBMappingPort(serviceID string, containerPort int) (*model.TenantServiceLBMappingPort, error) {
	var mapPorts []*model.TenantServiceLBMappingPort
	var mapPort model.TenantServiceLBMappingPort
	err := t.DB.Where("service_id=? and container_port=?", serviceID, containerPort).Find(&mapPort).Error
	if err == nil {
		return &mapPort, nil
	}
	//分配端口
	var ports []int
	err = t.DB.Order("port asc").Find(&mapPorts).Error
	if err != nil {
		return nil, fmt.Errorf("select all exist port error,%s", err.Error())
	}
	for _, p := range mapPorts {
		ports = append(ports, p.Port)
	}
	maxPort, _ := strconv.Atoi(os.Getenv("MIN_LB_PORT"))
	minPort, _ := strconv.Atoi(os.Getenv("MAX_LB_PORT"))
	if minPort == 0 {
		minPort = 20001
	}
	if maxPort == 0 {
		maxPort = 35000
	}
	var maxUsePort int
	if len(ports) > 0 {
		maxUsePort = ports[len(ports)-1]
	} else {
		maxUsePort = 20001
	}
	//顺序分配端口
	selectPort := maxUsePort + 1
	if selectPort <= maxPort {
		mp := &model.TenantServiceLBMappingPort{
			ServiceID:     serviceID,
			Port:          selectPort,
			ContainerPort: containerPort,
		}
		if err := t.DB.Save(mp).Error; err == nil {
			return mp, nil
		}
	}
	//捡漏以前端口
	selectPort = minPort
	errCount := 0
	for _, p := range ports {
		if p == selectPort {
			selectPort = selectPort + 1
			continue
		}
		if p > selectPort {
			mp := &model.TenantServiceLBMappingPort{
				ServiceID:     serviceID,
				Port:          selectPort,
				ContainerPort: containerPort,
			}
			if err := t.DB.Save(mp).Error; err != nil {
				logrus.Errorf("save select map vs port %d error %s", selectPort, err.Error())
				errCount++
				if errCount > 2 { //尝试3次
					break
				}
			} else {
				return mp, nil
			}
		}
		selectPort = selectPort + 1
	}
	if selectPort <= maxPort {
		mp := &model.TenantServiceLBMappingPort{
			ServiceID:     serviceID,
			Port:          selectPort,
			ContainerPort: containerPort,
		}
		if err := t.DB.Save(mp).Error; err != nil {
			logrus.Errorf("save select map vs port %d error %s", selectPort, err.Error())
			return nil, fmt.Errorf("can not select a good port for service stream port")
		}
		return mp, nil
	}
	logrus.Errorf("no more lb port can be use,max port is %d", maxPort)
	return nil, fmt.Errorf("no more lb port can be use,max port is %d", maxPort)
}

//GetTenantServiceLBMappingPortByService 获取端口映射
func (t *TenantServiceLBMappingPortDaoImpl) GetTenantServiceLBMappingPortByService(serviceID string) (*model.TenantServiceLBMappingPort, error) {
	var mapPort model.TenantServiceLBMappingPort
	if err := t.DB.Where("service_id=?", serviceID).Find(&mapPort).Error; err != nil {
		return nil, err
	}
	return &mapPort, nil
}

//DELServiceLBMappingPortByServiceID DELServiceLBMappingPortByServiceID
func (t *TenantServiceLBMappingPortDaoImpl) DELServiceLBMappingPortByServiceID(serviceID string) error {
	mapPorts := &model.TenantServiceLBMappingPort{
		ServiceID: serviceID,
	}
	if err := t.DB.Where("service_id=?", serviceID).Delete(mapPorts).Error; err != nil {
		return err
	}
	return nil
}

//ServiceLabelDaoImpl ServiceLabelDaoImpl
type ServiceLabelDaoImpl struct {
	DB *gorm.DB
}

//AddModel 添加应用Label
func (t *ServiceLabelDaoImpl) AddModel(mo model.Interface) error {
	label := mo.(*model.TenantServiceLable)
	var oldLabel model.TenantServiceLable
	if label.LabelKey == model.LabelKeyServiceType { //LabelKeyServiceType 只能有一条
		if ok := t.DB.Where("service_id = ? and label_key=?", label.ServiceID, label.LabelKey).Find(&oldLabel).RecordNotFound(); ok {
			if err := t.DB.Create(label).Error; err != nil {
				return err
			}
		} else {
			return fmt.Errorf("label key %s of service %s is exist", label.LabelKey, label.ServiceID)
		}
	} else {
		if ok := t.DB.Where("service_id = ? and label_key=? and label_value=?", label.ServiceID, label.LabelKey, label.LabelValue).Find(&oldLabel).RecordNotFound(); ok {
			if err := t.DB.Create(label).Error; err != nil {
				return err
			}
		} else {
			return fmt.Errorf("label key %s value %s of service %s is exist", label.LabelKey, label.LabelValue, label.ServiceID)
		}
	}
	return nil
}

//UpdateModel 更新应用Label
func (t *ServiceLabelDaoImpl) UpdateModel(mo model.Interface) error {
	label := mo.(*model.TenantServiceLable)
	if label.ID == 0 {
		return fmt.Errorf("label id can not be empty when update ")
	}
	if err := t.DB.Save(label).Error; err != nil {
		return err
	}
	return nil
}

//DeleteModel 删除应用label
func (t *ServiceLabelDaoImpl) DeleteModel(serviceID string, args ...interface{}) error {
	label := &model.TenantServiceLable{
		ServiceID:  serviceID,
		LabelKey:   args[0].(string),
		LabelValue: args[1].(string),
	}
	if err := t.DB.Where("service_id=? and label_key=? and label_value=?",
		serviceID, label.LabelKey, label.LabelValue).Delete(label).Error; err != nil {
		return err
	}
	return nil
}

//GetTenantServiceLabel GetTenantServiceLabel
func (t *ServiceLabelDaoImpl) GetTenantServiceLabel(serviceID string) ([]*model.TenantServiceLable, error) {
	var labels []*model.TenantServiceLable
	if err := t.DB.Where("service_id=?", serviceID).Find(&labels).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return labels, nil
		}
		return nil, err
	}
	return labels, nil
}

//GetTenantServiceNodeSelectorLabel GetTenantServiceNodeSelectorLabel
func (t *ServiceLabelDaoImpl) GetTenantServiceNodeSelectorLabel(serviceID string) ([]*model.TenantServiceLable, error) {
	var labels []*model.TenantServiceLable
	if err := t.DB.Where("service_id=? and label_value=?", serviceID, model.LabelKeyNodeSelector).Find(&labels).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return labels, nil
		}
		return nil, err
	}
	return labels, nil
}

//GetTenantServiceAffinityLabel GetTenantServiceAffinityLabel
func (t *ServiceLabelDaoImpl) GetTenantServiceAffinityLabel(serviceID string) ([]*model.TenantServiceLable, error) {
	var labels []*model.TenantServiceLable
	if err := t.DB.Where("service_id=? and label_key in (?)", serviceID, []string{model.LabelKeyNodeAffinity, model.LabelKeyNodeAntyAffinity,
		model.LabelKeyServiceAffinity, model.LabelKeyServiceAntyAffinity}).Find(&labels).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return labels, nil
		}
		return nil, err
	}
	return labels, nil
}

//GetTenantServiceTypeLabel GetTenantServiceTypeLabel
func (t *ServiceLabelDaoImpl) GetTenantServiceTypeLabel(serviceID string) (*model.TenantServiceLable, error) {
	var label model.TenantServiceLable
	if err := t.DB.Where("service_id=? and label_key=?", serviceID, model.LabelKeyServiceType).Find(&label).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &label, nil
}

//DELTenantServiceLabelsByLabelvaluesAndServiceID DELTenantServiceLabelsByLabelvaluesAndServiceID
func (t *ServiceLabelDaoImpl) DELTenantServiceLabelsByLabelvaluesAndServiceID(serviceID string, labelValues []string) error {
	label := &model.TenantServiceLable{
		ServiceID: serviceID,
	}
	if err := t.DB.Where("service_id=? and label_value=? and label_key in (?)", serviceID, "node_select", labelValues).Delete(label).Error; err != nil {
		return err
	}
	return nil
}

//ServiceStatusDaoImpl 更新应用状态
type ServiceStatusDaoImpl struct {
	DB *gorm.DB
}

//AddModel 添加应用状态
func (t *ServiceStatusDaoImpl) AddModel(mo model.Interface) error {
	status := mo.(*model.TenantServiceStatus)
	var oldStatus model.TenantServiceStatus
	if ok := t.DB.Where("service_id=?", oldStatus.ServiceID).Find(&oldStatus).RecordNotFound(); ok {
		if err := t.DB.Create(status).Error; err != nil {
			return err
		}
	} else {
		return t.UpdateModel(mo)
	}
	return nil
}

//UpdateModel 更新应用状态
func (t *ServiceStatusDaoImpl) UpdateModel(mo model.Interface) error {
	status := mo.(*model.TenantServiceStatus)
	if status.Status == "" {
		return fmt.Errorf("service status is undefined when update service status")
	}
	if err := t.DB.Save(status).Error; err != nil {
		return err
	}
	return nil
}

//GetTenantServiceStatus 获取应用状态
func (t *ServiceStatusDaoImpl) GetTenantServiceStatus(serviceID string) (*model.TenantServiceStatus, error) {
	var status model.TenantServiceStatus
	if err := t.DB.Where("service_id=?", serviceID).Find(&status).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			var tenantService model.TenantServices
			if err := t.DB.Where("service_id=?", serviceID).Find(&tenantService).Error; err != nil {
				return nil, err
			}
			status.Status = tenantService.CurStatus
			status.ServiceID = tenantService.ServiceID
			if err := t.DB.Create(&status).Error; err != nil {
				return nil, err
			}
		}
		return nil, err
	}
	return &status, nil
}

//SetTenantServiceStatus 设置应用状态
func (t *ServiceStatusDaoImpl) SetTenantServiceStatus(serviceID, status string) error {
	var oldStatus model.TenantServiceStatus
	if ok := t.DB.Where("service_id=?", serviceID).Find(&oldStatus).RecordNotFound(); !ok {
		oldStatus.Status = status
		if err := t.DB.Save(&oldStatus).Error; err != nil {
			return fmt.Errorf("set service status failed, %v", err)
		}
	} else {
		oldStatus.ServiceID = serviceID
		oldStatus.Status = status
		oldStatus.CreatedAt = time.Now()
		if err := t.DB.Create(&oldStatus).Error; err != nil {
			return err
		}
	}
	return nil
}

//GetRunningService GetRunningService
func (t *ServiceStatusDaoImpl) GetRunningService() ([]*model.TenantServiceStatus, error) {
	var statuss []*model.TenantServiceStatus
	if err := t.DB.Where("status in (?)", []string{"running", "starting"}).Find(&statuss).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return statuss, nil
		}
		return nil, err
	}
	return statuss, nil
}

//GetTenantStatus GetTenantStatus
func (t *ServiceStatusDaoImpl) GetTenantStatus(tenantID string) ([]*model.TenantServiceStatus, error) {
	var statuss []*model.TenantServiceStatus
	if err := t.DB.Table("tenant_service_status").Raw("select * from tenant_service_status where service_id in (select service_id from tenant_services where tenant_id=?)", tenantID).Find(&statuss).Error; err != nil {
		return nil, err
	}
	return statuss, nil
}

//GetTenantServicesStatus GetTenantServicesStatus
func (t *ServiceStatusDaoImpl) GetTenantServicesStatus(serviceIDs []string) ([]*model.TenantServiceStatus, error) {
	var statuss []*model.TenantServiceStatus
	if err := t.DB.Where("service_id in (?)", serviceIDs).Find(&statuss).Error; err != nil {
		return nil, err
	}
	return statuss, nil
}

//PluginDaoImpl PluginDaoImpl
type PluginDaoImpl struct {
	DB *gorm.DB
}

//AddModel 创建插件
func (t *PluginDaoImpl) AddModel(mo model.Interface) error {
	plugin := mo.(*model.TenantPlugin)
	var oldPlugin model.TenantPlugin
	if ok := t.DB.Where("plugin_name = ?", plugin.PluginName).Find(&oldPlugin).RecordNotFound(); ok {
		if err := t.DB.Create(plugin).Error; err != nil {
			return err
		}
	} else {
		return fmt.Errorf("plugin  %s is exist", plugin.PluginName)
	}
	return nil
}

//UpdateModel 更新插件
func (t *PluginDaoImpl) UpdateModel(mo model.Interface) error {
	plugin := mo.(*model.TenantPlugin)
	if err := t.DB.Save(plugin).Error; err != nil {
		return err
	}
	return nil
}

//GetPluginByID GetPluginByID
func (t *PluginDaoImpl) GetPluginByID(id string) (*model.TenantPlugin, error) {
	var plugin model.TenantPlugin
	if err := t.DB.Where("plugin_id = ? ", id).Find(&plugin).Error; err != nil {
		return nil, err
	}
	return &plugin, nil
}

//DeletePluginByID DeletePluginByID
func (t *PluginDaoImpl) DeletePluginByID(id string) error {
	relation := &model.TenantPlugin{
		PluginID: id,
	}
	if err := t.DB.Where("plugin_id=?", id).Delete(relation).Error; err != nil {
		return err
	}
	return nil
}

//GetPluginsByTenantID GetPluginsByTenantID
func (t *PluginDaoImpl) GetPluginsByTenantID(tenantID string) ([]*model.TenantPlugin, error) {
	var plugins []*model.TenantPlugin
	if err := t.DB.Where("tenant_id=?", tenantID).Find(&plugins).Error; err != nil {
		return nil, err
	}
	return plugins, nil
}

//PluginDefaultENVDaoImpl PluginDefaultENVDaoImpl
type PluginDefaultENVDaoImpl struct {
	DB *gorm.DB
}

//AddModel 添加插件默认变量
func (t *PluginDefaultENVDaoImpl) AddModel(mo model.Interface) error {
	env := mo.(*model.TenantPluginDefaultENV)
	var oldENV model.TenantPluginDefaultENV
	if ok := t.DB.Where(" env_name = ?", env.ENVName).Find(&oldENV).RecordNotFound(); ok {
		if err := t.DB.Create(env).Error; err != nil {
			return err
		}
	} else {
		return fmt.Errorf("env %s is exist", env.ENVName)
	}
	return nil
}

//UpdateModel 更新插件默认变量
func (t *PluginDefaultENVDaoImpl) UpdateModel(mo model.Interface) error {
	env := mo.(*model.TenantPluginDefaultENV)
	if err := t.DB.Save(env).Error; err != nil {
		return err
	}
	return nil
}

//GetDefaultENVByName GetDefaultENVByName
func (t *PluginDefaultENVDaoImpl) GetDefaultENVByName(name string) (*model.TenantPluginDefaultENV, error) {
	var env model.TenantPluginDefaultENV
	if err := t.DB.Where("env_name=?", name).Find(&env).Error; err != nil {
		return nil, err
	}
	return &env, nil
}

//GetDefaultENVSByPluginID GetDefaultENVSByPluginID
func (t *PluginDefaultENVDaoImpl) GetDefaultENVSByPluginID(pluginID string) ([]*model.TenantPluginDefaultENV, error) {
	var envs []*model.TenantPluginDefaultENV
	if err := t.DB.Where("plugin_id=?", pluginID).Find(&envs).Error; err != nil {
		return nil, err
	}
	return envs, nil
}

//GetDefaultENVSByPluginIDCantBeSet GetDefaultENVSByPluginIDCantBeSet
func (t *PluginDefaultENVDaoImpl) GetDefaultENVSByPluginIDCantBeSet(pluginID string) ([]*model.TenantPluginDefaultENV, error) {
	var envs []*model.TenantPluginDefaultENV
	if err := t.DB.Where("plugin_id=? and change=0", pluginID).Find(&envs).Error; err != nil {
		return nil, err
	}
	return envs, nil
}

//DeleteDefaultENVByName DeleteDefaultENVByName
func (t *PluginDefaultENVDaoImpl) DeleteDefaultENVByName(name string) error {
	relation := &model.TenantPluginDefaultENV{
		ENVName: name,
	}
	if err := t.DB.Where("env_name=?", name).Delete(relation).Error; err != nil {
		return err
	}
	return nil
}

//DeleteAllDefaultENVByPluginID DeleteAllDefaultENVByPluginID
func (t *PluginDefaultENVDaoImpl) DeleteAllDefaultENVByPluginID(id string) error {
	relation := &model.TenantPluginDefaultENV{
		PluginID: id,
	}
	if err := t.DB.Where("plugin_id=?", id).Delete(relation).Error; err != nil {
		return err
	}
	return nil
}

//GetDefaultEnvWhichCanBeSetByPluginID GetDefaultEnvWhichCanBeSetByPluginID
func (t *PluginDefaultENVDaoImpl) GetDefaultEnvWhichCanBeSetByPluginID(pluginID string) ([]*model.TenantPluginDefaultENV, error) {
	var envs []*model.TenantPluginDefaultENV
	if err := t.DB.Where("plugin_id=? and change=1", pluginID).Find(&envs).Error; err != nil {
		return nil, err
	}
	return envs, nil
}

//PluginBuildVersionDaoImpl PluginBuildVersionDaoImpl
type PluginBuildVersionDaoImpl struct {
	DB *gorm.DB
}

//AddModel 添加插件构建版本信息
func (t *PluginBuildVersionDaoImpl) AddModel(mo model.Interface) error {
	version := mo.(*model.TenantPluginBuildVersion)
	var oldVersion model.TenantPluginBuildVersion
	if ok := t.DB.Where("version_id = ?", version.VersionID).Find(&oldVersion).RecordNotFound(); ok {
		if err := t.DB.Create(version).Error; err != nil {
			return err
		}
	} else {
		return fmt.Errorf("plugin build version %s is exist", version.VersionID)
	}
	return nil
}

//UpdateModel 更新插件默认变量
//主体信息一般不变更，仅构建的本地镜像名与status需要变更
func (t *PluginBuildVersionDaoImpl) UpdateModel(mo model.Interface) error {
	version := mo.(*model.TenantPluginBuildVersion)
	if version.ID == 0 {
		return fmt.Errorf("id can not be empty when update build verion")
	}
	if err := t.DB.Save(version).Error; err != nil {
		return err
	}
	return nil
}

//DeleteBuildVersionByVersionID DeleteBuildVersionByVersionID
func (t *PluginBuildVersionDaoImpl) DeleteBuildVersionByVersionID(versionID string) error {
	relation := &model.TenantPluginBuildVersion{
		VersionID: versionID,
	}
	if err := t.DB.Where("version_id=?", versionID).Delete(relation).Error; err != nil {
		return err
	}
	return nil
}

//DeleteBuildVersionByPluginID DeleteBuildVersionByPluginID
func (t *PluginBuildVersionDaoImpl) DeleteBuildVersionByPluginID(pluginID string) error {
	relation := &model.TenantPluginBuildVersion{
		PluginID: pluginID,
	}
	if err := t.DB.Where("plugin_id=?", pluginID).Delete(relation).Error; err != nil {
		return err
	}
	return nil
}

//GetBuildVersionByPluginID GetBuildVersionByPluginID
func (t *PluginBuildVersionDaoImpl) GetBuildVersionByPluginID(pluginID string) ([]*model.TenantPluginBuildVersion, error) {
	var versions []*model.TenantPluginBuildVersion
	if err := t.DB.Where("plugin_id = ? and status= ?", pluginID, "complete").Find(&versions).Error; err != nil {
		return nil, err
	}
	return versions, nil
}

//GetBuildVersionByVersionID GetBuildVersionByVersionID
func (t *PluginBuildVersionDaoImpl) GetBuildVersionByVersionID(pluginID, versionID string) (*model.TenantPluginBuildVersion, error) {
	var version model.TenantPluginBuildVersion
	if err := t.DB.Where("plugin_id=? and version_id = ? ", pluginID, versionID).Find(&version).Error; err != nil {
		return nil, err
	}
	return &version, nil
}

//PluginVersionEnvDaoImpl PluginVersionEnvDaoImpl
type PluginVersionEnvDaoImpl struct {
	DB *gorm.DB
}

//AddModel 添加插件默认变量
func (t *PluginVersionEnvDaoImpl) AddModel(mo model.Interface) error {
	env := mo.(*model.TenantPluginVersionEnv)
	var oldENV model.TenantPluginVersionEnv
	if ok := t.DB.Where(" env_name = ?", env.EnvName).Find(&oldENV).RecordNotFound(); ok {
		if err := t.DB.Create(env).Error; err != nil {
			return err
		}
	} else {
		return fmt.Errorf("env %s is exist", env.EnvName)
	}
	return nil
}

//UpdateModel 更新插件默认变量
func (t *PluginVersionEnvDaoImpl) UpdateModel(mo model.Interface) error {
	env := mo.(*model.TenantPluginVersionEnv)
	if env.ID == 0 || env.ServiceID == "" || env.PluginID == "" {
		return fmt.Errorf("id can not be empty when update plugin version env")
	}
	if err := t.DB.Save(env).Error; err != nil {
		return err
	}
	return nil
}

//DeleteEnvByEnvName 删除单个env
func (t *PluginVersionEnvDaoImpl) DeleteEnvByEnvName(envName, pluginID, serviceID string) error {
	env := &model.TenantPluginVersionEnv{
		PluginID:  pluginID,
		EnvName:   envName,
		ServiceID: serviceID,
	}
	return t.DB.Where("env_name=? and plugin_id=? and service_id=?", envName, pluginID, serviceID).Delete(env).Error
}

//DeleteEnvByPluginID 删除插件依赖关系时，需要操作删除对应env
func (t *PluginVersionEnvDaoImpl) DeleteEnvByPluginID(serviceID, pluginID string) error {
	env := &model.TenantPluginVersionEnv{
		PluginID:  pluginID,
		ServiceID: serviceID,
	}
	return t.DB.Where("plugin_id=? and service_id= ?", pluginID, serviceID).Delete(env).Error
}

//DeleteEnvByServiceID 删除应用时，需要进行此操作
func (t *PluginVersionEnvDaoImpl) DeleteEnvByServiceID(serviceID string) error {
	env := &model.TenantPluginVersionEnv{
		ServiceID: serviceID,
	}
	return t.DB.Where("service_id=?", serviceID).Delete(env).Error
}

//GetVersionEnvByServiceID 获取该应用下使用的某个插件依赖的插件变量
func (t *PluginVersionEnvDaoImpl) GetVersionEnvByServiceID(serviceID string, pluginID string) ([]*model.TenantPluginVersionEnv, error) {
	var envs []*model.TenantPluginVersionEnv
	if err := t.DB.Where("service_id=? and plugin_id=?", serviceID, pluginID).Find(&envs).Error; err != nil {
		return nil, err
	}
	return envs, nil
}

//TenantServicePluginRelationDaoImpl TenantServicePluginRelationDaoImpl
type TenantServicePluginRelationDaoImpl struct {
	DB *gorm.DB
}

//AddModel 添加插件默认变量
func (t *TenantServicePluginRelationDaoImpl) AddModel(mo model.Interface) error {
	relation := mo.(*model.TenantServicePluginRelation)
	var oldRelation model.TenantServicePluginRelation
	if ok := t.DB.Where("service_id= ? and plugin_id=?", relation.ServiceID, relation.PluginID).Find(&oldRelation).RecordNotFound(); ok {
		if err := t.DB.Create(relation).Error; err != nil {
			return err
		}
	} else {
		return fmt.Errorf("relation between %s and %s is exist", relation.ServiceID, relation.PluginID)
	}
	return nil
}

//UpdateModel 更新插件默认变量 更新依赖的version id
func (t *TenantServicePluginRelationDaoImpl) UpdateModel(mo model.Interface) error {
	relation := mo.(*model.TenantServicePluginRelation)
	if relation.ID == 0 {
		return fmt.Errorf("id can not be empty when update service plugin relation")
	}
	if err := t.DB.Save(relation).Error; err != nil {
		return err
	}
	return nil
}

//DeleteRelationByServiceIDAndPluginID 删除service plugin 对应关系
func (t *TenantServicePluginRelationDaoImpl) DeleteRelationByServiceIDAndPluginID(serviceID, pluginID string) error {
	relation := &model.TenantServicePluginRelation{
		ServiceID: serviceID,
		PluginID:  pluginID,
	}
	return t.DB.Where("plugin_id=? and service_id=?",
		pluginID,
		serviceID).Delete(relation).Error
}

//DeleteALLRelationByServiceID 删除serviceID所有插件依赖 一般用于删除应用时使用
func (t *TenantServicePluginRelationDaoImpl) DeleteALLRelationByServiceID(serviceID string) error {
	relation := &model.TenantServicePluginRelation{
		ServiceID: serviceID,
	}
	return t.DB.Where("service_id=?", serviceID).Delete(relation).Error
}

//DeleteALLRelationByPluginID 删除pluginID所有依赖 一般不要使用 会影响关联过的应用启动
func (t *TenantServicePluginRelationDaoImpl) DeleteALLRelationByPluginID(pluginID string) error {
	relation := &model.TenantServicePluginRelation{
		PluginID: pluginID,
	}
	return t.DB.Where("plugin_id=?", pluginID).Delete(relation).Error
}

//GetALLRelationByServiceID 获取当前应用所有的插件依赖关系
func (t *TenantServicePluginRelationDaoImpl) GetALLRelationByServiceID(serviceID string) ([]*model.TenantServicePluginRelation, error) {
	var relations []*model.TenantServicePluginRelation
	if err := t.DB.Where("service_id=?", serviceID).Find(&relations).Error; err != nil {
		return nil, err
	}
	return relations, nil
}

//GetRelateionByServiceIDAndPluginID GetRelateionByServiceIDAndPluginID
func (t *TenantServicePluginRelationDaoImpl) GetRelateionByServiceIDAndPluginID(serviceID, pluginID string) (*model.TenantServicePluginRelation, error) {
	relation := &model.TenantServicePluginRelation{
		PluginID:  pluginID,
		ServiceID: serviceID,
	}
	if err := t.DB.Where("plugin_id=? and service_id=?", pluginID, serviceID).Find(relation).Error; err != nil {
		return nil, err
	}
	return relation, nil
}
