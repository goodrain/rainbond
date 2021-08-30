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

package monitor

import (
	"context"
	"net/http"

	"github.com/goodrain/rainbond/worker/master"

	"github.com/goodrain/rainbond/cmd/worker/option"
	httputil "github.com/goodrain/rainbond/util/http"
	"github.com/goodrain/rainbond/worker/appm/controller"
	"github.com/goodrain/rainbond/worker/discover"
	"github.com/goodrain/rainbond/worker/monitor/collector"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

//ExporterManager app resource exporter
type ExporterManager struct {
	ctx               context.Context
	cancel            context.CancelFunc
	config            option.Config
	stopChan          chan struct{}
	masterController  *master.Controller
	controllermanager *controller.Manager
}

//NewManager return *NewManager
func NewManager(c option.Config, masterController *master.Controller, controllermanager *controller.Manager) *ExporterManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &ExporterManager{
		ctx:               ctx,
		cancel:            cancel,
		config:            c,
		stopChan:          make(chan struct{}),
		masterController:  masterController,
		controllermanager: controllermanager,
	}
}
func (t *ExporterManager) handler(w http.ResponseWriter, r *http.Request) {
	registry := prometheus.NewRegistry()
	registry.MustRegister(collector.New(t.masterController, t.controllermanager))

	gatherers := prometheus.Gatherers{
		prometheus.DefaultGatherer,
		registry,
	}
	// Delegate http serving to Prometheus client library, which will call collector.Collect.
	h := promhttp.HandlerFor(gatherers, promhttp.HandlerOpts{})
	h.ServeHTTP(w, r)
}

//Start 启动
func (t *ExporterManager) Start() error {
	http.HandleFunc(t.config.PrometheusMetricPath, t.handler)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
			<head><title>Worker exporter</title></head>
			<body>
			<h1>Worker exporter</h1>
			<p><a href='` + t.config.PrometheusMetricPath + `'>Metrics</a></p>
			</body>
			</html>
			`))
	})
	http.HandleFunc("/worker/health", func(w http.ResponseWriter, r *http.Request) {
		healthStatus := discover.HealthCheck()
		if healthStatus["status"] != "health" {
			httputil.ReturnError(r, w, 400, "worker service unusual")
		}
		httputil.ReturnSuccess(r, w, healthStatus)
	})
	logrus.Infoln("Listening on", t.config.Listen)
	go func() {
		logrus.Fatal(http.ListenAndServe(t.config.Listen, nil))
	}()
	logrus.Info("start app resource exporter success.")
	return nil
}

//Stop 停止
func (t *ExporterManager) Stop() {
	t.cancel()
}
