package mirror

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// capability_id: rainbond.builder.dynamic-mirror-fetch
func TestFetchCandidates(t *testing.T) {
	validJSON := `{
		"version": 1,
		"updated_at": "2026-06-12T00:00:00Z",
		"mirrors": [
			{"url": "https://docker.1ms.run", "note": "1ms"},
			{"url": "https://docker.m.daocloud.io"},
			{"url": "https://docker.1ms.run", "note": "dup"},
			{"url": "  "},
			{"url": "http://insecure.example.com"}
		]
	}`

	tests := []struct {
		name    string
		body    string
		status  int
		want    []string
		wantErr bool
	}{
		{
			name:   "valid source dedups and drops empty urls",
			body:   validJSON,
			status: http.StatusOK,
			want:   []string{"https://docker.1ms.run", "https://docker.m.daocloud.io", "http://insecure.example.com"},
		},
		{
			name:    "invalid json is an error",
			body:    "not-json",
			status:  http.StatusOK,
			wantErr: true,
		},
		{
			name:    "unsupported version is an error",
			body:    `{"version": 2, "mirrors": [{"url": "https://a.example.com"}]}`,
			status:  http.StatusOK,
			wantErr: true,
		},
		{
			name:    "empty mirror list is an error",
			body:    `{"version": 1, "mirrors": []}`,
			status:  http.StatusOK,
			wantErr: true,
		},
		{
			name:    "http error status is an error",
			body:    validJSON,
			status:  http.StatusBadGateway,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			got, err := FetchCandidates(context.Background(), []string{srv.URL}, 2*time.Second)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			assertStringSlice(t, got, tt.want)
		})
	}
}

// capability_id: rainbond.builder.dynamic-mirror-fetch-fallback
func TestFetchCandidatesFallsBackToNextURL(t *testing.T) {
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer bad.Close()
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"version": 1, "mirrors": [{"url": "https://docker.1ms.run"}]}`))
	}))
	defer good.Close()

	got, err := FetchCandidates(context.Background(), []string{bad.URL, good.URL}, 2*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertStringSlice(t, got, []string{"https://docker.1ms.run"})
}

func TestFetchCandidatesAllSourcesFail(t *testing.T) {
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer bad.Close()

	if _, err := FetchCandidates(context.Background(), []string{bad.URL, "http://127.0.0.1:1"}, time.Second); err == nil {
		t.Fatal("expected error when every source fails")
	}
}

func TestFetchCandidatesNoSources(t *testing.T) {
	if _, err := FetchCandidates(context.Background(), nil, time.Second); err == nil {
		t.Fatal("expected error for empty source list")
	}
}

func assertStringSlice(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}
