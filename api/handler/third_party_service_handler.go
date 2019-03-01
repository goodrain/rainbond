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
	"context"
	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/worker/appm/thirdparty"
	"github.com/goodrain/rainbond/worker/appm/types/v1"
	"github.com/goodrain/rainbond/worker/client"
	"github.com/goodrain/rainbond/worker/server/pb"
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
	return thirdparty.AddEndpoints(sid, v1.AddEndpointReq{
		IP:       d.IP,
		IsOnline: d.IsOnline,
	}, t.dbmanager)
}

// UpdEndpoints updates endpoints for third-party service.
func (t *ThirdPartyServiceHanlder) UpdEndpoints(d *model.UpdEndpiontsReq, sid string) error {
	req := v1.UpdEndpointReq{
		EpID:     d.EpID,
		IP:       d.IP,
		IsOnline: d.IsOnline,
	}
	return thirdparty.UpdEndpoints(sid, req, t.dbmanager)
}

// DelEndpoints deletes endpoints for third-party service.
func (t *ThirdPartyServiceHanlder) DelEndpoints(sid string, data *model.DelEndpiontsReq) error {
	return thirdparty.DelEndpoint(t.dbmanager, sid, data.EpID)
}

// ListEndpoints lists third-party service endpoints.
func (t *ThirdPartyServiceHanlder) ListEndpoints(sid string) ([]*model.EndpointResp, error) {
	status, err := t.statusCli.GetThirdPartyEndpointsStatus(context.Background(), &pb.ServiceRequest{
		ServiceId: sid,
	})
	if err != nil {
		logrus.Warningf("error getting third-party endpoints status: %v", err)
	}
	if status != nil {
		for key, val := range status.Status {
			logrus.Debugf("third-party service status, Key: %s, Value: %v", key, val)
		}
	}
	eps, err := thirdparty.ListEndpoints(sid, t.dbmanager)
	if err != nil {
		logrus.Errorf("error listing endpoints: %v", err)
		return nil, err
	}
	var res []*model.EndpointResp
	for _, ep := range eps {
		r := &model.EndpointResp{
			EpID:     ep.UUID,
			IP:       ep.IP,
			IsOnline: *ep.IsOnline,
		}
		r.Status = func(status *pb.ThirdPartyEndpointsStatus, ip string) string {
			if status == nil {
				return "unknown"
			}
			item, ok := status.Status[ip]
			if !ok {
				return "unknown"
			}
			if item {
				return "healthy"
			}
			return "unhealthy"
		}(status, ep.IP)
		res = append(res, r)
	}
	return res, nil
}
