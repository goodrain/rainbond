package exector

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/goodrain/rainbond/event"
)

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

// capability_id: rainbond.service-check.eventlog-progress
func TestLogServiceCheckProgressMirrorsMessageToEventLogger(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	logger := event.NewMockLogger(ctrl)
	expectedInfo := map[string]string{"step": "service_check", "status": "running"}

	logger.EXPECT().Info("开始检查服务，类型: vm-run", expectedInfo)

	logServiceCheckProgress(logger, "开始检查服务，类型: %s", "vm-run")
}
