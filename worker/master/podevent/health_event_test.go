package podevent

import (
	"testing"

	"github.com/goodrain/rainbond/db/model"
)

func TestHealthCheckFailureEventStateIsChecking(t *testing.T) {
	healthEvents := []EventType{
		EventTypeReadinessUnhealthy,
		EventTypeLivenessRestart,
		EventTypeStartupProbeFailure,
	}

	for _, eventType := range healthEvents {
		status, finalStatus := healthCheckFailureEventState(eventType)
		if status != model.EventStatusChecking.String() {
			t.Fatalf("expected %s status %q, got %q", eventType, model.EventStatusChecking.String(), status)
		}
		if finalStatus != model.EventFinalStatusRunning.String() {
			t.Fatalf("expected %s final status %q, got %q", eventType, model.EventFinalStatusRunning.String(), finalStatus)
		}
	}
}

func TestNonHealthCheckFailureEventStateStaysFailureEmpty(t *testing.T) {
	status, finalStatus := healthCheckFailureEventState(EventTypeCrashLoopBackOff)

	if status != model.EventStatusFailure.String() {
		t.Fatalf("expected non-health failure status %q, got %q", model.EventStatusFailure.String(), status)
	}
	if finalStatus != model.EventFinalStatusEmpty.String() {
		t.Fatalf("expected non-health final status %q, got %q", model.EventFinalStatusEmpty.String(), finalStatus)
	}
}
