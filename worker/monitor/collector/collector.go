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

package collector

import (
	"github.com/goodrain/rainbond/worker/master"

	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/worker/appm/controller"
	"github.com/goodrain/rainbond/worker/discover"
	"github.com/prometheus/client_golang/prometheus"
)

//Exporter 收集器
type Exporter struct {
	error                     prometheus.Gauge
	totalScrapes              prometheus.Counter
	scrapeErrors              *prometheus.CounterVec
	workerUp                  prometheus.Gauge
	dbmanager                 db.Manager
	masterController          *master.Controller
	controllermanager         *controller.Manager
	taskNum                   prometheus.Counter
	taskUpNum                 prometheus.Gauge
	taskError                 prometheus.Counter
	storeComponentNum         prometheus.Gauge
	thirdComponentDiscoverNum prometheus.Gauge
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

//Describe Describe
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

//New 创建一个收集器
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
