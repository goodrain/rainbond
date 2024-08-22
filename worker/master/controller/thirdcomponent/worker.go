package thirdcomponent

// 本文件实现了第三方组件（ThirdComponent）发现进程中的工作器（Worker）模块，用于在 Rainbond 平台中管理和维护组件的发现任务。Worker 负责执行组件的发现操作，并根据组件的配置进行相应的探测和状态更新。

// 文件的主要功能如下：

// 1. `Worker` 结构体：
//    - `Worker` 代表一个具体的组件发现任务，包含了发现任务的上下文（context）、取消函数（cancel）、更新通道（updateChan）以及探测管理器（proberManager）。
//    - `discover` 字段用于存储当前的组件发现任务，`stoped` 字段标识当前任务是否已经停止。

// 2. `Start` 方法：
//    - 该方法是 Worker 的核心运行逻辑，用于启动组件的发现任务。
//    - 在启动时，Worker 会将 `stoped` 标志设为 `false`，并调用 `discover` 的 `Discover` 方法来执行发现操作。
//    - 当发现任务结束时，`stoped` 标志会被设为 `true`，并且如果配置了探测管理器（proberManager），会调用其 `Stop` 方法停止探测任务。

// 3. `UpdateDiscover` 方法：
//    - 该方法用于更新 Worker 中的发现任务，并根据组件的配置动态添加探测任务。
//    - 如果组件配置了静态端点（Static Endpoints），则会将组件添加到探测管理器中，并设置新的探测管理器。

// 4. `Stop` 方法：
//    - 该方法用于停止 Worker 的发现任务，通过调用 `cancel` 函数来取消上下文，同时停止探测管理器的任务。

// 5. `IsStop` 方法：
//    - 该方法用于检查当前 Worker 是否已经停止。
//    - 返回 `true` 表示 Worker 已经停止，返回 `false` 表示 Worker 仍在运行。

// 总的来说，本文件实现了一个用于第三方组件发现的 Worker 模块，通过启动和管理组件的发现任务，确保组件的状态能够被及时探测和更新。Worker 模块在 Rainbond 平台的组件管理中扮演着重要的角色，确保平台的稳定运行。

import (
	"context"

	"github.com/goodrain/rainbond/pkg/apis/rainbond/v1alpha1"
	dis "github.com/goodrain/rainbond/worker/master/controller/thirdcomponent/discover"
	"github.com/goodrain/rainbond/worker/master/controller/thirdcomponent/prober"
	"github.com/sirupsen/logrus"
)

// Worker -
type Worker struct {
	discover   dis.Discover
	cancel     context.CancelFunc
	ctx        context.Context
	updateChan chan *v1alpha1.ThirdComponent
	stoped     bool

	proberManager prober.Manager
}

// Start -
func (w *Worker) Start() {
	defer func() {
		logrus.Infof("discover endpoint list worker %s/%s stoed", w.discover.GetComponent().Namespace, w.discover.GetComponent().Name)
		w.stoped = true
		if w.proberManager != nil {
			w.proberManager.Stop()
		}
	}()
	w.stoped = false
	logrus.Infof("discover endpoint list worker %s/%s  started", w.discover.GetComponent().Namespace, w.discover.GetComponent().Name)
	w.discover.Discover(w.ctx, w.updateChan)
}

// UpdateDiscover -
func (w *Worker) UpdateDiscover(discover dis.Discover) {
	component := discover.GetComponent()
	if component.Spec.IsStaticEndpoints() {
		w.proberManager.AddThirdComponent(discover.GetComponent())
		discover.SetProberManager(w.proberManager)
	}
	w.discover = discover
}

// Stop -
func (w *Worker) Stop() {
	w.cancel()
	if w.proberManager != nil {
		w.proberManager.Stop()
	}
}

// IsStop -
func (w *Worker) IsStop() bool {
	return w.stoped
}
