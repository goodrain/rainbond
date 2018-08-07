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
	"github.com/goodrain/rainbond/monitor/utils"
	"github.com/prometheus/common/model"
	"time"
)

type Entrance struct {
	discover.Callback
	Prometheus      *prometheus.Manager
	sortedEndpoints []string
}

func (e *Entrance) UpdateEndpoints(endpoints ...*config.Endpoint) {
	newArr := utils.TrimAndSort(endpoints)

	if utils.ArrCompare(e.sortedEndpoints, newArr) {
		logrus.Debugf("The endpoints is not modify: %s", e.Name())
		return
	}

	e.sortedEndpoints = newArr

	scrape := e.toScrape()
	e.Prometheus.UpdateScrape(scrape)
}

func (e *Entrance) Error(err error) {
	logrus.Error(err)
}

func (e *Entrance) Name() string {
	return "entrance"
}

func (e *Entrance) toScrape() *prometheus.ScrapeConfig {
	ts := make([]string, 0, len(e.sortedEndpoints))
	for _, end := range e.sortedEndpoints {
		ts = append(ts, end)
	}

	return &prometheus.ScrapeConfig{
		JobName:        e.Name(),
		ScrapeInterval: model.Duration(1 * time.Minute),
		ScrapeTimeout:  model.Duration(30 * time.Second),
		MetricsPath:    "/metrics",
		HonorLabels:    true,
		ServiceDiscoveryConfig: prometheus.ServiceDiscoveryConfig{
			StaticConfigs: []*prometheus.Group{
				{
					Targets: ts,
					Labels: map[model.LabelName]model.LabelValue{
						"service_name": model.LabelValue(e.Name()),
						"component": model.LabelValue(e.Name()),
					},
				},
			},
		},
	}
}
