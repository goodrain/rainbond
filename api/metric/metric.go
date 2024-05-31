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
	"github.com/goodrain/rainbond/pkg/gogo"
	"strconv"
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
		clusterGPUTotal: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: exporter,
			Name:      "cluster_gpu_total",
			Help:      "rainbond cluster gpu total",
		}),
		clusterSharedStorageUsage: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: exporter,
			Name:      "cluster_shared_storage_usage",
			Help:      "rainbond cluster shared storage usage, path is /grdata",
		}),
		clusterSharedStorageTotal: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: exporter,
			Name:      "cluster_shared_storage_total",
			Help:      "rainbond cluster shared_storage total, path is /grdata",
		}),
		nodeGPUCount: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: exporter,
			Name:      "node_gpu_count",
			Help:      "rainbond node GPU count",
		}, []string{"node_name"}),
		nodeGPUMem: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: exporter,
			Name:      "node_gpu_memory",
			Help:      "rainbond node GPU memory",
		}, []string{"node_name"}),
		clusterPodsNumber: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: exporter,
			Name:      "cluster_pod_number",
			Help:      "rainbond cluster pods number",
		}),
		clusterPodMemory: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: exporter,
			Name:      "cluster_pod_memory",
			Help:      "rainbond cluster pod memory",
		}, []string{"node_name", "app_id", "service_id", "resource_version"}),
		clusterPodCPU: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: exporter,
			Name:      "cluster_pod_cpu",
			Help:      "rainbond cluster pod CPU",
		}, []string{"node_name", "app_id", "service_id", "resource_version"}),
		clusterPodGPU: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: exporter,
			Name:      "cluster_pod_gpu",
			Help:      "rainbond cluster pod GPU",
		}, []string{"node_name", "app_id", "service_id", "resource_version", "gpu_idx"}),
		clusterPodStorageEphemeral: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: exporter,
			Name:      "cluster_pod_ephemeral_storage",
			Help:      "rainbond cluster pod StorageEphemeral",
		}, []string{"node_name", "app_id", "service_id", "resource_version"}),
	}
}

// Exporter exporter
type Exporter struct {
	apiRequest                 *prometheus.CounterVec
	tenantLimit                *prometheus.GaugeVec
	clusterPodMemory           *prometheus.GaugeVec
	clusterPodCPU              *prometheus.GaugeVec
	clusterPodGPU              *prometheus.GaugeVec
	nodeGPUMem                 *prometheus.GaugeVec
	nodeGPUCount               *prometheus.GaugeVec
	clusterPodStorageEphemeral *prometheus.GaugeVec
	clusterPodsNumber          prometheus.Gauge
	clusterCPUTotal            prometheus.Gauge
	clusterGPUTotal            prometheus.Gauge
	clusterMemoryTotal         prometheus.Gauge
	clusterSharedStorageTotal  prometheus.Gauge
	clusterSharedStorageUsage  prometheus.Gauge
}

// RequestInc request inc
func (e *Exporter) RequestInc(code int, path string) {
	e.apiRequest.WithLabelValues(fmt.Sprintf("%d", code), path).Inc()
}

// Describe implements prometheus.Collector.
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	metricCh := make(chan prometheus.Metric)
	doneCh := make(chan struct{})

	_ = gogo.Go(func(ctx context.Context) error {
		for m := range metricCh {
			ch <- m.Desc()
		}
		close(doneCh)
		return nil
	})

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
		e.clusterGPUTotal.Set(float64(resource.AllGPU))
		e.clusterPodsNumber.Set(float64(resource.AllPods))
		e.clusterPodMemory.Reset()
		e.clusterPodCPU.Reset()
		e.clusterPodGPU.Reset()
		e.nodeGPUMem.Reset()
		e.nodeGPUCount.Reset()
		e.clusterPodStorageEphemeral.Reset()
		for _, nodeGPU := range resource.NodeGPU {
			e.nodeGPUCount.WithLabelValues(nodeGPU.NodeName).Set(float64(nodeGPU.GPUCount))
			e.nodeGPUMem.WithLabelValues(nodeGPU.NodeName).Set(float64(nodeGPU.GPUMem))
		}
		for _, pod := range resource.NodePods {
			e.clusterPodMemory.WithLabelValues(pod.NodeName, pod.AppID, pod.ServiceID, pod.ResourceVersion).Set(float64(pod.Memory))
			e.clusterPodCPU.WithLabelValues(pod.NodeName, pod.AppID, pod.ServiceID, pod.ResourceVersion).Set(float64(pod.CPU))
			floatGPU, _ := strconv.ParseFloat(pod.GPU, 64)
			e.clusterPodGPU.WithLabelValues(pod.NodeName, pod.AppID, pod.ServiceID, pod.ResourceVersion, pod.GPUIDX).Set(floatGPU)
			e.clusterPodStorageEphemeral.WithLabelValues(pod.NodeName, pod.AppID, pod.ServiceID, pod.ResourceVersion).Set(float64(pod.StorageEphemeral))
		}
		e.clusterSharedStorageTotal.Set(float64(resource.TotalDisk))
		e.clusterSharedStorageUsage.Set(float64(resource.UsageDisk))
	}
	e.tenantLimit.Collect(ch)
	e.clusterMemoryTotal.Collect(ch)
	e.clusterCPUTotal.Collect(ch)
	e.clusterPodGPU.Collect(ch)
	e.clusterSharedStorageTotal.Collect(ch)
	e.clusterSharedStorageUsage.Collect(ch)
	e.clusterPodsNumber.Collect(ch)
	e.clusterPodStorageEphemeral.Collect(ch)
	e.nodeGPUMem.Collect(ch)
	e.nodeGPUCount.Collect(ch)
	e.clusterGPUTotal.Collect(ch)
	e.clusterPodCPU.Collect(ch)
	e.clusterPodMemory.Collect(ch)
}
