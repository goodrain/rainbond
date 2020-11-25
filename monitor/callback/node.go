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
	"time"

	"github.com/goodrain/rainbond/discover"
	"github.com/goodrain/rainbond/discover/config"
	"github.com/goodrain/rainbond/monitor/prometheus"
	"github.com/goodrain/rainbond/monitor/utils"
	"github.com/goodrain/rainbond/util/watch"
	"github.com/prometheus/common/model"
	"github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
)

//Node node discover
type Node struct {
	discover.Callback
	Prometheus      *prometheus.Manager
	sortedEndpoints []string

	endpoints []*config.Endpoint
}

//UpdateEndpoints update endpoints
func (e *Node) UpdateEndpoints(endpoints ...*config.Endpoint) {
	newArr := utils.TrimAndSort(endpoints)

	if utils.ArrCompare(e.sortedEndpoints, newArr) {
		logrus.Debugf("The endpoints is not modify: %s", e.Name())
		return
	}

	e.sortedEndpoints = newArr

	scrapes := e.toScrape()
	for _, scrape := range scrapes {
		e.Prometheus.UpdateScrape(scrape)
	}
}

func (e *Node) Error(err error) {
	logrus.Error(err)
}

//Name name
func (e *Node) Name() string {
	return "rbd_node"
}

func (e *Node) toScrape() []*prometheus.ScrapeConfig {
	ts := make([]string, 0, len(e.sortedEndpoints))
	for _, end := range e.sortedEndpoints {
		ts = append(ts, end)
	}

	return []*prometheus.ScrapeConfig{&prometheus.ScrapeConfig{
		JobName:        e.Name(),
		ScrapeInterval: model.Duration(30 * time.Second),
		ScrapeTimeout:  model.Duration(30 * time.Second),
		MetricsPath:    "/node/metrics",
		ServiceDiscoveryConfig: prometheus.ServiceDiscoveryConfig{
			StaticConfigs: []*prometheus.Group{
				{
					Targets: ts,
					Labels: map[model.LabelName]model.LabelValue{
						"component": model.LabelValue(e.Name()),
					},
				},
			},
		},
	},
		&prometheus.ScrapeConfig{
			JobName:        "rbd_cluster",
			ScrapeInterval: model.Duration(30 * time.Second),
			ScrapeTimeout:  model.Duration(30 * time.Second),
			MetricsPath:    "/cluster/metrics",
			ServiceDiscoveryConfig: prometheus.ServiceDiscoveryConfig{
				StaticConfigs: []*prometheus.Group{
					{
						Targets: ts,
						Labels:  map[model.LabelName]model.LabelValue{},
					},
				},
			},
		},
	}
}

//AddEndpoint add endpoint
func (e *Node) AddEndpoint(end *config.Endpoint) {
	e.endpoints = append(e.endpoints, end)
	e.UpdateEndpoints(e.endpoints...)
}

//Add add
func (e *Node) Add(event *watch.Event) {
	url := gjson.Get(event.GetValueString(), "internal_ip").String() + ":6100"
	end := &config.Endpoint{
		Name: event.GetKey(),
		URL:  url,
	}
	e.AddEndpoint(end)
}

//Modify modify
func (e *Node) Modify(event *watch.Event) {
	var update bool
	url := gjson.Get(event.GetValueString(), "internal_ip").String() + ":6100"
	for i, end := range e.endpoints {
		if end.Name == event.GetKey() {
			e.endpoints[i].URL = url
			e.UpdateEndpoints(e.endpoints...)
			update = true
			break
		}
	}
	if !update {
		e.endpoints = append(e.endpoints, &config.Endpoint{
			Name: event.GetKey(),
			URL:  url,
		})
		e.UpdateEndpoints(e.endpoints...)
	}
}

//Delete delete
func (e *Node) Delete(event *watch.Event) {
	for i, end := range e.endpoints {
		if end.Name == event.GetKey() {
			e.endpoints = append(e.endpoints[:i], e.endpoints[i+1:]...)
			e.UpdateEndpoints(e.endpoints...)
			break
		}
	}
}
