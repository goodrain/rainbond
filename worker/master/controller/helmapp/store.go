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

// 该文件实现了 Rainbond 平台中 Helm 应用的数据存储和管理功能，主要用于管理 Helm 应用的状态信息和执行操作。

// 文件的核心功能包括以下几部分：

// 1. `Storer` 接口：
//    - 该接口定义了两个主要方法：`Run` 和 `GetHelmApp`。
//    - `Run` 方法启动存储器的运行，确保缓存中的 Helm 应用数据是最新的。
//    - `GetHelmApp` 方法用于从指定命名空间中获取 Helm 应用的详细信息。

// 2. `store` 结构体：
//    - 实现了 `Storer` 接口，持有 `informer` 和 `lister` 两个字段。
//    - `informer` 用于监听 Helm 应用的增删改操作，并将这些操作添加到工作队列中。
//    - `lister` 提供了从缓存中快速查询 Helm 应用的方法。

// 3. `NewStorer` 函数：
//    - 该函数用于创建并返回一个新的 `store` 实例，初始化 `informer` 和 `lister`，并注册事件处理函数。
//    - 在 Helm 应用被添加、更新或删除时，事件处理函数会将相关操作加入到相应的工作队列中，以供后续处理。

// 4. `Run` 方法：
//    - 启动 `informer` 的运行，确保缓存同步完成后再开始处理工作队列中的任务。
//    - 为了防止在大集群环境中缓存同步未完成就开始处理任务，`Run` 方法中会等待缓存同步完成，并引入一定的延迟。

// 5. `GetHelmApp` 方法：
//    - 从缓存中获取指定命名空间和名称的 Helm 应用对象。如果对象不存在或出现错误，将返回相应的错误信息。

// 总的来说，该文件的主要作用是确保 Helm 应用的状态在 Rainbond 平台上得以准确维护，通过监听应用的状态变化，实时更新缓存中的数据，并提供了快速查询和获取 Helm 应用状态的能力。

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
