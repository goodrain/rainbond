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
	"github.com/prometheus/client_golang/prometheus"
	"github.com/goodrain/rainbond/mq/api/mq"
)

// Metric name parts.
const (
	// Namespace for all metrics.
	namespace = "acp_mq"
	// Subsystem(s).
	exporter = "exporter"
)

// Metric descriptors.
var (
	scrapeDurationDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, exporter, "collector_duration_seconds"),
		"Collector time duration.",
		[]string{"collector"}, nil,
	)
)

//Exporter collects entrance metrics. It implements prometheus.Collector.
type Exporter struct {
	error         prometheus.Gauge
	totalScrapes  prometheus.Counter
	scrapeErrors  *prometheus.CounterVec
	lbPluginUp    prometheus.Gauge
	enqueueNumber prometheus.Counter
	dequeueNumber prometheus.Counter
}

var healthDesc = prometheus.NewDesc(
	prometheus.BuildFQName(namespace, exporter, "health_status"),
	"health status.",
	[]string{"service_name"}, nil,
)

//NewExporter new a exporter
func NewExporter() *Exporter {
	return &Exporter{
		totalScrapes: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: exporter,
			Name:      "scrapes_total",
			Help:      "Total number of times Entrance was scraped for metrics.",
		}),
		scrapeErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: exporter,
			Name:      "scrape_errors_total",
			Help:      "Total number of times an error occurred scraping a Entrance.",
		}, []string{"collector"}),
		error: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: exporter,
			Name:      "last_scrape_error",
			Help:      "Whether the last scrape of metrics from Entrance resulted in an error (1 for error, 0 for success).",
		}),
		lbPluginUp: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "up",
			Help:      "Whether the default lb plugin is up.",
		}),
		enqueueNumber: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "enqueue_number",
			Help:      "Message queue enqueue total.",
		}),
		dequeueNumber: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "dequeue_number",
			Help:      "Message queue dequeue total.",
		}),
	}
}

//Describe implements prometheus.Collector.
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
}

func (e *Exporter) scrape(ch chan<- prometheus.Metric) {
	e.totalScrapes.Inc()
	ch <- prometheus.MustNewConstMetric(e.enqueueNumber.Desc(), prometheus.CounterValue, mq.EnqueueNumber)
	ch <- prometheus.MustNewConstMetric(e.dequeueNumber.Desc(), prometheus.CounterValue, mq.DequeueNumber)
	ch <- prometheus.MustNewConstMetric(healthDesc, prometheus.GaugeValue, 1, "mq")
}
