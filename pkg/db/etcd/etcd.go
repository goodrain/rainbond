
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

package etcd

import (
	"github.com/goodrain/rainbond/pkg/db/config"
	"github.com/goodrain/rainbond/pkg/db/model"
	"sync"
	"time"

	etcdutil "github.com/goodrain/rainbond/pkg/util/etcd"

	"github.com/Sirupsen/logrus"
	"github.com/coreos/etcd/clientv3"
)

//Manager db manager
type Manager struct {
	client  *clientv3.Client
	config  config.Config
	initOne sync.Once
	models  []model.Interface
}

//CreateManager 创建manager
func CreateManager(config config.Config) (*Manager, error) {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   config.EtcdEndPoints,
		DialTimeout: time.Duration(config.EtcdTimeout) * time.Second,
	})
	if err != nil {
		etcdutil.HandleEtcdError(err)
		return nil, err
	}
	manager := &Manager{
		client:  cli,
		config:  config,
		initOne: sync.Once{},
	}
	logrus.Debug("etcd db driver create")
	return manager, nil
}

//CloseManager 关闭管理器
func (m *Manager) CloseManager() error {
	return m.client.Close()
}
