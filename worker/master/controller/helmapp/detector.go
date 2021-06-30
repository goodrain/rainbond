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
