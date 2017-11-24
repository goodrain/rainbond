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

	"github.com/goodrain/rainbond/pkg/db/model"
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
	if ok := t.DB.Where("plugin_id=? and env_name = ?", env.PluginID, env.ENVName).Find(&oldENV).RecordNotFound(); ok {
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
func (t *PluginDefaultENVDaoImpl) GetDefaultENVByName(pluginID string, name string) (*model.TenantPluginDefaultENV, error) {
	var env model.TenantPluginDefaultENV
	if err := t.DB.Where("plugin_id=? and env_name=?", name).Find(&env).Error; err != nil {
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
	if err := t.DB.Where("plugin_id=? and is_change=0", pluginID).Find(&envs).Error; err != nil {
		return nil, err
	}
	return envs, nil
}

//DeleteDefaultENVByName DeleteDefaultENVByName
func (t *PluginDefaultENVDaoImpl) DeleteDefaultENVByName(pluginID, name string) error {
	relation := &model.TenantPluginDefaultENV{
		ENVName: name,
	}
	if err := t.DB.Where("plugin_id=? and env_name=?", pluginID, name).Delete(relation).Error; err != nil {
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
	if err := t.DB.Where("plugin_id=? and is_change=1", pluginID).Find(&envs).Error; err != nil {
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
	if ok := t.DB.Where("plugin_id =? and version_id = ?", version.PluginID, version.VersionID).Find(&oldVersion).RecordNotFound(); ok {
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
	if ok := t.DB.Where("plugin_id=? and env_name = ?", env.PluginID, env.EnvName).Find(&oldENV).RecordNotFound(); ok {
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
	var env *model.TenantPluginVersionEnv
	if err := t.DB.Where("service_id=? and plugin_id=? and env_name=?", serviceID, pluginID, envName).Find(&env).Error; err != nil {
		return nil, err
	}
	return env, nil
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
