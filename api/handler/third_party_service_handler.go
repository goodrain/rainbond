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
	"strconv"
	"strings"

	"github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/util"
	"github.com/sirupsen/logrus"

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
	address, port := convertAddressPort(d.Address)
	if port == 0 {
		//set default port by service port
		ports, _ := t.dbmanager.TenantServicesPortDao().GetPortsByServiceID(sid)
		if len(ports) > 0 {
			port = ports[0].ContainerPort
		}
	}
	ep := &dbmodel.Endpoint{
		UUID:      util.NewUUID(),
		ServiceID: sid,
		IP:        address,
		Port:      port,
		IsOnline:  &d.IsOnline,
	}
	if err := t.dbmanager.EndpointsDao().AddModel(ep); err != nil {
		return err
	}

	logrus.Debugf("add new endpoint[address: %s, port: %d]", address, port)
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
	if d.Address != "" {
		address, port := convertAddressPort(d.Address)
		ep.IP = address
		ep.Port = port
	}
	ep.IsOnline = &d.IsOnline
	if err := t.dbmanager.EndpointsDao().UpdateModel(ep); err != nil {
		return err
	}

	t.statusCli.UpdThirdPartyEndpoint(ep)

	return nil
}

func convertAddressPort(s string) (address string, port int) {
	prefix := ""
	if strings.HasPrefix(s, "https://") {
		s = strings.Split(s, "https://")[1]
		prefix = "https://"
	}
	if strings.HasPrefix(s, "http://") {
		s = strings.Split(s, "http://")[1]
		prefix = "http://"
	}

	if strings.Contains(s, ":") {
		sp := strings.Split(s, ":")
		address = prefix + sp[0]
		port, _ = strconv.Atoi(sp[1])
	} else {
		address = prefix + s
	}

	return address, port
}

// DelEndpoints deletes endpoints for third-party service.
func (t *ThirdPartyServiceHanlder) DelEndpoints(epid, sid string) error {
	ep, err := t.dbmanager.EndpointsDao().GetByUUID(epid)
	if err != nil {
		logrus.Warningf("EpID: %s; error getting endpoints: %v", epid, err)
		return err
	}
	if err := t.dbmanager.EndpointsDao().DelByUUID(epid); err != nil {
		return err
	}
	t.statusCli.DelThirdPartyEndpoint(ep)

	return nil
}

// ListEndpoints lists third-party service endpoints.
func (t *ThirdPartyServiceHanlder) ListEndpoints(sid string) ([]*model.EndpointResp, error) {
	endpoints, err := t.dbmanager.EndpointsDao().List(sid)
	if err != nil {
		logrus.Warningf("ServiceID: %s; error listing endpoints from db; %v", sid, err)
	}
	m := make(map[string]*model.EndpointResp)
	for _, item := range endpoints {
		ep := &model.EndpointResp{
			EpID: item.UUID,
			Address: func(ip string, p int) string {
				if p != 0 {
					return fmt.Sprintf("%s:%d", ip, p)
				}
				return ip
			}(item.IP, item.Port),
			Status:   "-",
			IsOnline: false,
			IsStatic: true,
		}
		m[ep.Address] = ep
	}
	thirdPartyEndpoints, err := t.statusCli.ListThirdPartyEndpoints(sid)
	if err != nil {
		logrus.Warningf("ServiceID: %s; grpc; error listing third-party endpoints: %v", sid, err)
		return nil, err
	}
	if thirdPartyEndpoints != nil && thirdPartyEndpoints.Obj != nil {
		for _, item := range thirdPartyEndpoints.Obj {
			ep := m[fmt.Sprintf("%s:%d", item.Ip, item.Port)]
			if ep != nil {
				ep.IsOnline = true
				ep.Status = item.Status
				continue
			}
			rep := &model.EndpointResp{
				EpID:     item.Uuid,
				Address:  item.Ip,
				Status:   item.Status,
				IsOnline: true,
				IsStatic: false,
			}
			m[rep.Address] = rep
		}
	}
	var res []*model.EndpointResp
	for _, item := range m {
		res = append(res, item)
	}
	return res, nil
}
