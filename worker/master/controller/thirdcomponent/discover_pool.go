// RAINBOND, Application Management Platform
// Copyright (C) 2021-2021 Goodrain Co., Ltd.

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
// 本文件是 Rainbond 平台中第三方组件（ThirdComponent）发现池（Discover Pool）的实现，用于管理和维护多个第三方组件的发现进程。通过 Discover Pool，可以有效地管理这些组件的状态发现和更新任务，确保组件的运行状态与实际状态保持一致。

// 文件的主要功能如下：

// 1. `DiscoverPool` 结构体：
//    - 该结构体是发现池的核心，用于管理多个第三方组件的发现任务。
//    - 通过维护一个工作池（discoverWorker），发现池能够跟踪和管理所有正在进行的组件发现任务。
//    - 它还包含一个更新通道（updateChan），用于接收来自发现任务的状态更新。

// 2. `NewDiscoverPool` 函数：
//    - 这是 DiscoverPool 的构造函数，初始化发现池并启动发现任务的管理进程。
//    - 该函数创建 DiscoverPool 的实例，并启动一个独立的 Goroutine 来处理更新任务。

// 3. `Start` 方法：
//    - 该方法是发现池的主要运行循环，用于持续监听和处理组件状态的更新请求。
//    - 它会检查是否有新的组件状态更新，并将其与已有的状态进行比较，如果有变化则更新组件的状态。

// 4. `newWorker` 方法：
//    - 该方法用于为每个第三方组件创建一个新的发现任务（Worker）。
//    - 如果组件使用的是静态端点（Static Endpoints），还会为其创建一个探测管理器（proberManager），用于定期探测组件的健康状况。

// 5. `AddDiscover` 方法：
//    - 该方法用于向发现池中添加新的组件发现任务。
//    - 如果该组件已经存在于发现池中，则更新其发现任务；否则，创建新的发现任务并启动。

// 6. `RemoveDiscover` 和 `RemoveDiscoverByName` 方法：
//    - 这些方法用于从发现池中移除指定的组件发现任务。
//    - 当组件被删除或发现任务停止时，调用这些方法以清理和移除相关的发现任务。

// 7. `Worker` 结构体：
//    - 该结构体代表每个具体的发现任务，负责执行组件的发现操作。
//    - Worker 通过其上下文（context）管理生命周期，并通过 `Start` 和 `Stop` 方法控制任务的启动和停止。

// 总的来说，本文件实现了对 Rainbond 平台中第三方组件发现任务的集中管理。通过 Discover Pool，平台能够有效地管理多个组件的发现进程，确保组件的运行状态能够被及时发现和更新，为平台的稳定运行提供了重要支持。

package thirdcomponent

import (
	"context"
	"reflect"
	"sync"
	"time"

	"github.com/goodrain/rainbond/pkg/apis/rainbond/v1alpha1"
	dis "github.com/goodrain/rainbond/worker/master/controller/thirdcomponent/discover"
	"github.com/goodrain/rainbond/worker/master/controller/thirdcomponent/prober"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DiscoverPool -
type DiscoverPool struct {
	ctx            context.Context
	lock           sync.Mutex
	discoverWorker map[string]*Worker
	updateChan     chan *v1alpha1.ThirdComponent
	reconciler     *Reconciler

	recorder record.EventRecorder
}

// NewDiscoverPool -
func NewDiscoverPool(ctx context.Context,
	reconciler *Reconciler,
	recorder record.EventRecorder) *DiscoverPool {
	dp := &DiscoverPool{
		ctx:            ctx,
		discoverWorker: make(map[string]*Worker),
		updateChan:     make(chan *v1alpha1.ThirdComponent, 1024),
		reconciler:     reconciler,
		recorder:       recorder,
	}
	go dp.Start()
	return dp
}

// GetSize -
func (d *DiscoverPool) GetSize() float64 {
	d.lock.Lock()
	defer d.lock.Unlock()
	return float64(len(d.discoverWorker))
}

// Start -
func (d *DiscoverPool) Start() {
	logrus.Infof("third component discover pool started")
	for {
		select {
		case <-d.ctx.Done():
			logrus.Infof("third component discover pool stoped")
			return
		case component := <-d.updateChan:
			func() {
				ctx, cancel := context.WithTimeout(d.ctx, time.Second*10)
				defer cancel()
				var old v1alpha1.ThirdComponent
				name := client.ObjectKey{Name: component.Name, Namespace: component.Namespace}
				d.reconciler.Client.Get(ctx, name, &old)
				if !reflect.DeepEqual(component.Status.Endpoints, old.Status.Endpoints) {
					if err := d.reconciler.updateStatus(ctx, component); err != nil {
						if apierrors.IsNotFound(err) {
							d.RemoveDiscover(component)
							return
						}
						logrus.Errorf("update component status failure: %s", err.Error())
					}
					logrus.Infof("update component %s status success by discover pool", name)
				}
			}()
		}
	}
}

func (d *DiscoverPool) newWorker(dis dis.Discover) *Worker {
	ctx, cancel := context.WithCancel(d.ctx)

	worker := &Worker{
		ctx:        ctx,
		discover:   dis,
		cancel:     cancel,
		updateChan: d.updateChan,
	}

	component := dis.GetComponent()
	if component.Spec.IsStaticEndpoints() {
		proberManager := prober.NewManager(d.recorder)
		dis.SetProberManager(proberManager)
		worker.proberManager = proberManager
	}

	return worker
}

// AddDiscover -
func (d *DiscoverPool) AddDiscover(dis dis.Discover) {
	d.lock.Lock()
	defer d.lock.Unlock()
	component := dis.GetComponent()
	if component == nil {
		return
	}
	key := component.Namespace + component.Name
	olddis, exist := d.discoverWorker[key]
	if exist {
		olddis.UpdateDiscover(dis)
		if olddis.IsStop() {
			go olddis.Start()
		}
		return
	}
	worker := d.newWorker(dis)
	if component.Spec.IsStaticEndpoints() {
		worker.proberManager.AddThirdComponent(dis.GetComponent())
	}
	go worker.Start()
	d.discoverWorker[key] = worker
}

// RemoveDiscover -
func (d *DiscoverPool) RemoveDiscover(component *v1alpha1.ThirdComponent) {
	d.lock.Lock()
	defer d.lock.Unlock()
	key := component.Namespace + component.Name
	olddis, exist := d.discoverWorker[key]
	if exist {
		olddis.Stop()
		delete(d.discoverWorker, key)
	}
}

// RemoveDiscoverByName -
func (d *DiscoverPool) RemoveDiscoverByName(req types.NamespacedName) {
	d.lock.Lock()
	defer d.lock.Unlock()
	key := req.Namespace + req.Name
	olddis, exist := d.discoverWorker[key]
	if exist {
		olddis.Stop()
		delete(d.discoverWorker, key)
	}
}
