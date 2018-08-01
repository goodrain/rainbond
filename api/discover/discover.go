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

	"github.com/goodrain/rainbond/api/proxy"
	corediscover "github.com/goodrain/rainbond/discover"
	corediscoverconfig "github.com/goodrain/rainbond/discover/config"

	"github.com/Sirupsen/logrus"
)

//EndpointDiscover 后端服务自动发现
type EndpointDiscover interface {
	AddProject(name string, proxy proxy.Proxy)
	Remove(name string)
	Stop()
}

var defaultEndpointDiscover EndpointDiscover

//CreateEndpointDiscover create endpoint discover
func CreateEndpointDiscover(etcdEndpoints []string) (EndpointDiscover, error) {
	if etcdEndpoints == nil {
		etcdEndpoints = []string{"127.0.0.1:2379"}
	}
	if defaultEndpointDiscover == nil {
		dis, err := corediscover.GetDiscover(corediscoverconfig.DiscoverConfig{
			EtcdClusterEndpoints: etcdEndpoints,
		})
		if err != nil {
			return nil, err
		}
		defaultEndpointDiscover = &endpointDiscover{
			dis:      dis,
			projects: make(map[string]*defalt),
		}
	}
	return defaultEndpointDiscover, nil
}

//GetEndpointDiscover 获取endpointsdiscover
func GetEndpointDiscover(etcdEndpoints []string) EndpointDiscover {
	return defaultEndpointDiscover
}

type endpointDiscover struct {
	projects map[string]*defalt
	lock     sync.Mutex
	dis      corediscover.Discover
}

func (e *endpointDiscover) AddProject(name string, pro proxy.Proxy) {
	e.lock.Lock()
	defer e.lock.Unlock()
	if def, ok := e.projects[name]; !ok {
		e.projects[name] = &defalt{name: name, proxys: []proxy.Proxy{pro}}
		e.dis.AddProject(name, e.projects[name])
	} else {
		def.proxys = append(def.proxys, pro)
	}

}
func (e *endpointDiscover) Remove(name string) {
	e.lock.Lock()
	defer e.lock.Unlock()
	if _, ok := e.projects[name]; ok {
		delete(e.projects, name)
	}
}
func (e *endpointDiscover) Stop() {
	e.dis.Stop()
}

type defalt struct {
	name   string
	proxys []proxy.Proxy
}

func (e *defalt) Error(err error) {
	logrus.Errorf("%s project auto discover occurred error.%s", e.name, err.Error())
	defaultEndpointDiscover.Remove(e.name)
}

func (e *defalt) UpdateEndpoints(endpoints ...*corediscoverconfig.Endpoint) {
	var endStr []string
	for _, end := range endpoints {
		if end.URL != "" {
			endStr = append(endStr, end.Name + "=>" + end.URL)
		}
	}
	logrus.Debugf("endstr is %v, name is %v", endStr, e.name)
	for _, p := range e.proxys {
		p.UpdateEndpoints(endStr...)
	}
}

//when watch occurred error,will exec this method
func (e *endpointDiscover) Error(err error) {
	logrus.Errorf("discover error %s", err.Error())
}
