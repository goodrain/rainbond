package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func TestGetNodeFilesystemStatsUsesKubeletSummaryProxy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/nodes/worker-1/proxy/stats/summary", r.URL.Path)
		_, err := w.Write([]byte(`{
			"node": {
				"fs": {"capacityBytes": 1000, "availableBytes": 250},
				"runtime": {"imageFs": {"capacityBytes": 2000, "availableBytes": 400}}
			}
		}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	clientset, err := kubernetes.NewForConfig(&rest.Config{Host: server.URL})
	require.NoError(t, err)

	handler := &nodesHandle{clientset: clientset}
	stats, err := handler.getNodeFilesystemStats(context.Background(), "worker-1")
	require.NoError(t, err)
	assert.Equal(t, uint64(750), stats.root.usedBytes)
	assert.Equal(t, uint64(1600), stats.container.usedBytes)
}

func TestGetNodeFilesystemStatsReturnsProxyErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "kubelet unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	clientset, err := kubernetes.NewForConfig(&rest.Config{Host: server.URL})
	require.NoError(t, err)

	handler := &nodesHandle{clientset: clientset}
	_, err = handler.getNodeFilesystemStats(context.Background(), "worker-1")
	require.Error(t, err)
}

func TestListNodesIncludesKubeletFilesystemStats(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/nodes":
			_, err := w.Write([]byte(`{
				"apiVersion": "v1",
				"kind": "NodeList",
				"items": [{
					"metadata": {"name": "worker-1"},
					"status": {
						"capacity": {"cpu": "4", "memory": "8Gi", "ephemeral-storage": "100Gi"},
						"nodeInfo": {"architecture": "amd64", "operatingSystem": "linux"}
					}
				}]
			}`))
			require.NoError(t, err)
		case "/api/v1/nodes/worker-1/proxy/stats/summary":
			_, err := w.Write([]byte(`{
				"node": {
					"fs": {"capacityBytes": 1000, "availableBytes": 250},
					"runtime": {"imageFs": {"capacityBytes": 2000, "availableBytes": 400}}
				}
			}`))
			require.NoError(t, err)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	clientset, err := kubernetes.NewForConfig(&rest.Config{Host: server.URL})
	require.NoError(t, err)

	handler := &nodesHandle{clientset: clientset, prometheusCli: noopPrometheus{}}
	nodes, err := handler.ListNodes(context.Background())
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	assert.Equal(t, uint64(1000), nodes[0].Resource.CapDisk)
	assert.Equal(t, uint64(750), nodes[0].Resource.ReqDisk)
	assert.Equal(t, uint64(2000), nodes[0].Resource.CapContainerDisk)
	assert.Equal(t, uint64(1600), nodes[0].Resource.ReqContainerDisk)
}

func TestParseNodeFilesystemStatsUsesKubeletReportedFilesystems(t *testing.T) {
	raw := []byte(`{
		"node": {
			"fs": {"capacityBytes": 1000, "availableBytes": 250, "usedBytes": 700},
			"runtime": {
				"imageFs": {"capacityBytes": 2000, "availableBytes": 400, "usedBytes": 1500}
			}
		}
	}`)

	stats, err := parseNodeFilesystemStats(raw)
	require.NoError(t, err)
	assert.Equal(t, uint64(1000), stats.root.capacityBytes)
	assert.Equal(t, uint64(750), stats.root.usedBytes)
	assert.Equal(t, uint64(2000), stats.container.capacityBytes)
	assert.Equal(t, uint64(1600), stats.container.usedBytes)
}

func TestParseNodeFilesystemStatsSelectsMostUtilizedRuntimeFilesystem(t *testing.T) {
	raw := []byte(`{
		"node": {
			"fs": {"capacityBytes": 1000, "availableBytes": 500},
			"runtime": {
				"imageFs": {"capacityBytes": 2000, "availableBytes": 800},
				"containerFs": {"capacityBytes": 500, "availableBytes": 50}
			}
		}
	}`)

	stats, err := parseNodeFilesystemStats(raw)
	require.NoError(t, err)
	assert.Equal(t, uint64(500), stats.container.capacityBytes)
	assert.Equal(t, uint64(450), stats.container.usedBytes)
}

func TestParseNodeFilesystemStatsFallsBackToUsedBytes(t *testing.T) {
	raw := []byte(`{
		"node": {
			"fs": {"capacityBytes": 1000, "usedBytes": 300},
			"runtime": {
				"imageFs": {"capacityBytes": 2000, "usedBytes": 600}
			}
		}
	}`)

	stats, err := parseNodeFilesystemStats(raw)
	require.NoError(t, err)
	assert.Equal(t, uint64(300), stats.root.usedBytes)
	assert.Equal(t, uint64(600), stats.container.usedBytes)
}

func TestParseNodeFilesystemStatsDoesNotInventContainerData(t *testing.T) {
	raw := []byte(`{
		"node": {
			"fs": {"capacityBytes": 1000, "availableBytes": 250}
		}
	}`)

	stats, err := parseNodeFilesystemStats(raw)
	require.NoError(t, err)
	assert.Equal(t, uint64(1000), stats.root.capacityBytes)
	assert.Equal(t, uint64(750), stats.root.usedBytes)
	assert.Zero(t, stats.container.capacityBytes)
	assert.Zero(t, stats.container.usedBytes)
}

func TestParseNodeFilesystemStatsRejectsMalformedSummary(t *testing.T) {
	_, err := parseNodeFilesystemStats([]byte(`{"node":`))
	require.Error(t, err)
}
