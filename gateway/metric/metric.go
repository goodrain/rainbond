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

package metric

import (
	"github.com/goodrain/rainbond/gateway/metric/collectors"
	"github.com/prometheus/client_golang/prometheus"
)

// Collector defines the interface for a metric collector
type Collector interface {
	Start()
	Stop()

	SetServerNum(httpNum, tcpNum int)
}

type collector struct {
	registry *prometheus.Registry

	gatewayController *collectors.Controller
}

// NewCollector creates a new metric collector the for ingress controller
func NewCollector(registry *prometheus.Registry) (Collector, error) {
	ic := collectors.NewController()

	return Collector(&collector{
		gatewayController: ic,

		registry: registry,
	}), nil
}

func (c *collector) Start() {
	c.registry.MustRegister(c.gatewayController)
}

func (c *collector) Stop() {
	c.registry.Unregister(c.gatewayController)
}

func (c *collector) SetServerNum(httpNum, tcpNum int) {
	c.gatewayController.SetServerNum(httpNum, tcpNum)
}
