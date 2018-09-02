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

package monitor

import (
	"context"
	"time"

	"github.com/Sirupsen/logrus"
	v3 "github.com/coreos/etcd/clientv3"
	"github.com/goodrain/rainbond/cmd/monitor/option"
	discoverv1 "github.com/goodrain/rainbond/discover"
	discoverv2 "github.com/goodrain/rainbond/discover.v2"
	"github.com/goodrain/rainbond/discover/config"
	"github.com/goodrain/rainbond/monitor/callback"
	"github.com/goodrain/rainbond/monitor/prometheus"
	"github.com/goodrain/rainbond/util/watch"
	"github.com/tidwall/gjson"
)

type Monitor struct {
	config     *option.Config
	ctx        context.Context
	cancel     context.CancelFunc
	client     *v3.Client
	timeout    time.Duration
	manager    *prometheus.Manager
	discoverv1 discoverv1.Discover
	discoverv2 discoverv2.Discover
}

func (d *Monitor) Start() {
	d.discoverv1.AddProject("prometheus", &callback.Prometheus{Prometheus: d.manager})
	d.discoverv1.AddProject("event_log_event_http", &callback.EventLog{Prometheus: d.manager})
	d.discoverv1.AddProject("acp_entrance", &callback.Entrance{Prometheus: d.manager})
	d.discoverv2.AddProject("app_sync_runtime_server", &callback.AppStatus{Prometheus: d.manager})
	d.discoverv2.AddProject("builder", &callback.Builder{Prometheus: d.manager})
	d.discoverv2.AddProject("acp_webcli", &callback.Webcli{Prometheus: d.manager})
	d.discoverv2.AddProject("mq", &callback.Mq{Prometheus: d.manager})

	// node and app runtime metrics needs to be monitored separately
	go d.discoverNodes(&callback.Node{Prometheus: d.manager}, &callback.App{Prometheus: d.manager}, d.ctx.Done())

	// monitor etcd members
	go d.discoverEtcd(&callback.Etcd{Prometheus: d.manager}, d.ctx.Done())

	// monitor Cadvisor
	go d.discoverCadvisor(&callback.Cadvisor{Prometheus: d.manager}, d.ctx.Done())
}

func (d *Monitor) discoverNodes(node *callback.Node, app *callback.App, done <-chan struct{}) {
	// start listen node modified
	watcher := watch.New(d.client, "")
	w, err := watcher.WatchList(d.ctx, "/rainbond/nodes", "")
	if err != nil {
		logrus.Error("failed to watch list for discover all nodes: ", err)
		return
	}
	defer w.Stop()

	for {
		select {
		case event, ok := <-w.ResultChan():
			if !ok {
				logrus.Warn("the events channel is closed.")
				return
			}

			switch event.Type {
			case watch.Added:
				node.Add(&event)

				isSlave := gjson.Get(event.GetValueString(), "labels.rainbond_node_rule_compute").String()
				if isSlave == "true" {
					app.Add(&event)
				}
			case watch.Modified:
				node.Modify(&event)

				isSlave := gjson.Get(event.GetValueString(), "labels.rainbond_node_rule_compute").String()
				if isSlave == "true" {
					app.Modify(&event)
				}
			case watch.Deleted:
				node.Delete(&event)

				isSlave := gjson.Get(event.GetValueString(), "labels.rainbond_node_rule_compute").String()
				if isSlave == "true" {
					app.Delete(&event)
				}
			case watch.Error:
				logrus.Error("error when read a event from result chan for discover all nodes: ", event.Error)
			}
		case <-done:
			logrus.Info("stop discover nodes because received stop signal.")
			return
		}

	}

}

func (d *Monitor) discoverCadvisor(c *callback.Cadvisor, done <-chan struct{}) {
	// start listen node modified
	watcher := watch.New(d.client, "")
	w, err := watcher.WatchList(d.ctx, "/rainbond/nodes", "")
	if err != nil {
		logrus.Error("failed to watch list for discover all nodes: ", err)
		return
	}
	defer w.Stop()

	for {
		select {
		case event, ok := <-w.ResultChan():
			if !ok {
				logrus.Warn("the events channel is closed.")
				return
			}

			switch event.Type {
			case watch.Added:

				isSlave := gjson.Get(event.GetValueString(), "labels.rainbond_node_rule_compute").String()
				if isSlave == "true" {
					c.Add(&event)
				}
			case watch.Modified:

				isSlave := gjson.Get(event.GetValueString(), "labels.rainbond_node_rule_compute").String()
				if isSlave == "true" {
					c.Modify(&event)
				}
			case watch.Deleted:

				isSlave := gjson.Get(event.GetValueString(), "labels.rainbond_node_rule_compute").String()
				if isSlave == "true" {
					c.Delete(&event)
				}
			case watch.Error:
				logrus.Error("error when read a event from result chan for discover all nodes: ", event.Error)
			}
		case <-done:
			logrus.Info("stop discover nodes because received stop signal.")
			return
		}

	}

}

func (d *Monitor) discoverEtcd(e *callback.Etcd, done <-chan struct{}) {
	t := time.Tick(time.Minute)
	for {
		select {
		case <-done:
			logrus.Info("stop discover etcd because received stop signal.")
			return
		case <-t:
			resp, err := d.client.MemberList(d.ctx)
			if err != nil {
				logrus.Error("Failed to list etcd members for discover etcd.")
				continue
			}

			endpoints := make([]*config.Endpoint, 0, 5)
			for _, member := range resp.Members {
				url := member.GetName() + ":2379"
				end := &config.Endpoint{
					URL: url,
				}
				endpoints = append(endpoints, end)
			}

			e.UpdateEndpoints(endpoints...)
		}
	}
}

func (d *Monitor) Stop() {
	logrus.Info("Stopping all child process for monitor")
	d.cancel()
	d.discoverv1.Stop()
	d.discoverv2.Stop()
	d.client.Close()
}

func NewMonitor(opt *option.Config, p *prometheus.Manager) *Monitor {
	ctx, cancel := context.WithCancel(context.Background())
	defaultTimeout := time.Second * 3

	cli, err := v3.New(v3.Config{
		Endpoints:   opt.EtcdEndpoints,
		DialTimeout: defaultTimeout,
	})
	if err != nil {
		logrus.Fatal(err)
	}

	dc1, err := discoverv1.GetDiscover(config.DiscoverConfig{
		EtcdClusterEndpoints: opt.EtcdEndpoints,
	})
	if err != nil {
		logrus.Fatal(err)
	}

	dc3, err := discoverv2.GetDiscover(config.DiscoverConfig{
		EtcdClusterEndpoints: opt.EtcdEndpoints,
	})
	if err != nil {
		logrus.Fatal(err)
	}

	d := &Monitor{
		config:     opt,
		ctx:        ctx,
		cancel:     cancel,
		manager:    p,
		client:     cli,
		discoverv1: dc1,
		discoverv2: dc3,
		timeout:    defaultTimeout,
	}

	return d
}
