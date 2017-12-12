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

package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/goodrain/rainbond/pkg/node/masterserver"

	"github.com/goodrain/rainbond/pkg/node/api/controller"
	"github.com/goodrain/rainbond/pkg/node/api/model"
	"github.com/goodrain/rainbond/pkg/node/api/router"
	"github.com/goodrain/rainbond/pkg/node/core/store"

	"context"
	"strings"

	"github.com/goodrain/rainbond/cmd/node/option"

	_ "net/http/pprof"

	"github.com/Sirupsen/logrus"
	client "github.com/coreos/etcd/clientv3"
	"github.com/go-chi/chi"
)

//Manager api manager
type Manager struct {
	ctx    context.Context
	cancel context.CancelFunc
	conf   option.Conf
	router *chi.Mux
	node   *model.HostNode
	lID    client.LeaseID // lease id
	ms     *masterserver.MasterServer
}

//NewManager api manager
func NewManager(c option.Conf, node *model.HostNode, ms *masterserver.MasterServer) *Manager {
	r := router.Routers(c.RunMode)
	ctx, cancel := context.WithCancel(context.Background())
	controller.Init(&c, ms)
	return &Manager{
		ctx:    ctx,
		cancel: cancel,
		conf:   c,
		router: r,
		node:   node,
		ms:     ms,
	}
}

//Start 启动
func (m *Manager) Start(errChan chan error) {
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
		go m.keepAlive()
	}
}
func (m *Manager) set() error {
	key := fmt.Sprintf("/traefik/backends/acp_node/servers/%s/url", m.node.ID)
	portinfo := strings.Split(m.conf.APIAddr, ":")
	var port string
	if len(portinfo) != 2 {
		port = "6100"
	} else {
		port = portinfo[1]
	}
	value := fmt.Sprintf("%s:%s", m.node.InternalIP, port)
	resp, err := store.DefalutClient.Grant(7)
	if err != nil {
		return err
	}
	if _, err = store.DefalutClient.Put(key, value, client.WithLease(resp.ID)); err != nil {
		return err
	}
	m.lID = resp.ID
	return nil
}
func (m *Manager) keepAlive() {
	duration := time.Duration(5) * time.Second
	timer := time.NewTimer(duration)
	for {
		select {
		case <-m.ctx.Done():

			return
		case <-timer.C:
			if m.lID > 0 {
				_, err := store.DefalutClient.KeepAliveOnce(m.lID)
				if err == nil {
					timer.Reset(duration)
					continue
				}
				logrus.Warnf("%s lid[%x] keepAlive err: %s, try to reset...", "rainbond node api", m.lID, err.Error())
				m.lID = 0
			}
			if err := m.set(); err != nil {
				logrus.Warnf("%s set lid err: %s, try to reset after 5 seconds...", "rainbond node api", err.Error())
			} else {
				logrus.Infof("%s set lid[%x] success", "rainbond node api", m.lID)
			}
			timer.Reset(duration)
		}
	}
}

//Stop 停止
func (m *Manager) Stop() error {
	logrus.Info("api server is stoping.")
	m.cancel()
	return nil
}

func (m *Manager) prometheus() {
	//prometheus.MustRegister(version.NewCollector("acp_node"))
	// exporter := monitor.NewExporter(m.coreManager)
	// prometheus.MustRegister(exporter)

	//todo 我注释的
	//m.container.Handle(m.conf.PrometheusMetricPath, promhttp.Handler())
}
