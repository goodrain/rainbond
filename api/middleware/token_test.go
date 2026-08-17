package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

type tokenTestBody struct {
	closed bool
}

func (*tokenTestBody) Read([]byte) (int, error) {
	return 0, io.EOF
}

func (b *tokenTestBody) Close() error {
	b.closed = true
	return nil
}

// capability_id: rainbond.region-api.configured-token-only
func TestFullTokenAllowsOnlyConfiguredRegionToken(t *testing.T) {
	t.Setenv("TOKEN", "cluster-token")

	testCases := []struct {
		name            string
		configuredToken string
		authorization   string
		wantStatus      int
		wantCalled      bool
		wantBodyClosed  bool
	}{
		{
			name:            "configured token scheme",
			configuredToken: "cluster-token",
			authorization:   "Token cluster-token",
			wantStatus:      http.StatusNoContent,
			wantCalled:      true,
		},
		{
			name:            "configured bearer scheme",
			configuredToken: "cluster-token",
			authorization:   "Bearer cluster-token",
			wantStatus:      http.StatusNoContent,
			wantCalled:      true,
		},
		{
			name:            "legacy enterprise token",
			configuredToken: "cluster-token",
			authorization:   "Token legacy-enterprise-token",
			wantStatus:      http.StatusUnauthorized,
			wantBodyClosed:  true,
		},
		{
			name:            "token disabled",
			configuredToken: "",
			authorization:   "Token legacy-enterprise-token",
			wantStatus:      http.StatusUnauthorized,
			wantBodyClosed:  true,
		},
		{
			name:            "unknown token",
			configuredToken: "cluster-token",
			authorization:   "Token unknown",
			wantStatus:      http.StatusUnauthorized,
			wantBodyClosed:  true,
		},
		{
			name:            "missing authorization",
			configuredToken: "cluster-token",
			wantStatus:      http.StatusUnauthorized,
			wantBodyClosed:  true,
		},
		{
			name:            "malformed authorization",
			configuredToken: "cluster-token",
			authorization:   "Token cluster-token extra",
			wantStatus:      http.StatusUnauthorized,
			wantBodyClosed:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("TOKEN", tc.configuredToken)
			called := false
			next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				called = true
				w.WriteHeader(http.StatusNoContent)
			})
			body := &tokenTestBody{}
			req := httptest.NewRequest(http.MethodGet, "/v2/tenants/target/services", nil)
			req.Body = body
			if tc.authorization != "" {
				req.Header.Set("Authorization", tc.authorization)
			}
			resp := httptest.NewRecorder()

			FullToken(next).ServeHTTP(resp, req)

			if resp.Code != tc.wantStatus {
				t.Fatalf("expected status %d, got %d", tc.wantStatus, resp.Code)
			}
			if called != tc.wantCalled {
				t.Fatalf("expected handler called=%t, got %t", tc.wantCalled, called)
			}
			if body.closed != tc.wantBodyClosed {
				t.Fatalf("expected body closed=%t, got %t", tc.wantBodyClosed, body.closed)
			}
		})
	}
}
