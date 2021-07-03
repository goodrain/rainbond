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

package thirdcomponent

import (
	"context"
	"reflect"
	"sync"
	"time"

	"github.com/goodrain/rainbond/pkg/apis/rainbond/v1alpha1"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type DiscoverPool struct {
	ctx            context.Context
	lock           sync.Mutex
	discoverWorker map[string]*Worker
	updateChan     chan *v1alpha1.ThirdComponent
	reconciler     *Reconciler
}

func NewDiscoverPool(ctx context.Context, reconciler *Reconciler) *DiscoverPool {
	dp := &DiscoverPool{
		ctx:            ctx,
		discoverWorker: make(map[string]*Worker),
		updateChan:     make(chan *v1alpha1.ThirdComponent, 1024),
		reconciler:     reconciler,
	}
	go dp.Start()
	return dp
}
func (d *DiscoverPool) GetSize() float64 {
	d.lock.Lock()
	defer d.lock.Unlock()
	return float64(len(d.discoverWorker))
}
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
						logrus.Errorf("update component status failure", err.Error())
					}
					logrus.Infof("update component %s status success by discover pool", name)
				} else {
					logrus.Debugf("component %s status endpoints not change", name)
				}
			}()
		}
	}
}

type Worker struct {
	discover   Discover
	cancel     context.CancelFunc
	ctx        context.Context
	updateChan chan *v1alpha1.ThirdComponent
	stoped     bool
}

func (w *Worker) Start() {
	defer func() {
		logrus.Infof("discover endpoint list worker %s/%s stoed", w.discover.GetComponent().Namespace, w.discover.GetComponent().Name)
		w.stoped = true
	}()
	w.stoped = false
	logrus.Infof("discover endpoint list worker %s/%s  started", w.discover.GetComponent().Namespace, w.discover.GetComponent().Name)
	for {
		w.discover.Discover(w.ctx, w.updateChan)
		select {
		case <-w.ctx.Done():
			return
		default:
		}
	}
}

func (w *Worker) UpdateDiscover(discover Discover) {
	w.discover = discover
}

func (w *Worker) Stop() {
	w.cancel()
}

func (w *Worker) IsStop() bool {
	return w.stoped
}

func (d *DiscoverPool) newWorker(dis Discover) *Worker {
	ctx, cancel := context.WithCancel(d.ctx)
	return &Worker{
		ctx:        ctx,
		discover:   dis,
		cancel:     cancel,
		updateChan: d.updateChan,
	}
}

func (d *DiscoverPool) AddDiscover(dis Discover) {
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
	go worker.Start()
	d.discoverWorker[key] = worker
}

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
