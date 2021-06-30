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
	"strings"
	"time"

	"github.com/goodrain/rainbond/pkg/apis/rainbond/v1alpha1"
	"github.com/goodrain/rainbond/pkg/generated/clientset/versioned"
	"github.com/goodrain/rainbond/pkg/helm"
	"github.com/sirupsen/logrus"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/workqueue"
)

const (
	defaultTimeout = 3 * time.Second
)

var defaultConditionTypes = []v1alpha1.HelmAppConditionType{
	v1alpha1.HelmAppChartReady,
	v1alpha1.HelmAppPreInstalled,
	v1alpha1.HelmAppInstalled,
}

// ControlLoop is a control loop to get helm app and reconcile it.
type ControlLoop struct {
	ctx        context.Context
	log        *logrus.Entry
	kubeClient clientset.Interface
	clientset  versioned.Interface
	storer     Storer
	workQueue  workqueue.Interface
	repo       *helm.Repo
	repoFile   string
	repoCache  string
	chartCache string
}

// NewControlLoop -
func NewControlLoop(ctx context.Context,
	kubeClient clientset.Interface,
	clientset versioned.Interface,
	storer Storer,
	workQueue workqueue.Interface,
	repoFile string,
	repoCache string,
	chartCache string,
) *ControlLoop {
	repo := helm.NewRepo(repoFile, repoCache)
	return &ControlLoop{
		ctx:        ctx,
		log:        logrus.WithField("WHO", "Helm App ControlLoop"),
		kubeClient: kubeClient,
		clientset:  clientset,
		storer:     storer,
		workQueue:  workQueue,
		repo:       repo,
		repoFile:   repoFile,
		repoCache:  repoCache,
		chartCache: chartCache,
	}
}

// Run runs the control loop.
func (c *ControlLoop) Run() {
	for {
		obj, shutdown := c.workQueue.Get()
		if shutdown {
			return
		}

		c.run(obj)
	}
}

// Stop stops the control loop.
func (c *ControlLoop) Stop() {
	c.log.Info("stopping...")
	c.workQueue.ShutDown()
}

func (c *ControlLoop) run(obj interface{}) {
	key, ok := obj.(string)
	if !ok {
		return
	}
	defer c.workQueue.Done(obj)
	name, ns := nameNamespace(key)

	helmApp, err := c.storer.GetHelmApp(ns, name)
	if err != nil {
		logrus.Warningf("[HelmAppController] [ControlLoop] get helm app(%s): %v", key, err)
		return
	}

	if err := c.Reconcile(helmApp); err != nil {
		// ignore the error, informer will push the same time into queue later.
		logrus.Warningf("[HelmAppController] [ControlLoop] [Reconcile]: %v", err)
		return
	}
}

// nameNamespace -
func nameNamespace(key string) (string, string) {
	strs := strings.Split(key, "/")
	return strs[0], strs[1]
}

// Reconcile -
func (c *ControlLoop) Reconcile(helmApp *v1alpha1.HelmApp) error {
	app, err := NewApp(c.ctx, c.kubeClient, c.clientset, helmApp, c.repoFile, c.repoCache, c.chartCache)
	if err != nil {
		return err
	}

	app.log.Debug("start reconcile")

	// update running status
	defer app.UpdateRunningStatus()

	// setups the default values of the helm app.
	if app.NeedSetup() {
		return app.Setup()
	}

	// detect the helm app.
	if app.NeedDetect() {
		return app.Detect()
	}

	// install or update the helm app.
	if app.NeedUpdate() {
		return app.InstallOrUpdate()
	}

	return nil
}
