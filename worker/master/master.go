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

// 本文件实现了 Rainbond 应用管理平台中的主控制器组件，负责管理和监控应用的运行状态，并与 Kubernetes 集成。
// 主控制器是应用运行时的核心部分，负责处理应用的生命周期管理、资源分配、监控指标收集等关键任务。

// 1. **Controller 结构体**：
//    - `Controller` 是 Rainbond 平台的核心控制器，包含了多个 Prometheus 指标用于监控内存使用、CPU使用、文件系统使用、命名空间资源等。
//    - 控制器还集成了多个子控制器，如 Helm 应用控制器、第三方组件控制器、卷控制器等，用于处理不同类型的应用和资源。

// 2. **NewMasterController 函数**：
//    - `NewMasterController` 函数用于创建和初始化主控制器。
//    - 该函数设置了多个核心组件，如卷控制器、Helm 应用控制器、磁盘缓存、Pod 事件监听器等。
//    - 还初始化了与 Kubernetes API 服务器的连接，用于获取集群版本信息并进行资源管理。

// 3. **Start 函数**：
//    - `Start` 函数启动主控制器，并执行领导者选举，确保集群中只有一个控制器在处理关键任务。
//    - 如果当前控制器成为领导者，它将启动所有子控制器，并开始收集和暴露应用运行时的监控指标。
//    - 该函数还处理 Pod 事件和卷类型事件，确保集群中所有应用的状态和资源都能被正确管理。

// 4. **Scrape 函数**：
//    - `Scrape` 函数用于收集应用的运行时指标，并通过 Prometheus 进行暴露。
//    - 该函数遍历所有应用服务，收集内存、CPU、文件系统等资源的使用情况，并将这些信息作为监控指标暴露给外部系统。
//    - 还会收集命名空间的资源限制和请求，并将这些信息纳入监控范围。

// 5. **领导者选举**：
//    - 通过 `leader.RunAsLeader` 函数实现领导者选举，确保在多实例部署的情况下，只有一个实例在处理集群的核心任务。
//    - 领导者选举是通过 Kubernetes 的 ConfigMap 锁机制实现的，确保在高可用部署中集群状态的一致性。

// 6. **Prometheus 指标**：
//    - 文件中定义的 Prometheus 指标包括内存使用、CPU 使用、文件系统使用、命名空间资源使用、磁盘缓存等多个维度。
//    - 这些指标帮助运维人员监控集群中应用的资源使用情况，识别可能存在的性能瓶颈和资源不足问题。

// 7. **集成的控制器**：
//    - `Controller` 集成了多个控制器，用于管理不同类型的应用和资源。
//    - 这些控制器包括 Helm 应用控制器、第三方组件控制器、卷控制器等，它们协同工作，确保应用的高可用性和稳定性。

// 总的来说，本文件定义的主控制器是 Rainbond 平台的核心组件，负责管理和监控集群中的所有应用。
// 它通过 Kubernetes 提供的 API 和 Prometheus 监控系统，确保集群中应用的高效运行，并帮助运维人员及时了解和处理可能的问题。

package master

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/goodrain/rainbond/cmd/worker/option"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/pkg/common"
	"github.com/goodrain/rainbond/pkg/generated/clientset/versioned"
	"github.com/goodrain/rainbond/util/leader"
	"github.com/goodrain/rainbond/worker/appm/store"
	mcontroller "github.com/goodrain/rainbond/worker/master/controller"
	"github.com/goodrain/rainbond/worker/master/controller/helmapp"
	"github.com/goodrain/rainbond/worker/master/controller/thirdcomponent"
	"github.com/goodrain/rainbond/worker/master/podevent"
	"github.com/goodrain/rainbond/worker/master/volumes/provider"
	"github.com/goodrain/rainbond/worker/master/volumes/provider/lib/controller"
	"github.com/goodrain/rainbond/worker/master/volumes/statistical"
	"github.com/goodrain/rainbond/worker/master/volumes/sync"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Controller app runtime master controller
type Controller struct {
	ctx                 context.Context
	cancel              context.CancelFunc
	conf                option.Config
	restConfig          *rest.Config
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
	helmAppController   *helmapp.Controller
	controllers         []mcontroller.Controller
	isLeader            bool

	kubeClient kubernetes.Interface

	stopCh          chan struct{}
	podEvent        *podevent.PodEvent
	volumeTypeEvent *sync.VolumeTypeEvent

	version      *version.Info
	rainbondsssc controller.Provisioner
	rainbondsslc controller.Provisioner
	mgr          ctrl.Manager
}

// NewMasterController new master controller
func NewMasterController(conf option.Config, store store.Storer, kubeClient kubernetes.Interface, rainbondClient versioned.Interface, restConfig *rest.Config) (*Controller, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// The controller needs to know what the server version is because out-of-tree
	// provisioners aren't officially supported until 1.5
	serverVersion, err := kubeClient.Discovery().ServerVersion()
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
	rainbondsslcProvisioner := provider.NewRainbondsslcProvisioner(kubeClient, store)
	// Start the provision controller which will dynamically provision hostPath
	// PVs
	pc := controller.NewProvisionController(kubeClient, &conf, map[string]controller.Provisioner{
		rainbondssscProvisioner.Name(): rainbondssscProvisioner,
		rainbondsslcProvisioner.Name(): rainbondsslcProvisioner,
	}, serverVersion.GitVersion)
	stopCh := make(chan struct{})

	helmAppController := helmapp.NewController(ctx, stopCh, kubeClient, rainbondClient,
		store.Informer().HelmApp, store.Lister().HelmApp, conf.Helm.RepoFile, conf.Helm.RepoCache, conf.Helm.RepoCache)

	return &Controller{
		conf:              conf,
		restConfig:        restConfig,
		pc:                pc,
		helmAppController: helmAppController,
		store:             store,
		stopCh:            stopCh,
		cancel:            cancel,
		ctx:               ctx,
		dbmanager:         db.GetManager(),
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
		kubeClient:      kubeClient,
		rainbondsssc:    rainbondssscProvisioner,
		rainbondsslc:    rainbondsslcProvisioner,
		version:         serverVersion,
	}, nil
}

// IsLeader is leader
func (m *Controller) IsLeader() bool {
	return m.isLeader
}

// Start start
func (m *Controller) Start() error {
	logrus.Debug("master controller starting")
	start := func(ctx context.Context) {
		pc := controller.NewProvisionController(m.kubeClient, &m.conf, map[string]controller.Provisioner{
			m.rainbondsslc.Name(): m.rainbondsslc,
			m.rainbondsssc.Name(): m.rainbondsssc,
		}, m.version.GitVersion)

		m.isLeader = true
		defer func() {
			m.isLeader = false
		}()
		go m.diskCache.Start()
		defer m.diskCache.Stop()
		go pc.Run(ctx)
		m.store.RegistPodUpdateListener("podEvent", m.podEvent.GetChan())
		defer m.store.UnRegistPodUpdateListener("podEvent")
		go m.podEvent.Handle()
		m.store.RegisterVolumeTypeListener("volumeTypeEvent", m.volumeTypeEvent.GetChan())
		defer m.store.UnRegisterVolumeTypeListener("volumeTypeEvent")
		go m.volumeTypeEvent.Handle()

		// helm app controller
		go m.helmAppController.Start()
		defer m.helmAppController.Stop()

		// start controller
		mgr, err := ctrl.NewManager(m.restConfig, ctrl.Options{
			Scheme:           common.Scheme,
			LeaderElection:   false,
			LeaderElectionID: "controllers.rainbond.io",
		})
		if err != nil {
			logrus.Errorf("create new manager: %v", err)
			return
		}
		thirdComponentController, err := thirdcomponent.Setup(ctx, mgr)
		if err != nil {
			logrus.Errorf("setup third component controller: %v", err)
			return
		}
		m.mgr = mgr
		m.controllers = append(m.controllers, thirdComponentController)
		stopchan := make(chan struct{})
		go m.mgr.Start(ctx)

		defer func() { stopchan <- struct{}{} }()

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

	// Become leader again on stop leading.
	leaderCh := make(chan struct{}, 1)
	go func() {
		for {
			select {
			case <-m.ctx.Done():
				return
			case <-leaderCh:
				logrus.Info("run as leader")
				ctx, cancel := context.WithCancel(m.ctx)
				defer cancel()
				leader.RunAsLeader(ctx, m.kubeClient, m.conf.LeaderElectionNamespace, m.conf.LeaderElectionIdentity, lockName, start, func() {
					leaderCh <- struct{}{}
					logrus.Info("restart leader")
				})
			}
		}
	}()

	leaderCh <- struct{}{}

	return nil
}

// Stop stop
func (m *Controller) Stop() {
	close(m.stopCh)
}

// Scrape scrape app runtime
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
			m.cpuUse.WithLabelValues(service.TenantID, service.AppID, service.ServiceID, "running").Set(float64(service.GetCPURequest()))
		}
		if service.IsClosed() {
			if m.memoryUse.DeleteLabelValues(service.TenantID, service.AppID, service.ServiceID, "running") {
				logrus.Infof("remove memory usage for [%s/%s/%s]", service.TenantID, service.AppID, service.ServiceID)
			}
			if m.cpuUse.DeleteLabelValues(service.TenantID, service.AppID, service.ServiceID, "running") {
				logrus.Infof("remove cpu usage for [%s/%s/%s]", service.TenantID, service.AppID, service.ServiceID)
			}
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
	for _, contro := range m.controllers {
		contro.Collect(ch)
	}
	logrus.Infof("success collect worker master metric")
}

// GetStore -
func (m *Controller) GetStore() store.Storer {
	return m.store
}
