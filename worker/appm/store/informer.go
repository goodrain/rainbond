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

package store

import (
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/tools/cache"
)

//Informer kube-api client cache
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

//StartCRS -
func (i *Informer) StartCRS(stop chan struct{}) {
	for k := range i.CRS {
		go i.CRS[k].Run(stop)
	}
}

//Start statrt
func (i *Informer) Start(stop chan struct{}) {
	logrus.Infof("[Informer启动] 开始启动所有 Informers...")
	go i.Namespace.Run(stop)
	go i.Ingress.Run(stop)
	go i.Service.Run(stop)
	go i.Secret.Run(stop)
	go i.StatefulSet.Run(stop)
	logrus.Infof("[Informer启动] Deployment Informer 正在启动...")
	go i.Deployment.Run(stop)
	logrus.Infof("[Informer启动] Pod Informer 正在启动...")
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
	logrus.Infof("[Informer启动] 所有 Informers 已启动，等待同步完成...")
}

//Ready if all kube informers is syncd, store is ready
func (i *Informer) Ready() bool {
	deploymentSynced := i.Deployment.HasSynced()
	podSynced := i.Pod.HasSynced()

	if !deploymentSynced {
		logrus.Debugf("[Informer同步] Deployment Informer 尚未完成初始同步")
	}
	if !podSynced {
		logrus.Debugf("[Informer同步] Pod Informer 尚未完成初始同步")
	}

	if i.Namespace.HasSynced() && i.Ingress.HasSynced() && i.Service.HasSynced() && i.Secret.HasSynced() &&
		i.StatefulSet.HasSynced() && deploymentSynced && podSynced && i.CronJob.HasSynced() &&
		i.ConfigMap.HasSynced() && i.Nodes.HasSynced() && i.Events.HasSynced() &&
		i.HorizontalPodAutoscaler.HasSynced() && i.StorageClass.HasSynced() && i.Claims.HasSynced() && i.CRD.HasSynced() {
		logrus.Infof("[Informer同步] 所有 Informers 已完成初始同步，Store 就绪！")
		return true
	}
	return false
}
