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
	"reflect"
	"strings"

	"github.com/jinzhu/gorm"
	"github.com/goodrain/rainbond/db/model"
)

// EndpointDaoImpl implements EndpintDao
type EndpointDaoImpl struct {
	DB *gorm.DB
}

// AddModel add one record for table 3rd_party_svc_endpoint
func (e *EndpointDaoImpl) AddModel(mo model.Interface) error {
	ep, ok := mo.(*model.Endpoint)
	if !ok {
		return fmt.Errorf("Type conversion error. From %s to *model.Endpoint", reflect.TypeOf(mo))
	}
	var o model.Endpoint
	if ok := e.DB.Where("service_id=? and ip=?", ep.ServiceID, ep.IP).Find(&o).RecordNotFound(); ok {
		if err := e.DB.Create(ep).Error; err != nil {
			return err
		}
	} else {
		return fmt.Errorf("Endpoint exists based on servicd_id(%s) and ip(%s)", ep.ServiceID, ep.IP)
	}
	return nil
}

// UpdateModel updates one record for table 3rd_party_svc_endpoint
func (e *EndpointDaoImpl) UpdateModel(mo model.Interface) error {
	ep, ok := mo.(*model.Endpoint)
	if !ok {
		return fmt.Errorf("Type conversion error. From %s to *model.Endpoint", reflect.TypeOf(mo))
	}
	if strings.Replace(ep.UUID, " ", "", -1) == "" {
		return fmt.Errorf("uuid can not be empty.")
	}
	return e.DB.Save(ep).Error
}

// GetByUUID returns endpints matching the given uuid.
func (e *EndpointDaoImpl) GetByUUID(uuid string) (*model.Endpoint, error) {
	var ep model.Endpoint
	if err := e.DB.Where("uuid=?", uuid).Find(&ep).Error; err != nil {
		return nil, err
	}
	return &ep, nil
}

// List list all endpints matching the given serivce_id(sid).
func (e *EndpointDaoImpl) List(sid string) ([]*model.Endpoint, error){
	var eps []*model.Endpoint
	if err := e.DB.Where("service_id=?", sid).Find(&eps).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return eps, nil
}

// DelByUUID deletes endpoints matching uuid.
func (e *EndpointDaoImpl) DelByUUID(uuid string) error {
	if err := e.DB.Where("uuid=?", uuid).Delete(model.Endpoint{}).Error; err != nil {
		return err
	}
	return nil
}

// ThirdPartyServiceProbeDaoImpl implements ThirdPartyServiceProbeDao
type ThirdPartyServiceProbeDaoImpl struct {
	DB *gorm.DB
}
 
// AddModel add one record for table 3rd_party_svc_probe.
func (t *ThirdPartyServiceProbeDaoImpl) AddModel(mo model.Interface) error {
	probe, ok := mo.(*model.ThirdPartyServiceProbe)
	if !ok {
		return fmt.Errorf("Type conversion error. From %s to *model.ThirdPartyServiceProbe", reflect.TypeOf(mo))
	}
	var old model.ThirdPartyServiceProbe
	if ok := t.DB.Where("service_id=?", probe.ServiceID).Find(&old).RecordNotFound(); ok {
		if err := t.DB.Create(probe).Error; err != nil {
			return err
		}
	} else {
		return fmt.Errorf("Probe exists based on servicd_id(%s)", probe.ServiceID)
	}
	return nil
}

// UpdateModel updates one record for table 3rd_party_svc_probe.
func (t *ThirdPartyServiceProbeDaoImpl) UpdateModel(mo model.Interface) error {
	probe, ok := mo.(*model.ThirdPartyServiceProbe)
	if !ok {
		return fmt.Errorf("Type conversion error. From %s to *model.ThirdPartyServiceProbe", reflect.TypeOf(mo))
	}
	return t.DB.Table(probe.TableName()).
		Where("service_id = ?", probe.ServiceID).
		Update(probe).Error
}

// GetByServiceID returns a *model.ThirdPartyServiceProbe matching sid(service_id).
func (t *ThirdPartyServiceProbeDaoImpl) GetByServiceID(sid string) (*model.ThirdPartyServiceProbe, error) {
	var probe model.ThirdPartyServiceProbe
	if err := t.DB.Where("service_id=?", sid).Find(&probe).Error; err != nil {
		return nil, err
	}
	return &probe, nil
}

// ThirdPartyServiceDiscoveryCfgDaoImpl implements ThirdPartyServiceDiscoveryCfgDao
type ThirdPartyServiceDiscoveryCfgDaoImpl struct {
	DB *gorm.DB
}
 
// AddModel add one record for table 3rd_party_svc_discovery_cfg.
func (t *ThirdPartyServiceDiscoveryCfgDaoImpl) AddModel(mo model.Interface) error {
	cfg, ok := mo.(*model.ThirdPartyServiceDiscoveryCfg)
	if !ok {
		return fmt.Errorf("Type conversion error. From %s to *model.ThirdPartyServiceDiscoveryCfg",
			reflect.TypeOf(mo))
	}
	var old model.ThirdPartyServiceDiscoveryCfg
	if ok := t.DB.Where("service_id=?", cfg.ServiceID).Find(&old).RecordNotFound(); ok {
		if err := t.DB.Create(cfg).Error; err != nil {
			return err
		}
	} else {
		return fmt.Errorf("Probe exists based on servicd_id(%s)", cfg.ServiceID)
	}
	return nil
}

// UpdateModel blabla
func (t *ThirdPartyServiceDiscoveryCfgDaoImpl) UpdateModel(mo model.Interface) error {
	return nil
}