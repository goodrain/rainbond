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

// 该文件实现了与第三方服务交互的功能，主要用于在Rainbond平台中管理和监控外部服务的端点信息。
// 这些外部服务可能是静态的（即端点信息固定不变），也可能是动态的（即端点信息会随时变化）。
// 文件中提供了两种类型的接口：静态交互器和动态交互器，它们分别处理静态和动态服务端点。

// 文件的主要内容包括：
// 1. `Interacter` 接口：定义了与数据库或服务注册中心交互的方法，包括获取服务端点列表和监听端点变化。
// 2. `NewInteracter` 方法：根据传入的服务ID创建一个新的 `Interacter` 实例。如果服务端点是动态的，
//    它将返回一个动态交互器实例；否则，它将返回一个静态交互器实例。
// 3. `static` 结构体：实现了 `Interacter` 接口，处理静态服务的端点信息。静态服务的端点信息存储在数据库中，
//    该结构体从数据库中获取服务的端点列表。
// 4. `dynamic` 结构体：实现了 `Interacter` 接口，处理动态服务的端点信息。动态服务的端点信息存储在服务发现中心（如Etcd）中，
//    该结构体通过 `discovery` 包与服务发现中心进行交互，获取和监控服务端点的变化。
// 5. `Watch` 方法：动态交互器中的方法，负责监听服务端点的变化。当检测到服务端点变化时，
//    它会通过通道通知其他组件进行处理。

// 总的来说，该文件为Rainbond平台提供了一个灵活的机制来管理和监控外部服务的端点信息，
// 无论这些服务是静态的还是动态的，都可以通过相应的接口进行有效的管理和监控。

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
			UUID: ep.UUID,
			Sid:  ep.ServiceID,
			IP:   ep.IP,
			Port: ep.Port,
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
