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
// 本文件实现了 Rainbond 平台中的 Helm 应用管理逻辑。Helm 是 Kubernetes 的一个包管理工具，用于定义、安装和管理 Kubernetes 应用程序。
// 该文件主要用于管理 Helm 应用的安装、更新、卸载以及状态监控等操作。

// 文件的主要功能包括：

// 1. `App` 结构体：
//    - 表示一个 Helm 应用。包含应用的上下文、日志记录器、Rainbond 客户端、事件记录器等。
//    - 该结构体封装了与 Helm 应用管理相关的所有操作。

// 2. `NewApp` 函数：
//    - 创建并返回一个新的 `App` 实例，用于管理指定的 Helm 应用。

// 3. `Chart` 方法：
//    - 返回当前 Helm 应用的 Chart 名称，通常由仓库名称和模板名称组成。

// 4. `NeedSetup` 方法：
//    - 检查是否需要设置 Helm 应用的默认值。

// 5. `NeedDetect` 方法：
//    - 检查是否需要检测 Helm 应用的状态。

// 6. `NeedUpdate` 方法：
//    - 检查 Helm 应用是否需要更新。

// 7. `Setup` 方法：
//    - 为 Helm 应用设置默认值，包括默认的 PreStatus、Phase 和条件。

// 8. `Update` 方法：
//    - 更新 Helm 应用的状态和规格。通过 `UpdateStatus` 和 `UpdateSpec` 方法分别更新状态和规格。

// 9. `UpdateRunningStatus` 方法：
//    - 更新 Helm 应用的运行状态。通过查询 Helm 应用的当前状态来更新 Rainbond 中的记录。

// 10. `UpdateStatus` 方法：
//     - 更新 Helm 应用的状态信息，通常在检测、安装、更新等操作后调用。

// 11. `UpdateSpec` 方法：
//     - 更新 Helm 应用的规格信息，避免资源版本冲突。

// 12. `Detect` 方法：
//     - 检测 Helm 应用的状态，通过调用检测器（`Detector`）来完成该操作。

// 13. `LoadChart` 方法：
//     - 从仓库中加载 Helm 应用的 Chart。

// 14. `PreInstall` 方法：
//     - 在安装 Helm 应用前进行检查，确保可以安装。

// 15. `Status` 方法：
//     - 返回 Helm 应用的当前状态。

// 16. `InstallOrUpdate` 方法：
//     - 安装或更新 Helm 应用。如果应用已经存在，则执行升级操作；如果不存在，则进行安装。

// 17. `Uninstall` 方法：
//     - 卸载 Helm 应用。

// 该文件通过封装 Helm 应用管理的常见操作，使得 Rainbond 平台能够有效地管理基于 Helm 的 Kubernetes 应用程序，支持应用的安装、升级、卸载以及状态监控等操作。

package helmapp

import (
	"context"
	"github.com/goodrain/rainbond/pkg/generated/clientset/versioned"
	"github.com/goodrain/rainbond/pkg/generated/listers/rainbond/v1alpha1"
	"github.com/sirupsen/logrus"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

// Controller -
type Controller struct {
	storer      Storer
	stopCh      chan struct{}
	controlLoop *ControlLoop
	finalizer   *Finalizer
}

// NewController creates a new helm app controller.
func NewController(ctx context.Context,
	stopCh chan struct{},
	kubeClient clientset.Interface,
	clientset versioned.Interface,
	informer cache.SharedIndexInformer,
	lister v1alpha1.HelmAppLister,
	repoFile, repoCache, chartCache string) *Controller {
	workQueue := workqueue.New()
	finalizerQueue := workqueue.New()
	storer := NewStorer(informer, lister, workQueue, finalizerQueue)

	controlLoop := NewControlLoop(ctx, kubeClient, clientset, storer, workQueue, repoFile, repoCache, chartCache)
	finalizer := NewFinalizer(ctx, kubeClient, clientset, finalizerQueue, repoFile, repoCache, chartCache)

	return &Controller{
		storer:      storer,
		stopCh:      stopCh,
		controlLoop: controlLoop,
		finalizer:   finalizer,
	}
}

// Start starts the controller.
func (c *Controller) Start() {
	logrus.Info("start helm app controller")
	c.storer.Run(c.stopCh)
	go c.controlLoop.Run()
	c.finalizer.Run()
}

// Stop stops the controller.
func (c *Controller) Stop() {
	c.controlLoop.Stop()
	c.finalizer.Stop()
}
