package controller

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	httputil "github.com/goodrain/rainbond/util/http"
)

func TestReturnAgentKubeconfigBootstrapError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{
			name:       "parameter error",
			err:        httputil.NewErrBadRequest(errors.New("unsupported credential_profile")),
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "runtime error",
			err:        errors.New("create service account token"),
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/v2/cluster/agent-kubernetes/bootstrap-credential", nil)
			response := httptest.NewRecorder()

			returnAgentKubeconfigBootstrapError(request, response, tt.err)

			if response.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d: %s", tt.wantStatus, response.Code, response.Body.String())
			}
		})
	}
}
