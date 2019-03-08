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
	"fmt"

	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/goodrain/rainbond/gateway/metric/collectors"
	"github.com/prometheus/client_golang/prometheus"
)

// Collector defines the interface for a metric collector
type Collector interface {
	Start()
	Stop()
	SetHosts(sets.String)
	SetServerNum(httpNum, tcpNum int)
	RemoveHostMetric([]string)
}

type collector struct {
	registry          *prometheus.Registry
	socket            *collectors.SocketCollector
	gatewayController *collectors.Controller
}

// NewCollector creates a new metric collector the for ingress controller
func NewCollector(gatewayHost string, registry *prometheus.Registry) (Collector, error) {
	ic := collectors.NewController()
	socketCollector, err := collectors.NewSocketCollector(gatewayHost, true)
	if err != nil {
		return nil, fmt.Errorf("create socket collector failure %s", err.Error())
	}
	return Collector(&collector{
		gatewayController: ic,
		socket:            socketCollector,
		registry:          registry,
	}), nil
}

func (c *collector) Start() {
	c.registry.MustRegister(c.gatewayController)
	c.registry.MustRegister(c.socket)
	go c.socket.Start()
}

func (c *collector) Stop() {
	c.registry.Unregister(c.gatewayController)
	c.registry.Unregister(c.socket)
}

func (c *collector) SetServerNum(httpNum, tcpNum int) {
	c.gatewayController.SetServerNum(httpNum, tcpNum)
}

func (c *collector) SetHosts(hosts sets.String) {
	c.socket.SetHosts(hosts)
}

//RemoveHostMetric -
func (c *collector) RemoveHostMetric(hosts []string) {
	c.socket.RemoveMetrics(hosts, c.registry)
}
