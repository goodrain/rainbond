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
	"fmt"
	"time"

	"github.com/goodrain/rainbond/discover"
	"github.com/goodrain/rainbond/discover/config"
	"github.com/goodrain/rainbond/monitor/prometheus"
	"github.com/goodrain/rainbond/monitor/utils"
	"github.com/goodrain/rainbond/util"
	"github.com/goodrain/rainbond/util/watch"
	"github.com/prometheus/common/model"
	"github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
)

// Cadvisor 指容器监控数据，来源于所有子节点上的kubelet
// 127.0.0.1:4194/metrics
type Cadvisor struct {
	discover.Callback
	Prometheus      *prometheus.Manager
	sortedEndpoints []string
	ListenPort      int

	endpoints []*config.Endpoint
}

//UpdateEndpoints update endpoint
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

//Name name
func (c *Cadvisor) Name() string {
	return "cadvisor"
}

func (c *Cadvisor) toScrape() *prometheus.ScrapeConfig {
	apiServerHost := util.Getenv("KUBERNETES_SERVICE_HOST", "kubernetes.default.svc")
	apiServerPort := util.Getenv("KUBERNETES_SERVICE_PORT", "443")

	return &prometheus.ScrapeConfig{
		JobName:        c.Name(),
		ScrapeInterval: model.Duration(15 * time.Second),
		ScrapeTimeout:  model.Duration(10 * time.Second),
		Scheme:         "https",
		HTTPClientConfig: prometheus.HTTPClientConfig{
			TLSConfig: prometheus.TLSConfig{
				CAFile:             "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt",
				InsecureSkipVerify: true,
			},
			BearerTokenFile: "/var/run/secrets/kubernetes.io/serviceaccount/token",
		},
		ServiceDiscoveryConfig: prometheus.ServiceDiscoveryConfig{
			KubernetesSDConfigs: []*prometheus.SDConfig{
				{
					Role: "node",
				},
			},
		},
		RelabelConfigs: []*prometheus.RelabelConfig{
			{
				TargetLabel: "__address__",
				Replacement: apiServerHost + ":" + apiServerPort,
			},
			{
				SourceLabels: []model.LabelName{
					"__meta_kubernetes_node_name",
				},
				Regex:       prometheus.MustNewRegexp("(.+)"),
				TargetLabel: "__metrics_path__",
				Replacement: "/api/v1/nodes/${1}/proxy/metrics/cadvisor",
			},
			{
				Action: prometheus.RelabelAction("labelmap"),
				Regex:  prometheus.MustNewRegexp("__meta_kubernetes_node_label_(.+)"),
			},
		},
		MetricRelabelConfigs: []*prometheus.RelabelConfig{
			{
				SourceLabels: []model.LabelName{"name"},
				Regex:        prometheus.MustNewRegexp("k8s_(.*)_(.*)_(.*)_(.*)_(.*)"),
				TargetLabel:  "service_id",
				Replacement:  "${1}",
			},
			{
				SourceLabels: []model.LabelName{"name"},
				//k8s_POD_709dfaa8d9b9498a827fd5c503e0d1a1-deployment-8679ff667-j8fj8_5201d8a00fa743c18eb6553778f77c84_d6670db0-00a7-4d2c-a92e-18a19541268d_0
				Regex:       prometheus.MustNewRegexp("k8s_POD_(.*)-deployment-(.*)"),
				TargetLabel: "service_id",
				Replacement: "${1}",
			},
		},
	}
}

//AddEndpoint add endpoint
func (c *Cadvisor) AddEndpoint(end *config.Endpoint) {
	c.endpoints = append(c.endpoints, end)
	c.UpdateEndpoints(c.endpoints...)
}

//Add add
func (c *Cadvisor) Add(event *watch.Event) {
	url := fmt.Sprintf("%s:%d", gjson.Get(event.GetValueString(), "internal_ip").String(), c.ListenPort)
	end := &config.Endpoint{
		Name: event.GetKey(),
		URL:  url,
	}
	c.AddEndpoint(end)
}

//Modify update
func (c *Cadvisor) Modify(event *watch.Event) {
	var update bool
	url := fmt.Sprintf("%s:%d", gjson.Get(event.GetValueString(), "internal_ip").String(), c.ListenPort)
	for i, end := range c.endpoints {
		if end.Name == event.GetKey() {
			c.endpoints[i].URL = url
			c.UpdateEndpoints(c.endpoints...)
			update = true
			break
		}
	}
	if !update {
		c.endpoints = append(c.endpoints, &config.Endpoint{
			Name: event.GetKey(),
			URL:  url,
		})
	}
}

//Delete delete
func (c *Cadvisor) Delete(event *watch.Event) {
	for i, end := range c.endpoints {
		if end.Name == event.GetKey() {
			c.endpoints = append(c.endpoints[:i], c.endpoints[i+1:]...)
			c.UpdateEndpoints(c.endpoints...)
			break
		}
	}
}
