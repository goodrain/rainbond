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
// 该文件实现了在 Rainbond 平台中，当 Helm 应用被删除时执行清理工作的逻辑。Helm 是 Kubernetes 的一个包管理工具，用于定义、安装和管理 Kubernetes 应用程序。
// 本文件的主要职责是在 Helm 应用被删除时，确保其相关资源得到正确的清理，以防止残留的资源影响系统的稳定性。

// 文件的主要功能包括：

// 1. `Finalizer` 结构体：
//    - 代表一个清理器，用于在 Helm 应用被删除时执行清理工作。
//    - 包含了与 Kubernetes 集群和 Helm 应用交互的客户端、工作队列以及缓存路径等信息。

// 2. `NewFinalizer` 函数：
//    - 创建并返回一个新的 `Finalizer` 实例。该函数初始化了清理器所需的所有资源，如 Kubernetes 客户端、工作队列以及 Helm 应用的缓存路径。

// 3. `Run` 方法：
//    - 启动清理器，持续从工作队列中获取需要清理的 Helm 应用，并调用 `run` 方法执行具体的清理操作。

// 4. `Stop` 方法：
//    - 停止清理器的运行，关闭工作队列。

// 5. `run` 方法：
//    - 执行具体的清理操作。该方法首先将队列中的对象转换为 `HelmApp` 实例，
//      然后通过创建一个 `App` 实例来卸载（uninstall）对应的 Helm 应用，从而清理其相关资源。
//    - 如果在执行过程中遇到错误，会记录日志并继续处理下一个任务。

// 通过这个清理逻辑，Rainbond 平台能够在 Helm 应用被删除时，自动清理其相关的所有资源，确保系统的整洁和稳定。

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
