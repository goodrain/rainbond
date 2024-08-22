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

// 本文件定义了一个名为 Exporter 的收集器，用于在 Rainbond 平台上收集应用程序的运行状态和性能指标，
// 并通过 Prometheus 监控系统进行暴露。这个收集器集成了 Prometheus 的接口，能够将 Worker 组件的各类运行信息作为指标进行上报。

// 1. **Exporter 结构体**：
//    - `Exporter` 是一个 Prometheus 的收集器实现，它包含了多个 Gauge 和 Counter 类型的指标，
//      这些指标用于跟踪 Worker 组件的状态、任务数量、任务错误数以及存储组件的数量等。
//    - 通过 `dbmanager` 和 `masterController` 等组件，收集器可以获取数据库状态、控制器状态以及健康检查信息。

// 2. **New 函数**：
//    - `New` 函数创建并初始化一个 `Exporter` 实例，设置了多个 Prometheus 指标，用于记录 Worker 的各种状态和操作统计信息。
//    - 这些指标包括总抓取次数、抓取错误次数、当前任务数量、任务错误数和存储组件数量等。

// 3. **Describe 函数**：
//    - `Describe` 函数是 Prometheus 收集器接口的实现之一，用于描述当前收集器中所有指标的元数据。
//    - 它通过启动一个 Goroutine 来异步收集和发送指标描述信息。

// 4. **Collect 函数**：
//    - `Collect` 函数是 Prometheus 收集器接口的另一实现，用于实际收集指标数据。
//    - 在该函数中，收集器会调用 `scrape` 函数来抓取当前的指标数据，并将这些数据发送给 Prometheus。

// 5. **scrape 函数**：
//    - `scrape` 函数执行实际的数据抓取工作，
//      它会从 `masterController` 中收集组件的状态信息，并检查 Worker 的健康状态，将这些信息更新到相应的指标中。

// 6. **健康检查**：
//    - `scrape` 函数还会调用 `discover.HealthCheck()` 来检查 Worker 服务的健康状态，并根据检查结果更新相应的指标。

// 7. **Prometheus 指标**：
//    - 文件中定义的 Prometheus 指标涵盖了抓取持续时间、健康状态、任务数量、任务错误数以及存储组件数量等多个维度，
//      这些指标可以帮助运维人员了解 Worker 组件的运行状况和性能表现。

// 总的来说，本文件通过定义 Exporter 组件，集成了 Prometheus 监控能力，为 Rainbond 的 Worker 组件提供了详细的运行时状态监控，
// 帮助用户及时掌握系统状态，并对潜在问题进行快速诊断。

package collector

import (
	"github.com/goodrain/rainbond/worker/master"

	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/worker/appm/controller"
	"github.com/goodrain/rainbond/worker/discover"
	"github.com/prometheus/client_golang/prometheus"
)

// Exporter 收集器
type Exporter struct {
	error             prometheus.Gauge
	totalScrapes      prometheus.Counter
	scrapeErrors      *prometheus.CounterVec
	workerUp          prometheus.Gauge
	dbmanager         db.Manager
	masterController  *master.Controller
	controllermanager *controller.Manager
	taskNum           prometheus.Counter
	taskUpNum         prometheus.Gauge
	taskError         prometheus.Counter
	storeComponentNum prometheus.Gauge
	//thirdComponentDiscoverNum prometheus.Gauge
}

var scrapeDurationDesc = prometheus.NewDesc(
	prometheus.BuildFQName(namespace, "exporter", "collector_duration_seconds"),
	"Collector time duration.",
	[]string{"collector"}, nil,
)

var healthDesc = prometheus.NewDesc(
	prometheus.BuildFQName(namespace, "exporter", "health_status"),
	"health status.",
	[]string{"service_name"}, nil,
)

// Describe Describe
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	metricCh := make(chan prometheus.Metric)
	doneCh := make(chan struct{})

	go func() {
		for m := range metricCh {
			ch <- m.Desc()
		}
		close(doneCh)
	}()

	e.Collect(metricCh)
	close(metricCh)
	<-doneCh
}

// Collect implements prometheus.Collector.
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	e.scrape(ch)
	ch <- e.totalScrapes
	ch <- e.error
	e.scrapeErrors.Collect(ch)
	ch <- e.workerUp
}

func (e *Exporter) scrape(ch chan<- prometheus.Metric) {
	e.totalScrapes.Inc()
	e.masterController.Scrape(ch, scrapeDurationDesc)
	healthInfo := discover.HealthCheck()
	healthStatus := healthInfo["status"]
	var val float64
	if healthStatus == "health" {
		val = 1
	} else {
		val = 0
	}
	ch <- prometheus.MustNewConstMetric(healthDesc, prometheus.GaugeValue, val, "worker")
	ch <- prometheus.MustNewConstMetric(e.taskUpNum.Desc(),
		prometheus.GaugeValue,
		float64(e.controllermanager.GetControllerSize()))
	ch <- prometheus.MustNewConstMetric(e.taskNum.Desc(), prometheus.CounterValue, discover.TaskNum)
	ch <- prometheus.MustNewConstMetric(e.taskError.Desc(), prometheus.CounterValue, discover.TaskError)
	ch <- prometheus.MustNewConstMetric(e.storeComponentNum.Desc(), prometheus.GaugeValue, float64(len(e.masterController.GetStore().GetAllAppServices())))
}

var namespace = "worker"

// New 创建一个收集器
func New(masterController *master.Controller, controllermanager *controller.Manager) *Exporter {
	return &Exporter{
		totalScrapes: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "exporter",
			Name:      "scrapes_total",
			Help:      "Total number of times Worker was scraped for metrics.",
		}),
		scrapeErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "exporter",
			Name:      "scrape_errors_total",
			Help:      "Total number of times an error occurred scraping a Worker.",
		}, []string{"collector"}),
		error: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "exporter",
			Name:      "last_scrape_error",
			Help:      "Whether the last scrape of metrics from Worker resulted in an error (1 for error, 0 for success).",
		}),
		workerUp: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "up",
			Help:      "Whether the Worker server is up.",
		}),
		taskUpNum: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "task_up_number",
			Help:      "Number of tasks being performed",
		}),
		taskNum: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "exporter",
			Name:      "worker_task_number",
			Help:      "worker total number of tasks.",
		}),
		taskError: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "exporter",
			Name:      "worker_task_error",
			Help:      "worker number of task errors.",
		}),
		storeComponentNum: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "store_component_number",
			Help:      "Number of components in the store cache.",
		}),
		dbmanager:         db.GetManager(),
		masterController:  masterController,
		controllermanager: controllermanager,
	}
}
