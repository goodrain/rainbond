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
	"context"

	"github.com/goodrain/rainbond/pkg/apis/rainbond/v1alpha1"
	"github.com/goodrain/rainbond/pkg/generated/clientset/versioned"
	"github.com/sirupsen/logrus"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/workqueue"
)

// Finalizer does some cleanup work when helmApp is deleted
type Finalizer struct {
	ctx        context.Context
	log        *logrus.Entry
	kubeClient clientset.Interface
	clientset  versioned.Interface
	queue      workqueue.Interface
	repoFile   string
	repoCache  string
	chartCache string
}

// NewFinalizer creates a new finalizer.
func NewFinalizer(ctx context.Context,
	kubeClient clientset.Interface,
	clientset versioned.Interface,
	workQueue workqueue.Interface,
	repoFile string,
	repoCache string,
	chartCache string,
) *Finalizer {
	return &Finalizer{
		ctx:        ctx,
		log:        logrus.WithField("WHO", "Helm App Finalizer"),
		kubeClient: kubeClient,
		clientset:  clientset,
		queue:      workQueue,
		repoFile:   repoFile,
		repoCache:  repoCache,
		chartCache: chartCache,
	}
}

// Run runs the finalizer.
func (c *Finalizer) Run() {
	for {
		obj, shutdown := c.queue.Get()
		if shutdown {
			return
		}

		err := c.run(obj)
		if err != nil {
			c.log.Warningf("run: %v", err)
			continue
		}
		c.queue.Done(obj)
	}
}

// Stop stops the finalizer.
func (c *Finalizer) Stop() {
	c.log.Info("stopping...")
	c.queue.ShutDown()
}

func (c *Finalizer) run(obj interface{}) error {
	helmApp, ok := obj.(*v1alpha1.HelmApp)
	if !ok {
		return nil
	}

	logrus.Infof("start uninstall helm app: %s/%s", helmApp.Name, helmApp.Namespace)

	app, err := NewApp(c.ctx, c.kubeClient, c.clientset, helmApp, c.repoFile, c.repoCache, c.chartCache)
	if err != nil {
		return err
	}

	return app.Uninstall()
}
