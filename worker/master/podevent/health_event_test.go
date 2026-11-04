package podevent

import (
	"testing"
	"time"

	"github.com/goodrain/rainbond/db/model"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// capability_id: rainbond.lifecycle.stop-suppresses-exit-event
func TestShouldReportContainerExit(t *testing.T) {
	now := metav1.NewTime(time.Now())
	tests := []struct {
		name       string
		pod        *corev1.Pod
		terminated *corev1.ContainerStateTerminated
		want       bool
	}{
		{
			name:       "active pod non-zero exit",
			pod:        &corev1.Pod{},
			terminated: &corev1.ContainerStateTerminated{ExitCode: 255, Reason: "Error"},
			want:       true,
		},
		{
			name:       "deleting pod non-zero exit",
			pod:        &corev1.Pod{ObjectMeta: metav1.ObjectMeta{DeletionTimestamp: &now}},
			terminated: &corev1.ContainerStateTerminated{ExitCode: 255, Reason: "Error"},
			want:       false,
		},
		{
			name:       "successful exit",
			pod:        &corev1.Pod{},
			terminated: &corev1.ContainerStateTerminated{ExitCode: 0, Reason: "Completed"},
			want:       false,
		},
		{
			name:       "oom exit handled separately",
			pod:        &corev1.Pod{},
			terminated: &corev1.ContainerStateTerminated{ExitCode: 137, Reason: "OOMKilled"},
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldReportContainerExit(tt.pod, tt.terminated); got != tt.want {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}
