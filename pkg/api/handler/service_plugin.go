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

package handler

import (
	"context"
	"fmt"
	"strings"

	"github.com/Sirupsen/logrus"
	api_model "github.com/goodrain/rainbond/pkg/api/model"
	"github.com/goodrain/rainbond/pkg/api/util"
	"github.com/goodrain/rainbond/pkg/db"
	dbmodel "github.com/goodrain/rainbond/pkg/db/model"
	"github.com/pquerna/ffjson/ffjson"
)

//GetTenantServicePluginRelation GetTenantServicePluginRelation
func (s *ServiceAction) GetTenantServicePluginRelation(serviceID string) ([]*dbmodel.TenantServicePluginRelation, *util.APIHandleError) {
	gps, err := db.GetManager().TenantServicePluginRelationDao().GetALLRelationByServiceID(serviceID)
	if err != nil {
		return nil, util.CreateAPIHandleErrorFromDBError("get service relation by ID", err)
	}
	return gps, nil
}

//TenantServiceDeletePluginRelation 删除应用的plugin依赖
func (s *ServiceAction) TenantServiceDeletePluginRelation(serviceID, pluginID string) *util.APIHandleError {
	tx := db.GetManager().Begin()
	if err := db.GetManager().TenantServicePluginRelationDaoTransactions(tx).DeleteRelationByServiceIDAndPluginID(serviceID, pluginID); err != nil {
		tx.Rollback()
		return util.CreateAPIHandleErrorFromDBError("delete plugin relation", err)
	}
	if err := db.GetManager().TenantPluginVersionENVDaoTransactions(tx).DeleteEnvByPluginID(serviceID, pluginID); err != nil {
		tx.Rollback()
		return util.CreateAPIHandleErrorFromDBError("delete relation env", err)
	}
	if err := db.GetManager().TenantServicesStreamPluginPortDaoTransactions(tx).DeleteAllPluginMappingPortByServiceID(serviceID); err != nil {
		tx.Rollback()
		return util.CreateAPIHandleErrorFromDBError("delete upstream plugin mapping port", err)
	}
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return util.CreateAPIHandleErrorFromDBError("commit delete err", err)
	}
	return nil
}

//SetTenantServicePluginRelation SetTenantServicePluginRelation
func (s *ServiceAction) SetTenantServicePluginRelation(tenantID, serviceID string, pss *api_model.PluginSetStruct) *util.APIHandleError {
	plugin, err := db.GetManager().TenantPluginDao().GetPluginByID(pss.Body.PluginID, tenantID)
	if err != nil {
		return util.CreateAPIHandleErrorFromDBError("get plugin by plugin id", err)
	}
	catePlugin := strings.Split(plugin.PluginModel, ":")[0]
	//TODO:检查是否存在该大类插件
	crt, err := db.GetManager().TenantServicePluginRelationDao().CheckSomeModelLikePluginByServiceID(
		serviceID,
		catePlugin,
	)
	if err != nil {
		return util.CreateAPIHandleErrorFromDBError("check plugin model", err)
	}
	if crt {
		return util.CreateAPIHandleError(400, fmt.Errorf("can not add this kind plugin, a same kind plugin has been linked"))
	}
	tx := db.GetManager().Begin()
	if plugin.PluginModel == dbmodel.UpNetPlugin {
		ports, err := db.GetManager().TenantServicesPortDao().GetPortsByServiceID(serviceID)
		if err != nil {
			tx.Rollback()
			return util.CreateAPIHandleErrorFromDBError("get ports by service id", err)
		}
		for _, p := range ports {
			if p.IsInnerService || p.IsOuterService {
				pluginPort, err := db.GetManager().TenantServicesStreamPluginPortDaoTransactions(tx).SetPluginMappingPort(
					tenantID,
					serviceID,
					dbmodel.UpNetPlugin,
					p.ContainerPort,
				)
				if err != nil {
					tx.Rollback()
					logrus.Errorf(fmt.Sprintf("set upstream port %d error, %v", p.ContainerPort, err))
					return util.CreateAPIHandleErrorFromDBError(
						fmt.Sprintf("set upstream port %d error ", p.ContainerPort),
						err,
					)
				}
				logrus.Debugf("set plugin upsteam port %d->%d", p.ContainerPort, pluginPort)
				continue
			}
		}
	}
	relation := &dbmodel.TenantServicePluginRelation{
		VersionID:   pss.Body.VersionID,
		ServiceID:   serviceID,
		PluginID:    pss.Body.PluginID,
		Switch:      pss.Body.Switch,
		PluginModel: plugin.PluginModel,
	}
	if err := db.GetManager().TenantServicePluginRelationDaoTransactions(tx).AddModel(relation); err != nil {
		tx.Rollback()
		return util.CreateAPIHandleErrorFromDBError("set service plugin relation", err)
	}
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return util.CreateAPIHandleErrorFromDBError("commit set service plugin relation", err)
	}
	return nil
}

//UpdateTenantServicePluginRelation UpdateTenantServicePluginRelation
func (s *ServiceAction) UpdateTenantServicePluginRelation(serviceID string, pss *api_model.PluginSetStruct) *util.APIHandleError {
	relation, err := db.GetManager().TenantServicePluginRelationDao().GetRelateionByServiceIDAndPluginID(serviceID, pss.Body.PluginID)
	if err != nil {
		return util.CreateAPIHandleErrorFromDBError("get relation by serviceid and pluginid", err)
	}
	relation.VersionID = pss.Body.VersionID
	relation.Switch = pss.Body.Switch
	err = db.GetManager().TenantServicePluginRelationDao().UpdateModel(relation)
	if err != nil {
		return util.CreateAPIHandleErrorFromDBError("update relation between plugin and service", err)
	}
	return nil
}

//SetVersionEnv SetVersionEnv
func (s *ServiceAction) SetVersionEnv(sve *api_model.SetVersionEnv) *util.APIHandleError {
	if len(sve.Body.ConfigEnvs.NormalEnvs) != 0 {
		if err := s.normalEnvs(sve); err != nil {
			return util.CreateAPIHandleErrorFromDBError("set version env", err)
		}
	}
	if sve.Body.ConfigEnvs.ComplexEnvs != nil {
		if err := s.complexEnvs(sve); err != nil {
			if strings.Contains(err.Error(), "is exist") {
				return util.CreateAPIHandleError(405, err)
			}
			return util.CreateAPIHandleError(500, fmt.Errorf("set complex error, %v", err))
		}
	}
	if len(sve.Body.ConfigEnvs.NormalEnvs) == 0 && sve.Body.ConfigEnvs.ComplexEnvs == nil {
		return util.CreateAPIHandleError(200, fmt.Errorf("no envs need to be changed"))
	}
	return nil
}

func (s *ServiceAction) normalEnvs(sve *api_model.SetVersionEnv) error {
	tx := db.GetManager().Begin()
	for _, env := range sve.Body.ConfigEnvs.NormalEnvs {
		tpv := &dbmodel.TenantPluginVersionEnv{
			PluginID:  sve.PluginID,
			ServiceID: sve.Body.ServiceID,
			EnvName:   env.EnvName,
			EnvValue:  env.EnvValue,
		}
		if err := db.GetManager().TenantPluginVersionENVDaoTransactions(tx).AddModel(tpv); err != nil {
			tx.Rollback()
			return err
		}
	}
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return err
	}
	return nil
}

func (s *ServiceAction) complexEnvs(sve *api_model.SetVersionEnv) error {
	k := fmt.Sprintf("/resources/define/%s/%s/%s",
		sve.Body.TenantID,
		sve.ServiceAlias,
		sve.PluginID)
	if CheckKeyIfExist(s.EtcdCli, k) {
		return fmt.Errorf("key %v is exist", k)
	}
	v, err := ffjson.Marshal(sve.Body.ConfigEnvs.ComplexEnvs)
	if err != nil {
		logrus.Errorf("mashal etcd value error, %v", err)
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, err = s.EtcdCli.Put(ctx, k, string(v))
	if err != nil {
		logrus.Errorf("put k %s into etcd error, %v", k, err)
		return err
	}
	return nil
}

//DeleteComplexEnvs DeleteComplexEnvs
func (s *ServiceAction) DeleteComplexEnvs(tenantID, serviceAlias, pluginID string) *util.APIHandleError {
	k := fmt.Sprintf("/resources/define/%s/%s/%s",
		tenantID,
		serviceAlias,
		pluginID)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, err := s.EtcdCli.Delete(ctx, k)
	if err != nil {
		logrus.Errorf("delete k %s from etcd error, %v", k, err)
		return util.CreateAPIHandleError(500, fmt.Errorf("delete k %s from etcd error, %v", k, err))
	}
	return nil
}

//UpdateVersionEnv UpdateVersionEnv
func (s *ServiceAction) UpdateVersionEnv(uve *api_model.SetVersionEnv) *util.APIHandleError {
	if len(uve.Body.ConfigEnvs.NormalEnvs) != 0 {
		if err := s.upNormalEnvs(uve); err != nil {
			return util.CreateAPIHandleErrorFromDBError("update version env", err)
		}
	}
	if uve.Body.ConfigEnvs.ComplexEnvs != nil {
		if err := s.upComplexEnvs(uve); err != nil {
			if strings.Contains(err.Error(), "is not exist") {
				return util.CreateAPIHandleError(405, err)
			}
			return util.CreateAPIHandleError(500, fmt.Errorf("update complex error, %v", err))
		}
	}
	if len(uve.Body.ConfigEnvs.NormalEnvs) == 0 && uve.Body.ConfigEnvs.ComplexEnvs == nil {
		return util.CreateAPIHandleError(200, fmt.Errorf("no envs need to be changed"))
	}
	return nil
}

func (s *ServiceAction) upNormalEnvs(uve *api_model.SetVersionEnv) *util.APIHandleError {
	err := db.GetManager().TenantPluginVersionENVDao().DeleteEnvByPluginID(uve.Body.ServiceID, uve.PluginID)
	if err != nil {
		return util.CreateAPIHandleErrorFromDBError("delete version env", err)
	}
	if err := s.normalEnvs(uve); err != nil {
		return util.CreateAPIHandleErrorFromDBError("update version env", err)
	}
	return nil
}

func (s *ServiceAction) upComplexEnvs(uve *api_model.SetVersionEnv) *util.APIHandleError {
	k := fmt.Sprintf("/resources/define/%s/%s/%s",
		uve.Body.TenantID,
		uve.ServiceAlias,
		uve.PluginID)
	if !CheckKeyIfExist(s.EtcdCli, k) {
		return util.CreateAPIHandleError(404,
			fmt.Errorf("key %v is not exist", k))
	}
	v, err := ffjson.Marshal(uve.Body.ConfigEnvs.ComplexEnvs)
	if err != nil {
		logrus.Errorf("mashal etcd value error, %v", err)
		return util.CreateAPIHandleError(500, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, err = s.EtcdCli.Put(ctx, k, string(v))
	if err != nil {
		logrus.Errorf("put k %s into etcd error, %v", k, err)
		return util.CreateAPIHandleError(500, err)
	}
	return nil
}
