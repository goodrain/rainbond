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
	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	util "github.com/goodrain/rainbond/util"
	"github.com/goodrain/rainbond/worker/client"
)

// ThirdPartyServiceHanlder handles business logic for all third-party services
type ThirdPartyServiceHanlder struct {
	dbmanager db.Manager
	statusCli *client.AppRuntimeSyncClient
}

// Create3rdPartySvcHandler creates a new *ThirdPartyServiceHanlder.
func Create3rdPartySvcHandler(dbmanager db.Manager, statusCli *client.AppRuntimeSyncClient) *ThirdPartyServiceHanlder {
	return &ThirdPartyServiceHanlder{
		dbmanager: dbmanager,
		statusCli: statusCli,
	}
}

// AddEndpoints adds endpoints for third-party service.
func (t *ThirdPartyServiceHanlder) AddEndpoints(sid string, d *model.AddEndpiontsReq) error {
	ep := &dbmodel.Endpoint{
		UUID:      util.NewUUID(),
		ServiceID: sid,
		IP:        d.IP,
		Port:      0,
		IsOnline:  &d.IsOnline,
	}
	if err := t.dbmanager.EndpointsDao().AddModel(ep); err != nil {
		return err
	}

	t.statusCli.AddThirdPartyEndpoint(ep)
	return nil
}

// UpdEndpoints updates endpoints for third-party service.
func (t *ThirdPartyServiceHanlder) UpdEndpoints(d *model.UpdEndpiontsReq) error {
	ep, err := t.dbmanager.EndpointsDao().GetByUUID(d.EpID)
	if err != nil {
		logrus.Warningf("EpID: %s; error getting endpoints: %v", d.EpID, err)
		return err
	}
	if d.IP != "" {
		ep.IP = d.IP
	}
	ep.IsOnline = &d.IsOnline
	if err := t.dbmanager.EndpointsDao().UpdateModel(ep); err != nil {
		return err
	}

	t.statusCli.UpdThirdPartyEndpoint(ep)
	
	return nil
}

// DelEndpoints deletes endpoints for third-party service.
func (t *ThirdPartyServiceHanlder) DelEndpoints(epid, sid string) error {
	if err := t.dbmanager.EndpointsDao().DelByUUID(epid); err != nil {
		return err
	}

	t.statusCli.DelThirdPartyEndpoint(epid, sid)

	return nil
}

// ListEndpoints lists third-party service endpoints.
func (t *ThirdPartyServiceHanlder) ListEndpoints(sid string) ([]*model.EndpointResp, error) {
	thirdPartyEndpoints, err := t.statusCli.ListThirdPartyEndpoints(sid)
	if err != nil {
		logrus.Warningf("ServiceID: %s; grpc; error listing third-party endpoints: %v", sid, err)
		return nil, err
	}
	if thirdPartyEndpoints == nil || thirdPartyEndpoints.Obj == nil {
		return nil, nil
	}
	var res []*model.EndpointResp
	for _, item := range thirdPartyEndpoints.Obj {
		res = append(res, &model.EndpointResp{
			EpID:     item.Uuid,
			IP:       item.Ip,
			Status:   item.Status,
			IsOnline: item.IsOnline,
		})
	}

	return res, nil
}
