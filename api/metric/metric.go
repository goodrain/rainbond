// RAINBOND, Application Management Platform
// Copyright (C) 2020-2020 Goodrain Co., Ltd.

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

package metric

import (
	"context"
	"fmt"
	"time"

	"github.com/goodrain/rainbond/api/handler"
	"github.com/prometheus/client_golang/prometheus"
)

// Metric name parts.
const (
	// Namespace for all metrics.
	namespace = "rbd_api"
	// Subsystem(s).
	exporter = "exporter"
)

//NewExporter new exporter
func NewExporter() *Exporter {
	return &Exporter{
		apiRequest: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: exporter,
			Name:      "api_request",
			Help:      "rainbond cluster api request metric",
		}, []string{"code", "path"}),
		tenantLimit: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: exporter,
			Name:      "tenant_memory_limit",
			Help:      "rainbond tenant memory limit",
		}, []string{"tenant_id", "namespace"}),
		clusterMemoryTotal: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: exporter,
			Name:      "cluster_memory_total",
			Help:      "rainbond cluster memory total",
		}),
		clusterCPUTotal: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: exporter,
			Name:      "cluster_cpu_total",
			Help:      "rainbond cluster cpu total",
		}),
	}
}

//Exporter exporter
type Exporter struct {
	apiRequest         *prometheus.CounterVec
	tenantLimit        *prometheus.GaugeVec
	clusterCPUTotal    prometheus.Gauge
	clusterMemoryTotal prometheus.Gauge
}

//RequestInc request inc
func (e *Exporter) RequestInc(code int, path string) {
	e.apiRequest.WithLabelValues(fmt.Sprintf("%d", code), path).Inc()
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
	e.apiRequest.Collect(ch)
	// tenant limit value
	tenants, _ := handler.GetTenantManager().GetTenants("")
	for _, t := range tenants {
		e.tenantLimit.WithLabelValues(t.UUID, t.UUID).Set(float64(t.LimitMemory))
	}
	// cluster memory
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer cancel()
	resource := handler.GetTenantManager().GetClusterResource(ctx)
	if resource != nil {
		e.clusterMemoryTotal.Set(float64(resource.AllMemory))
		e.clusterCPUTotal.Set(float64(resource.AllCPU))
	}
	e.tenantLimit.Collect(ch)
	e.clusterMemoryTotal.Collect(ch)
	e.clusterCPUTotal.Collect(ch)
}
