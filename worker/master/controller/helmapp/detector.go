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

// 该文件实现了 Rainbond 平台中 Helm 应用的检测逻辑。Helm 是 Kubernetes 的一个包管理工具，用于定义、安装和管理 Kubernetes 应用程序。
// 本文件的主要职责是通过检测逻辑来确保 Helm 应用的 Chart 和相关资源已准备好进行安装或更新。

// 文件的主要功能包括：

// 1. `Detector` 结构体：
//    - 代表一个 Helm 应用的检测器。该检测器负责验证 Helm 应用的 Chart 和仓库状态，确保它们已经准备好进行后续的安装或更新操作。
//    - 检测器包含 Helm 应用的定义（`HelmApp`）、Helm 仓库（`Repo`）以及应用实例（`App`）等核心组件。

// 2. `NewDetector` 函数：
//    - 创建并返回一个新的 `Detector` 实例。该函数初始化了检测器所需的所有资源，如 Helm 仓库和应用实例。

// 3. `Detect` 方法：
//    - 执行 Helm 应用的检测逻辑。该方法按照以下步骤依次检测和更新 Helm 应用的状态：
//      1. 添加 Helm 仓库：
//         - 检查 Helm 应用的仓库是否已经添加，如果未添加则尝试添加仓库。
//         - 如果仓库添加失败，则更新应用状态为 `HelmAppChartReady` 条件为 `False`，并记录错误信息。
//      2. 加载 Helm Chart：
//         - 检查 Helm Chart 是否已经加载，如果未加载则尝试加载 Chart。
//         - 如果加载失败，则更新应用状态为 `HelmAppChartReady` 条件为 `False`，并记录错误信息；如果加载成功，则将 `HelmAppChartReady` 条件更新为 `True`。
//      3. 预安装检查：
//         - 检查 Helm Chart 是否已经通过预安装检查，如果未通过则尝试执行预安装检查。
//         - 如果预安装检查失败，则更新应用状态为 `HelmAppPreInstalled` 条件为 `False`，并记录错误信息；如果检查通过，则将 `HelmAppPreInstalled` 条件更新为 `True`。

// 通过这个检测逻辑，Rainbond 平台能够在实际安装或更新 Helm 应用之前确保所有相关资源和配置已准备就绪，从而提高部署过程的稳定性和可靠性。

package helmapp

import (
	"github.com/goodrain/rainbond/pkg/apis/rainbond/v1alpha1"
	"github.com/goodrain/rainbond/pkg/helm"
	corev1 "k8s.io/api/core/v1"
)

// Detector is responsible for detecting the helm app.
type Detector struct {
	helmApp *v1alpha1.HelmApp
	repo    *helm.Repo
	app     *App
}

// NewDetector creates a new Detector.
func NewDetector(helmApp *v1alpha1.HelmApp, app *App, repo *helm.Repo) *Detector {
	return &Detector{
		helmApp: helmApp,
		repo:    repo,
		app:     app,
	}
}

// Detect detects the helm app.
func (d *Detector) Detect() error {
	// add repo
	if !d.helmApp.Status.IsConditionTrue(v1alpha1.HelmAppChartReady) {
		appStore := d.helmApp.Spec.AppStore
		if err := d.repo.Add(appStore.Name, appStore.URL, "", ""); err != nil {
			d.helmApp.Status.SetCondition(*v1alpha1.NewHelmAppCondition(
				v1alpha1.HelmAppChartReady, corev1.ConditionFalse, "RepoFailed", err.Error()))
			return err
		}
	}

	// load chart
	if !d.helmApp.Status.IsConditionTrue(v1alpha1.HelmAppChartReady) {
		err := d.app.LoadChart()
		if err != nil {
			d.helmApp.Status.UpdateCondition(v1alpha1.NewHelmAppCondition(
				v1alpha1.HelmAppChartReady, corev1.ConditionFalse, "ChartFailed", err.Error()))
			return err
		}
		d.helmApp.Status.UpdateConditionStatus(v1alpha1.HelmAppChartReady, corev1.ConditionTrue)
		return nil
	}

	// check if the chart is valid
	if !d.helmApp.Status.IsConditionTrue(v1alpha1.HelmAppPreInstalled) {
		if err := d.app.PreInstall(); err != nil {
			d.helmApp.Status.UpdateCondition(v1alpha1.NewHelmAppCondition(
				v1alpha1.HelmAppPreInstalled, corev1.ConditionFalse, "PreInstallFailed", err.Error()))
			return err
		}
		d.helmApp.Status.UpdateConditionStatus(v1alpha1.HelmAppPreInstalled, corev1.ConditionTrue)
		return nil
	}

	return nil
}
