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
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
)

//Storer app runtime store interface
type Storer interface {
}

//appRuntimeStore app runtime store
//cache all kubernetes object and appservice
type appRuntimeStore struct {
	informers *Informer
	listers   *Lister
}

//NewStore new app runtime store
func NewStore(client kubernetes.Interface) Storer {
	store := &appRuntimeStore{
		informers: &Informer{},
	}
	// create informers factory, enable and assign required informers
	infFactory := informers.NewFilteredSharedInformerFactory(client, time.Second, corev1.NamespaceAll,
		func(options *metav1.ListOptions) {
			options.LabelSelector = "creater=Rainbond"
		})
	store.informers.Deployment = infFactory.Apps().V1().Deployments().Informer()
	store.listers.Deployment = infFactory.Apps().V1().Deployments().Lister()

	store.informers.StatefulSet = infFactory.Apps().V1().StatefulSets().Informer()
	store.listers.StatefulSet = infFactory.Apps().V1().StatefulSets().Lister()

	store.informers.Service = infFactory.Core().V1().Services().Informer()
	store.listers.Service = infFactory.Core().V1().Services().Lister()

	store.informers.Endpoint = infFactory.Core().V1().Endpoints().Informer()
	store.listers.Endpoint = infFactory.Core().V1().Endpoints().Lister()

	store.informers.Pod = infFactory.Core().V1().Pods().Informer()
	store.listers.Pod = infFactory.Core().V1().Pods().Lister()

	store.informers.Secret = infFactory.Core().V1().Secrets().Informer()
	store.listers.Secret = infFactory.Core().V1().Secrets().Lister()

	store.informers.ConfigMap = infFactory.Core().V1().ConfigMaps().Informer()
	store.listers.ConfigMap = infFactory.Core().V1().ConfigMaps().Lister()

	store.informers.Ingress = infFactory.Extensions().V1beta1().Ingresses().Informer()
	store.listers.Ingress = infFactory.Extensions().V1beta1().Ingresses().Lister()

	store.informers.Deployment.AddEventHandler(store)
	store.informers.StatefulSet.AddEventHandler(store)
	store.informers.Pod.AddEventHandler(store)
	store.informers.Secret.AddEventHandler(store)
	store.informers.Service.AddEventHandler(store)
	store.informers.Endpoint.AddEventHandler(store)
	store.informers.Ingress.AddEventHandler(store)
	store.informers.ConfigMap.AddEventHandler(store)
	return store
}

func (a *appRuntimeStore) OnAdd(obj interface{}) {

}
func (a *appRuntimeStore) OnUpdate(oldObj, newObj interface{}) {

}
func (a *appRuntimeStore) OnDelete(obj interface{}) {

}
