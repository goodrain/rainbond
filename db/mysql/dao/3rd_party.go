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
	gormbulkups "github.com/atcdot/gorm-bulk-upsert"
	pkgerr "github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"reflect"
	"strings"

	"github.com/goodrain/rainbond/db/errors"
	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
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
	if ok := e.DB.Where("service_id=? and ip=? and port=?", ep.ServiceID, ep.IP, ep.Port).Find(&o).RecordNotFound(); ok {
		if err := e.DB.Save(ep).Error; err != nil {
			return err
		}
	} else {
		return errors.ErrRecordAlreadyExist
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
		return fmt.Errorf("uuid can not be empty")
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
func (e *EndpointDaoImpl) List(sid string) ([]*model.Endpoint, error) {
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
	if err := e.DB.Where("uuid=?", uuid).Delete(&model.Endpoint{}).Error; err != nil {
		return err
	}
	return nil
}

// DeleteByServiceID delete endpoints based on service id.
func (e *EndpointDaoImpl) DeleteByServiceID(sid string) error {
	return e.DB.Where("service_id=?", sid).Delete(&model.Endpoint{}).Error
}

// ThirdPartySvcDiscoveryCfgDaoImpl implements ThirdPartySvcDiscoveryCfgDao
type ThirdPartySvcDiscoveryCfgDaoImpl struct {
	DB *gorm.DB
}

// AddModel add one record for table 3rd_party_svc_discovery_cfg.
func (t *ThirdPartySvcDiscoveryCfgDaoImpl) AddModel(mo model.Interface) error {
	cfg, ok := mo.(*model.ThirdPartySvcDiscoveryCfg)
	if !ok {
		return fmt.Errorf("Type conversion error. From %s to *model.ThirdPartySvcDiscoveryCfg",
			reflect.TypeOf(mo))
	}
	var old model.ThirdPartySvcDiscoveryCfg
	if ok := t.DB.Where("service_id=?", cfg.ServiceID).Find(&old).RecordNotFound(); ok {
		if err := t.DB.Create(cfg).Error; err != nil {
			return err
		}
	} else {
		return fmt.Errorf("Discovery configuration exists based on servicd_id(%s)", cfg.ServiceID)
	}
	return nil
}

// UpdateModel blabla
func (t *ThirdPartySvcDiscoveryCfgDaoImpl) UpdateModel(mo model.Interface) error {
	return nil
}

// GetByServiceID return third-party service discovery configuration according to service_id.
func (t *ThirdPartySvcDiscoveryCfgDaoImpl) GetByServiceID(sid string) (*model.ThirdPartySvcDiscoveryCfg, error) {
	var cfg model.ThirdPartySvcDiscoveryCfg
	if err := t.DB.Where("service_id=?", sid).Find(&cfg).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &cfg, nil
}

// DeleteByServiceID delete discovery config based on service id.
func (t *ThirdPartySvcDiscoveryCfgDaoImpl) DeleteByServiceID(sid string) error {
	return t.DB.Where("service_id=?", sid).Delete(&model.ThirdPartySvcDiscoveryCfg{}).Error
}

// DeleteByComponentIDs delete discovery config based on componentIDs.
func (t *ThirdPartySvcDiscoveryCfgDaoImpl) DeleteByComponentIDs(componentIDs []string) error {
	return t.DB.Where("service_id in (?)", componentIDs).Delete(&model.ThirdPartySvcDiscoveryCfg{}).Error
}

// CreateOrUpdate3rdSvcDiscoveryCfgInBatch -
func (t *ThirdPartySvcDiscoveryCfgDaoImpl) CreateOrUpdate3rdSvcDiscoveryCfgInBatch(cfgs []*model.ThirdPartySvcDiscoveryCfg) error {
	dbType := t.DB.Dialect().GetName()
	if dbType == "sqlite3" {
		for _, cfg := range cfgs {
			if err := t.DB.Create(&cfg).Error; err != nil {
				logrus.Error("batch create or update cfgs error:", err)
				return err
			}
		}
		return nil
	}
	var objects []interface{}
	for _, cfg := range cfgs {
		objects = append(objects, *cfg)
	}
	if err := gormbulkups.BulkUpsert(t.DB, objects, 2000); err != nil {
		return pkgerr.Wrap(err, "create or update third party svc discovery config in batch")
	}
	return nil
}
