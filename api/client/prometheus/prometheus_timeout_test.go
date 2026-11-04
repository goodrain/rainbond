package prometheus

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// capability_id: rainbond.api.apiserver-request-metrics
func TestGetMetricCallerContextPreservesSuccessfulResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[{"metric":{},"value":[1,"1"]}]}}`))
	}))
	defer server.Close()
	client, err := NewPrometheus(&Options{Endpoint: server.URL})
	require.NoError(t, err)
	metric := client.GetMetric("up", time.Now())
	require.Empty(t, metric.Error)
	require.Equal(t, MetricTypeVector, metric.MetricType)
	require.Len(t, metric.MetricValues, 1)
	require.NotNil(t, metric.MetricValues[0].Sample)
	assert.Equal(t, float64(1), metric.MetricValues[0].Sample.Value())
}

// capability_id: rainbond.api.apiserver-request-metrics
func TestGetMetricHonorsCallerDeadline(t *testing.T) {
	t.Setenv("PROMETHEUS_QUERY_TIMEOUT", "5s")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(time.Second):
			_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[]}}`))
		}
	}))
	defer server.Close()
	client, err := NewPrometheus(&Options{Endpoint: server.URL})
	require.NoError(t, err)
	reader, ok := client.(interface {
		GetMetricWithContext(context.Context, string, time.Time) Metric
	})
	require.True(t, ok, "a scrape caller must be able to enforce its own total collection deadline")
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	started := time.Now()
	metric := reader.GetMetricWithContext(ctx, "up", time.Now())
	assert.Less(t, time.Since(started), 500*time.Millisecond)
	assert.Contains(t, metric.Error, "context deadline")
}

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
