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
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/worker/appm/thirdparty/discovery"
	"github.com/goodrain/rainbond/worker/appm/types/v1"
)

// ListEndpoints lists third-party endpoints.
func ListEndpoints(sid string, dbm db.Manager) ([]*v1.Endpoint, error) {
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
		return conv(endpoints)
	}
	// static endpoints
	endpoints, err := dbm.EndpointsDao().ListIsOnline(sid)
	if err != nil {
		return nil, err
	}
	if endpoints == nil || len(endpoints) == 0 {
		return nil, fmt.Errorf("error not found endpoints")
	}
	return conv(endpoints)
}

func conv(eps []*model.Endpoint) ([]*v1.Endpoint, error) {
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
