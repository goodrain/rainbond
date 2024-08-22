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

// 该文件用于管理 Rainbond 平台中 Helm 应用的状态。Helm 是 Kubernetes 的包管理工具，
// 本文件的功能是跟踪和更新 Helm 应用的状态，确保应用的生命周期在 Rainbond 平台上得以正确管理。

// 文件的主要功能包括：

// 1. `Status` 结构体：
//    - 代表 Helm 应用的状态管理器，负责跟踪和更新 Helm 应用的状态。
//    - 包含了与 Rainbond 平台交互的客户端、当前应用的上下文以及应用本身的实例信息。

// 2. `NewStatus` 函数：
//    - 创建并返回一个新的 `Status` 实例。该函数初始化了状态管理器所需的所有资源，如 Rainbond 客户端和应用上下文。

// 3. `Update` 方法：
//    - 更新 Helm 应用的状态。使用 Kubernetes 的 `RetryOnConflict` 机制处理并发更新时可能出现的冲突，确保状态更新的可靠性。
//    - 该方法会通过 Rainbond 客户端获取当前应用的最新状态，并根据应用的检测状态、配置状态以及安装状态更新应用的 `Phase` 字段。

// 4. `getPhase` 方法：
//    - 根据应用的当前条件和状态，确定并返回应用的生命周期阶段（Phase）。
//    - 阶段包括检测中（Detecting）、配置中（Configuring）、安装中（Installing）和已安装（Installed）。

// 5. `isDetected` 方法：
//    - 检查应用是否已经通过了初步检测。检测条件包括应用 Chart 是否准备好以及应用是否通过了预安装检查。
//    - 如果所有必要条件都满足，返回 `true`，否则返回 `false`。

// 通过这些功能，Rainbond 平台能够在 Helm 应用的整个生命周期内准确跟踪和管理其状态，确保应用的各个阶段都得到正确的处理。

package helmapp

import (
	"context"

	"github.com/goodrain/rainbond/pkg/apis/rainbond/v1alpha1"
	"github.com/goodrain/rainbond/pkg/generated/clientset/versioned"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

// Status represents the status of helm app.
type Status struct {
	ctx            context.Context
	rainbondClient versioned.Interface
	helmApp        *v1alpha1.HelmApp
}

// NewStatus creates a new helm app status.
func NewStatus(ctx context.Context, app *v1alpha1.HelmApp, rainbondClient versioned.Interface) *Status {
	return &Status{
		ctx:            ctx,
		helmApp:        app,
		rainbondClient: rainbondClient,
	}
}

// Update updates helm app status.
func (s *Status) Update() error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		ctx, cancel := context.WithTimeout(s.ctx, defaultTimeout)
		defer cancel()

		helmApp, err := s.rainbondClient.RainbondV1alpha1().HelmApps(s.helmApp.Namespace).Get(ctx, s.helmApp.Name, metav1.GetOptions{})
		if err != nil {
			return errors.Wrap(err, "get helm app before update")
		}

		s.helmApp.Status.Phase = s.getPhase()
		s.helmApp.ResourceVersion = helmApp.ResourceVersion
		_, err = s.rainbondClient.RainbondV1alpha1().HelmApps(s.helmApp.Namespace).UpdateStatus(ctx, s.helmApp, metav1.UpdateOptions{})
		return err
	})
}

func (s *Status) getPhase() v1alpha1.HelmAppStatusPhase {
	phase := v1alpha1.HelmAppStatusPhaseDetecting
	if s.isDetected() {
		phase = v1alpha1.HelmAppStatusPhaseConfiguring
	}
	if s.helmApp.Spec.PreStatus == v1alpha1.HelmAppPreStatusConfigured {
		phase = v1alpha1.HelmAppStatusPhaseInstalling
	}
	idx, condition := s.helmApp.Status.GetCondition(v1alpha1.HelmAppInstalled)
	if idx != -1 && condition.Status == corev1.ConditionTrue {
		phase = v1alpha1.HelmAppStatusPhaseInstalled
	}
	return phase
}

func (s *Status) isDetected() bool {
	types := []v1alpha1.HelmAppConditionType{
		v1alpha1.HelmAppChartReady,
		v1alpha1.HelmAppPreInstalled,
	}
	for _, t := range types {
		if !s.helmApp.Status.IsConditionTrue(t) {
			return false
		}
	}
	return true
}
