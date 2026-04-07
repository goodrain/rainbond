package v1alpha1

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

// capability_id: rainbond.worker.helmapp.condition-lifecycle
func TestHelmAppStatusConditionLifecycle(t *testing.T) {
	status := &HelmAppStatus{}

	condition := NewHelmAppCondition(HelmAppChartReady, corev1.ConditionFalse, "Loading", "chart is loading")
	if condition == nil || condition.Type != HelmAppChartReady || condition.Status != corev1.ConditionFalse {
		t.Fatalf("unexpected condition: %+v", condition)
	}

	changed := status.UpdateCondition(condition)
	if !changed {
		t.Fatal("expected initial condition insert to report changed")
	}
	_, got := status.GetCondition(HelmAppChartReady)
	if got == nil || got.Reason != "Loading" {
		t.Fatalf("unexpected stored condition: %+v", got)
	}
	if status.IsConditionTrue(HelmAppChartReady) {
		t.Fatal("condition should not be true yet")
	}

	status.UpdateConditionStatus(HelmAppChartReady, corev1.ConditionTrue)
	if !status.IsConditionTrue(HelmAppChartReady) {
		t.Fatal("expected chart ready condition to become true")
	}
	_, got = status.GetCondition(HelmAppChartReady)
	if got == nil || got.Reason != "" || got.Message != "" {
		t.Fatalf("expected cleared reason/message after success, got %+v", got)
	}
}

// capability_id: rainbond.worker.helmapp.condition-query
func TestHelmAppStatusConditionQuery(t *testing.T) {
	status := &HelmAppStatus{
		Conditions: []HelmAppCondition{
			{Type: HelmAppChartReady, Status: corev1.ConditionTrue},
			{Type: HelmAppInstalled, Status: corev1.ConditionFalse},
		},
	}

	idx, cond := status.GetCondition(HelmAppChartReady)
	if idx != 0 || cond == nil || cond.Type != HelmAppChartReady {
		t.Fatalf("unexpected condition lookup result: idx=%d cond=%+v", idx, cond)
	}
	if !status.IsConditionTrue(HelmAppChartReady) {
		t.Fatal("expected chart ready condition to be true")
	}
	if status.IsConditionTrue(HelmAppInstalled) {
		t.Fatal("expected installed condition to be false")
	}
}

// capability_id: rainbond.worker.helmapp.condition-transition-time
func TestHelmAppStatusUpdateConditionPreservesTransitionTimeOnSameStatus(t *testing.T) {
	status := &HelmAppStatus{}
	condition := NewHelmAppCondition(HelmAppChartReady, corev1.ConditionFalse, "Loading", "chart is loading")
	status.UpdateCondition(condition)

	_, old := status.GetCondition(HelmAppChartReady)
	if old == nil {
		t.Fatal("expected stored condition")
	}
	oldTime := old.LastTransitionTime

	condition = NewHelmAppCondition(HelmAppChartReady, corev1.ConditionFalse, "StillLoading", "still loading")
	status.UpdateCondition(condition)
	_, updated := status.GetCondition(HelmAppChartReady)
	if updated == nil {
		t.Fatal("expected updated condition")
	}
	if !updated.LastTransitionTime.Equal(&oldTime) {
		t.Fatalf("expected transition time to be preserved, old=%v new=%v", oldTime, updated.LastTransitionTime)
	}
}

// capability_id: rainbond.worker.helmapp.condition-set-noop
func TestHelmAppStatusSetConditionDoesNotDuplicateUnchangedCondition(t *testing.T) {
	status := &HelmAppStatus{
		Conditions: []HelmAppCondition{{
			Type:    HelmAppChartReady,
			Status:  corev1.ConditionTrue,
			Reason:  "Ready",
			Message: "chart ready",
		}},
	}

	status.SetCondition(HelmAppCondition{
		Type:    HelmAppChartReady,
		Status:  corev1.ConditionTrue,
		Reason:  "Ready",
		Message: "chart ready",
	})
	if len(status.Conditions) != 1 {
		t.Fatalf("expected single unchanged condition, got %d", len(status.Conditions))
	}
}

// capability_id: rainbond.worker.helmapp.condition-status-default-create
func TestHelmAppStatusUpdateConditionStatusCreatesMissingCondition(t *testing.T) {
	status := &HelmAppStatus{}
	status.UpdateConditionStatus(HelmAppInstalled, corev1.ConditionFalse)
	_, cond := status.GetCondition(HelmAppInstalled)
	if cond == nil || cond.Status != corev1.ConditionFalse {
		t.Fatalf("expected created false condition, got %+v", cond)
	}
}

// capability_id: rainbond.worker.helmapp.overrides-compare
func TestHelmAppOverridesEqual(t *testing.T) {
	app := &HelmApp{
		Spec: HelmAppSpec{
			Overrides: []string{"replicaCount=2", "service.type=ClusterIP"},
		},
		Status: HelmAppStatus{
			Overrides: []string{"service.type=ClusterIP", "replicaCount=2"},
		},
	}
	if !app.OverridesEqual() {
		t.Fatal("expected overrides to be equal regardless of order")
	}

	app.Status.Overrides = []string{"replicaCount=3", "service.type=ClusterIP"}
	if app.OverridesEqual() {
		t.Fatal("expected overrides mismatch to be detected")
	}
}

// capability_id: rainbond.worker.helmapp.store-full-name
func TestHelmAppSpecFullName(t *testing.T) {
	spec := &HelmAppSpec{
		EID: "eid-1",
		AppStore: &HelmAppStore{
			Name: "bitnami",
		},
	}
	if got := spec.FullName(); got != "eid-1-bitnami" {
		t.Fatalf("unexpected full name: %q", got)
	}

	spec.AppStore = nil
	if got := spec.FullName(); got != "" {
		t.Fatalf("expected empty full name without app store, got %q", got)
	}
}
