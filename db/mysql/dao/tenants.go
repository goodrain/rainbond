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
	"reflect"
	"strconv"
	"time"

	gormbulkups "github.com/atcdot/gorm-bulk-upsert"
	"github.com/goodrain/rainbond/api/util/bcode"
	"github.com/goodrain/rainbond/db/dao"
	"github.com/goodrain/rainbond/db/errors"
	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
	pkgerr "github.com/pkg/errors"
	"github.com/sirupsen/logrus"
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
func (t *TenantDaoImpl) GetALLTenants(query string) ([]*model.Tenants, error) {
	var tenants []*model.Tenants
	if query != "" {
		if err := t.DB.Where("name like ?", "%"+query+"%").Find(&tenants).Error; err != nil {
			return nil, err
		}
	} else {
		if err := t.DB.Find(&tenants).Error; err != nil {
			return nil, err
		}
	}
	return tenants, nil
}

//GetTenantByEid get tenants by eid
func (t *TenantDaoImpl) GetTenantByEid(eid, query string) ([]*model.Tenants, error) {
	var tenants []*model.Tenants
	if query != "" {
		if err := t.DB.Where("eid = ? and name like '%?%'", eid, query).Find(&tenants).Error; err != nil {
			return nil, err
		}
	} else {
		if err := t.DB.Where("eid = ?", eid).Find(&tenants).Error; err != nil {
			return nil, err
		}
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

//GetTenantLimitsByNames get tenants memory limit
func (t *TenantDaoImpl) GetTenantLimitsByNames(names []string) (limit map[string]int, err error) {
	limit = make(map[string]int)
	rows, err := t.DB.Raw("select uuid,limit_memory from tenants where name in (?)", names).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var limitmemory int
		var uuid string
		rows.Scan(&uuid, &limitmemory)
		limit[uuid] = limitmemory
	}
	return
}

// GetPagedTenants -
func (t *TenantDaoImpl) GetPagedTenants(offset, len int) ([]*model.Tenants, error) {
	var tenants []*model.Tenants
	if err := t.DB.Find(&tenants).Group("").Error; err != nil {
		return nil, err
	}
	return tenants, nil
}

// DelByTenantID -
func (t *TenantDaoImpl) DelByTenantID(tenantID string) error {
	if err := t.DB.Where("uuid=?", tenantID).Delete(&model.Tenants{}).Error; err != nil {
		return err
	}

	return nil
}

//TenantServicesDaoImpl 租户应用dao
type TenantServicesDaoImpl struct {
	DB *gorm.DB
}

// GetServiceTypeByID  get service type by service id
func (t *TenantServicesDaoImpl) GetServiceTypeByID(serviceID string) (*model.TenantServices, error) {
	var service model.TenantServices
	if err := t.DB.Select("tenant_id, service_id, service_alias, extend_method").Where("service_id=?", serviceID).Find(&service).Error; err != nil {
		return nil, err
	}
	if service.ExtendMethod == "" {
		// for before V5.2 version
		logrus.Infof("get low version service[%s] type", serviceID)
		rows, err := t.DB.Raw("select label_value from tenant_services_label where service_id=? and label_key=?", serviceID, "service-type").Rows()
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			rows.Scan(&service.ExtendMethod)
		}
	}
	return &service, nil
}

//GetAllServicesID get all service sample info
func (t *TenantServicesDaoImpl) GetAllServicesID() ([]*model.TenantServices, error) {
	var services []*model.TenantServices
	if err := t.DB.Select("service_id,service_alias,tenant_id,app_id").Find(&services).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return services, nil
		}
		return nil, err
	}
	return services, nil
}

// ListServicesByTenantID -
func (t *TenantServicesDaoImpl) ListServicesByTenantID(tenantID string) ([]*model.TenantServices, error) {
	var services []*model.TenantServices
	if err := t.DB.Where("tenant_id=?", tenantID).Find(&services).Error; err != nil {
		return nil, err
	}

	return services, nil
}

//UpdateDeployVersion update service current deploy version
func (t *TenantServicesDaoImpl) UpdateDeployVersion(serviceID, deployversion string) error {
	if err := t.DB.Exec("update tenant_services set deploy_version=? where service_id=?", deployversion, serviceID).Error; err != nil {
		return err
	}
	return nil
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

//GetServiceByServiceAlias 获取服务通过服务别名
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
	rows, err := t.DB.Raw("select service_id, container_cpu as cpu, container_memory as memory from tenant_services where service_id in (?)", serviceIDs).Rows()
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
	if err != nil {
		return nil, 0, pkgerr.Wrap(err, "list tenants")
	}
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

// GetWorkloadNameByIDs -
func (t *TenantServicesDaoImpl) GetWorkloadNameByIDs(uids []string) ([]*model.ComponentWorkload, error) {
	var componentWorkload []*model.ComponentWorkload
	if err := t.DB.Model(&model.TenantServices{}).
		Select("applications.k8s_app, tenant_services.k8s_component_name, tenant_services.service_id, tenant_services.service_alias").
		Joins("join applications on applications.app_id=tenant_services.app_id").
		Where("service_id in (?)", uids).
		Scan(&componentWorkload).Error; err != nil {
		return nil, err
	}
	return componentWorkload, nil
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

// GetServicesInfoByAppID Get Services Info By ApplicationID
func (t *TenantServicesDaoImpl) GetServicesInfoByAppID(appID string, page, pageSize int) ([]*model.TenantServices, int64, error) {
	var (
		total    int64
		services []*model.TenantServices
	)
	offset := (page - 1) * pageSize
	db := t.DB.Where("app_id=?", appID).Order("create_time desc")

	if err := db.Model(&model.TenantServices{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := db.Limit(pageSize).Offset(offset).Find(&services).Error; err != nil {
		return nil, 0, err
	}
	return services, total, nil
}

// CountServiceByAppID get Service number by AppID
func (t *TenantServicesDaoImpl) CountServiceByAppID(appID string) (int64, error) {
	var total int64

	if err := t.DB.Model(&model.TenantServices{}).Where("app_id=?", appID).Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

// GetServiceIDsByAppID get ServiceIDs by AppID
func (t *TenantServicesDaoImpl) GetServiceIDsByAppID(appID string) (re []model.ServiceID) {
	if err := t.DB.Raw("SELECT service_id FROM tenant_services WHERE app_id=?", appID).
		Scan(&re).Error; err != nil {
		logrus.Errorf("select service_id failure %s", err.Error())
		return
	}
	return
}

//GetServicesByServiceIDs Get Services By ServiceIDs
func (t *TenantServicesDaoImpl) GetServicesByServiceIDs(serviceIDs []string) ([]*model.TenantServices, error) {
	var services []*model.TenantServices
	if err := t.DB.Where("service_id in (?)", serviceIDs).Find(&services).Error; err != nil {
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

// ListThirdPartyServices lists all third party services
func (t *TenantServicesDaoImpl) ListThirdPartyServices() ([]*model.TenantServices, error) {
	var res []*model.TenantServices
	if err := t.DB.Where("kind=?", model.ServiceKindThirdParty.String()).Find(&res).Error; err != nil {
		return nil, err
	}
	return res, nil
}

// BindAppByServiceIDs binding application by serviceIDs
func (t *TenantServicesDaoImpl) BindAppByServiceIDs(appID string, serviceIDs []string) error {
	var service model.TenantServices
	if err := t.DB.Model(&service).Where("service_id in (?)", serviceIDs).Update("app_id", appID).Error; err != nil {
		return err
	}
	return nil
}

// CreateOrUpdateComponentsInBatch Batch insert or update component
func (t *TenantServicesDaoImpl) CreateOrUpdateComponentsInBatch(components []*model.TenantServices) error {
	var objects []interface{}
	for _, component := range components {
		objects = append(objects, *component)
	}
	if err := gormbulkups.BulkUpsert(t.DB, objects, 2000); err != nil {
		return pkgerr.Wrap(err, "create or update component in batch")
	}
	return nil
}

// DeleteByComponentIDs deletes components based on the given componentIDs.
func (t *TenantServicesDaoImpl) DeleteByComponentIDs(tenantID, appID string, componentIDs []string) error {
	if err := t.DB.Where("tenant_id=? and app_id=? and service_id in (?)", tenantID, appID, componentIDs).Delete(&model.TenantServices{}).Error; err != nil {
		return pkgerr.Wrap(err, "delete component failed")
	}
	return nil
}

//IsK8sComponentNameDuplicate -
func (t *TenantServicesDaoImpl) IsK8sComponentNameDuplicate(appID, serviceID, k8sComponentName string) bool {
	var count int64
	if err := t.DB.Model(&model.TenantServices{}).Where("app_id=? and service_id<>? and k8s_component_name=?", appID, serviceID, k8sComponentName).Count(&count).Error; err != nil {
		logrus.Errorf("judge K8s Component Name Duplicate failed %v", err)
		return true
	}
	return count > 0
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

// GetTenantServicesDeleteByCreateTime -
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

// DeleteTenantServicesDelete -
func (t *TenantServicesDeleteImpl) DeleteTenantServicesDelete(record *model.TenantServicesDelete) error {
	if err := t.DB.Delete(record).Error; err != nil {
		return err
	}
	return nil
}

// List returns a list of TenantServicesDeletes.
func (t *TenantServicesDeleteImpl) List() ([]*model.TenantServicesDelete, error) {
	var components []*model.TenantServicesDelete
	if err := t.DB.Find(&components).Error; err != nil {
		return nil, pkgerr.Wrap(err, "list deleted components")
	}
	return components, nil
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
		return errors.ErrRecordAlreadyExist
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

// CreateOrUpdatePortsInBatch Batch insert or update ports variables
func (t *TenantServicesPortDaoImpl) CreateOrUpdatePortsInBatch(ports []*model.TenantServicesPort) error {
	var objects []interface{}
	// dedup
	existPorts := make(map[string]struct{})
	for _, port := range ports {
		if _, ok := existPorts[port.Key()]; ok {
			continue
		}
		existPorts[port.Key()] = struct{}{}

		objects = append(objects, *port)
	}
	if err := gormbulkups.BulkUpsert(t.DB, objects, 2000); err != nil {
		return pkgerr.Wrap(err, "create or update ports in batch")
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
		if err == gorm.ErrRecordNotFound {
			return pkgerr.Wrap(bcode.ErrPortNotFound, "delete component port")
		}
		return err
	}
	return nil
}

// GetByTenantAndName -
func (t *TenantServicesPortDaoImpl) GetByTenantAndName(tenantID, name string) (*model.TenantServicesPort, error) {
	var port model.TenantServicesPort
	if err := t.DB.Where("tenant_id=? and k8s_service_name=?", tenantID, name).Find(&port).Error; err != nil {
		return nil, err
	}
	return &port, nil
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
		if err == gorm.ErrRecordNotFound {
			return nil, pkgerr.Wrap(bcode.ErrPortNotFound, fmt.Sprintf("component id: %s; port: %d; get port: %v", serviceID, port, err))
		}
		return nil, err
	}
	return &oldPort, nil
}

// GetOpenedPorts returns opened ports.
func (t *TenantServicesPortDaoImpl) GetOpenedPorts(serviceID string) ([]*model.TenantServicesPort, error) {
	var ports []*model.TenantServicesPort
	if err := t.DB.Where("service_id = ? and (is_inner_service=1 or is_outer_service=1)", serviceID).
		Find(&ports).Error; err != nil {
		return nil, err
	}
	return ports, nil
}

//DELPortsByServiceID DELPortsByServiceID
func (t *TenantServicesPortDaoImpl) DELPortsByServiceID(serviceID string) error {
	var port model.TenantServicesPort
	if err := t.DB.Where("service_id=?", serviceID).Delete(&port).Error; err != nil {
		return err
	}
	return nil
}

// HasOpenPort checks if the given service(according to sid) has open port.
func (t *TenantServicesPortDaoImpl) HasOpenPort(sid string) bool {
	var port model.TenantServicesPort
	if err := t.DB.Where("service_id = ? and (is_outer_service=1 or is_inner_service=1)", sid).
		Find(&port).Error; err != nil {
		if err != gorm.ErrRecordNotFound {
			logrus.Warningf("error getting TenantServicesPort: %v", err)
		}
		return false
	}
	return true
}

//GetDepUDPPort get all depend service udp port
func (t *TenantServicesPortDaoImpl) GetDepUDPPort(serviceID string) ([]*model.TenantServicesPort, error) {
	var portInfos []*model.TenantServicesPort
	var port model.TenantServicesPort
	var relation model.TenantServiceRelation
	if err := t.DB.Raw(fmt.Sprintf("select * from %s where protocol=? and service_id in (select dep_service_id from %s where service_id=?)", port.TableName(), relation.TableName()), "udp", serviceID).Scan(&portInfos).Error; err != nil {
		return nil, err
	}
	return portInfos, nil
}

// DelByServiceID deletes TenantServicesPort matching sid(service_id).
func (t *TenantServicesPortDaoImpl) DelByServiceID(sid string) error {
	return t.DB.Where("service_id=?", sid).Delete(&model.TenantServicesPort{}).Error
}

// DeleteByComponentIDs -
func (t *TenantServicesPortDaoImpl) DeleteByComponentIDs(componentIDs []string) error {
	return t.DB.Where("service_id in (?)", componentIDs).Delete(&model.TenantServicesPort{}).Error
}

// ListInnerPortsByServiceIDs -
func (t *TenantServicesPortDaoImpl) ListInnerPortsByServiceIDs(serviceIDs []string) ([]*model.TenantServicesPort, error) {
	var ports []*model.TenantServicesPort
	if err := t.DB.Where("service_id in (?) and is_inner_service=?", serviceIDs, true).Find(&ports).Error; err != nil {
		return nil, err
	}

	return ports, nil
}

// ListByK8sServiceNames -
func (t *TenantServicesPortDaoImpl) ListByK8sServiceNames(k8sServiceNames []string) ([]*model.TenantServicesPort, error) {
	var ports []*model.TenantServicesPort
	if err := t.DB.Where("k8s_service_name in (?)", k8sServiceNames).Find(&ports).Error; err != nil {
		return nil, err
	}
	return ports, nil
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
		return errors.ErrRecordAlreadyExist
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

//DeleteByComponentIDs -
func (t *TenantServiceRelationDaoImpl) DeleteByComponentIDs(componentIDs []string) error {
	return t.DB.Where("service_id in (?)", componentIDs).Delete(&model.TenantServiceRelation{}).Error
}

// CreateOrUpdateRelationsInBatch -
func (t *TenantServiceRelationDaoImpl) CreateOrUpdateRelationsInBatch(relations []*model.TenantServiceRelation) error {
	var objects []interface{}
	for _, relation := range relations {
		objects = append(objects, *relation)
	}
	if err := gormbulkups.BulkUpsert(t.DB, objects, 2000); err != nil {
		return pkgerr.Wrap(err, "create or update relation in batch")
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

// ListByServiceIDs -
func (t *TenantServiceRelationDaoImpl) ListByServiceIDs(serviceIDs []string) ([]*model.TenantServiceRelation, error) {
	var relations []*model.TenantServiceRelation
	if err := t.DB.Where("service_id in (?)", serviceIDs).Find(&relations).Error; err != nil {
		return nil, err
	}

	return relations, nil
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
	logrus.Debugf("service id: %s; delete service relation successfully", serviceID)
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
		if len(relation.AttrValue) > 65532 {
			relation.AttrValue = relation.AttrValue[:65532]
		}
		if err := t.DB.Create(relation).Error; err != nil {
			return err
		}
	} else {
		return errors.ErrRecordAlreadyExist
	}
	return nil
}

//UpdateModel update env support attr_value\is_change\scope
func (t *TenantServiceEnvVarDaoImpl) UpdateModel(mo model.Interface) error {
	env := mo.(*model.TenantServiceEnvVar)
	return t.DB.Table(env.TableName()).Where("service_id=? and attr_name = ?", env.ServiceID, env.AttrName).Update(map[string]interface{}{
		"attr_value": env.AttrValue,
		"is_change":  env.IsChange,
		"scope":      env.Scope,
	}).Error
}

//DeleteByComponentIDs -
func (t *TenantServiceEnvVarDaoImpl) DeleteByComponentIDs(componentIDs []string) error {
	return t.DB.Where("service_id in (?)", componentIDs).Delete(&model.TenantServiceEnvVar{}).Error
}

// CreateOrUpdateEnvsInBatch Batch insert or update environment variables
func (t *TenantServiceEnvVarDaoImpl) CreateOrUpdateEnvsInBatch(envs []*model.TenantServiceEnvVar) error {
	var objects []interface{}
	existEnvs := make(map[string]struct{})
	for _, env := range envs {
		key := fmt.Sprintf("%s+%s+%s", env.TenantID, env.ServiceID, env.AttrName)
		if _, ok := existEnvs[key]; ok {
			continue
		}
		existEnvs[key] = struct{}{}

		objects = append(objects, *env)
	}
	if err := gormbulkups.BulkUpsert(t.DB, objects, 2000); err != nil {
		return pkgerr.Wrap(err, "create or update envs in batch")
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
	var env model.TenantServiceEnvVar
	if err := t.DB.Where("service_id=?", serviceID).Find(&env).Error; err != nil {
		return err
	}
	if err := t.DB.Where("service_id=?", serviceID).Delete(&env).Error; err != nil {
		return err
	}
	return nil
}

// DelByServiceIDAndScope deletes TenantServiceEnvVar based on sid(service_id) and scope.
func (t *TenantServiceEnvVarDaoImpl) DelByServiceIDAndScope(sid, scope string) error {
	var env model.TenantServiceEnvVar
	if err := t.DB.Where("service_id=? and scope=?", sid, scope).Delete(&env).Error; err != nil {
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
		return errors.ErrRecordAlreadyExist
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

// DeleteByComponentIDs -
func (t *TenantServiceMountRelationDaoImpl) DeleteByComponentIDs(componentIDs []string) error {
	return t.DB.Where("service_id in (?)", componentIDs).Delete(&model.TenantServiceMountRelation{}).Error
}

// CreateOrUpdateVolumeRelsInBatch -
func (t *TenantServiceMountRelationDaoImpl) CreateOrUpdateVolumeRelsInBatch(volRels []*model.TenantServiceMountRelation) error {
	var objects []interface{}
	for _, volRel := range volRels {
		objects = append(objects, *volRel)
	}
	if err := gormbulkups.BulkUpsert(t.DB, objects, 2000); err != nil {
		return pkgerr.Wrap(err, "create or update volume relation in batch")
	}
	return nil
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

// ListVolumesByComponentIDs -
func (t *TenantServiceVolumeDaoImpl) ListVolumesByComponentIDs(componentIDs []string) ([]*model.TenantServiceVolume, error) {
	var volumes []*model.TenantServiceVolume
	if err := t.DB.Where("service_id in (?)", componentIDs).Find(&volumes).Error; err != nil {
		return nil, err
	}
	return volumes, nil
}

//DeleteByVolumeIDs -
func (t *TenantServiceVolumeDaoImpl) DeleteByVolumeIDs(volumeIDs []uint) error {
	return t.DB.Where("ID in (?)", volumeIDs).Delete(&model.TenantServiceVolume{}).Error
}

//DeleteByComponentIDs -
func (t *TenantServiceVolumeDaoImpl) DeleteByComponentIDs(componentIDs []string) error {
	return t.DB.Where("service_id in (?)", componentIDs).Delete(&model.TenantServiceVolume{}).Error
}

// CreateOrUpdateVolumesInBatch -
func (t *TenantServiceVolumeDaoImpl) CreateOrUpdateVolumesInBatch(volumes []*model.TenantServiceVolume) error {
	var objects []interface{}
	for _, volume := range volumes {
		objects = append(objects, *volume)
	}
	if err := gormbulkups.BulkUpsert(t.DB, objects, 2000); err != nil {
		return pkgerr.Wrap(err, "create or update volumes in batch")
	}
	return nil
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

//GetVolumeByID get volume by id
func (t *TenantServiceVolumeDaoImpl) GetVolumeByID(id int) (*model.TenantServiceVolume, error) {
	var volume model.TenantServiceVolume
	if err := t.DB.Where("ID=?", id).Find(&volume).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, dao.ErrVolumeNotFound
		}
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

// DelShareableBySID deletes shareable volumes based on sid(service_id)
func (t *TenantServiceVolumeDaoImpl) DelShareableBySID(sid string) error {
	query := "service_id=? and volume_type in ('share-file', 'config-file')"
	return t.DB.Where(query, sid).Delete(&model.TenantServiceVolume{}).Error
}

//TenantServiceConfigFileDaoImpl is a implementation of TenantServiceConfigFileDao
type TenantServiceConfigFileDaoImpl struct {
	DB *gorm.DB
}

// AddModel creates a new TenantServiceConfigFile
func (t *TenantServiceConfigFileDaoImpl) AddModel(mo model.Interface) error {
	configFile, ok := mo.(*model.TenantServiceConfigFile)
	if !ok {
		return fmt.Errorf("can't convert %s to *model.TenantServiceConfigFile", reflect.TypeOf(mo))
	}
	var old model.TenantServiceConfigFile
	if ok := t.DB.Where("service_id=? and volume_name=?", configFile.ServiceID,
		configFile.VolumeName).Find(&old).RecordNotFound(); ok {
		if err := t.DB.Create(configFile).Error; err != nil {
			return err
		}
	} else {
		old.FileContent = configFile.FileContent
		if err := t.DB.Save(&old).Error; err != nil {
			return err
		}
	}
	return nil
}

// UpdateModel updates config file
func (t *TenantServiceConfigFileDaoImpl) UpdateModel(mo model.Interface) error {
	configFile, ok := mo.(*model.TenantServiceConfigFile)
	if !ok {
		return fmt.Errorf("can't convert %s to *model.TenantServiceConfigFile", reflect.TypeOf(mo))
	}
	return t.DB.Table(configFile.TableName()).
		Where("service_id=? and volume_name=?", configFile.ServiceID, configFile.VolumeName).
		Update(configFile).Error
}

// GetConfigFileByServiceID -
func (t *TenantServiceConfigFileDaoImpl) GetConfigFileByServiceID(serviceID string) ([]*model.TenantServiceConfigFile, error) {
	var configFiles []*model.TenantServiceConfigFile
	if err := t.DB.Where("service_id=?", serviceID).Find(&configFiles).Error; err != nil {
		return nil, err
	}
	return configFiles, nil
}

// GetByVolumeName get config file by volume name
func (t *TenantServiceConfigFileDaoImpl) GetByVolumeName(sid string, volumeName string) (*model.TenantServiceConfigFile, error) {
	var res model.TenantServiceConfigFile
	if err := t.DB.Where("service_id=? and volume_name = ?", sid, volumeName).
		Find(&res).Error; err != nil {
		return nil, err
	}
	return &res, nil
}

// DelByVolumeID deletes config files according to service id and volume id.
func (t *TenantServiceConfigFileDaoImpl) DelByVolumeID(sid, volumeName string) error {
	var cfs []model.TenantServiceConfigFile
	return t.DB.Where("service_id=? and volume_name = ?", sid, volumeName).Delete(&cfs).Error
}

// DelByServiceID deletes config files according to service id.
func (t *TenantServiceConfigFileDaoImpl) DelByServiceID(sid string) error {
	return t.DB.Where("service_id=?", sid).Delete(&model.TenantServiceConfigFile{}).Error
}

// DeleteByComponentIDs -
func (t *TenantServiceConfigFileDaoImpl) DeleteByComponentIDs(componentIDs []string) error {
	return t.DB.Where("service_id in (?)", componentIDs).Delete(&model.TenantServiceConfigFile{}).Error
}

// CreateOrUpdateConfigFilesInBatch -
func (t *TenantServiceConfigFileDaoImpl) CreateOrUpdateConfigFilesInBatch(configFiles []*model.TenantServiceConfigFile) error {
	var objects []interface{}
	for _, configFile := range configFiles {
		objects = append(objects, *configFile)
	}
	if err := gormbulkups.BulkUpsert(t.DB, objects, 2000); err != nil {
		return pkgerr.Wrap(err, "create or update config files in batch")
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
	if ok := t.DB.Where("port=? ", mapPort.Port).Find(&oldMapPort).RecordNotFound(); ok {
		if err := t.DB.Create(mapPort).Error; err != nil {
			return err
		}
	} else {
		return fmt.Errorf("external port(%d) is exist ", mapPort.Port)
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

// GetLBMappingPortByServiceIDAndPort returns a LBMappingPort by serviceID and port
func (t *TenantServiceLBMappingPortDaoImpl) GetLBMappingPortByServiceIDAndPort(serviceID string, port int) (*model.TenantServiceLBMappingPort, error) {
	var mapPort model.TenantServiceLBMappingPort
	if err := t.DB.Where("service_id=? and port=?", serviceID, port).Find(&mapPort).Error; err != nil {
		return nil, err
	}
	return &mapPort, nil
}

// GetLBPortsASC gets all LBMappingPorts ascending
func (t *TenantServiceLBMappingPortDaoImpl) GetLBPortsASC() ([]*model.TenantServiceLBMappingPort, error) {
	var ports []*model.TenantServiceLBMappingPort
	if err := t.DB.Order("port asc").Find(&ports).Error; err != nil {
		return nil, fmt.Errorf("select all exist port error,%s", err.Error())
	}
	return ports, nil
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

// PortExists checks if the port exists
func (t *TenantServiceLBMappingPortDaoImpl) PortExists(port int) bool {
	var mapPorts model.TenantServiceLBMappingPort
	return !t.DB.Where("port=?", port).Find(&mapPorts).RecordNotFound()
}

//ServiceLabelDaoImpl ServiceLabelDaoImpl
type ServiceLabelDaoImpl struct {
	DB *gorm.DB
}

//AddModel 添加应用Label
func (t *ServiceLabelDaoImpl) AddModel(mo model.Interface) error {
	label := mo.(*model.TenantServiceLable)
	var oldLabel model.TenantServiceLable
	if ok := t.DB.Where("service_id = ? and label_key=? and label_value=?", label.ServiceID, label.LabelKey, label.LabelValue).Find(&oldLabel).RecordNotFound(); ok {
		if err := t.DB.Create(label).Error; err != nil {
			return err
		}
	} else {
		return fmt.Errorf("label key %s value %s of service %s is exist", label.LabelKey, label.LabelValue, label.ServiceID)
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
	if err := t.DB.Where("service_id=? and label_key=?", serviceID, model.LabelKeyNodeSelector).Find(&labels).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return labels, nil
		}
		return nil, err
	}
	return labels, nil
}

// GetLabelByNodeSelectorKey returns a label by node-selector and label_value
func (t *ServiceLabelDaoImpl) GetLabelByNodeSelectorKey(serviceID string, labelValue string) (*model.TenantServiceLable, error) {
	var label model.TenantServiceLable
	if err := t.DB.Where("service_id=? and label_key = ? and label_value=?", serviceID, model.LabelKeyNodeSelector,
		labelValue).Find(&label).Error; err != nil {
		return nil, err
	}
	return &label, nil
}

//GetTenantNodeAffinityLabel returns TenantServiceLable matching serviceID and LabelKeyNodeAffinity
func (t *ServiceLabelDaoImpl) GetTenantNodeAffinityLabel(serviceID string) (*model.TenantServiceLable, error) {
	var label model.TenantServiceLable
	if err := t.DB.Where("service_id=? and label_key = ?", serviceID, model.LabelKeyNodeAffinity).
		Find(&label).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return &label, nil
		}
		return nil, err
	}
	return &label, nil
}

//GetTenantServiceAffinityLabel GetTenantServiceAffinityLabel
func (t *ServiceLabelDaoImpl) GetTenantServiceAffinityLabel(serviceID string) ([]*model.TenantServiceLable, error) {
	var labels []*model.TenantServiceLable
	if err := t.DB.Where("service_id=? and label_key in (?)", serviceID, []string{model.LabelKeyNodeSelector, model.LabelKeyNodeAffinity,
		model.LabelKeyServiceAffinity, model.LabelKeyServiceAntyAffinity}).Find(&labels).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return labels, nil
		}
		return nil, err
	}
	return labels, nil
}

//GetTenantServiceTypeLabel GetTenantServiceTypeLabel
// no usages func. get tenant service type use TenantServiceDao.GetServiceTypeByID(serviceID string)
func (t *ServiceLabelDaoImpl) GetTenantServiceTypeLabel(serviceID string) (*model.TenantServiceLable, error) {
	var label model.TenantServiceLable
	return &label, nil
}

// GetPrivilegedLabel -
func (t *ServiceLabelDaoImpl) GetPrivilegedLabel(serviceID string) (*model.TenantServiceLable, error) {
	var label model.TenantServiceLable
	if err := t.DB.Where("service_id=? and label_value=?", serviceID, model.LabelKeyServicePrivileged).Find(&label).Error; err != nil {
		return nil, err
	}
	return &label, nil
}

//DelTenantServiceLabelsByLabelValuesAndServiceID DELTenantServiceLabelsByLabelvaluesAndServiceID
func (t *ServiceLabelDaoImpl) DelTenantServiceLabelsByLabelValuesAndServiceID(serviceID string) error {
	var label model.TenantServiceLable
	if err := t.DB.Where("service_id=? and label_value=?", serviceID, model.LabelKeyNodeSelector).Delete(&label).Error; err != nil {
		return err
	}
	return nil
}

//DelTenantServiceLabelsByServiceIDKeyValue deletes labels
func (t *ServiceLabelDaoImpl) DelTenantServiceLabelsByServiceIDKeyValue(serviceID string, labelKey string,
	labelValue string) error {
	var label model.TenantServiceLable
	if err := t.DB.Where("service_id=? and label_key=? and label_value=?", serviceID, labelKey,
		labelValue).Delete(&label).Error; err != nil {
		return err
	}
	return nil
}

//DelTenantServiceLabelsByServiceIDKey deletes labels by serviceID and labelKey
func (t *ServiceLabelDaoImpl) DelTenantServiceLabelsByServiceIDKey(serviceID string, labelKey string) error {
	var label model.TenantServiceLable
	if err := t.DB.Where("service_id=? and label_key=?", serviceID, labelKey).Delete(&label).Error; err != nil {
		return err
	}
	return nil
}

// DeleteByComponentIDs deletes labels based on componentIDs
func (t *ServiceLabelDaoImpl) DeleteByComponentIDs(componentIDs []string) error {
	return t.DB.Where("service_id in (?)", componentIDs).Delete(&model.TenantServiceLable{}).Error
}

// CreateOrUpdateLabelsInBatch -
func (t *ServiceLabelDaoImpl) CreateOrUpdateLabelsInBatch(labels []*model.TenantServiceLable) error {
	var objects []interface{}
	for _, label := range labels {
		objects = append(objects, *label)
	}
	if err := gormbulkups.BulkUpsert(t.DB, objects, 2000); err != nil {
		return pkgerr.Wrap(err, "create or update label in batch")
	}
	return nil
}

// TenantServceAutoscalerRulesDaoImpl -
type TenantServceAutoscalerRulesDaoImpl struct {
	DB *gorm.DB
}

// AddModel -
func (t *TenantServceAutoscalerRulesDaoImpl) AddModel(mo model.Interface) error {
	rule := mo.(*model.TenantServiceAutoscalerRules)
	var old model.TenantServiceAutoscalerRules
	if ok := t.DB.Where("rule_id = ?", rule.RuleID).Find(&old).RecordNotFound(); ok {
		if err := t.DB.Create(rule).Error; err != nil {
			return err
		}
	} else {
		return errors.ErrRecordAlreadyExist
	}
	return nil
}

// UpdateModel -
func (t *TenantServceAutoscalerRulesDaoImpl) UpdateModel(mo model.Interface) error {
	rule := mo.(*model.TenantServiceAutoscalerRules)
	if err := t.DB.Save(rule).Error; err != nil {
		return err
	}
	return nil
}

// GetByRuleID -
func (t *TenantServceAutoscalerRulesDaoImpl) GetByRuleID(ruleID string) (*model.TenantServiceAutoscalerRules, error) {
	var rule model.TenantServiceAutoscalerRules
	if err := t.DB.Where("rule_id=?", ruleID).Find(&rule).Error; err != nil {
		return nil, err
	}
	return &rule, nil
}

// ListByServiceID -
func (t *TenantServceAutoscalerRulesDaoImpl) ListByServiceID(serviceID string) ([]*model.TenantServiceAutoscalerRules, error) {
	var rules []*model.TenantServiceAutoscalerRules
	if err := t.DB.Where("service_id=?", serviceID).Find(&rules).Error; err != nil {
		return nil, err
	}
	return rules, nil
}

// ListEnableOnesByServiceID -
func (t *TenantServceAutoscalerRulesDaoImpl) ListEnableOnesByServiceID(serviceID string) ([]*model.TenantServiceAutoscalerRules, error) {
	var rules []*model.TenantServiceAutoscalerRules
	if err := t.DB.Where("service_id=? and enable=?", serviceID, true).Find(&rules).Error; err != nil {
		return nil, err
	}
	return rules, nil
}

// ListByComponentIDs -
func (t *TenantServceAutoscalerRulesDaoImpl) ListByComponentIDs(componentIDs []string) ([]*model.TenantServiceAutoscalerRules, error) {
	var rules []*model.TenantServiceAutoscalerRules
	if err := t.DB.Where("service_id in (?)", componentIDs).Find(&rules).Error; err != nil {
		return nil, err
	}
	return rules, nil
}

// DeleteByComponentIDs deletes rule based on componentIDs
func (t *TenantServceAutoscalerRulesDaoImpl) DeleteByComponentIDs(componentIDs []string) error {
	return t.DB.Where("service_id in (?)", componentIDs).Delete(&model.TenantServiceAutoscalerRules{}).Error
}

// CreateOrUpdateScaleRulesInBatch -
func (t *TenantServceAutoscalerRulesDaoImpl) CreateOrUpdateScaleRulesInBatch(rules []*model.TenantServiceAutoscalerRules) error {
	var objects []interface{}
	for _, rule := range rules {
		objects = append(objects, *rule)
	}
	if err := gormbulkups.BulkUpsert(t.DB, objects, 2000); err != nil {
		return pkgerr.Wrap(err, "create or update scale rule in batch")
	}
	return nil
}

// TenantServceAutoscalerRuleMetricsDaoImpl -
type TenantServceAutoscalerRuleMetricsDaoImpl struct {
	DB *gorm.DB
}

// AddModel -
func (t *TenantServceAutoscalerRuleMetricsDaoImpl) AddModel(mo model.Interface) error {
	metric := mo.(*model.TenantServiceAutoscalerRuleMetrics)
	var old model.TenantServiceAutoscalerRuleMetrics
	if ok := t.DB.Where("rule_id=? and metric_type=? and metric_name=?", metric.RuleID, metric.MetricsType, metric.MetricsName).Find(&old).RecordNotFound(); ok {
		if err := t.DB.Create(metric).Error; err != nil {
			return err
		}
	} else {
		return errors.ErrRecordAlreadyExist
	}
	return nil
}

// UpdateModel -
func (t *TenantServceAutoscalerRuleMetricsDaoImpl) UpdateModel(mo model.Interface) error {
	metric := mo.(*model.TenantServiceAutoscalerRuleMetrics)
	if err := t.DB.Save(metric).Error; err != nil {
		return err
	}
	return nil
}

// UpdateOrCreate -
func (t *TenantServceAutoscalerRuleMetricsDaoImpl) UpdateOrCreate(metric *model.TenantServiceAutoscalerRuleMetrics) error {
	var old model.TenantServiceAutoscalerRuleMetrics
	if ok := t.DB.Where("rule_id=? and metric_type=? and metric_name=?", metric.RuleID, metric.MetricsType, metric.MetricsName).Find(&old).RecordNotFound(); ok {
		if err := t.DB.Create(metric).Error; err != nil {
			return err
		}
	} else {
		old.MetricTargetType = metric.MetricTargetType
		old.MetricTargetValue = metric.MetricTargetValue
		if err := t.DB.Save(&old).Error; err != nil {
			return err
		}
	}
	return nil
}

// ListByRuleID -
func (t *TenantServceAutoscalerRuleMetricsDaoImpl) ListByRuleID(ruleID string) ([]*model.TenantServiceAutoscalerRuleMetrics, error) {
	var metrics []*model.TenantServiceAutoscalerRuleMetrics
	if err := t.DB.Where("rule_id=?", ruleID).Find(&metrics).Error; err != nil {
		return nil, err
	}
	return metrics, nil
}

// DeleteByRuleID -
func (t *TenantServceAutoscalerRuleMetricsDaoImpl) DeleteByRuleID(ruldID string) error {
	if err := t.DB.Where("rule_id=?", ruldID).Delete(&model.TenantServiceAutoscalerRuleMetrics{}).Error; err != nil {
		return err
	}

	return nil
}

// DeleteByRuleIDs deletes rule metrics based on componentIDs
func (t *TenantServceAutoscalerRuleMetricsDaoImpl) DeleteByRuleIDs(ruleIDs []string) error {
	return t.DB.Where("rule_id in (?)", ruleIDs).Delete(&model.TenantServiceAutoscalerRuleMetrics{}).Error
}

// CreateOrUpdateScaleRuleMetricsInBatch -
func (t *TenantServceAutoscalerRuleMetricsDaoImpl) CreateOrUpdateScaleRuleMetricsInBatch(metrics []*model.TenantServiceAutoscalerRuleMetrics) error {
	var objects []interface{}
	for _, metric := range metrics {
		objects = append(objects, *metric)
	}
	if err := gormbulkups.BulkUpsert(t.DB, objects, 2000); err != nil {
		return pkgerr.Wrap(err, "create or update rule metric in batch")
	}
	return nil
}

// TenantServiceScalingRecordsDaoImpl -
type TenantServiceScalingRecordsDaoImpl struct {
	DB *gorm.DB
}

// AddModel -
func (t *TenantServiceScalingRecordsDaoImpl) AddModel(mo model.Interface) error {
	record := mo.(*model.TenantServiceScalingRecords)
	var old model.TenantServiceScalingRecords
	if ok := t.DB.Where("event_name=?", record.EventName).Find(&old).RecordNotFound(); ok {
		if err := t.DB.Create(record).Error; err != nil {
			return err
		}
	} else {
		return errors.ErrRecordAlreadyExist
	}
	return nil
}

// UpdateModel -
func (t *TenantServiceScalingRecordsDaoImpl) UpdateModel(mo model.Interface) error {
	record := mo.(*model.TenantServiceScalingRecords)
	if err := t.DB.Save(record).Error; err != nil {
		return err
	}
	return nil
}

// UpdateOrCreate -
func (t *TenantServiceScalingRecordsDaoImpl) UpdateOrCreate(new *model.TenantServiceScalingRecords) error {
	var old model.TenantServiceScalingRecords

	if ok := t.DB.Where("event_name=?", new.EventName).Find(&old).RecordNotFound(); ok {
		return t.DB.Create(new).Error
	}

	old.Count = new.Count
	old.LastTime = new.LastTime
	return t.DB.Save(&old).Error
}

// ListByServiceID -
func (t *TenantServiceScalingRecordsDaoImpl) ListByServiceID(serviceID string, offset, limit int) ([]*model.TenantServiceScalingRecords, error) {
	var records []*model.TenantServiceScalingRecords
	if err := t.DB.Where("service_id=?", serviceID).Offset(offset).Limit(limit).Order("last_time desc").Find(&records).Error; err != nil {
		return nil, err
	}

	return records, nil
}

// CountByServiceID -
func (t *TenantServiceScalingRecordsDaoImpl) CountByServiceID(serviceID string) (int, error) {
	record := model.TenantServiceScalingRecords{}
	var count int
	if err := t.DB.Table(record.TableName()).Where("service_id=?", serviceID).Count(&count).Error; err != nil {
		return 0, err
	}

	return count, nil
}

// ComponentK8sAttributeDaoImpl The K8s attribute value of the component
type ComponentK8sAttributeDaoImpl struct {
	DB *gorm.DB
}

// AddModel -
func (t *ComponentK8sAttributeDaoImpl) AddModel(mo model.Interface) error {
	attr := mo.(*model.ComponentK8sAttributes)
	var old model.ComponentK8sAttributes
	if ok := t.DB.Where("component_id=? and name=?", attr.ComponentID, attr.Name).Find(&old).RecordNotFound(); ok {
		return t.DB.Create(attr).Error
	}
	return errors.ErrRecordAlreadyExist
}

// UpdateModel -
func (t *ComponentK8sAttributeDaoImpl) UpdateModel(mo model.Interface) error {
	attr := mo.(*model.ComponentK8sAttributes)
	return t.DB.Save(attr).Error
}

// GetByComponentIDAndName -
func (t *ComponentK8sAttributeDaoImpl) GetByComponentIDAndName(componentID, name string) (*model.ComponentK8sAttributes, error) {
	var record model.ComponentK8sAttributes
	if err := t.DB.Where("component_id=? and name=?", componentID, name).Take(&record).Error; err != nil {
		return nil, err
	}
	return &record, nil
}

// CreateOrUpdateAttributesInBatch Batch insert or update component attributes
func (t *ComponentK8sAttributeDaoImpl) CreateOrUpdateAttributesInBatch(attributes []*model.ComponentK8sAttributes) error {
	var objects []interface{}
	for _, attribute := range attributes {
		objects = append(objects, *attribute)
	}
	if err := gormbulkups.BulkUpsert(t.DB, objects, 2000); err != nil {
		return pkgerr.Wrap(err, "create or update component attributes in batch")
	}
	return nil
}

// DeleteByComponentIDAndName delete by componentID and name
func (t *ComponentK8sAttributeDaoImpl) DeleteByComponentIDAndName(componentID, name string) error {
	return t.DB.Where("component_id=? and name=?", componentID, name).Delete(&model.ComponentK8sAttributes{}).Error
}

// DeleteByComponentIDs delete by componentIDs
func (t *ComponentK8sAttributeDaoImpl) DeleteByComponentIDs(componentIDs []string) error {
	return t.DB.Where("component_id in (?)", componentIDs).Delete(&model.ComponentK8sAttributes{}).Error
}
