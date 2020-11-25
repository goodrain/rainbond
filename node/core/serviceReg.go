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

package core

import (
	"github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/node/core/store"
	"sync"

	"github.com/sirupsen/logrus"
)

type etcdRegistrar struct {
	//projects map[string]CallbackUpdate
	lock   sync.Mutex
	client *store.Client
	prefix string
}

//GetRegistrar GetRegistrar
func GetRegistrar() *etcdRegistrar {
	return &etcdRegistrar{
		prefix: option.Config.Service,
		client: store.DefalutClient,
	}
}
func (r *etcdRegistrar) RegService(serviceName, hostname, url string) error {
	r.lock.Lock()
	_, err := r.client.Put(r.getPath(serviceName, hostname), url)
	if err != nil {
		logrus.Infof("reg service %s to path %s failed,details %s", serviceName, r.getPath(serviceName, hostname), err.Error())
		return err
	}
	r.lock.Unlock()
	return nil
}
func (r *etcdRegistrar) RemoveService(serviceName, hostname string) error {
	r.lock.Lock()
	_, err := r.client.Delete(r.getPath(serviceName, hostname))
	if err != nil {
		logrus.Infof("del  service %s from path %s failed,details %s", serviceName, r.getPath(serviceName, hostname), err.Error())
		return err
	}
	r.lock.Unlock()
	return nil
}
func (r *etcdRegistrar) getPath(serviceName, hostName string) string {
	return r.prefix + serviceName + "/servers/" + hostName + "/url"
}
