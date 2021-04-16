package helmapp

import (
	"github.com/goodrain/rainbond/pkg/apis/rainbond/v1alpha1"
)

type Status struct {
	v1alpha1.HelmAppStatus
}

// NewStatus creates a new helm app status.
func NewStatus(status v1alpha1.HelmAppStatus) *Status {
	return &Status{
		HelmAppStatus: status,
	}
}

func (s *Status) isDetected() bool {
	types := []v1alpha1.HelmAppConditionType{
		v1alpha1.HelmAppRepoReady,
		v1alpha1.HelmAppChartReady,
		v1alpha1.HelmAppPreInstalled,
		v1alpha1.HelmAppChartParsed,
	}
	for _, t := range types {
		if !s.IsConditionTrue(t) {
			return false
		}
	}
	return true
}
