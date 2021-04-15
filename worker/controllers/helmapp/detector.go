package helmapp

import (
	"github.com/goodrain/rainbond/pkg/apis/rainbond/v1alpha1"
	"github.com/goodrain/rainbond/worker/controllers/helmapp/helm"
	corev1 "k8s.io/api/core/v1"
)

type Detector struct {
	helmApp *v1alpha1.HelmApp
	status  *Status
	repo    *helm.Repo
}

func NewDetector(helmApp *v1alpha1.HelmApp, status *Status, repo *helm.Repo) *Detector {
	return &Detector{
		helmApp: helmApp,
		status:  status,
		repo:    repo,
	}
}

func (d *Detector) Detect() error {
	if d.status.isDetected() {
		return nil
	}

	// add repo
	if !d.status.IsConditionTrue(v1alpha1.HelmAppRepoReady) {
		appStore := d.helmApp.Spec.AppStore
		if err := d.repo.Add(appStore.Name, appStore.URL, "", ""); err != nil {
			d.status.SetCondition(*v1alpha1.NewHelmAppCondition(
				v1alpha1.HelmAppRepoReady, corev1.ConditionFalse, "RepoFailed", err.Error()))
			return err
		}
		d.status.UpdateConditionStatus(v1alpha1.HelmAppRepoReady, corev1.ConditionTrue)
	}

	return nil
}
