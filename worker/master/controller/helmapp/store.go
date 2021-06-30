// RAINBOND, Application Management Platform
// Copyright (C) 2014-2021 Goodrain Co., Ltd.

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

package helmapp

import (
	"fmt"
	"time"

	rainbondv1alpha1 "github.com/goodrain/rainbond/pkg/apis/rainbond/v1alpha1"
	"github.com/goodrain/rainbond/pkg/generated/listers/rainbond/v1alpha1"
	k8sutil "github.com/goodrain/rainbond/util/k8s"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

// Storer -
type Storer interface {
	Run(stopCh <-chan struct{})
	GetHelmApp(ns, name string) (*rainbondv1alpha1.HelmApp, error)
}

type store struct {
	informer cache.SharedIndexInformer
	lister   v1alpha1.HelmAppLister
}

// NewStorer creates a new storer.
func NewStorer(informer cache.SharedIndexInformer,
	lister v1alpha1.HelmAppLister,
	workqueue workqueue.Interface,
	finalizerQueue workqueue.Interface) Storer {
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			helmApp := obj.(*rainbondv1alpha1.HelmApp)
			workqueue.Add(k8sutil.ObjKey(helmApp))
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			helmApp := newObj.(*rainbondv1alpha1.HelmApp)
			workqueue.Add(k8sutil.ObjKey(helmApp))
		},
		DeleteFunc: func(obj interface{}) {
			// Two purposes of using finalizerQueue
			// 1. non-block DeleteFunc
			// 2. retry if the finalizer is failed
			finalizerQueue.Add(obj)
		},
	})
	return &store{
		informer: informer,
		lister:   lister,
	}
}

func (i *store) Run(stopCh <-chan struct{}) {
	go i.informer.Run(stopCh)

	// wait for all involved caches to be synced before processing items
	// from the queue
	if !cache.WaitForCacheSync(stopCh,
		i.informer.HasSynced,
	) {
		runtime.HandleError(fmt.Errorf("timed out waiting for caches to sync"))
	}

	// in big clusters, deltas can keep arriving even after HasSynced
	// functions have returned 'true'
	time.Sleep(1 * time.Second)
}

func (i *store) GetHelmApp(ns, name string) (*rainbondv1alpha1.HelmApp, error) {
	return i.lister.HelmApps(ns).Get(name)
}
