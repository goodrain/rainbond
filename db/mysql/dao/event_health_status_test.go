package dao

import (
	"testing"

	"github.com/goodrain/rainbond/db/model"
)

func TestHealthCheckEventStatusesIncludeCheckingForLookups(t *testing.T) {
	statuses := abnormalEventStatuses("ReadinessUnhealthy")

	if len(statuses) != 2 {
		t.Fatalf("expected health check event lookup to include two statuses, got %v", statuses)
	}
	if statuses[0] != model.EventStatusFailure.String() {
		t.Fatalf("expected failure status first for compatibility, got %q", statuses[0])
	}
	if statuses[1] != model.EventStatusChecking.String() {
		t.Fatalf("expected checking status for health check event lookup, got %q", statuses[1])
	}
}

func TestNonHealthCheckEventStatusesStayFailureOnly(t *testing.T) {
	statuses := abnormalEventStatuses("CrashLoopBackOff")

	if len(statuses) != 1 || statuses[0] != model.EventStatusFailure.String() {
		t.Fatalf("expected non-health event lookup to stay failure-only, got %v", statuses)
	}
}
