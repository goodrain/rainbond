
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
	"github.com/goodrain/rainbond/pkg/node/api/controller"
	"github.com/goodrain/rainbond/pkg/node/api/router"
	"net/http"

	"github.com/goodrain/rainbond/cmd/node/option"
	"context"

	"github.com/Sirupsen/logrus"
	"github.com/go-chi/chi"
)

//Manager api manager
type Manager struct {
	ctx    context.Context
	cancel context.CancelFunc
	conf   option.Conf
	router *chi.Mux
}

//NewManager api manager
func NewManager(c option.Conf) *Manager {
	r := router.Routers(c.RunMode)
	ctx, cancel := context.WithCancel(context.Background())
	controller.Init(&c)
	return &Manager{
		ctx:    ctx,
		cancel: cancel,
		conf:   c,
		router: r,
	}
}

//Start 启动
func (m *Manager) Start(errChan chan error) {
	logrus.Infof("api server start listening on %s", m.conf.APIAddr)
	//m.prometheus()
	go func() {
		if err := http.ListenAndServe(m.conf.APIAddr, m.router); err != nil {
			logrus.Error("entrance api listen error.", err.Error())
			errChan <- err
		}
	}()
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
