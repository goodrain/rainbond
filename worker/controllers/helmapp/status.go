package helmapp

import (
	"github.com/goodrain/rainbond/pkg/apis/rainbond/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

type Status struct {
	v1alpha1.HelmAppStatus
}

// NewStatus creates a new helm app status.
func NewStatus(status v1alpha1.HelmAppStatus) *Status {
	idx, _ := status.GetCondition(v1alpha1.HelmAppInstalled)
	if idx == -1 {
		status.SetCondition(*v1alpha1.NewHelmAppCondition(
			v1alpha1.HelmAppInstalled,
			corev1.ConditionFalse,
			"",
			"",
		))
	}
	return &Status{
		HelmAppStatus: status,
	}
}

func (s *Status) isDetected() bool {
	types := []v1alpha1.HelmAppConditionType{
		v1alpha1.HelmAppRepoReady,
		v1alpha1.HelmAppChartReady,
	}
	for _, t := range types {
		if !s.IsConditionTrue(t) {
			return false
		}
	}
	return true
}
