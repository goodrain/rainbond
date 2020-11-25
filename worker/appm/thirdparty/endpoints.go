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
	"github.com/eapache/channels"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/worker/appm/thirdparty/discovery"
	v1 "github.com/goodrain/rainbond/worker/appm/types/v1"
	"github.com/sirupsen/logrus"
)

// Interacter is the interface that wraps the required methods to interact
// with DB or service registry that holds the endpoints information.
type Interacter interface {
	List() ([]*v1.RbdEndpoint, error)
	// if endpoints type is static, do nothing.
	// if endpoints type is dynamic, watch the changes in endpoints.
	Watch()
}

// NewInteracter creates a new Interacter.
func NewInteracter(sid string, updateCh *channels.RingChannel, stopCh chan struct{}) Interacter {
	cfg, err := db.GetManager().ThirdPartySvcDiscoveryCfgDao().GetByServiceID(sid)
	if err != nil {
		logrus.Warningf("ServiceID: %s;error getting third-party discovery configuration"+
			": %s", sid, err.Error())
	}
	if err == nil && cfg != nil {
		d := &dynamic{
			cfg:      cfg,
			updateCh: updateCh,
			stopCh:   stopCh,
		}
		return d
	}
	return &static{
		sid: sid,
	}
}

// NewStaticInteracter creates a new static interacter.
func NewStaticInteracter(sid string) Interacter {
	return &static{
		sid: sid,
	}
}

type static struct {
	sid string
}

func (s *static) List() ([]*v1.RbdEndpoint, error) {
	eps, err := db.GetManager().EndpointsDao().List(s.sid)
	if err != nil {
		return nil, err
	}
	var res []*v1.RbdEndpoint
	for _, ep := range eps {
		res = append(res, &v1.RbdEndpoint{
			UUID:     ep.UUID,
			Sid:      ep.ServiceID,
			IP:       ep.IP,
			Port:     ep.Port,
			IsOnline: *ep.IsOnline,
		})
	}
	return res, nil
}

func (s *static) Watch() {
	// do nothing
}

// NewDynamicInteracter creates a new static interacter.
func NewDynamicInteracter(sid string, updateCh *channels.RingChannel, stopCh chan struct{}) Interacter {
	cfg, err := db.GetManager().ThirdPartySvcDiscoveryCfgDao().GetByServiceID(sid)
	if err != nil {
		logrus.Warningf("ServiceID: %s;error getting third-party discovery configuration"+
			": %s", sid, err.Error())
		return nil
	}
	if cfg == nil {
		return nil
	}
	d := &dynamic{
		cfg:      cfg,
		updateCh: updateCh,
		stopCh:   stopCh,
	}
	return d

}

type dynamic struct {
	cfg *model.ThirdPartySvcDiscoveryCfg

	updateCh *channels.RingChannel
	stopCh   chan struct{}
}

func (d *dynamic) List() ([]*v1.RbdEndpoint, error) {
	discoverier, err := discovery.NewDiscoverier(d.cfg, d.updateCh, d.stopCh)
	if err != nil {
		return nil, err
	}
	if err := discoverier.Connect(); err != nil {
		return nil, err
	}
	defer discoverier.Close()
	return discoverier.Fetch()
}

func (d *dynamic) Watch() {
	discoverier, err := discovery.NewDiscoverier(d.cfg, d.updateCh, d.stopCh)
	if err != nil {
		logrus.Warningf("error creating discoverier: %s", err.Error())
		return
	}
	if err := discoverier.Connect(); err != nil {
		logrus.Warningf("error connecting service discovery center: %s", err.Error())
		return
	}
	defer discoverier.Close()
	discoverier.Watch()
}
