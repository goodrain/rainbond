// RAINBOND, Application Management Platform
// Copyright (C) 2014-2019 Goodrain Co., Ltd.

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

package node

import (
	"strconv"
	"time"

	"github.com/goodrain/rainbond/node/nodem/client"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	namespace          = "rainbond"
	scrapeDurationDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "cluster", "collector_duration_seconds"),
		"cluster_exporter: Duration of a collector scrape.",
		[]string{},
		nil,
	)
	nodeStatus = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "cluster", "node_health"),
		"node_health: Rainbond node health status.",
		[]string{"node_id", "node_ip", "status", "healthy"},
		nil,
	)
	componentStatus = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "cluster", "component_health"),
		"component_health: Rainbond node component health status.",
		[]string{"node_id", "node_ip", "component"},
		nil,
	)
)

//Collect prometheus collect
func (n *Cluster) Collect(ch chan<- prometheus.Metric) {
	begin := time.Now()
	for _, node := range n.GetAllNode() {
		ch <- prometheus.MustNewConstMetric(nodeStatus, prometheus.GaugeValue, func() float64 {
			if node.Status == client.Running && node.NodeStatus.NodeHealth {
				return 0
			}
			return 1
		}(), node.ID, node.InternalIP, node.Status, strconv.FormatBool(node.NodeStatus.NodeHealth))
		for _, con := range node.NodeStatus.Conditions {
			ch <- prometheus.MustNewConstMetric(componentStatus, prometheus.GaugeValue, func() float64 {
				if con.Status == client.ConditionTrue {
					return 0
				}
				return 1
			}(), node.ID, node.InternalIP, string(con.Type))
		}
	}
	duration := time.Since(begin)
	ch <- prometheus.MustNewConstMetric(scrapeDurationDesc, prometheus.GaugeValue, duration.Seconds())
}

//Describe prometheus describe
func (n *Cluster) Describe(ch chan<- *prometheus.Desc) {
	ch <- scrapeDurationDesc
	ch <- nodeStatus
	ch <- componentStatus
}
