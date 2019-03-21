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
	"encoding/json"
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/util"
	"github.com/goodrain/rainbond/worker/client"
	"strconv"
	"strings"
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
	endpoints, err := t.dbmanager.EndpointsDao().List(sid)
	b, _ := json.Marshal(endpoints)
	logrus.Debugf("Endpoints from db: %s", string(b))
	if err != nil {
		logrus.Warningf("ServiceID: %s; error listing endpoints from db; %v", sid, err)
	}
	m := make(map[string]*model.EndpointResp)
	for _, item := range endpoints {
		m[item.UUID] = &model.EndpointResp{
			EpID: item.UUID,
			IP: func(ip string, port int) string {
				if port != 0 {
					return fmt.Sprintf("%s:%d", ip, port)
				}
				return ip
			}(item.IP, item.Port),
			Status:   "-",
			IsOnline: false,
			IsStatic: true,
		}
	}

	thirdPartyEndpoints, err := t.statusCli.ListThirdPartyEndpoints(sid)
	if err != nil {
		logrus.Warningf("ServiceID: %s; grpc; error listing third-party endpoints: %v", sid, err)
		return nil, err
	}
	if thirdPartyEndpoints != nil && thirdPartyEndpoints.Obj != nil {
		b, _ = json.Marshal(thirdPartyEndpoints)
		logrus.Debugf("Endpoints from rpc: %s", string(b))
		for _, item := range thirdPartyEndpoints.Obj {
			ep := m[item.Uuid]
			if ep != nil {
				ep.IsOnline = true
				ep.Status = item.Status
				continue
			}
			m[item.Uuid] = &model.EndpointResp{
				EpID: item.Uuid,
				IP: func(ip string, port int32) string {
					if port != 0 {
						return fmt.Sprintf("%s:%d", ip, port)
					}
					return ip
				}(item.Ip, item.Port),
				Status:   item.Status,
				IsOnline: true,
				IsStatic: false,
			}
		}
	}

	var res []*model.EndpointResp
	for _, item := range m {
		res = append(res, item)
	}

	return res, nil
}

func splitIP(input string) (string, int) {
	sli := strings.Split(input, ":")
	if len(sli) == 2 {
		return sli[0], func(port string) int {
			p, err := strconv.Atoi(port)
			if err != nil {
				logrus.Warningf("String: %s; error converting string to int", port)
				return 0
			}
			return p
		}(sli[1])
	}
	return input, 0
}
