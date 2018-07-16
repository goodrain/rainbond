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

// App 指app运行时信息，来源于所有子节点上的node
// 127.0.0.1:6100/app/metrics
type App struct {
	discover.Callback
	Prometheus      *prometheus.Manager
	sortedEndpoints []string

	endpoints []*config.Endpoint
}

func (e *App) UpdateEndpoints(endpoints ...*config.Endpoint) {
	newArr := utils.TrimAndSort(endpoints)

	if utils.ArrCompare(e.sortedEndpoints, newArr) {
		logrus.Debugf("The endpoints is not modify: %s", e.Name())
		return
	}

	e.sortedEndpoints = newArr

	scrape := e.toScrape()
	e.Prometheus.UpdateScrape(scrape)
}

func (e *App) Error(err error) {
	logrus.Error(err)
}

func (e *App) Name() string {
	return "app"
}

func (e *App) toScrape() *prometheus.ScrapeConfig {
	ts := make([]string, 0, len(e.sortedEndpoints))
	for _, end := range e.sortedEndpoints {
		ts = append(ts, end)
	}

	return &prometheus.ScrapeConfig{
		JobName:        e.Name(),
		ScrapeInterval: model.Duration(5 * time.Second),
		ScrapeTimeout:  model.Duration(4 * time.Second),
		MetricsPath:    "/app/metrics",
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
	}
}

func (e *App) AddEndpoint(end *config.Endpoint) {
	e.endpoints = append(e.endpoints, end)
	e.UpdateEndpoints(e.endpoints...)
}

func (e *App) Add(event *watch.Event) {
	url := gjson.Get(event.GetValueString(), "external_ip").String() + ":6100"
	end := &config.Endpoint{
		URL: url,
	}

	e.AddEndpoint(end)
}

func (e *App) Modify(event *watch.Event) {
	for i, end := range e.endpoints {
		if end.URL == event.GetValueString() {
			url := gjson.Get(event.GetValueString(), "external_ip").String() + ":6100"
			e.endpoints[i].URL = url
			e.UpdateEndpoints(e.endpoints...)
			break
		}
	}
}

func (e *App) Delete(event *watch.Event) {
	for i, end := range e.endpoints {
		url := gjson.Get(event.GetValueString(), "external_ip").String() + ":6100"
		if end.URL == url {
			e.endpoints = append(e.endpoints[:i], e.endpoints[i+1:]...)
			e.UpdateEndpoints(e.endpoints...)
			break
		}
	}
}
