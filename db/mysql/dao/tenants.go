
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
	"os"
	"strconv"
	"time"

	"github.com/goodrain/rainbond/db/model"

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
		return fmt.Errorf("tenant uuid  %s or name %s is exist", tenant.UUID, tenant.Name)
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

//GetTenantByUUIDIsExist 获取租户
func (t *TenantDaoImpl) GetTenantByUUIDIsExist(uuid string) bool {
	var tenant model.Tenants
	isExist := t.DB.Where("uuid = ?", uuid).First(&tenant).RecordNotFound()
	return isExist

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

//GetTenantsByEid
func (t *TenantDaoImpl) GetTenantByEid(eid string) ([]*model.Tenants, error) {
	var tenants []*model.Tenants
	if err := t.DB.Where("eid = ?", eid).Find(&tenants).Error; err != nil {
		return nil, err
	}
	return tenants, nil
}

//GetTenantIDsByNames get tenant ids by names
func (t *TenantDaoImpl) GetTenantIDsByNames(names []string) (re []string, err error) {
	rows, err := t.DB.Raw("select uuid from tenants where name in (?)", names).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var uuid string
		rows.Scan(&uuid)
		re = append(re, uuid)
	}
	return
}

//GetALLTenants GetALLTenants
func (t *TenantDaoImpl) GetPagedTenants(offset, len int) ([]*model.Tenants, error) {

	var tenants []*model.Tenants
	if err := t.DB.Find(&tenants).Group("").Error; err != nil {
		return nil, err
	}
	return tenants, nil
}

//TenantServicesDaoImpl 租户应用dao
type TenantServicesDaoImpl struct {
	DB *gorm.DB
}

//GetAllServices 获取全部应用信息的资源相关信息
func (t *TenantServicesDaoImpl) GetAllServices() ([]*model.TenantServices, error) {
	var services []*model.TenantServices
	if err := t.DB.Select("tenant_id,service_id,service_alias,host_path,replicas,container_memory").Find(&services).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return services, nil
		}
		return nil, err
	}
	return services, nil
}

func (t *TenantServicesDaoImpl) GetAllServicesID() ([]*model.TenantServices, error) {
	var services []*model.TenantServices
	if err := t.DB.Select("service_id,service_alias,tenant_id").Find(&services).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return services, nil
		}
		return nil, err
	}
	return services, nil
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

//GetServiceByID 获取服务通过服务别名
func (t *TenantServicesDaoImpl) GetServiceByServiceAlias(serviceAlias string) (*model.TenantServices, error) {
	var service model.TenantServices
	if err := t.DB.Where("service_alias=?", serviceAlias).Find(&service).Error; err != nil {
		return nil, err
	}
	return &service, nil
}

//GetServiceMemoryByTenantIDs get service memory by tenant ids
func (t *TenantServicesDaoImpl) GetServiceMemoryByTenantIDs(tenantIDs []string, runningServiceIDs []string) (map[string]map[string]interface{}, error) {
	rows, err := t.DB.Raw("select tenant_id, sum(container_cpu) as cpu,sum(container_memory * replicas) as memory from tenant_services where tenant_id in (?) and service_id in (?) group by tenant_id", tenantIDs, runningServiceIDs).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var rc = make(map[string]map[string]interface{})
	for rows.Next() {
		var cpu, mem int
		var tenantID string
		rows.Scan(&tenantID, &cpu, &mem)
		res := make(map[string]interface{})
		res["cpu"] = cpu
		res["memory"] = mem
		rc[tenantID] = res
	}
	for _, sid := range tenantIDs {
		if _, ok := rc[sid]; !ok {
			rc[sid] = make(map[string]interface{})
			rc[sid]["cpu"] = 0
			rc[sid]["memory"] = 0
		}
	}
	return rc, nil
}

//GetServiceMemoryByServiceIDs get service memory by service ids
func (t *TenantServicesDaoImpl) GetServiceMemoryByServiceIDs(serviceIDs []string) (map[string]map[string]interface{}, error) {
	rows, err := t.DB.Raw("select service_id, container_cpu as cpu,container_memory * replicas as memory from tenant_services where service_id in (?)", serviceIDs).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var rc = make(map[string]map[string]interface{})
	for rows.Next() {
		var cpu, mem int
		var serviceID string
		rows.Scan(&serviceID, &cpu, &mem)
		res := make(map[string]interface{})
		res["cpu"] = cpu
		res["memory"] = mem
		rc[serviceID] = res
	}
	for _, sid := range serviceIDs {
		if _, ok := rc[sid]; !ok {
			rc[sid] = make(map[string]interface{})
			rc[sid]["cpu"] = 0
			rc[sid]["memory"] = 0
		}
	}
	return rc, nil
}

//GetPagedTenantService GetPagedTenantResource
func (t *TenantServicesDaoImpl) GetPagedTenantService(offset, length int, serviceIDs []string) ([]map[string]interface{}, int, error) {
	var count int
	var service model.TenantServices
	var result []map[string]interface{}
	if len(serviceIDs) == 0 {
		return result, count, nil
	}
	var re []*model.TenantServices
	if err := t.DB.Table(service.TableName()).Select("tenant_id").Where("service_id in (?)", serviceIDs).Group("tenant_id").Find(&re).Error; err != nil {
		return nil, count, err
	}
	count = len(re)
	rows, err := t.DB.Raw("SELECT tenant_id, SUM(container_cpu * replicas) AS use_cpu, SUM(container_memory * replicas) AS use_memory FROM tenant_services where service_id in (?) GROUP BY tenant_id ORDER BY use_memory DESC LIMIT ?,?", serviceIDs, offset, length).Rows()
	if err != nil {
		return nil, count, err
	}
	defer rows.Close()
	var rc = make(map[string]*map[string]interface{}, length)
	var tenantIDs []string
	for rows.Next() {
		var tenantID string
		var useCPU int
		var useMem int
		rows.Scan(&tenantID, &useCPU, &useMem)
		res := make(map[string]interface{})
		res["usecpu"] = useCPU
		res["usemem"] = useMem
		res["tenant"] = tenantID
		rc[tenantID] = &res
		result = append(result, res)
		tenantIDs = append(tenantIDs, tenantID)
	}
	newrows, err := t.DB.Raw("SELECT tenant_id, SUM(container_cpu * replicas) AS cap_cpu, SUM(container_memory * replicas) AS cap_memory FROM tenant_services where tenant_id in (?) GROUP BY tenant_id", tenantIDs).Rows()
	if err != nil {
		return nil, count, err
	}
	defer newrows.Close()
	for newrows.Next() {
		var tenantID string
		var capCPU int
		var capMem int
		newrows.Scan(&tenantID, &capCPU, &capMem)
		if _, ok := rc[tenantID]; ok {
			s := (*rc[tenantID])
			s["capcpu"] = capCPU
			s["capmem"] = capMem
			*rc[tenantID] = s
		}
	}
	tenants, err := t.DB.Raw("SELECT uuid,name,eid from tenants where uuid in (?)", tenantIDs).Rows()
	defer tenants.Close()
	for tenants.Next() {
		var tenantID string
		var name string
		var eid string
		tenants.Scan(&tenantID, &name, &eid)
		if _, ok := rc[tenantID]; ok {
			s := (*rc[tenantID])
			s["eid"] = eid
			s["tenant_name"] = name
			*rc[tenantID] = s
		}
	}
	return result, count, nil
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

//GetServiceByIDs get some service by service ids
func (t *TenantServicesDaoImpl) GetServiceByIDs(uids []string) ([]*model.TenantServices, error) {
	var services []*model.TenantServices
	if err := t.DB.Where("service_id in (?)", uids).Find(&services).Error; err != nil {
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
	if err := t.DB.Where("tenant_id=?", tenantID).Find(&services).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return services, nil
		}
		return nil, err
	}
	return services, nil
}

//GetServicesByTenantIDs GetServicesByTenantIDs
func (t *TenantServicesDaoImpl) GetServicesByTenantIDs(tenantIDs []string) ([]*model.TenantServices, error) {
	var services []*model.TenantServices
	if err := t.DB.Where("tenant_id in (?)", tenantIDs).Find(&services).Error; err != nil {
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

func (t *TenantServicesDeleteImpl) GetTenantServicesDeleteByCreateTime(createTime time.Time) ([]*model.TenantServicesDelete, error) {
	var ServiceDel []*model.TenantServicesDelete
	if err := t.DB.Where("create_time < ?", createTime).Find(&ServiceDel).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return ServiceDel, nil
		}
		return nil, err
	}
	return ServiceDel, nil
}

func (t *TenantServicesDeleteImpl) DeleteTenantServicesDelete(record *model.TenantServicesDelete) error {
	if err := t.DB.Delete(record).Error; err != nil {
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
	var port model.TenantServicesPort
	if err := t.DB.Where("service_id=?", serviceID).Delete(&port).Error; err != nil {
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

//UpdateModel 更新应用环境变量,只能更新环境变量值
func (t *TenantServiceEnvVarDaoImpl) UpdateModel(mo model.Interface) error {
	env := mo.(*model.TenantServiceEnvVar)
	return t.DB.Table(env.TableName()).Where("service_id=? and attr_name = ?", env.ServiceID, env.AttrName).Update("attr_value", env.AttrValue).Error
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
	var env model.TenantServiceEnvVar
	if err := t.DB.Where("service_id=?", serviceID).Find(&env).Error; err != nil {
		return err
	}
	if err := t.DB.Where("service_id=?", serviceID).Delete(&env).Error; err != nil {
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
	if ok := t.DB.Where("service_id = ? and dep_service_id = ? and volume_name=?", relation.ServiceID, relation.DependServiceID, relation.VolumeName).Find(&oldRelation).RecordNotFound(); ok {
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

//GetAllVolumes 获取全部存储信息
func (t *TenantServiceVolumeDaoImpl) GetAllVolumes() ([]*model.TenantServiceVolume, error) {
	var volumes []*model.TenantServiceVolume
	if err := t.DB.Find(&volumes).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return volumes, nil
		}
		return nil, err
	}
	return volumes, nil
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

//UpdateModel 更��应用挂载
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
	if ok := t.DB.Where("(service_id=? and container_port=?) or port=? ", mapPort.ServiceID, mapPort.ContainerPort, mapPort.Port).Find(&oldMapPort).RecordNotFound(); ok {
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
func (t *TenantServiceLBMappingPortDaoImpl) GetTenantServiceLBMappingPortByService(serviceID string) ([]*model.TenantServiceLBMappingPort, error) {
	var mapPort []*model.TenantServiceLBMappingPort
	if err := t.DB.Where("service_id=?", serviceID).Find(&mapPort).Error; err != nil {
		return nil, err
	}
	return mapPort, nil
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

//DELServiceLBMappingPortByServiceIDAndPort DELServiceLBMappingPortByServiceIDAndPort
func (t *TenantServiceLBMappingPortDaoImpl) DELServiceLBMappingPortByServiceIDAndPort(serviceID string, lbport int) error {
	var mapPorts model.TenantServiceLBMappingPort
	if err := t.DB.Where("service_id=? and port=?", serviceID, lbport).Delete(&mapPorts).Error; err != nil {
		return err
	}
	return nil
}

// GetLBPortByTenantAndPort  GetLBPortByTenantAndPort
func (t *TenantServiceLBMappingPortDaoImpl) GetLBPortByTenantAndPort(tenantID string, lbport int) (*model.TenantServiceLBMappingPort, error) {
	var mapPort model.TenantServiceLBMappingPort
	if err := t.DB.Raw("select * from tenant_lb_mapping_port where port=? and service_id in(select service_id from tenant_services where tenant_id=?)", lbport, tenantID).Scan(&mapPort).Error; err != nil {
		return nil, err
	}
	return &mapPort, nil
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

//DeleteLabelByServiceID 删除应用全部label
func (t *ServiceLabelDaoImpl) DeleteLabelByServiceID(serviceID string) error {
	label := &model.TenantServiceLable{
		ServiceID: serviceID,
	}
	if err := t.DB.Where("service_id=?", serviceID).Delete(label).Error; err != nil {
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
	var label model.TenantServiceLable
	if err := t.DB.Where("service_id=? and label_value=? and label_key in (?)", serviceID, model.LabelKeyNodeSelector, labelValues).Delete(&label).Error; err != nil {
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

//GetAll get all app status
func (t *ServiceStatusDaoImpl) GetAll() ([]*model.TenantServiceStatus, error) {
	var statuss []*model.TenantServiceStatus
	if err := t.DB.Find(&statuss).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return statuss, nil
		}
		return nil, err
	}
	return statuss, nil
}

//GetNeedBillingService get need billing service status
func (t *ServiceStatusDaoImpl) GetNeedBillingService() ([]*model.TenantServiceStatus, error) {
	var statuss []*model.TenantServiceStatus
	if err := t.DB.Where("status not in (?)", []string{"closed", "undeploy", "deploying"}).Find(&statuss).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return statuss, nil
		}
		return nil, err
	}
	return statuss, nil
}

//DeleteByServiceID 状态删除
func (t *ServiceStatusDaoImpl) DeleteByServiceID(serviceID string) error {
	var status model.TenantServiceStatus
	if err := t.DB.Where("service_id=?", serviceID).Delete(&status).Error; err != nil {
		return err
	}
	return nil
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
