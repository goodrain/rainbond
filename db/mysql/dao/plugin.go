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

	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
)

//PluginDaoImpl PluginDaoImpl
type PluginDaoImpl struct {
	DB *gorm.DB
}

//AddModel 创建插件
func (t *PluginDaoImpl) AddModel(mo model.Interface) error {
	plugin := mo.(*model.TenantPlugin)
	var oldPlugin model.TenantPlugin
	if ok := t.DB.Where("plugin_id = ? and tenant_id = ?", plugin.PluginID, plugin.TenantID).Find(&oldPlugin).RecordNotFound(); ok {
		if err := t.DB.Create(plugin).Error; err != nil {
			return err
		}
	} else {
		return fmt.Errorf("plugin %s in tenant %s is exist", plugin.PluginName, plugin.TenantID)
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
func (t *PluginDaoImpl) GetPluginByID(id, tenantID string) (*model.TenantPlugin, error) {
	var plugin model.TenantPlugin
	if err := t.DB.Where("plugin_id = ? and tenant_id = ?", id, tenantID).Find(&plugin).Error; err != nil {
		return nil, err
	}
	return &plugin, nil
}

//DeletePluginByID DeletePluginByID
func (t *PluginDaoImpl) DeletePluginByID(id, tenantID string) error {
	var plugin model.TenantPlugin
	if err := t.DB.Where("plugin_id=? and tenant_id=?", id, tenantID).Delete(&plugin).Error; err != nil {
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
	if ok := t.DB.Where("plugin_id=? and env_name = ? and version_id = ?",
		env.PluginID,
		env.ENVName,
		env.VersionID).Find(&oldENV).RecordNotFound(); ok {
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

//GetALLMasterDefultENVs GetALLMasterDefultENVs
func (t *PluginDefaultENVDaoImpl) GetALLMasterDefultENVs(pluginID string) ([]*model.TenantPluginDefaultENV, error) {
	var envs []*model.TenantPluginDefaultENV
	if err := t.DB.Where("plugin_id=? and version_id=?", pluginID, "master_rb").Find(&envs).Error; err != nil {
		return nil, err
	}
	return envs, nil
}

//GetDefaultENVByName GetDefaultENVByName
func (t *PluginDefaultENVDaoImpl) GetDefaultENVByName(pluginID, name, versionID string) (*model.TenantPluginDefaultENV, error) {
	var env model.TenantPluginDefaultENV
	if err := t.DB.Where("plugin_id=? and env_name=? and version_id=?",
		pluginID,
		name,
		versionID).Find(&env).Error; err != nil {
		return nil, err
	}
	return &env, nil
}

//GetDefaultENVSByPluginID GetDefaultENVSByPluginID
func (t *PluginDefaultENVDaoImpl) GetDefaultENVSByPluginID(pluginID, versionID string) ([]*model.TenantPluginDefaultENV, error) {
	var envs []*model.TenantPluginDefaultENV
	if err := t.DB.Where("plugin_id=? and version_id=?", pluginID, versionID).Find(&envs).Error; err != nil {
		return nil, err
	}
	return envs, nil
}

//DeleteDefaultENVByName DeleteDefaultENVByName
func (t *PluginDefaultENVDaoImpl) DeleteDefaultENVByName(pluginID, name, versionID string) error {
	relation := &model.TenantPluginDefaultENV{
		ENVName: name,
	}
	if err := t.DB.Where("plugin_id=? and env_name=? and version_id=?",
		pluginID, name, versionID).Delete(relation).Error; err != nil {
		return err
	}
	return nil
}

//DeleteDefaultENVByPluginIDAndVersionID DeleteDefaultENVByPluginIDAndVersionID
func (t *PluginDefaultENVDaoImpl) DeleteDefaultENVByPluginIDAndVersionID(pluginID, versionID string) error {
	relation := &model.TenantPluginDefaultENV{
		PluginID: pluginID,
	}
	if err := t.DB.Where("plugin_id=? and version_id=?", pluginID, versionID).Delete(relation).Error; err != nil {
		return err
	}
	return nil
}

//DeleteAllDefaultENVByPluginID DeleteAllDefaultENVByPluginID
func (t *PluginDefaultENVDaoImpl) DeleteAllDefaultENVByPluginID(pluginID string) error {
	relation := &model.TenantPluginDefaultENV{
		PluginID: pluginID,
	}
	if err := t.DB.Where("plugin_id=?", pluginID).Delete(relation).Error; err != nil {
		return err
	}
	return nil
}

//GetDefaultEnvWhichCanBeSetByPluginID GetDefaultEnvWhichCanBeSetByPluginID
func (t *PluginDefaultENVDaoImpl) GetDefaultEnvWhichCanBeSetByPluginID(pluginID, versionID string) ([]*model.TenantPluginDefaultENV, error) {
	var envs []*model.TenantPluginDefaultENV
	if err := t.DB.Where("plugin_id=? and is_change=? and version_id=?", pluginID, true, versionID).Find(&envs).Error; err != nil {
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
	if ok := t.DB.Where("plugin_id =? and version_id = ? and deploy_version=?", version.PluginID, version.VersionID, version.DeployVersion).Find(&oldVersion).RecordNotFound(); ok {
		if err := t.DB.Create(version).Error; err != nil {
			return err
		}
	} else {
		return fmt.Errorf("plugin build version %s and deploy_verson %s is exist", version.VersionID, version.DeployVersion)
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

//GetBuildVersionByDeployVersion GetBuildVersionByDeployVersion
func (t *PluginBuildVersionDaoImpl) GetBuildVersionByDeployVersion(pluginID, versionID, deployVersion string) (*model.TenantPluginBuildVersion, error) {
	var version model.TenantPluginBuildVersion
	if err := t.DB.Where("plugin_id=? and version_id = ? and deploy_version=?", pluginID, versionID, deployVersion).Find(&version).Error; err != nil {
		return nil, err
	}
	return &version, nil
}

//GetLastBuildVersionByVersionID get last success build version
func (t *PluginBuildVersionDaoImpl) GetLastBuildVersionByVersionID(pluginID, versionID string) (*model.TenantPluginBuildVersion, error) {
	var version model.TenantPluginBuildVersion
	if err := t.DB.Where("plugin_id=? and version_id = ? and status=?", pluginID, versionID, "complete").Order("ID desc").Limit("1").Find(&version).Error; err != nil {
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
	if ok := t.DB.Where("service_id=? and plugin_id=? and env_name = ?", env.ServiceID, env.PluginID, env.EnvName).Find(&oldENV).RecordNotFound(); ok {
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

//GetVersionEnvByEnvName GetVersionEnvByEnvName
func (t *PluginVersionEnvDaoImpl) GetVersionEnvByEnvName(serviceID, pluginID, envName string) (*model.TenantPluginVersionEnv, error) {
	var env model.TenantPluginVersionEnv
	if err := t.DB.Where("service_id=? and plugin_id=? and env_name=?", serviceID, pluginID, envName).Find(&env).Error; err != nil {
		return nil, err
	}
	return &env, nil
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

//CheckSomeModelPluginByServiceID 检查是否绑定了某种插件且处于启用状态
func (t *TenantServicePluginRelationDaoImpl) CheckSomeModelPluginByServiceID(serviceID, pluginModel string) (bool, error) {
	var relations []*model.TenantServicePluginRelation
	if err := t.DB.Where("service_id=? and plugin_model=? and switch=?", serviceID, pluginModel, true).Find(&relations).Error; err != nil {
		return false, err
	}
	if len(relations) == 1 {
		return true, nil
	}
	return false, nil
}

//CheckSomeModelLikePluginByServiceID 检查是否绑定了某大类插件
func (t *TenantServicePluginRelationDaoImpl) CheckSomeModelLikePluginByServiceID(serviceID, pluginModel string) (bool, error) {
	var relations []*model.TenantServicePluginRelation
	catePlugin := "%" + pluginModel + "%"
	if err := t.DB.Where("service_id=? and plugin_model LIKE ?", serviceID, catePlugin).Find(&relations).Error; err != nil {
		return false, err
	}
	if len(relations) == 1 {
		return true, nil
	}
	return false, nil
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

//TenantServicesStreamPluginPortDaoImpl TenantServicesStreamPluginPortDaoImpl
type TenantServicesStreamPluginPortDaoImpl struct {
	DB *gorm.DB
}

//AddModel 添加插件端口映射信息
func (t *TenantServicesStreamPluginPortDaoImpl) AddModel(mo model.Interface) error {
	port := mo.(*model.TenantServicesStreamPluginPort)
	var oldPort model.TenantServicesStreamPluginPort
	if ok := t.DB.Where("service_id= ? and container_port= ? and plugin_model=? ",
		port.ServiceID,
		port.ContainerPort,
		port.PluginModel).Find(&oldPort).RecordNotFound(); ok {
		if err := t.DB.Create(port).Error; err != nil {
			return err
		}
	} else {
		return fmt.Errorf("plugin port %d mappint to %d is exist", port.ContainerPort, port.PluginPort)
	}
	return nil
}

//UpdateModel 更新插件端口映射信息
func (t *TenantServicesStreamPluginPortDaoImpl) UpdateModel(mo model.Interface) error {
	port := mo.(*model.TenantServicesStreamPluginPort)
	if port.ID == 0 {
		return fmt.Errorf("id can not be empty when update plugin mapping port")
	}
	if err := t.DB.Save(port).Error; err != nil {
		return err
	}
	return nil
}

//GetPluginMappingPorts GetPluginMappingPorts  降序排列
func (t *TenantServicesStreamPluginPortDaoImpl) GetPluginMappingPorts(
	serviceID string, pluginModel string) ([]*model.TenantServicesStreamPluginPort, error) {
	var ports []*model.TenantServicesStreamPluginPort
	if err := t.DB.Where("service_id=? and plugin_model=?",
		serviceID, pluginModel).Order("plugin_port asc").Find(&ports).Error; err != nil {
		return nil, err
	}
	return ports, nil
}

//GetPluginMappingPortByServiceIDAndContainerPort GetPluginMappingPortByServiceIDAndContainerPort
func (t *TenantServicesStreamPluginPortDaoImpl) GetPluginMappingPortByServiceIDAndContainerPort(
	serviceID string,
	pluginModel string,
	containerPort int,
) (*model.TenantServicesStreamPluginPort, error) {
	var port model.TenantServicesStreamPluginPort
	if err := t.DB.Where(
		"service_id=? and plugin_model=? and container_port=?",
		serviceID,
		pluginModel,
		containerPort,
	).Find(&port).Error; err != nil {
		return nil, err
	}
	return &port, nil
}

//SetPluginMappingPort SetPluginMappingPort
func (t *TenantServicesStreamPluginPortDaoImpl) SetPluginMappingPort(
	tenantID string,
	serviceID string,
	pluginModel string,
	containerPort int) (int, error) {
	ports, err := t.GetPluginMappingPorts(serviceID, pluginModel)
	if err != nil {
		return 0, err
	}
	//if have been allocated,return
	for _, oldp := range ports {
		if oldp.ContainerPort == containerPort {
			return oldp.PluginPort, nil
		}
	}
	//Distribution port range
	minPort := 65301
	maxPort := 65400
	newPort := &model.TenantServicesStreamPluginPort{
		TenantID:      tenantID,
		ServiceID:     serviceID,
		PluginModel:   pluginModel,
		ContainerPort: containerPort,
	}
	if len(ports) == 0 {
		newPort.PluginPort = minPort
		if err := t.AddModel(newPort); err != nil {
			return 0, err
		}
		return newPort.PluginPort, nil
	}
	oldMaxPort := ports[len(ports)-1]
	//已分配端口+2大于最大端口限制则从原范围内扫描端口使用
	if oldMaxPort.PluginPort > (maxPort - 2) {
		waitPort := minPort
		for _, p := range ports {
			if p.PluginPort == waitPort {
				waitPort++
				continue
			}
			newPort.PluginPort = waitPort
			if err := t.AddModel(newPort); err != nil {
				return 0, nil
			}
			continue
		}
	}
	//端口与预分配端口相同重新分配
	if containerPort == (oldMaxPort.PluginPort + 1) {
		newPort.PluginPort = oldMaxPort.PluginPort + 2
		if err := t.AddModel(newPort); err != nil {
			return 0, err
		}
		return newPort.PluginPort, nil
	}
	newPort.PluginPort = oldMaxPort.PluginPort + 1
	if err := t.AddModel(newPort); err != nil {
		return 0, err
	}
	return newPort.PluginPort, nil
}

//DeletePluginMappingPortByContainerPort DeletePluginMappingPortByContainerPort
func (t *TenantServicesStreamPluginPortDaoImpl) DeletePluginMappingPortByContainerPort(
	serviceID string,
	pluginModel string,
	containerPort int) error {
	relation := &model.TenantServicesStreamPluginPort{
		ServiceID:     serviceID,
		PluginModel:   pluginModel,
		ContainerPort: containerPort,
	}
	return t.DB.Where("service_id=? and plugin_model=? and container_port=?",
		serviceID,
		pluginModel,
		containerPort).Delete(relation).Error
}

//DeleteAllPluginMappingPortByServiceID DeleteAllPluginMappingPortByServiceID
func (t *TenantServicesStreamPluginPortDaoImpl) DeleteAllPluginMappingPortByServiceID(serviceID string) error {
	relation := &model.TenantServicesStreamPluginPort{
		ServiceID: serviceID,
	}
	return t.DB.Where("service_id=?", serviceID).Delete(relation).Error
}
