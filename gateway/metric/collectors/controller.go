// RAINBOND, Application Management Platform
// Copyright (C) 2014-2017 Goodrain Co., Ltd.

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

package collectors

import "github.com/prometheus/client_golang/prometheus"

// PrometheusNamespace default metric namespace
var PrometheusNamespace = "gateway"

// Controller defines base metrics about the rbd-gateway
type Controller struct {
	prometheus.Collector

	activeDomain *prometheus.GaugeVec

	constLabels prometheus.Labels
}

// NewController creates a new prometheus collector for the
// gateway controller operations
func NewController() *Controller {
	constLabels := prometheus.Labels{}
	cm := &Controller{
		activeDomain: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   "nginx",
				Name:        "active_server",
				Help:        "Cumulative number of active server",
				ConstLabels: constLabels,
			},
			[]string{"type"}),
	}
	return cm
}

// Describe implements prometheus.Collector
func (cm Controller) Describe(ch chan<- *prometheus.Desc) {
	cm.activeDomain.Describe(ch)
}

// Collect implements the prometheus.Collector interface.
func (cm Controller) Collect(ch chan<- prometheus.Metric) {
	cm.activeDomain.Collect(ch)
}

// SetServerNum sets the number of active domain
func (cm *Controller) SetServerNum(httpNum, tcpNum int) {
	labels := make(prometheus.Labels, 1)
	labels["type"] = "http"
	cm.activeDomain.With(labels).Set(float64(httpNum))
	labels["type"] = "tcp"
	cm.activeDomain.With(labels).Set(float64(tcpNum))
}
