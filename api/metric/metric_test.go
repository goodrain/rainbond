package metric

import (
	"fmt"
	"strings"
	"testing"

	"github.com/goodrain/rainbond/api/handler"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestClusterPodMetricsUseStablePodUIDLabel(t *testing.T) {
	exporter := NewExporter()
	exporter.setClusterPodMetrics([]handler.PodResourceInformation{
		{NodeName: "node-a", Namespace: "tenant-a", PodName: "same-name", PodUID: "uid-a", ResourceVersion: "101", Memory: 1, CPU: 1, StorageEphemeral: 1},
		{NodeName: "node-a", Namespace: "tenant-b", PodName: "same-name", PodUID: "uid-b", ResourceVersion: "202", Memory: 2, CPU: 2, StorageEphemeral: 2},
	})
	tests := []struct {
		name      string
		help      string
		metricVec *prometheus.GaugeVec
	}{
		{name: "memory", help: "rainbond cluster pod memory", metricVec: exporter.clusterPodMemory},
		{name: "cpu", help: "rainbond cluster pod CPU", metricVec: exporter.clusterPodCPU},
		{name: "ephemeral_storage", help: "rainbond cluster pod StorageEphemeral", metricVec: exporter.clusterPodStorageEphemeral},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			metricName := "rbd_api_exporter_cluster_pod_" + tc.name
			expected := fmt.Sprintf(`# HELP %[1]s %[2]s
# TYPE %[1]s gauge
%[1]s{app_id="",node_name="node-a",pod_uid="uid-a",service_id=""} 1
%[1]s{app_id="",node_name="node-a",pod_uid="uid-b",service_id=""} 2
`, metricName, tc.help)
			if err := testutil.CollectAndCompare(tc.metricVec, strings.NewReader(expected), metricName); err != nil {
				t.Fatalf("unexpected pod metric series: %v", err)
			}
		})
	}
}

func TestClusterPodMetricsUseNamespaceAndNameFallbackForEmptyUID(t *testing.T) {
	exporter := NewExporter()
	exporter.setClusterPodMetrics([]handler.PodResourceInformation{
		{NodeName: "node-a", Namespace: "tenant-a", PodName: "same-name", Memory: 1},
		{NodeName: "node-a", Namespace: "tenant-b", PodName: "same-name", Memory: 2},
	})

	const expected = `# HELP rbd_api_exporter_cluster_pod_memory rainbond cluster pod memory
# TYPE rbd_api_exporter_cluster_pod_memory gauge
rbd_api_exporter_cluster_pod_memory{app_id="",node_name="node-a",pod_uid="namespace_name:tenant-a/same-name",service_id=""} 1
rbd_api_exporter_cluster_pod_memory{app_id="",node_name="node-a",pod_uid="namespace_name:tenant-b/same-name",service_id=""} 2
`
	if err := testutil.CollectAndCompare(exporter.clusterPodMemory, strings.NewReader(expected), "rbd_api_exporter_cluster_pod_memory"); err != nil {
		t.Fatalf("unexpected empty UID fallback series: %v", err)
	}
}
