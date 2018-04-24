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

package sync

import (
	"github.com/goodrain/rainbond/entrance/core"
	"github.com/goodrain/rainbond/entrance/source"
	"github.com/goodrain/rainbond/entrance/store"

	"github.com/Sirupsen/logrus"
	"github.com/coreos/etcd/client"
)

//Manaer data sync manager
type Manaer struct {
	sourceManager *source.Manager
	storeManager  *store.Manager
	coreManager   core.Manager
}

//NewManager new data sync manager
func NewManager(sourceManager *source.Manager, storeManager *store.Manager, coreManager core.Manager) *Manaer {
	return &Manaer{
		sourceManager: sourceManager,
		storeManager:  storeManager,
		coreManager:   coreManager,
	}
}

//Start start sync
func (m *Manaer) Start() error {
	logrus.Info("Start sync all data")
	//step 1: sync node data
	nodes, err := m.storeManager.GetAllNodes()
	if err != nil && !client.IsKeyNotFound(err) {
		logrus.Error("get all old node data error when sync data.", err.Error())
		return err
	}
	if nodes != nil {
		for _, node := range nodes {
			if ok := m.sourceManager.NodeIsReady(node); !ok {
				logrus.Infof("Check the Node %s is not the latest,will delete it.", node.NodeName)
				m.coreManager.EventChan() <- core.Event{Method: core.DELETEEventMethod, Source: node}
			}
		}
	}
	//step 2: sync pool data
	pools, err := m.storeManager.GetAllPools()
	if err != nil && !client.IsKeyNotFound(err) {
		logrus.Error("get all old pool data error when sync data.", err.Error())
		return err
	}
	if pools != nil {
		for _, pool := range pools {
			if ok := m.sourceManager.PoolIsReady(pool); !ok {
				logrus.Infof("Check the pool %s is not the latest,will delete it.", pool.Name)
				m.coreManager.EventChan() <- core.Event{Method: core.DELETEEventMethod, Source: pool}
			}
		}
	}

	//step 3: sync vs data
	vs, err := m.storeManager.GetAllVSs()
	if err != nil && !client.IsKeyNotFound(err) {
		logrus.Error("get all old vs data error when sync data.", err.Error())
		return err
	}
	if vs != nil {
		for _, v := range vs {
			if ok := m.sourceManager.VSIsReady(v); !ok {
				logrus.Infof("Check the VS %s is not the latest,will delete it.", v.Name)
				m.coreManager.EventChan() <- core.Event{Method: core.DELETEEventMethod, Source: v}
			}
		}
	}
	//step 4 :TODO: 验证rule
	logrus.Info("all data sync success")
	return nil
}
