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
	"context"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/cmd/node-proxy/option"
	"github.com/goodrain/rainbond/node/api/controller"
	"github.com/goodrain/rainbond/node/api/router"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"

	// pprof
	_ "net/http/pprof"
)

//Manager api manager
type Manager struct {
	ctx    context.Context
	cancel context.CancelFunc
	conf   option.Conf
	router *chi.Mux
}

//NewManager api manager
func NewManager(c option.Conf, kubecli *kubernetes.Clientset) *Manager {
	r := router.Routers()
	ctx, cancel := context.WithCancel(context.Background())
	controller.Init(&c, kubecli)
	m := &Manager{
		ctx:    ctx,
		cancel: cancel,
		conf:   c,
		router: r,
	}
	// set node cluster monitor route
	m.router.Get("/cluster/metrics", m.HandleClusterScrape)
	return m
}

//Start 启动
func (m *Manager) Start(errChan chan error) error {
	logrus.Infof("api server start listening on %s", m.conf.APIAddr)
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
	return nil
}

//Stop 停止
func (m *Manager) Stop() error {
	logrus.Info("api server is stoping.")
	m.cancel()
	return nil
}

//GetRouter GetRouter
func (m *Manager) GetRouter() *chi.Mux {
	return m.router
}

//HandleClusterScrape prometheus handle
func (m *Manager) HandleClusterScrape(w http.ResponseWriter, r *http.Request) {
	gatherers := prometheus.Gatherers{
		prometheus.DefaultGatherer,
	}
	// Delegate http serving to Prometheus client library, which will call collector.Collect.
	h := promhttp.HandlerFor(gatherers,
		promhttp.HandlerOpts{
			ErrorLog:      logrus.StandardLogger(),
			ErrorHandling: promhttp.ContinueOnError,
		})
	h.ServeHTTP(w, r)
}
