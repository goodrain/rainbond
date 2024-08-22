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

// 本文件实现了 Rainbond 平台中的 Helm 应用控制循环逻辑。Helm 是 Kubernetes 的一个包管理工具，用于定义、安装和管理 Kubernetes 应用程序。
// 该文件主要负责 Helm 应用的自动化管理，确保应用按照期望的状态进行安装、更新和运行。

// 文件的主要功能包括：

// 1. `ControlLoop` 结构体：
//    - 代表一个 Helm 应用的控制循环。控制循环负责不断地检查 Helm 应用的状态，并执行相应的操作（如安装、更新等）以确保应用处于预期的状态。
//    - 控制循环包括上下文、日志记录器、Kubernetes 客户端、Rainbond 客户端、存储接口、工作队列和 Helm 仓库等。

// 2. `NewControlLoop` 函数：
//    - 创建并返回一个新的 `ControlLoop` 实例。该函数初始化了控制循环所需的所有资源，如 Helm 仓库和工作队列。

// 3. `Run` 方法：
//    - 启动控制循环。该方法不断从工作队列中获取任务并进行处理，确保 Helm 应用按计划执行。

// 4. `Stop` 方法：
//    - 停止控制循环，关闭工作队列，并释放相关资源。

// 5. `run` 方法：
//    - 处理从工作队列中获取的任务。根据任务的键（即应用的名称和命名空间），从存储中获取 Helm 应用对象，并调用 `Reconcile` 方法进行状态调和。

// 6. `nameNamespace` 函数：
//    - 从任务键中解析出应用的名称和命名空间。

// 7. `Reconcile` 方法：
//    - 执行 Helm 应用的状态调和。该方法根据 Helm 应用的当前状态决定是否需要设置默认值、检测应用状态或进行安装/更新操作。
//    - 包含以下步骤：
//      - 更新运行状态：通过 `UpdateRunningStatus` 方法更新应用的实际运行状态。
//      - 检查并设置默认值：通过 `NeedSetup` 方法检查是否需要设置默认值，如果需要，则调用 `Setup` 方法进行设置。
//      - 检测应用状态：通过 `NeedDetect` 方法检查是否需要检测应用状态，如果需要，则调用 `Detect` 方法进行检测。
//      - 安装或更新应用：通过 `NeedUpdate` 方法检查是否需要更新应用，如果需要，则调用 `InstallOrUpdate` 方法进行安装或更新。

// 该文件通过控制循环自动化管理 Helm 应用，确保应用能够按计划执行部署、更新和维护操作，是 Rainbond 平台中应用管理功能的重要组成部分。

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
