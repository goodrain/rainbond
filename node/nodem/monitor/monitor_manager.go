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

package monitor

import (
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/node/api"
	"github.com/goodrain/rainbond/node/monitormessage"
	"github.com/goodrain/rainbond/node/statsd"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/node_exporter/collector"
)

//Manager Manager
type Manager interface {
	Start(errchan chan error) error
	Stop() error
	SetAPIRoute(apim *api.Manager) error
}

type manager struct {
	statsdExporter     *statsd.Exporter
	statsdRegistry     *prometheus.Registry
	nodeExporterRestry *prometheus.Registry
	meserver           *monitormessage.UDPServer
}

func createNodeExporterRestry() (*prometheus.Registry, error) {
	registry := prometheus.NewRegistry()
	filters := []string{"cpu", "diskstats", "filesystem", "ipvs", "loadavg", "meminfo", "netdev", "netstat", "uname", "mountstats", "nfs"}
	nc, err := collector.NewNodeCollector(filters...)
	if err != nil {
		return nil, err
	}
	for n := range nc.Collectors {
		logrus.Infof("node collector - %s", n)
	}
	err = registry.Register(nc)
	if err != nil {
		return nil, err
	}
	return registry, nil
}

//CreateManager CreateManager
func CreateManager(c *option.Conf) (Manager, error) {
	//statsd exporter
	statsdRegistry := prometheus.NewRegistry()
	exporter := statsd.CreateExporter(c.StatsdConfig, statsdRegistry)
	meserver := monitormessage.CreateUDPServer("0.0.0.0", 6666, c.Etcd.Endpoints)
	nodeExporterRestry, err := createNodeExporterRestry()
	if err != nil {
		return nil, err
	}
	manage := &manager{
		statsdExporter:     exporter,
		statsdRegistry:     statsdRegistry,
		nodeExporterRestry: nodeExporterRestry,
		meserver:           meserver,
	}
	return manage, nil
}

func (m *manager) Start(errchan chan error) error {
	if err := m.statsdExporter.Start(); err != nil {
		logrus.Errorf("start statsd exporter server error,%s", err.Error())
		return err
	}
	if err := m.meserver.Start(); err != nil {
		return err
	}

	return nil
}

func (m *manager) Stop() error {
	return nil
}

//ReloadStatsdMappConfig ReloadStatsdMappConfig
func (m *manager) ReloadStatsdMappConfig(w http.ResponseWriter, r *http.Request) {
	if err := m.statsdExporter.ReloadConfig(); err != nil {
		w.Write([]byte(err.Error()))
		w.WriteHeader(500)
	} else {
		w.Write([]byte("Success reload"))
		w.WriteHeader(200)
	}
}

//HandleStatsd statsd handle
func (m *manager) HandleStatsd(w http.ResponseWriter, r *http.Request) {
	gatherers := prometheus.Gatherers{
		prometheus.DefaultGatherer,
		m.statsdRegistry,
	}
	// Delegate http serving to Prometheus client library, which will call collector.Collect.
	h := promhttp.HandlerFor(gatherers,
		promhttp.HandlerOpts{
			ErrorLog:      logrus.StandardLogger(),
			ErrorHandling: promhttp.ContinueOnError,
		})
	h.ServeHTTP(w, r)
}

//NodeExporter node exporter
func (m *manager) NodeExporter(w http.ResponseWriter, r *http.Request) {
	gatherers := prometheus.Gatherers{
		prometheus.DefaultGatherer,
		m.nodeExporterRestry,
	}
	// Delegate http serving to Prometheus client library, which will call collector.Collect.
	h := promhttp.HandlerFor(gatherers,
		promhttp.HandlerOpts{
			ErrorLog:      logrus.StandardLogger(),
			ErrorHandling: promhttp.ContinueOnError,
		})
	h.ServeHTTP(w, r)
}

//SetAPIRoute set api route rule
func (m *manager) SetAPIRoute(apim *api.Manager) error {
	apim.GetRouter().Get("/app/metrics", m.HandleStatsd)
	apim.GetRouter().Get("/-/statsdreload", m.ReloadStatsdMappConfig)
	apim.GetRouter().Get("/node/metrics", m.NodeExporter)
	return nil
}
