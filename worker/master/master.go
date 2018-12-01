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

package master

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/worker/master/volumes/provider"

	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/worker/master/volumes/statistical"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/goodrain/rainbond/worker/appm/store"

	"github.com/goodrain/rainbond/cmd/worker/option"
	"github.com/goodrain/rainbond/util/leader"
	"github.com/goodrain/rainbond/worker/master/volumes/provider/lib/controller"
)

//Controller app runtime master controller
type Controller struct {
	ctx       context.Context
	cancel    context.CancelFunc
	conf      option.Config
	store     store.Storer
	dbmanager db.Manager
	memoryUse *prometheus.GaugeVec
	fsUse     *prometheus.GaugeVec
	diskCache *statistical.DiskCache
	pc        *controller.ProvisionController
	isLeader  bool
}

//NewMasterController new master controller
func NewMasterController(conf option.Config, store store.Storer) (*Controller, error) {
	ctx, cancel := context.WithCancel(context.Background())
	// The controller needs to know what the server version is because out-of-tree
	// provisioners aren't officially supported until 1.5
	serverVersion, err := conf.KubeClient.Discovery().ServerVersion()
	if err != nil {
		logrus.Errorf("Error getting server version: %v", err)
		cancel()
		return nil, err
	}

	// Create the provisioner: it implements the Provisioner interface expected by
	// the controller
	//statefulset share controller
	rainbondssscProvisioner := provider.NewRainbondssscProvisioner()
	//statefulset local controller
	rainbondsslcProvisioner := provider.NewRainbondsslcProvisioner(conf.KubeClient, store)
	// Start the provision controller which will dynamically provision hostPath
	// PVs
	pc := controller.NewProvisionController(conf.KubeClient, map[string]controller.Provisioner{
		rainbondssscProvisioner.Name(): rainbondssscProvisioner,
		rainbondsslcProvisioner.Name(): rainbondsslcProvisioner,
	}, serverVersion.GitVersion)
	return &Controller{
		conf:      conf,
		pc:        pc,
		store:     store,
		cancel:    cancel,
		ctx:       ctx,
		dbmanager: db.GetManager(),
		memoryUse: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "app_resource",
			Name:      "appmemory",
			Help:      "tenant service memory used.",
		}, []string{"tenant_id", "service_id", "service_status"}),
		fsUse: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "app_resource",
			Name:      "appfs",
			Help:      "tenant service fs used.",
		}, []string{"tenant_id", "service_id", "volume_type"}),
		diskCache: statistical.CreatDiskCache(ctx),
	}, nil
}

//IsLeader is leader
func (m *Controller) IsLeader() bool {
	return m.isLeader
}

//Start start
func (m *Controller) Start() error {
	start := func(stop <-chan struct{}) {
		m.isLeader = true
		defer func() {
			m.isLeader = false
		}()
		go m.diskCache.Start()
		defer m.diskCache.Stop()
		go m.pc.Run(stop)
		<-stop
	}
	// Leader election was requested.
	if m.conf.LeaderElectionNamespace == "" {
		return fmt.Errorf("-leader-election-namespace must not be empty")
	}
	if m.conf.LeaderElectionIdentity == "" {
		m.conf.LeaderElectionIdentity = m.conf.NodeName
	}
	if m.conf.LeaderElectionIdentity == "" {
		return fmt.Errorf("-leader-election-identity must not be empty")
	}
	// Name of config map with leader election lock
	lockName := "rainbond-appruntime-worker-leader"
	go leader.RunAsLeader(m.conf.KubeClient, m.conf.LeaderElectionNamespace, m.conf.LeaderElectionIdentity, lockName, start, func() {})
	return nil
}

//Stop stop
func (m *Controller) Stop() {

}

//Scrape scrape app runtime
func (m *Controller) Scrape(ch chan<- prometheus.Metric, scrapeDurationDesc *prometheus.Desc) {
	if !m.isLeader {
		return
	}
	scrapeTime := time.Now()
	services := m.store.GetAllAppServices()
	status := m.store.GetNeedBillingStatus(nil)
	//获取内存使用情况
	for _, service := range services {
		if _, ok := status[service.ServiceID]; ok {
			m.memoryUse.WithLabelValues(service.TenantID, service.ServiceID, "running").Set(float64(service.ContainerMemory * service.Replicas))
		}
	}
	ch <- prometheus.MustNewConstMetric(scrapeDurationDesc, prometheus.GaugeValue, time.Since(scrapeTime).Seconds(), "collect.memory")
	scrapeTime = time.Now()
	diskcache := m.diskCache.Get()
	for k, v := range diskcache {
		key := strings.Split(k, "_")
		if len(key) == 2 {
			m.fsUse.WithLabelValues(key[1], key[0], string(model.ShareFileVolumeType)).Set(v)
		}
	}
	ch <- prometheus.MustNewConstMetric(scrapeDurationDesc, prometheus.GaugeValue, time.Since(scrapeTime).Seconds(), "collect.fs")
	m.fsUse.Collect(ch)
	m.memoryUse.Collect(ch)
}
