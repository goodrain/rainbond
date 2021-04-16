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
	idx, _ := status.GetCondition(v1alpha1.HelmAppChartReady)
	if idx == -1 {
		status.UpdateConditionStatus(v1alpha1.HelmAppChartReady, corev1.ConditionFalse)
	}
	idx, _ = status.GetCondition(v1alpha1.HelmAppPreInstalled)
	if idx == -1 {
		status.UpdateConditionStatus(v1alpha1.HelmAppPreInstalled, corev1.ConditionFalse)
	}
	idx, _ = status.GetCondition(v1alpha1.HelmAppChartParsed)
	if idx == -1 {
		status.UpdateConditionStatus(v1alpha1.HelmAppChartParsed, corev1.ConditionFalse)
	}
	return &Status{
		HelmAppStatus: status,
	}
}

func (s *Status) GetHelmAppStatus() v1alpha1.HelmAppStatus {
	status := s.HelmAppStatus

	status.Phase = s.getPhase()

	return status
}

func (s *Status) getPhase() v1alpha1.HelmAppStatusPhase {
	phase := v1alpha1.HelmAppStatusPhaseInitialing
	if !s.isDetected() {
		phase = v1alpha1.HelmAppStatusPhaseDetecting
	}
	return phase
}

func (s *Status) isDetected() bool {
	types := []v1alpha1.HelmAppConditionType{
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
