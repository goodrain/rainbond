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
	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
	"time"
)

//TenantDaoImpl 租户信息管理
type MockTenantDaoImpl struct {
}

//AddModel 添加租户
func (t *MockTenantDaoImpl) AddModel(mo model.Interface) error {
	return nil
}

//UpdateModel 更新租户
func (t *MockTenantDaoImpl) UpdateModel(mo model.Interface) error {

	return nil
}

//GetTenantByUUID 获取租户
func (t *MockTenantDaoImpl) GetTenantByUUID(uuid string) (*model.Tenants, error) {
	createTime, _ := time.Parse(time.RFC3339, "2018-10-18T10:24:43Z")
	tenant := &model.Tenants{
		Model: model.Model{
			ID:        1,
			CreatedAt: createTime,
		},
		Name: "0enb7gyx",
		UUID: "e8539a9c33fd418db11cce26d2bca431",
		EID:  "214ec4d212582eb36a84cc180aad2783",
	}
	return tenant, nil
}

//GetTenantByUUIDIsExist 获取租户
func (t *MockTenantDaoImpl) GetTenantByUUIDIsExist(uuid string) bool {
	return false

}

//GetTenantIDByName 获取租户
func (t *MockTenantDaoImpl) GetTenantIDByName(name string) (*model.Tenants, error) {
	return nil, nil
}

//GetALLTenants GetALLTenants
func (t *MockTenantDaoImpl) GetALLTenants() ([]*model.Tenants, error) {
	return nil, nil
}

//GetTenantsByEid
func (t *MockTenantDaoImpl) GetTenantByEid(eid string) ([]*model.Tenants, error) {
	return nil, nil
}

//GetTenantIDsByNames get tenant ids by names
func (t *MockTenantDaoImpl) GetTenantIDsByNames(names []string) (re []string, err error) {
	return
}

//GetALLTenants GetALLTenants
func (t *MockTenantDaoImpl) GetPagedTenants(offset, len int) ([]*model.Tenants, error) {
	return nil, nil
}

//TenantServicesDaoImpl 租户应用dao
type MockTenantServicesDaoImpl struct {
	DB *gorm.DB
}

//GetAllServices 获取全部应用信息的资源相关信息
func (t *MockTenantServicesDaoImpl) GetAllServices() ([]*model.TenantServices, error) {

	return nil, nil
}

func (t *MockTenantServicesDaoImpl) GetAllServicesID() ([]*model.TenantServices, error) {

	return nil, nil
}

//AddModel 添加租户应用
func (t *MockTenantServicesDaoImpl) AddModel(mo model.Interface) error {

	return nil
}

//UpdateModel 更新租户应用
func (t *MockTenantServicesDaoImpl) UpdateModel(mo model.Interface) error {

	return nil
}

//GetServiceByID 获取服务通过服务id
func (t *MockTenantServicesDaoImpl) GetServiceByID(serviceID string) (*model.TenantServices, error) {
	createTime, _ := time.Parse(time.RFC3339, "2018-10-22T14:14:12Z")
	updateTime, _ := time.Parse(time.RFC3339, "2018-10-22T14:14:12Z")
	services := &model.TenantServices{
		Model: model.Model{
			ID:        1,
			CreatedAt: createTime,
		},
		TenantID:        "e8539a9c33fd418db11cce26d2bca431",
		ServiceID:       "43eaae441859eda35b02075d37d83589",
		ServiceKey:      "application",
		ServiceAlias:    "grd83589",
		Comment:         "application info",
		ServiceVersion:  "latest",
		ImageName:       "goodrain.me/runner:latest",
		ContainerCPU:    20,
		ContainerMemory: 128,
		ContainerCMD:    "start_web",
		VolumePath:      "vol43eaae4418",
		ExtendMethod:    "stateless",
		Replicas:        1,
		DeployVersion:   "20181022200709",
		Category:        "application",
		CurStatus:       "undeploy",
		Status:          0,
		ServiceType:     "application",
		Namespace:       "goodrain",
		VolumeType:      "shared",
		PortType:        "multi_outer",
		UpdateTime:      updateTime,
		ServiceOrigin:   "assistant",
		CodeFrom:        "gitlab_demo",
		Domain:          "0enb7gyx",
	}
	return services, nil
}

//GetServiceByID 获取服务通过服务别名
func (t *MockTenantServicesDaoImpl) GetServiceByServiceAlias(serviceAlias string) (*model.TenantServices, error) {

	return nil, nil
}

//GetServiceMemoryByTenantIDs get service memory by tenant ids
func (t *MockTenantServicesDaoImpl) GetServiceMemoryByTenantIDs(tenantIDs []string, runningServiceIDs []string) (map[string]map[string]interface{}, error) {

	return nil, nil
}

//GetServiceMemoryByServiceIDs get service memory by service ids
func (t *MockTenantServicesDaoImpl) GetServiceMemoryByServiceIDs(serviceIDs []string) (map[string]map[string]interface{}, error) {

	return nil, nil
}

//GetPagedTenantService GetPagedTenantResource
func (t *MockTenantServicesDaoImpl) GetPagedTenantService(offset, length int, serviceIDs []string) ([]map[string]interface{}, int, error) {

	return nil, 0, nil
}

//GetServiceAliasByIDs 获取应用别名
func (t *MockTenantServicesDaoImpl) GetServiceAliasByIDs(uids []string) ([]*model.TenantServices, error) {

	return nil, nil
}

//GetServiceByIDs get some service by service ids
func (t *MockTenantServicesDaoImpl) GetServiceByIDs(uids []string) ([]*model.TenantServices, error) {

	return nil, nil
}

//GetServiceByTenantIDAndServiceAlias 根据租户名和服务名
func (t *MockTenantServicesDaoImpl) GetServiceByTenantIDAndServiceAlias(tenantID, serviceName string) (*model.TenantServices, error) {

	return nil, nil
}

//GetServicesByTenantID GetServicesByTenantID
func (t *MockTenantServicesDaoImpl) GetServicesByTenantID(tenantID string) ([]*model.TenantServices, error) {

	return nil, nil
}

//GetServicesByTenantIDs GetServicesByTenantIDs
func (t *MockTenantServicesDaoImpl) GetServicesByTenantIDs(tenantIDs []string) ([]*model.TenantServices, error) {

	return nil, nil
}

//GetServicesAllInfoByTenantID GetServicesAllInfoByTenantID
func (t *MockTenantServicesDaoImpl) GetServicesAllInfoByTenantID(tenantID string) ([]*model.TenantServices, error) {

	return nil, nil
}

//SetTenantServiceStatus SetTenantServiceStatus
func (t *MockTenantServicesDaoImpl) SetTenantServiceStatus(serviceID, status string) error {

	return nil
}

//DeleteServiceByServiceID DeleteServiceByServiceID
func (t *MockTenantServicesDaoImpl) DeleteServiceByServiceID(serviceID string) error {

	return nil
}

//TenantServicesPortDaoImpl 租户应用端口操作
type MockTenantServicesPortDaoImpl struct {

}

//AddModel 添加应用端口
func (t *MockTenantServicesPortDaoImpl) AddModel(mo model.Interface) error {
	return nil
}

//UpdateModel 更新租户
func (t *MockTenantServicesPortDaoImpl) UpdateModel(mo model.Interface) error {
	return nil
}

//DeleteModel 删除端口
func (t *MockTenantServicesPortDaoImpl) DeleteModel(serviceID string, args ...interface{}) error {
	return nil
}

//GetPortsByServiceID 通过服务获取port
func (t *MockTenantServicesPortDaoImpl) GetPortsByServiceID(serviceID string) ([]*model.TenantServicesPort, error) {
	var ports []*model.TenantServicesPort
	createTime, _ := time.Parse(time.RFC3339, "2018-10-22T14:14:12Z")
	port := &model.TenantServicesPort{
		Model: model.Model{
			ID:        2,
			CreatedAt: createTime,
		},
		TenantID:       "e8539a9c33fd418db11cce26d2bca431",
		ServiceID:      "43eaae441859eda35b02075d37d83589",
		ContainerPort:  5000,
		MappingPort:    5000,
		Protocol:       "http",
		PortAlias:      "GRD835895000",
		IsInnerService: false,
		IsOuterService: true,
	}
	ports = append(ports, port)
	return ports, nil

	return nil, nil
}

//GetOuterPorts  获取对外端口
func (t *MockTenantServicesPortDaoImpl) GetOuterPorts(serviceID string) ([]*model.TenantServicesPort, error) {
	var ports []*model.TenantServicesPort
	createTime, _ := time.Parse(time.RFC3339, "2018-10-22T14:14:12Z")
	port := &model.TenantServicesPort{
		Model: model.Model{
			ID:        2,
			CreatedAt: createTime,
		},
		TenantID:       "e8539a9c33fd418db11cce26d2bca431",
		ServiceID:      "43eaae441859eda35b02075d37d83589",
		ContainerPort:  5000,
		MappingPort:    5000,
		Protocol:       "http",
		PortAlias:      "GRD835895000",
		IsInnerService: false,
		IsOuterService: true,
	}
	ports = append(ports, port)
	return ports, nil
}

//GetInnerPorts 获取对内端口
func (t *MockTenantServicesPortDaoImpl) GetInnerPorts(serviceID string) ([]*model.TenantServicesPort, error) {
	return nil, nil
}

//GetPort get port
func (t *MockTenantServicesPortDaoImpl) GetPort(serviceID string, port int) (*model.TenantServicesPort, error) {
	return nil, nil
}

//DELPortsByServiceID DELPortsByServiceID
func (t *MockTenantServicesPortDaoImpl) DELPortsByServiceID(serviceID string) error {
	return nil
}
