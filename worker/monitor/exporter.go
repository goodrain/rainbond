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

// 本文件实现了一个用于导出应用资源监控指标的管理器。该管理器使用 Prometheus 作为监控系统，通过 HTTP 服务提供应用资源的监控数据。

// 1. `ExporterManager` 结构体：
//    - 该结构体包含多个字段，用于管理监控服务的上下文、配置、控制器以及停止信号通道等。
//    - `ctx` 和 `cancel`：上下文和取消函数，用于管理服务的生命周期。
//    - `config`：配置选项，包含了 Prometheus 相关的配置信息。
//    - `stopChan`：停止信号通道，用于控制服务的停止。
//    - `masterController` 和 `controllermanager`：Rainbond 系统中的控制器，用于收集和管理应用资源的数据。

// 2. `NewManager` 函数：
//    - 该函数用于创建并初始化一个 `ExporterManager` 实例。
//    - 接受配置参数、主控制器以及控制器管理器作为输入，返回一个新的 `ExporterManager` 实例。

// 3. `handler` 方法：
//    - 该方法是 HTTP 请求的处理函数，用于响应 Prometheus 的监控数据采集请求。
//    - 它创建了一个新的 Prometheus 注册表，并注册了自定义的采集器，用于收集应用资源的指标数据。
//    - 最后通过 Prometheus 客户端库提供的 HTTP 处理程序来响应监控数据请求。

// 4. `Start` 方法：
//    - 该方法启动 HTTP 服务，并提供了三个 HTTP 处理路径：
//      - `t.config.PrometheusMetricPath`：用于返回 Prometheus 格式的监控指标数据。
//      - `/`：一个简单的欢迎页面，包含链接到监控指标数据页面。
//      - `/worker/health`：用于检查服务的健康状态，并返回健康检查的结果。
//    - 方法通过调用 `http.ListenAndServe` 开启 HTTP 服务，并开始监听配置中指定的端口。

// 5. `Stop` 方法：
//    - 该方法用于停止监控服务，取消上下文并释放资源。
//    - 通过调用 `t.cancel()`，可以停止正在运行的 HTTP 服务和所有关联的操作。

// 总体而言，本文件实现了一个简单而有效的监控管理器，能够为 Rainbond 系统中的应用资源提供实时的监控指标，并通过 Prometheus 进行采集和展示。

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

// ExporterManager app resource exporter
type ExporterManager struct {
	ctx               context.Context
	cancel            context.CancelFunc
	config            option.Config
	stopChan          chan struct{}
	masterController  *master.Controller
	controllermanager *controller.Manager
}

// NewManager return *NewManager
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

// Start 启动
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

// Stop 停止
func (t *ExporterManager) Stop() {
	t.cancel()
}
