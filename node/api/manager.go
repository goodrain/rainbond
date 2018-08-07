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

package api

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/goodrain/rainbond/discover"
	"github.com/goodrain/rainbond/node/kubecache"
	"github.com/goodrain/rainbond/node/masterserver"
	"github.com/goodrain/rainbond/node/statsd"

	"github.com/goodrain/rainbond/node/api/controller"
	"github.com/goodrain/rainbond/node/api/router"

	"context"
	"strings"

	"github.com/goodrain/rainbond/cmd/node/option"
	nodeclient "github.com/goodrain/rainbond/node/nodem/client"

	_ "net/http/pprof"

	"github.com/Sirupsen/logrus"
	client "github.com/coreos/etcd/clientv3"
	"github.com/go-chi/chi"
)

//Manager api manager
type Manager struct {
	ctx       context.Context
	cancel    context.CancelFunc
	conf      option.Conf
	router    *chi.Mux
	node      *nodeclient.HostNode
	lID       client.LeaseID // lease id
	ms        *masterserver.MasterServer
	keepalive *discover.KeepAlive
	exporter  *statsd.Exporter
}

//NewManager api manager
func NewManager(c option.Conf, node *nodeclient.HostNode, ms *masterserver.MasterServer, kubecli kubecache.KubeClient) *Manager {
	r := router.Routers(c.RunMode)
	ctx, cancel := context.WithCancel(context.Background())
	controller.Init(&c, ms, kubecli)
	m := &Manager{
		ctx:    ctx,
		cancel: cancel,
		conf:   c,
		router: r,
		node:   node,
		ms:     ms,
	}
	return m
}

//Start 启动
func (m *Manager) Start(errChan chan error) error {
	logrus.Infof("api server start listening on %s", m.conf.APIAddr)
	//m.prometheus()

	go func() {
		if err := http.ListenAndServe(m.conf.APIAddr, m.router); err != nil {
			logrus.Error("rainbond node api listen error.", err.Error())
			errChan <- err
		}
	}()
	go func() {
		if err := http.ListenAndServe(":6102", nil); err != nil {
			logrus.Error("rainbond node debug api listen error.", err.Error())
			errChan <- err
		}
	}()
	if m.conf.RunMode == "master" {
		portinfo := strings.Split(m.conf.APIAddr, ":")
		var port int
		if len(portinfo) != 2 {
			port = 6100
		} else {
			var err error
			port, err = strconv.Atoi(portinfo[1])
			if err != nil {
				return fmt.Errorf("get the api port info error.%s", err.Error())
			}
		}
		keepalive, err := discover.CreateKeepAlive(m.conf.Etcd.Endpoints, "acp_node", m.node.HostName, m.node.InternalIP, port)
		if err != nil {
			return err
		}
		if err := keepalive.Start(); err != nil {
			return err
		}
	}
	return nil
}

//Stop 停止
func (m *Manager) Stop() error {
	logrus.Info("api server is stoping.")
	m.cancel()
	if m.keepalive != nil {
		m.keepalive.Stop()
	}
	return nil
}

//GetRouter GetRouter
func (m *Manager) GetRouter() *chi.Mux {
	return m.router
}
