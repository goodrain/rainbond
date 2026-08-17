package prometheus

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetMetricStopsAtConfiguredQueryTimeout(t *testing.T) {
	t.Setenv("PROMETHEUS_QUERY_TIMEOUT", "50ms")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
			return
		case <-time.After(time.Second):
			_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[]}}`))
		}
	}))
	defer server.Close()
	client, err := NewPrometheus(&Options{Endpoint: server.URL})
	require.NoError(t, err)

	started := time.Now()
	metric := client.GetMetric("up", time.Now())

	assert.Less(t, time.Since(started), 500*time.Millisecond)
	assert.True(t, strings.Contains(metric.Error, "deadline exceeded") || strings.Contains(metric.Error, "context deadline"), metric.Error)
}

func TestMetadataAndSeriesQueriesStopAtConfiguredQueryTimeout(t *testing.T) {
	t.Setenv("PROMETHEUS_QUERY_TIMEOUT", "30ms")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
			return
		case <-time.After(200 * time.Millisecond):
			_, _ = w.Write([]byte(`{"status":"success","data":[]}`))
		}
	}))
	defer server.Close()
	client, err := NewPrometheus(&Options{Endpoint: server.URL})
	require.NoError(t, err)

	tests := []struct {
		name string
		call func()
	}{
		{name: "tenant metadata", call: func() { client.GetMetadata("tenant-ns") }},
		{name: "application metadata", call: func() { client.GetAppMetadata("tenant-ns", "app-id") }},
		{name: "component metadata", call: func() { client.GetComponentMetadata("tenant-ns", "component-id") }},
		{name: "metric label set", call: func() { client.GetMetricLabelSet("up", time.Now().Add(-time.Minute), time.Now()) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			started := time.Now()
			tt.call()
			assert.Less(t, time.Since(started), 150*time.Millisecond)
		})
	}
}
