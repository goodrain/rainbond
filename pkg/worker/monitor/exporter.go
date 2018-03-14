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

	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/cmd/worker/option"
	"github.com/goodrain/rainbond/pkg/status"
	"github.com/goodrain/rainbond/pkg/worker/monitor/cache"
	"github.com/goodrain/rainbond/pkg/worker/monitor/collector"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"
)

//ExporterManager app resource exporter
type ExporterManager struct {
	ctx           context.Context
	cancel        context.CancelFunc
	config        option.Config
	stopChan      chan struct{}
	statusManager status.ServiceStatusManager
	cache         *cache.DiskCache
}

//NewManager return *NewManager
func NewManager(c option.Config, statusManager status.ServiceStatusManager) *ExporterManager {
	ctx, cancel := context.WithCancel(context.Background())
	cache := cache.CreatDiskCache(ctx, statusManager)
	return &ExporterManager{
		ctx:           ctx,
		cancel:        cancel,
		config:        c,
		stopChan:      make(chan struct{}),
		statusManager: statusManager,
		cache:         cache,
	}
}
func (t *ExporterManager) handler(w http.ResponseWriter, r *http.Request) {
	registry := prometheus.NewRegistry()
	registry.MustRegister(collector.New(t.statusManager, t.cache))

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
	http.HandleFunc(t.config.PrometheusMetricPath, prometheus.InstrumentHandlerFunc("metrics", t.handler))
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
	go t.cache.Start()
	log.Infoln("Listening on", t.config.Listen)
	go func() {
		log.Fatal(http.ListenAndServe(t.config.Listen, nil))
	}()
	logrus.Info("start app resource exporter success.")
	return nil
}

//Stop 停止
func (t *ExporterManager) Stop() {
	t.cancel()
}
