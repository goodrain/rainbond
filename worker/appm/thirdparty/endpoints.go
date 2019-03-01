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

package thirdparty

import (
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/util"
	"github.com/goodrain/rainbond/worker/appm/thirdparty/discovery"
	"github.com/goodrain/rainbond/worker/appm/types/v1"
)

// ListEndpoints lists third-party endpoints.
func ListEndpoints(sid string, dbm db.Manager) ([]*model.Endpoint, error) {
	// dynamic endpoints
	cfg, err := dbm.ThirdPartySvcDiscoveryCfgDao().GetByServiceID(sid)
	if err != nil {
		return nil, err
	}
	if cfg != nil {
		d := discovery.NewDiscoverier(cfg)
		if err := d.Connect(); err != nil {
			return nil, err
		}
		defer d.Close()
		endpoints, err := d.Fetch()
		if err != nil {
			return nil, err
		}
		if endpoints == nil && len(endpoints) == 0 {
			return nil, fmt.Errorf("error not found endpoints")
		}
		return endpoints, nil
	}
	// static endpoints
	endpoints, err := dbm.EndpointsDao().ListIsOnline(sid)
	if err != nil {
		return nil, err
	}
	if endpoints == nil || len(endpoints) == 0 {
		return nil, fmt.Errorf("error not found endpoints")
	}
	return endpoints, nil
}

// AddEndpoints create a new endpoint.
func AddEndpoints(sid string, req v1.AddEndpointReq, dbm db.Manager) error {
	// dynamic endpoints
	cfg, err := dbm.ThirdPartySvcDiscoveryCfgDao().GetByServiceID(sid)
	if err != nil {
		return err
	}
	if cfg != nil {
		d := discovery.NewDiscoverier(cfg)
		if err := d.Connect(); err != nil {
			return err
		}
		defer d.Close()
		if d.Add(dbm, req) != nil {
			return err
		}
		return nil
	}
	ep := &model.Endpoint{
		UUID:      util.NewUUID(),
		ServiceID: sid,
		IP:        req.IP,
		IsOnline:  &req.IsOnline,
	}
	if err := dbm.EndpointsDao().AddModel(ep); err != nil {
		logrus.Errorf("error creating endpoint record: %v", err)
		return err
	}
	return nil
}

// UpdEndpoints -
func UpdEndpoints(sid string, req v1.UpdEndpointReq, dbm db.Manager) error {
	// dynamic endpoints
	cfg, err := dbm.ThirdPartySvcDiscoveryCfgDao().GetByServiceID(sid)
	if err != nil {
		return err
	}
	if cfg != nil {
		d := discovery.NewDiscoverier(cfg)
		if err := d.Connect(); err != nil {
			return err
		}
		defer d.Close()
		if d.Update(dbm, req) != nil {
			return err
		}
		return nil
	}
	// static endpoints
	ep, err := dbm.EndpointsDao().GetByUUID(req.EpID)
	if err != nil {
		return fmt.Errorf("uuid: %s, error getting endpoint: %v", req.EpID, err)
	}
	if req.IP != "" {
		ep.IP = req.IP
	}
	ep.IsOnline = &req.IsOnline
	if err := dbm.EndpointsDao().UpdateModel(ep); err != nil {
		return fmt.Errorf("uuid: %s, error updating endpoint: %v", req.IP, err)
	}
	return nil
}

// DelEndpoint
func DelEndpoint(dbm db.Manager, sid, epid string) error {
	cfg, err := dbm.ThirdPartySvcDiscoveryCfgDao().GetByServiceID(sid)
	if err != nil {
		return err
	}
	if cfg != nil {
		d := discovery.NewDiscoverier(cfg)
		if err := d.Connect(); err != nil {
			return err
		}
		defer d.Close()
		if err := d.Delete(epid); err != nil {
			return err
		}
		return nil
	}
	if err := dbm.EndpointsDao().DelByUUID(epid); err != nil {
		return fmt.Errorf("uuid: %s, error deleting endpoint: %v", epid, err)
	}
	return nil
}

// Conv Converts model.Endpoints to v1.Endpoints
func Conv(eps []*model.Endpoint) ([]*v1.Endpoint, error) {
	var res []*v1.Endpoint
	m := make(map[int]*v1.Endpoint)
	for _, ep := range eps {
		v1ep, ok := m[ep.Port] // the value of port may be 0
		if ok {
			v1ep.IPs = append(v1ep.IPs, ep.IP)
			continue
		}
		v1ep = &v1.Endpoint{
			Port: ep.Port,
			IPs: []string{
				ep.IP,
			},
		}
		m[ep.Port] = v1ep
		res = append(res, v1ep)
	}
	// TODO: If the port has three different values, one of them cannot be 0
	return res, nil
}
