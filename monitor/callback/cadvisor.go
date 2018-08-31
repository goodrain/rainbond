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

package callback

import (
	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/discover"
	"github.com/goodrain/rainbond/discover/config"
	"github.com/goodrain/rainbond/monitor/prometheus"
	"github.com/goodrain/rainbond/util/watch"
	"github.com/prometheus/common/model"
	"github.com/tidwall/gjson"
	"time"
	"github.com/goodrain/rainbond/monitor/utils"
)

// Cadvisor 指容器监控数据，来源于所有子节点上的kubelet
// 127.0.0.1:4194/metrics
type Cadvisor struct {
	discover.Callback
	Prometheus      *prometheus.Manager
	sortedEndpoints []string

	endpoints []*config.Endpoint
}

func (c *Cadvisor) UpdateEndpoints(endpoints ...*config.Endpoint) {
	newArr := utils.TrimAndSort(endpoints)

	if utils.ArrCompare(c.sortedEndpoints, newArr) {
		logrus.Debugf("The endpoints is not modify: %s", c.Name())
		return
	}

	c.sortedEndpoints = newArr

	scrape := c.toScrape()
	c.Prometheus.UpdateScrape(scrape)
}

func (c *Cadvisor) Error(err error) {
	logrus.Error(err)
}

func (c *Cadvisor) Name() string {
	return "cadvisor"
}

func (c *Cadvisor) toScrape() *prometheus.ScrapeConfig {
	ts := make([]string, 0, len(c.sortedEndpoints))
	for _, end := range c.sortedEndpoints {
		ts = append(ts, end)
	}

	return &prometheus.ScrapeConfig{
		JobName:        c.Name(),
		ScrapeInterval: model.Duration(15 * time.Second),
		ScrapeTimeout:  model.Duration(10 * time.Second),
		MetricsPath:    "/metrics",
		ServiceDiscoveryConfig: prometheus.ServiceDiscoveryConfig{
			StaticConfigs: []*prometheus.Group{
				{
					Targets: ts,
					Labels: map[model.LabelName]model.LabelValue{
						"component": model.LabelValue(c.Name()),
					},
				},
			},
		},
	}
}

func (c *Cadvisor) AddEndpoint(end *config.Endpoint) {
	c.endpoints = append(c.endpoints, end)
	c.UpdateEndpoints(c.endpoints...)
}

func (c *Cadvisor) Add(event *watch.Event) {
	url := gjson.Get(event.GetValueString(), "external_ip").String() + ":4194"
	end := &config.Endpoint{
		URL: url,
	}

	c.AddEndpoint(end)
}

func (c *Cadvisor) Modify(event *watch.Event) {
	for i, end := range c.endpoints {
		if end.URL == event.GetValueString() {
			url := gjson.Get(event.GetValueString(), "external_ip").String() + ":4194"
			c.endpoints[i].URL = url
			c.UpdateEndpoints(c.endpoints...)
			break
		}
	}
}

func (c *Cadvisor) Delete(event *watch.Event) {
	for i, end := range c.endpoints {
		url := gjson.Get(event.GetValueString(), "external_ip").String() + ":4194"
		if end.URL == url {
			c.endpoints = append(c.endpoints[:i], c.endpoints[i+1:]...)
			c.UpdateEndpoints(c.endpoints...)
			break
		}
	}
}
