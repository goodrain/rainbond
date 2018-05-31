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
	v3 "github.com/coreos/etcd/clientv3"
	"github.com/goodrain/rainbond/cmd/monitor/option"
	discover1 "github.com/goodrain/rainbond/discover"
	discover3 "github.com/goodrain/rainbond/discover.v2"
	"github.com/goodrain/rainbond/discover/config"
	"github.com/goodrain/rainbond/monitor/callback"
	"github.com/goodrain/rainbond/util/watch"
	"time"
	"github.com/Sirupsen/logrus"
	"os"
	"syscall"
	"os/signal"
	"github.com/goodrain/rainbond/monitor/prometheus"
	"github.com/tidwall/gjson"
)

type Monitor struct {
	config      *option.Config
	ctx         context.Context
	cancel      context.CancelFunc
	client      *v3.Client
	timeout     time.Duration
	stopperList []chan bool

	discover1   discover1.Discover
	discover3   discover3.Discover
}

func (d *Monitor) Start() {
	// create prometheus manager
	p := prometheus.NewManager(d.config)
	// start prometheus daemon and watching tis status in all time, exit monitor process if start failed
	p.StartDaemon(d.GetStopper())

	d.discover1.AddProject("event_log_event_grpc", &callback.EventLog{Prometheus: p})
	d.discover1.AddProject("acp_entrance", &callback.Entrance{Prometheus: p})
	d.discover3.AddProject("app_sync_runtime_server", &callback.AppStatus{Prometheus: p})

	// node and app runtime metrics needs to be monitored separately
	go d.discoverNodes(&callback.Node{Prometheus: p}, &callback.App{Prometheus: p}, d.GetStopper())

	d.listenStop()
}

func (d *Monitor) discoverNodes(node *callback.Node, app *callback.App, done chan bool) {
	// get all exist nodes by etcd
	resp, err := d.client.Get(d.ctx, "/rainbond/nodes/", v3.WithPrefix())
	if err != nil {
		logrus.Error("failed to get all nodes: ", err)
		return
	}

	for _, kv := range resp.Kvs {
		url := gjson.GetBytes(kv.Value, "external_ip").String() + ":6100"
		end := &config.Endpoint{
			URL: url,
		}

		node.AddEndpoint(end)

		isSlave := gjson.GetBytes(kv.Value, "labels.rainbond_node_rule_compute").String()
		if isSlave == "true" {
			app.AddEndpoint(end)
		}
	}

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
			close(done)
			return
		}

	}

}

func (d *Monitor) discoverEtcd(e *callback.Etcd, done chan bool) {
	t := time.Tick(time.Second * 5)
	for {
		select {
		case <-done:
			logrus.Info("stop discover etcd because received stop signal.")
			close(done)
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
	logrus.Info("Stop all child process for monitor.")
	for _, ch := range d.stopperList {
		ch <- true
	}

	d.discover1.Stop()
	d.discover3.Stop()
	d.client.Close()
	d.cancel()

	time.Sleep(time.Second)
}

func (d *Monitor) GetStopper() chan bool {
	ch := make(chan bool, 1)
	d.stopperList = append(d.stopperList, ch)

	return ch
}

func (d *Monitor) listenStop() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGKILL, syscall.SIGINT, syscall.SIGTERM)

	sig := <- sigs
	signal.Ignore(syscall.SIGKILL, syscall.SIGINT, syscall.SIGTERM)

	close(sigs)
	logrus.Warn("monitor manager received signal: ", sig)
	d.Stop()
}

func NewMonitor(opt *option.Config) *Monitor {
	ctx, cancel := context.WithCancel(context.Background())
	defaultTimeout := time.Second * 3

	cli, err := v3.New(v3.Config{
		Endpoints:   opt.EtcdEndpoints,
		DialTimeout: defaultTimeout,
	})
	if err != nil {
		logrus.Fatal(err)
	}

	dc1, err := discover1.GetDiscover(config.DiscoverConfig{
		EtcdClusterEndpoints: opt.EtcdEndpoints,
	})
	if err != nil {
		logrus.Fatal(err)
	}

	dc3, err := discover3.GetDiscover(config.DiscoverConfig{
		EtcdClusterEndpoints: opt.EtcdEndpoints,
	})
	if err != nil {
		logrus.Fatal(err)
	}

	d := &Monitor{
		config:    opt,
		ctx:       ctx,
		cancel:    cancel,
		client:    cli,
		discover1: dc1,
		discover3: dc3,
		timeout:   defaultTimeout,
	}

	return d
}
