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

package handler

import (
	"fmt"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/util"
)

// ThirdPartyServiceHanlder handles business logic for all third-party services
type ThirdPartyServiceHanlder struct {
	dbmanager db.Manager
}

// Create3rdPartySvcHandler creates a new *ThirdPartyServiceHanlder.
func Create3rdPartySvcHandler(dbmanager db.Manager) *ThirdPartyServiceHanlder {
	return &ThirdPartyServiceHanlder{dbmanager: dbmanager}
}

// AddEndpoints adds endpints for third-party service.
func (t *ThirdPartyServiceHanlder) AddEndpoints(sid string, data []*model.AddEndpiontsReq) error {
	tx := db.GetManager().Begin()
	for _, d := range data {
		ep := &dbmodel.Endpoint{
			UUID:      util.NewUUID(),
			ServiceID: sid,
			IP:        d.IP,
			IsOnline:  d.IsOnline,
		}
		if err := t.dbmanager.EndpointsDaoTransactions(tx).AddModel(ep); err != nil {
			tx.Rollback()
			logrus.Errorf("error creating endpoint record: %v", err)
			return err
		}
	}
	tx.Commit()
	return nil
}

// UpdEndpoints updates endpints for third-party service.
func (t *ThirdPartyServiceHanlder) UpdEndpoints(data []*model.UpdEndpiontsReq) error {
	tx := db.GetManager().Begin()
	for _, d := range data {
		ep, err := t.dbmanager.EndpointsDaoTransactions(tx).GetByUUID(d.UUID)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("uuid: %s, error getting endpoint: %v", d.UUID, err)
		}
		if err := t.dbmanager.EndpointsDaoTransactions(tx).UpdateModel(ep); err != nil {
			tx.Rollback()
			return fmt.Errorf("uuid: %s, error updating endpoint: %v", d.UUID, err)
		}
	}
	tx.Commit()
	return nil
}

// DelEndpoints deletes endpints for third-party service.
func (t *ThirdPartyServiceHanlder) DelEndpoints(uuids []model.DelEndpiontsReq) error {
	tx := db.GetManager().Begin()
	for _, ep := range uuids {
		if err := t.dbmanager.EndpointsDaoTransactions(tx).DelByUUID(ep.UUID); err != nil {
			tx.Rollback()
			return fmt.Errorf("uuid: %s, error deleting endpoint: %v", ep.UUID, err)
		}
	}
	tx.Commit()
	return nil
}

// UpdProbe updates third-party service probe with sid(service_id).
func (t *ThirdPartyServiceHanlder) UpdProbe(sid string, new *model.ThridPartyServiceProbe) error {
	tx := db.GetManager().Begin()
	probe, err := t.dbmanager.ThirdPartyServiceProbeDaoTransactions(tx).GetByServiceID(sid)
	if err == nil {
		logrus.Errorf("service_id: %s, error getting probe: %v", sid, err)
		tx.Rollback()
		return err
	}
	if strings.Replace(new.Scheme, " ", "", -1) != "" {
		probe.Scheme = new.Scheme
	}
	if new.Port > 0 && new.Port <= 65535 {
		probe.Port = new.Port
	}
	if strings.Replace(new.Path, " ", "", -1) != "" {
		probe.Path = new.Path
	}
	if new.TimeInterval > 0 {
		probe.TimeInterval = new.TimeInterval
	}
	if new.MaxErrorNum > 0 {
		probe.MaxErrorNum = new.MaxErrorNum
	}
	if err := t.dbmanager.ThirdPartyServiceProbeDaoTransactions(tx).UpdateModel(probe); err != nil {
		logrus.Errorf("service_id: %s, error updating probe: %v", sid, err)
		tx.Rollback()
		return err
	}
	tx.Commit()
	return nil
}

// GetProbe returns a third-party service probe matching sid(service_id).
func (t *ThirdPartyServiceHanlder) GetProbe(sid string) (*model.ThridPartyServiceProbe, error) {
	probe, err := t.dbmanager.ThirdPartyServiceProbeDao().GetByServiceID(sid)
	if err != nil {
		return nil, err
	}
	return &model.ThridPartyServiceProbe{
		Scheme: probe.Scheme,
		Port: probe.Port,
		Path: probe.Path,
		TimeInterval: probe.TimeInterval,
		MaxErrorNum: probe.MaxErrorNum,
		Action: probe.Action,
	}, nil
}