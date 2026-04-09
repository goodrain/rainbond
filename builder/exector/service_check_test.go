package exector

import "testing"

// capability_id: rainbond.service-check.completion-log-summary
func TestServiceCheckCompletionLogSummary(t *testing.T) {
	testCases := []struct {
		name        string
		sourceType  string
		checkStatus string
		want        string
	}{
		{
			name:        "success result keeps success summary",
			sourceType:  "docker-run",
			checkStatus: "Success",
			want:        "check service by type: docker-run completed with check status: success",
		},
		{
			name:        "failure result no longer claims success",
			sourceType:  "vm-run",
			checkStatus: "Failure",
			want:        "check service by type: vm-run completed with check status: failure",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := serviceCheckCompletionLogSummary(tc.sourceType, tc.checkStatus)
			if got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}
