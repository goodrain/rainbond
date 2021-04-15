package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewHelmAppCondition creates a new HelmApp condition.
func NewHelmAppCondition(condType HelmAppConditionType, status corev1.ConditionStatus, reason, message string) *HelmAppCondition {
	return &HelmAppCondition{
		Type:               condType,
		Status:             status,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}
}

// GetCondition returns a HelmApp condition based on the given type.
func (in *HelmAppStatus) GetCondition(t HelmAppConditionType) (int, *HelmAppCondition) {
	for i, c := range in.Conditions {
		if t == c.Type {
			return i, &c
		}
	}
	return -1, nil
}

// SetCondition setups the given HelmApp condition.
func (in *HelmAppStatus) SetCondition(c HelmAppCondition) {
	pos, cp := in.GetCondition(c.Type)
	if cp != nil &&
		cp.Status == c.Status && cp.Reason == c.Reason && cp.Message == c.Message {
		return
	}

	if cp != nil {
		in.Conditions[pos] = c
	} else {
		in.Conditions = append(in.Conditions, c)
	}
}
