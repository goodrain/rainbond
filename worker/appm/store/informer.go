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
	"k8s.io/client-go/tools/cache"
)

//Informer kube-api client cache
type Informer struct {
	Ingress     cache.SharedIndexInformer
	Service     cache.SharedIndexInformer
	Secret      cache.SharedIndexInformer
	StatefulSet cache.SharedIndexInformer
	Deployment  cache.SharedIndexInformer
	Pod         cache.SharedIndexInformer
	ConfigMap   cache.SharedIndexInformer
	ReplicaSet  cache.SharedIndexInformer
}

//Start statrt
func (i *Informer) Start(stop chan struct{}) {
	go i.Ingress.Run(stop)
	go i.Service.Run(stop)
	go i.Secret.Run(stop)
	go i.StatefulSet.Run(stop)
	go i.Deployment.Run(stop)
	go i.Pod.Run(stop)
	go i.ConfigMap.Run(stop)
	go i.ReplicaSet.Run(stop)
}

//Ready if all kube informers is syncd, store is ready
func (i *Informer) Ready() bool {
	if i.Ingress.HasSynced() && i.Service.HasSynced() && i.Secret.HasSynced() &&
		i.StatefulSet.HasSynced() && i.Deployment.HasSynced() && i.Pod.HasSynced() &&
		i.ConfigMap.HasSynced() {
		return true
	}
	return false
}
