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

package discover

import (
	"sync"

	"github.com/goodrain/rainbond/discover.v2/config"
)

//CallbackUpdate 每次返还变化
type CallbackUpdate interface {
	//TODO:
	//weight自动发现更改实现暂时不 Ready
	UpdateEndpoints(operation config.Operation, endpoints ...*config.Endpoint)
	//when watch occurred error,will exec this method
	Error(error)
}

//Callback 每次返回全部节点
type Callback interface {
	UpdateEndpoints(endpoints ...*config.Endpoint)
	//when watch occurred error,will exec this method
	Error(error)
}

//Discover 后端服务自动发现
type Discover interface {
	// Add project to cache if not exists, then watch the endpoints.
	AddProject(name string, callback Callback)
	// Update a project.
	AddUpdateProject(name string, callback CallbackUpdate)
	Stop()
}
type defaultCallBackUpdate struct {
	endpoints map[string]*config.Endpoint
	callback  Callback
	lock      sync.Mutex
}

func (d *defaultCallBackUpdate) UpdateEndpoints(operation config.Operation, endpoints ...*config.Endpoint) {
	d.lock.Lock()
	defer d.lock.Unlock()
	switch operation {
	case config.ADD:
		for _, e := range endpoints {
			if old, ok := d.endpoints[e.Name]; !ok {
				d.endpoints[e.Name] = e
			} else {
				if e.Mode == 0 {
					old.URL = e.URL
				}
				if e.Mode == 1 {
					old.Weight = e.Weight
				}
				if e.Mode == 2 {
					old.URL = e.URL
					old.Weight = e.Weight
				}
			}
		}
	case config.SYNC:
		for _, e := range endpoints {
			if old, ok := d.endpoints[e.Name]; !ok {
				d.endpoints[e.Name] = e
			} else {
				if e.Mode == 0 {
					old.URL = e.URL
				}
				if e.Mode == 1 {
					old.Weight = e.Weight
				}
				if e.Mode == 2 {
					old.URL = e.URL
					old.Weight = e.Weight
				}
			}
		}
	case config.DELETE:
		for _, e := range endpoints {
			if e.Mode == 0 {
				if old, ok := d.endpoints[e.Name]; ok {
					old.URL = ""
				}
			}
			if e.Mode == 1 {
				if old, ok := d.endpoints[e.Name]; ok {
					old.Weight = 0
				}
			}
			if e.Mode == 2 {
				if _, ok := d.endpoints[e.Name]; ok {
					delete(d.endpoints, e.Name)
				}
			}
		}
	case config.UPDATE:
		for _, e := range endpoints {
			if e.Mode == 0 {
				if old, ok := d.endpoints[e.Name]; ok {
					old.URL = e.URL
				}
			}
			if e.Mode == 1 {
				if old, ok := d.endpoints[e.Name]; ok {
					old.Weight = e.Weight
				}
			}
			if e.Mode == 2 {
				if old, ok := d.endpoints[e.Name]; ok {
					old.URL = e.URL
					old.Weight = e.Weight
				}
			}
		}
	}
	var re []*config.Endpoint
	for _, v := range d.endpoints {
		if v.URL != "" {
			re = append(re, v)
		}
	}
	d.callback.UpdateEndpoints(re...)
}

func (d *defaultCallBackUpdate) Error(err error) {
	d.callback.Error(err)
}
