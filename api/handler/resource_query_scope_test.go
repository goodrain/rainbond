package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	promcli "github.com/goodrain/rainbond/api/client/prometheus"
	apimodel "github.com/goodrain/rainbond/api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/goodrain/rainbond/worker/server/pb"
)

type recordingPrometheus struct {
	mu      sync.Mutex
	queries []string
	metrics map[string]promcli.Metric
	metric  func(string) promcli.Metric
}

type coordinatedServiceResourceRuntime struct {
	diskStarted   <-chan struct{}
	releaseDisk   chan<- struct{}
	wasConcurrent chan<- bool
}

func (r *coordinatedServiceResourceRuntime) GetStatuss(string) map[string]string {
	concurrent := false
	select {
	case <-r.diskStarted:
		concurrent = true
	case <-time.After(time.Second):
	}
	r.wasConcurrent <- concurrent
	return map[string]string{"component-a": "running"}
}

func (r *coordinatedServiceResourceRuntime) IsClosedStatus(status string) bool {
	return status == "closed"
}

func (r *coordinatedServiceResourceRuntime) GetMultiServicePods([]string) (*pb.MultiServiceAppPodList, error) {
	close(r.releaseDisk)
	return &pb.MultiServiceAppPodList{ServicePods: map[string]*pb.ServiceAppPodList{
		"component-a": {
			NewPods: []*pb.ServiceAppPod{{Containers: map[string]*pb.Container{
				"main": {MemoryRequest: 64 * 1024 * 1024, CpuRequest: 200},
			}}},
		},
	}}, nil
}

func (p *recordingPrometheus) GetMetric(expr string, _ time.Time) promcli.Metric {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.queries = append(p.queries, expr)
	if p.metric != nil {
		return p.metric(expr)
	}
	for contains, metric := range p.metrics {
		if strings.Contains(expr, contains) {
			return metric
		}
	}
	return promcli.Metric{}
}

func (p *recordingPrometheus) GetMetricOverTime(string, time.Time, time.Time, time.Duration) promcli.Metric {
	return promcli.Metric{}
}

func (p *recordingPrometheus) GetMetadata(string) []promcli.Metadata { return nil }

func (p *recordingPrometheus) GetAppMetadata(string, string) []promcli.Metadata { return nil }

func (p *recordingPrometheus) GetComponentMetadata(string, string) []promcli.Metadata { return nil }

func (p *recordingPrometheus) GetMetricLabelSet(string, time.Time, time.Time) []map[string]string {
	return nil
}

func metricValue(labels map[string]string, value float64) promcli.MetricValue {
	point := promcli.Point{0, value}
	return promcli.MetricValue{Metadata: labels, Sample: &point}
}

func vectorMetric(values ...promcli.MetricValue) promcli.Metric {
	return promcli.Metric{MetricData: promcli.MetricData{MetricValues: values}}
}

func TestGetDiskUsageScopesPrometheusQueryToRequestedApplications(t *testing.T) {
	prom := &recordingPrometheus{metrics: map[string]promcli.Metric{
		"app_resource_appfs": vectorMetric(metricValue(map[string]string{"app_id": "app-a"}, 42)),
	}}
	action := &ApplicationAction{promClient: prom}

	usage := action.getDiskUsage([]string{"app-b", "app-a", "app-a"})

	require.Len(t, prom.queries, 1)
	assert.Equal(t, `max(app_resource_appfs{app_id=~"^(app-a|app-b)$"}) by(app_id)`, prom.queries[0])
	assert.Equal(t, float64(42), usage["app-a"])
}

func TestGetDiskUsageUsesEqualityMatcherForOneApplication(t *testing.T) {
	prom := &recordingPrometheus{}
	action := &ApplicationAction{promClient: prom}

	action.getDiskUsage([]string{"app.a"})

	require.Len(t, prom.queries, 1)
	assert.Equal(t, `max(app_resource_appfs{app_id="app.a"}) by(app_id)`, prom.queries[0])
}

func TestGetDiskUsageSkipsPrometheusForEmptyApplicationList(t *testing.T) {
	prom := &recordingPrometheus{}
	action := &ApplicationAction{promClient: prom}

	usage := action.getDiskUsage(nil)

	assert.Empty(t, usage)
	assert.Empty(t, prom.queries)
}

func TestGetServicesDiskDeprecatedScopesPrometheusQueryToRequestedComponents(t *testing.T) {
	prom := &recordingPrometheus{metrics: map[string]promcli.Metric{
		"app_resource_appfs": vectorMetric(metricValue(map[string]string{"service_id": "component.a"}, 24)),
	}}

	usage := GetServicesDiskDeprecated([]string{"component-b", "component.a", "component.a", ""}, prom)

	require.Len(t, prom.queries, 1)
	assert.Equal(t, `max(app_resource_appfs{service_id=~"^(component-b|component\.a)$"}) by(service_id)`, prom.queries[0])
	assert.Equal(t, float64(24), usage["component.a"])
}

func TestGetServicesDiskDeprecatedSkipsPrometheusForEmptyComponentList(t *testing.T) {
	prom := &recordingPrometheus{}

	usage := GetServicesDiskDeprecated([]string{"", ""}, prom)

	assert.Empty(t, usage)
	assert.Empty(t, prom.queries)
}

func TestGetServicesDiskDeprecatedUsesEqualityMatcherForOneComponent(t *testing.T) {
	prom := &recordingPrometheus{}

	GetServicesDiskDeprecated([]string{"component.a"}, prom)

	require.Len(t, prom.queries, 1)
	assert.Equal(t, `max(app_resource_appfs{service_id="component.a"}) by(service_id)`, prom.queries[0])
}

func TestGetServicesResourcesQueriesDiskConcurrentlyWithRuntime(t *testing.T) {
	diskStarted := make(chan struct{})
	releaseDisk := make(chan struct{})
	wasConcurrent := make(chan bool, 1)
	prom := &recordingPrometheus{metric: func(string) promcli.Metric {
		close(diskStarted)
		<-releaseDisk
		return vectorMetric(metricValue(map[string]string{"service_id": "component-a"}, 2048))
	}}
	runtime := &coordinatedServiceResourceRuntime{
		diskStarted:   diskStarted,
		releaseDisk:   releaseDisk,
		wasConcurrent: wasConcurrent,
	}
	request := &apimodel.ServicesResources{}
	request.Body.ServiceIDs = []string{"component-a"}

	resources, err := getServicesResources(request, runtime, prom)

	require.NoError(t, err)
	assert.True(t, <-wasConcurrent)
	assert.Equal(t, int64(64), resources["component-a"]["memory"])
	assert.Equal(t, int64(200), resources["component-a"]["cpu"])
	assert.Equal(t, float64(2), resources["component-a"]["disk"])
}

func TestPopulatePodContainerMemoryUsesOneNamespaceScopedExactQuery(t *testing.T) {
	prom := &recordingPrometheus{metric: func(string) promcli.Metric {
		return vectorMetric(metricValue(map[string]string{
			"pod":       "component-a.0",
			"container": "main",
		}, 128))
	}}
	action := &ServiceAction{prometheusCli: prom}
	pods := []*K8sPodInfo{
		{PodName: "component-b", Container: map[string]map[string]string{"main": {"memory_usage": "0"}}},
		{PodName: "component-a.0", Container: map[string]map[string]string{"main": {"memory_usage": "0"}}},
	}

	action.populatePodContainerMemory("tenant-ns", pods)

	require.Len(t, prom.queries, 1)
	assert.Equal(t, `max(container_memory_rss{namespace="tenant-ns",pod=~"^(component-a\.0|component-b)$"}) by (pod,container)`, prom.queries[0])
	assert.Equal(t, "128", pods[1].Container["main"]["memory_usage"])
}

func TestPopulatePodContainerMemoryUsesEqualityMatcherForOnePod(t *testing.T) {
	prom := &recordingPrometheus{metric: func(string) promcli.Metric {
		return vectorMetric(metricValue(map[string]string{
			"pod":       "component-a.0",
			"container": "main",
		}, 128))
	}}
	action := &ServiceAction{prometheusCli: prom}
	pods := []*K8sPodInfo{
		{PodName: "component-a.0", Container: map[string]map[string]string{"main": {"memory_usage": "0"}}},
	}

	action.populatePodContainerMemory("tenant-ns", pods)

	require.Len(t, prom.queries, 1)
	assert.Equal(t, `max(container_memory_rss{namespace="tenant-ns",pod="component-a.0"}) by (pod,container)`, prom.queries[0])
}

func TestPopulatePodContainerMemoryFallsBackWhenNamespaceDoesNotMatchMetrics(t *testing.T) {
	prom := &recordingPrometheus{metric: func(expr string) promcli.Metric {
		if strings.Contains(expr, `namespace="stale-namespace"`) {
			return promcli.Metric{}
		}
		return vectorMetric(metricValue(map[string]string{
			"pod":       "component-a.0",
			"container": "main",
		}, 256))
	}}
	action := &ServiceAction{prometheusCli: prom}
	pods := []*K8sPodInfo{
		{PodName: "component-a.0", Container: map[string]map[string]string{"main": {"memory_usage": "0"}}},
	}

	action.populatePodContainerMemory("stale-namespace", pods)

	require.Len(t, prom.queries, 2)
	assert.Equal(t, `max(container_memory_rss{namespace="stale-namespace",pod="component-a.0"}) by (pod,container)`, prom.queries[0])
	assert.Equal(t, `max(container_memory_rss{pod="component-a.0"}) by (pod,container)`, prom.queries[1])
	assert.Equal(t, "256", pods[0].Container["main"]["memory_usage"])
}

func TestGetPodContainerMemoryDoesNotRetryPrometheusErrors(t *testing.T) {
	prom := &recordingPrometheus{metric: func(string) promcli.Metric {
		return promcli.Metric{Error: "query timeout"}
	}}
	action := &ServiceAction{prometheusCli: prom}

	usage, err := action.getPodContainerMemory("tenant-ns", []string{"component-a.0"})

	assert.Empty(t, usage)
	assert.EqualError(t, err, "query pod container memory: query timeout")
	assert.Len(t, prom.queries, 1)
}

func TestPopulatePodContainerMemorySkipsPrometheusWithoutPods(t *testing.T) {
	prom := &recordingPrometheus{}
	action := &ServiceAction{prometheusCli: prom}

	action.populatePodContainerMemory("tenant-ns", nil)

	assert.Empty(t, prom.queries)
}

func TestResolvePodMetricsNamespaceUsesTenantNamespaceForRainbondComponent(t *testing.T) {
	assert.Equal(t, "team-ns", resolvePodMetricsNamespace("tenant-uuid", "tenant-uuid", "team-ns"))
}

func TestResolvePodMetricsNamespacePreservesImportedComponentNamespace(t *testing.T) {
	assert.Equal(t, "imported-ns", resolvePodMetricsNamespace("imported-ns", "tenant-uuid", "team-ns"))
}

func TestResolvePodMetricsNamespaceFallsBackToUnscopedQuery(t *testing.T) {
	assert.Empty(t, resolvePodMetricsNamespace("tenant-uuid", "tenant-uuid", ""))
}

func TestListNodesAggregatesRequestedResourcesWithThreePrometheusQueries(t *testing.T) {
	nodes := []corev1.Node{
		newResourceQueryNode("node-a"),
		newResourceQueryNode("node-b"),
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/nodes":
			require.NoError(t, json.NewEncoder(w).Encode(&corev1.NodeList{
				TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "NodeList"},
				Items:    nodes,
			}))
		case strings.HasSuffix(r.URL.Path, "/proxy/stats/summary"):
			_, err := w.Write([]byte(`{"node":{"fs":{},"runtime":{"imageFs":{}}}}`))
			require.NoError(t, err)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client, err := kubernetes.NewForConfig(&rest.Config{Host: server.URL})
	require.NoError(t, err)
	prom := &recordingPrometheus{metrics: map[string]promcli.Metric{
		"cluster_pod_memory": vectorMetric(
			metricValue(map[string]string{"node_name": "node-a"}, 256*1024*1024),
			metricValue(map[string]string{"node_name": "node-b"}, 512*1024*1024),
		),
		"cluster_pod_cpu": vectorMetric(
			metricValue(map[string]string{"node_name": "node-a"}, 500),
			metricValue(map[string]string{"node_name": "node-b"}, 1000),
		),
		"cluster_pod_ephemeral_storage": vectorMetric(
			metricValue(map[string]string{"node_name": "node-a"}, 1024*1024),
			metricValue(map[string]string{"node_name": "node-b"}, 2*1024*1024),
		),
	}}
	handler := &nodesHandle{clientset: client, prometheusCli: prom}

	result, err := handler.ListNodes(context.Background())

	require.NoError(t, err)
	require.Len(t, result, 2)
	require.Len(t, prom.queries, 3)
	for _, query := range prom.queries {
		assert.Contains(t, query, "by (node_name, instance)")
		assert.NotContains(t, query, `node_name="`)
	}
	assert.Equal(t, 256, result[0].Resource.ReqMemory)
	assert.Equal(t, float32(0.5), result[0].Resource.ReqCPU)
	assert.Equal(t, float32(1), result[0].Resource.ReqStorageEq)
	assert.Equal(t, 512, result[1].Resource.ReqMemory)
	assert.Equal(t, float32(1), result[1].Resource.ReqCPU)
	assert.Equal(t, float32(2), result[1].Resource.ReqStorageEq)
}

func TestListNodesFetchesFilesystemStatsConcurrently(t *testing.T) {
	nodes := []corev1.Node{
		newResourceQueryNode("node-a"),
		newResourceQueryNode("node-b"),
		newResourceQueryNode("node-c"),
		newResourceQueryNode("node-d"),
	}
	active := 0
	maxActive := 0
	var activeMu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/nodes":
			require.NoError(t, json.NewEncoder(w).Encode(&corev1.NodeList{
				TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "NodeList"},
				Items:    nodes,
			}))
		case strings.HasSuffix(r.URL.Path, "/proxy/stats/summary"):
			activeMu.Lock()
			active++
			if active > maxActive {
				maxActive = active
			}
			activeMu.Unlock()
			time.Sleep(30 * time.Millisecond)
			activeMu.Lock()
			active--
			activeMu.Unlock()
			_, err := w.Write([]byte(`{"node":{"fs":{},"runtime":{"imageFs":{}}}}`))
			require.NoError(t, err)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client, err := kubernetes.NewForConfig(&rest.Config{Host: server.URL})
	require.NoError(t, err)
	handler := &nodesHandle{clientset: client, prometheusCli: &recordingPrometheus{}}

	_, err = handler.ListNodes(context.Background())

	require.NoError(t, err)
	assert.Greater(t, maxActive, 1)
	assert.LessOrEqual(t, maxActive, 8)
}

func newResourceQueryNode(name string) corev1.Node {
	return corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Status: corev1.NodeStatus{Capacity: corev1.ResourceList{
			corev1.ResourceCPU:              resource.MustParse("4"),
			corev1.ResourceMemory:           resource.MustParse("8Gi"),
			corev1.ResourceEphemeralStorage: resource.MustParse("100Gi"),
		}},
	}
}

func TestInitClusterResourceListsPodsOnceAndCoalescesColdRequests(t *testing.T) {
	nodes := []corev1.Node{
		newResourceQueryNode("node-a"),
		newResourceQueryNode("node-b"),
	}
	pods := []corev1.Pod{
		newResourceQueryPod("pod-a", "node-a", "service-a"),
		newResourceQueryPod("pod-b", "node-b", "service-b"),
	}
	var callsMu sync.Mutex
	nodeListCalls := 0
	podListCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/nodes":
			callsMu.Lock()
			nodeListCalls++
			callsMu.Unlock()
			time.Sleep(20 * time.Millisecond)
			require.NoError(t, json.NewEncoder(w).Encode(&corev1.NodeList{
				TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "NodeList"},
				Items:    nodes,
			}))
		case "/api/v1/pods":
			callsMu.Lock()
			podListCalls++
			callsMu.Unlock()
			require.NoError(t, json.NewEncoder(w).Encode(&corev1.PodList{
				TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "PodList"},
				Items:    pods,
			}))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client, err := kubernetes.NewForConfig(&rest.Config{Host: server.URL})
	require.NoError(t, err)
	action := &TenantAction{kubeClient: client}

	start := make(chan struct{})
	errs := make(chan error, 5)
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errs <- action.initClusterResource(context.Background())
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}

	callsMu.Lock()
	defer callsMu.Unlock()
	assert.Equal(t, 1, nodeListCalls)
	assert.Equal(t, 1, podListCalls)
	require.NotNil(t, action.cacheClusterResourceStats)
	assert.Equal(t, int64(2), action.cacheClusterResourceStats.AllPods)
	assert.Len(t, action.cacheClusterResourceStats.NodePods, 2)
}

func TestGetClusterPodResourcesAggregatesPrometheusResultsByNode(t *testing.T) {
	prom := &recordingPrometheus{}
	prom.metric = func(query string) promcli.Metric {
		switch {
		case strings.HasPrefix(query, "count("):
			return vectorMetric(metricValue(map[string]string{"node_name": "node-a"}, 3))
		case strings.Contains(query, "cluster_pod_memory"):
			return vectorMetric(metricValue(map[string]string{"node_name": "node-a"}, 256*1024*1024))
		case strings.Contains(query, "cluster_pod_cpu"):
			return vectorMetric(metricValue(map[string]string{"node_name": "node-a"}, 500))
		case strings.Contains(query, "cluster_pod_ephemeral_storage"):
			return vectorMetric(metricValue(map[string]string{"node_name": "node-a"}, 1024*1024))
		default:
			return promcli.Metric{}
		}
	}
	action := &clusterAction{prometheusCli: prom}

	resources := action.getClusterPodResources("rbd-api:8888")

	require.Len(t, prom.queries, 4)
	for _, query := range prom.queries {
		assert.Contains(t, query, `instance="rbd-api:8888"`)
		assert.Contains(t, query, "by (node_name)")
	}
	require.Contains(t, resources, "node-a")
	assert.Equal(t, int64(3), resources["node-a"].PodNumber)
	assert.Equal(t, int64(256*1024*1024), resources["node-a"].Memory)
	assert.Equal(t, int64(500), resources["node-a"].CPU)
	assert.Equal(t, int64(1024*1024), resources["node-a"].EphemeralStorage)
	assert.Equal(t, int64(256*1024*1024), resources["node-a"].RainbondMemory)
	assert.Equal(t, int64(500), resources["node-a"].RainbondCPU)
}

func newResourceQueryPod(name, nodeName, serviceID string) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			ResourceVersion: "1",
			Labels: map[string]string{
				"creator":    "Rainbond",
				"service_id": serviceID,
				"app_id":     "app-" + serviceID,
			},
		},
		Spec: corev1.PodSpec{
			NodeName: nodeName,
			Containers: []corev1.Container{{
				Resources: corev1.ResourceRequirements{Requests: corev1.ResourceList{
					corev1.ResourceCPU:              resource.MustParse("500m"),
					corev1.ResourceMemory:           resource.MustParse("256Mi"),
					corev1.ResourceEphemeralStorage: resource.MustParse("1Gi"),
				}},
			}},
		},
	}
}

func TestFetchTenantResourcesOnlyQueriesRequestedTenantsWithBoundedConcurrency(t *testing.T) {
	tenantIDs := make([]string, 0, 22)
	for i := 0; i < 20; i++ {
		tenantIDs = append(tenantIDs, fmt.Sprintf("tenant-%02d", i))
	}
	tenantIDs = append(tenantIDs, "tenant-00", "")

	var stateMu sync.Mutex
	calls := make(map[string]int)
	active := 0
	maxActive := 0
	fetch := func(tenantID string) (*pb.TenantResource, error) {
		stateMu.Lock()
		calls[tenantID]++
		active++
		if active > maxActive {
			maxActive = active
		}
		stateMu.Unlock()
		time.Sleep(5 * time.Millisecond)
		stateMu.Lock()
		active--
		stateMu.Unlock()
		return &pb.TenantResource{CpuLimit: 100}, nil
	}

	resources := fetchTenantResources(tenantIDs, fetch)

	assert.Len(t, resources, 20)
	assert.Len(t, calls, 20)
	for tenantID, count := range calls {
		assert.NotEmpty(t, tenantID)
		assert.Equal(t, 1, count)
	}
	assert.Greater(t, maxActive, 1)
	assert.LessOrEqual(t, maxActive, 8)
}
