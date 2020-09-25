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

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"

	corev1 "k8s.io/api/core/v1"

	"github.com/goodrain/rainbond/cmd/worker/option"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/util/leader"
	"github.com/goodrain/rainbond/worker/appm/store"
	"github.com/goodrain/rainbond/worker/master/podevent"
	"github.com/goodrain/rainbond/worker/master/volumes/provider"
	"github.com/goodrain/rainbond/worker/master/volumes/provider/lib/controller"
	"github.com/goodrain/rainbond/worker/master/volumes/statistical"
	"github.com/goodrain/rainbond/worker/master/volumes/sync"
)

//Controller app runtime master controller
type Controller struct {
	ctx                 context.Context
	cancel              context.CancelFunc
	conf                option.Config
	store               store.Storer
	dbmanager           db.Manager
	memoryUse           *prometheus.GaugeVec
	cpuUse              *prometheus.GaugeVec
	fsUse               *prometheus.GaugeVec
	diskCache           *statistical.DiskCache
	namespaceMemRequest *prometheus.GaugeVec
	namespaceMemLimit   *prometheus.GaugeVec
	namespaceCPURequest *prometheus.GaugeVec
	namespaceCPULimit   *prometheus.GaugeVec
	pc                  *controller.ProvisionController
	isLeader            bool

	stopCh          chan struct{}
	podEventChs     []chan *corev1.Pod
	podEvent        *podevent.PodEvent
	volumeTypeEvent *sync.VolumeTypeEvent
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
	pc := controller.NewProvisionController(conf.KubeClient, &conf, map[string]controller.Provisioner{
		rainbondssscProvisioner.Name(): rainbondssscProvisioner,
		rainbondsslcProvisioner.Name(): rainbondsslcProvisioner,
	}, serverVersion.GitVersion)
	stopCh := make(chan struct{})

	return &Controller{
		conf:      conf,
		pc:        pc,
		store:     store,
		stopCh:    stopCh,
		cancel:    cancel,
		ctx:       ctx,
		dbmanager: db.GetManager(),
		memoryUse: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "app_resource",
			Name:      "appmemory",
			Help:      "tenant service memory request.",
		}, []string{"tenant_id", "app_id", "service_id", "service_status"}),
		cpuUse: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "app_resource",
			Name:      "appcpu",
			Help:      "tenant service cpu request.",
		}, []string{"tenant_id", "app_id", "service_id", "service_status"}),
		fsUse: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "app_resource",
			Name:      "appfs",
			Help:      "tenant service fs used.",
		}, []string{"tenant_id", "app_id", "service_id", "volume_type"}),
		namespaceMemRequest: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "namespace_resource",
			Name:      "memory_request",
			Help:      "total memory request in namespace",
		}, []string{"namespace"}),
		namespaceMemLimit: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "namespace_resource",
			Name:      "memory_limit",
			Help:      "total memory limit in namespace",
		}, []string{"namespace"}),
		namespaceCPURequest: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "namespace_resource",
			Name:      "cpu_request",
			Help:      "total cpu request in namespace",
		}, []string{"namespace"}),
		namespaceCPULimit: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "namespace_resource",
			Name:      "cpu_limit",
			Help:      "total cpu limit in namespace",
		}, []string{"namespace"}),
		diskCache:       statistical.CreatDiskCache(ctx),
		podEvent:        podevent.New(conf.KubeClient, stopCh),
		volumeTypeEvent: sync.New(stopCh),
	}, nil
}

//IsLeader is leader
func (m *Controller) IsLeader() bool {
	return m.isLeader
}

//Start start
func (m *Controller) Start() error {
	logrus.Debug("master controller starting")
	start := func(ctx context.Context) {
		m.isLeader = true
		defer func() {
			m.isLeader = false
		}()
		go m.diskCache.Start()
		defer m.diskCache.Stop()
		go m.pc.Run(ctx)
		m.store.RegistPodUpdateListener("podEvent", m.podEvent.GetChan())
		defer m.store.UnRegistPodUpdateListener("podEvent")
		go m.podEvent.Handle()

		m.store.RegisterVolumeTypeListener("volumeTypeEvent", m.volumeTypeEvent.GetChan())
		defer m.store.UnRegisterVolumeTypeListener("volumeTypeEvent")
		go m.volumeTypeEvent.Handle()

		select {
		case <-ctx.Done():
		case <-m.ctx.Done():
		}
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
	go leader.RunAsLeader(m.ctx, m.conf.KubeClient, m.conf.LeaderElectionNamespace, m.conf.LeaderElectionIdentity, lockName, start, func() {})

	return nil
}

//Stop stop
func (m *Controller) Stop() {
	close(m.stopCh)
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
			m.memoryUse.WithLabelValues(service.TenantID, service.AppID, service.ServiceID, "running").Set(float64(service.GetMemoryRequest()))
			m.cpuUse.WithLabelValues(service.TenantID, service.AppID, service.ServiceID, "running").Set(float64(service.GetMemoryRequest()))
		}
	}
	ch <- prometheus.MustNewConstMetric(scrapeDurationDesc, prometheus.GaugeValue, time.Since(scrapeTime).Seconds(), "collect.memory")
	scrapeTime = time.Now()
	diskcache := m.diskCache.Get()
	for k, v := range diskcache {
		key := strings.Split(k, "_")
		if len(key) == 3 {
			m.fsUse.WithLabelValues(key[2], key[1], key[0], string(model.ShareFileVolumeType)).Set(v)
		}
	}
	ch <- prometheus.MustNewConstMetric(scrapeDurationDesc, prometheus.GaugeValue, time.Since(scrapeTime).Seconds(), "collect.fs")
	resources := m.store.GetTenantResourceList()
	for _, re := range resources {
		m.namespaceMemLimit.WithLabelValues(re.Namespace).Set(float64(re.MemoryLimit / 1024 / 1024))
		m.namespaceCPULimit.WithLabelValues(re.Namespace).Set(float64(re.CPULimit))
		m.namespaceMemRequest.WithLabelValues(re.Namespace).Set(float64(re.MemoryRequest / 1024 / 1024))
		m.namespaceCPURequest.WithLabelValues(re.Namespace).Set(float64(re.CPURequest))
	}
	m.fsUse.Collect(ch)
	m.memoryUse.Collect(ch)
	m.cpuUse.Collect(ch)
	m.namespaceMemLimit.Collect(ch)
	m.namespaceCPULimit.Collect(ch)
	m.namespaceMemRequest.Collect(ch)
	m.namespaceCPURequest.Collect(ch)
	logrus.Infof("success collect worker master metric")
}
