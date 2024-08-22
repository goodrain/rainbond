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
// 该文件定义了Rainbond平台中用于管理和同步Kubernetes资源对象的Informer结构体及其相关方法。
// 通过集成多个Kubernetes资源的Informer，该文件为Rainbond平台提供了对Kubernetes集群中各种资源的
// 实时监控和管理能力。

// 文件中的主要功能包括：
// 1. `Informer` 结构体：定义了多个Kubernetes资源的Informer，包括Namespace、Ingress、Service、Secret、
//    StatefulSet、Deployment、Pod、ConfigMap、ReplicaSet、Endpoints、Nodes、StorageClass、Claims、Events、
//    HorizontalPodAutoscaler、CRD、HelmApp、ComponentDefinition、ThirdComponent、Job、CronJob等。
//    这些Informer用于监控和缓存Kubernetes集群中相应资源的状态和变化。
// 2. `Start` 和 `StartCRS` 方法：用于启动各个Informer的运行，开始监听Kubernetes API Server的资源更新。
//    `Start` 方法启动标准的Kubernetes资源Informer，而 `StartCRS` 方法则专门用于启动自定义资源的Informer。
// 3. `Ready` 方法：用于判断所有定义的Informer是否已同步完成。当所有Informer都已成功同步时，表示存储已准备就绪，
//    可以开始进行进一步的操作。

// 总的来说，该文件通过定义和管理Kubernetes资源的Informer，使Rainbond平台能够实时获取和处理集群中的资源信息，
// 从而实现对应用服务的高效管理和监控。这对于确保平台的稳定性和响应能力至关重要，特别是在需要处理大量资源的场景中。

package store

import (
	"k8s.io/client-go/tools/cache"
)

// Informer kube-api client cache
type Informer struct {
	Namespace               cache.SharedIndexInformer
	Ingress                 cache.SharedIndexInformer
	Service                 cache.SharedIndexInformer
	Secret                  cache.SharedIndexInformer
	StatefulSet             cache.SharedIndexInformer
	Deployment              cache.SharedIndexInformer
	Pod                     cache.SharedIndexInformer
	ConfigMap               cache.SharedIndexInformer
	ReplicaSet              cache.SharedIndexInformer
	Endpoints               cache.SharedIndexInformer
	Nodes                   cache.SharedIndexInformer
	StorageClass            cache.SharedIndexInformer
	Claims                  cache.SharedIndexInformer
	Events                  cache.SharedIndexInformer
	HorizontalPodAutoscaler cache.SharedIndexInformer
	CRD                     cache.SharedIndexInformer
	HelmApp                 cache.SharedIndexInformer
	ComponentDefinition     cache.SharedIndexInformer
	ThirdComponent          cache.SharedIndexInformer
	Job                     cache.SharedIndexInformer
	CronJob                 cache.SharedIndexInformer
	CRS                     map[string]cache.SharedIndexInformer
}

// StartCRS -
func (i *Informer) StartCRS(stop chan struct{}) {
	for k := range i.CRS {
		go i.CRS[k].Run(stop)
	}
}

// Start statrt
func (i *Informer) Start(stop chan struct{}) {
	go i.Namespace.Run(stop)
	go i.Ingress.Run(stop)
	go i.Service.Run(stop)
	go i.Secret.Run(stop)
	go i.StatefulSet.Run(stop)
	go i.Deployment.Run(stop)
	go i.Pod.Run(stop)
	go i.ConfigMap.Run(stop)
	go i.ReplicaSet.Run(stop)
	go i.Endpoints.Run(stop)
	go i.Nodes.Run(stop)
	go i.StorageClass.Run(stop)
	go i.Events.Run(stop)
	go i.HorizontalPodAutoscaler.Run(stop)
	go i.Claims.Run(stop)
	go i.CRD.Run(stop)
	go i.HelmApp.Run(stop)
	go i.ComponentDefinition.Run(stop)
	go i.ThirdComponent.Run(stop)
	go i.Job.Run(stop)
	go i.CronJob.Run(stop)
}

// Ready if all kube informers is syncd, store is ready
func (i *Informer) Ready() bool {
	if i.Namespace.HasSynced() && i.Ingress.HasSynced() && i.Service.HasSynced() && i.Secret.HasSynced() &&
		i.StatefulSet.HasSynced() && i.Deployment.HasSynced() && i.Pod.HasSynced() && i.CronJob.HasSynced() &&
		i.ConfigMap.HasSynced() && i.Nodes.HasSynced() && i.Events.HasSynced() &&
		i.HorizontalPodAutoscaler.HasSynced() && i.StorageClass.HasSynced() && i.Claims.HasSynced() && i.CRD.HasSynced() {
		return true
	}
	return false
}
