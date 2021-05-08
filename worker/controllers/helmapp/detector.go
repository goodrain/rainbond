package helmapp

import (
	"github.com/goodrain/rainbond/pkg/apis/rainbond/v1alpha1"
	"github.com/goodrain/rainbond/worker/controllers/helmapp/helm"
	corev1 "k8s.io/api/core/v1"
)

type Detector struct {
	helmApp *v1alpha1.HelmApp
	repo    *helm.Repo
	app     *App
}

func NewDetector(helmApp *v1alpha1.HelmApp, app *App, repo *helm.Repo) *Detector {
	return &Detector{
		helmApp: helmApp,
		repo:    repo,
		app:     app,
	}
}

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
