package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/goodrain/rainbond/api/model"
	rainbondfake "github.com/goodrain/rainbond/pkg/generated/clientset/versioned/fake"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// capability_id: rainbond.overview.lightweight-nodes
func TestOverviewLightweightNodesSkipStats(t *testing.T) {
	var statsCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v1/nodes" {
			require.NoError(t, json.NewEncoder(w).Encode(&corev1.NodeList{Items: []corev1.Node{overviewResourceNode("worker", true)}}))
			return
		}
		statsCalls.Add(1)
		http.Error(w, "unexpected stats request", http.StatusInternalServerError)
	}))
	defer server.Close()
	cli, err := kubernetes.NewForConfig(&rest.Config{Host: server.URL})
	require.NoError(t, err)
	prom := &recordingPrometheus{}
	n := &nodesHandle{clientset: cli, prometheusCli: prom}
	got, err := n.ListNodesWithOptions(context.Background(), false)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "worker", got[0].Name)
	assert.Equal(t, "not_requested", got[0].DiskStatsStatus)
	assert.NotEmpty(t, got[0].Conditions)
	assert.Zero(t, statsCalls.Load())
	assert.Empty(t, prom.queries)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = n.ListNodesWithOptions(ctx, false)
	require.ErrorIs(t, err, context.Canceled)
}

// capability_id: rainbond.overview.disk-availability
func TestOverviewDiskStatsAvailability(t *testing.T) {
	tests := []struct {
		name, payload, want string
		fail                bool
	}{
		{name: "both filesystems", payload: `{"node":{"fs":{"capacityBytes":100,"availableBytes":20},"runtime":{"imageFs":{"capacityBytes":100,"availableBytes":80}}}}`, want: "ok"},
		{name: "only root", payload: `{"node":{"fs":{"capacityBytes":100,"availableBytes":20}}}`, want: "partial"},
		{name: "no usable filesystems", payload: `{"node":{}}`, want: "unavailable"},
		{name: "kubelet failed", fail: true, want: "unavailable"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path == "/api/v1/nodes" {
					require.NoError(t, json.NewEncoder(w).Encode(&corev1.NodeList{Items: []corev1.Node{overviewResourceNode("worker", true)}}))
				} else if tt.fail {
					http.Error(w, "denied", http.StatusForbidden)
				} else {
					_, _ = w.Write([]byte(tt.payload))
				}
			}))
			defer server.Close()
			cli, err := kubernetes.NewForConfig(&rest.Config{Host: server.URL})
			require.NoError(t, err)
			n := &nodesHandle{clientset: cli, prometheusCli: noopPrometheus{}}
			got, err := n.ListNodes(context.Background())
			require.NoError(t, err)
			require.Len(t, got, 1)
			assert.Equal(t, tt.want, got[0].DiskStatsStatus)
		})
	}
}

// capability_id: rainbond.overview.kubernetes-cluster-resources
func TestOverviewClusterResourcesUseKubernetesNotPrometheus(t *testing.T) {
	nodes := []corev1.Node{overviewResourceNode("healthy", true), overviewResourceNode("unhealthy", false), overviewResourceNode("tainted", true)}
	nodes[2].Spec.Taints = []corev1.Taint{{Key: "dedicated", Effect: corev1.TaintEffectNoSchedule}}
	pods := []corev1.Pod{
		overviewResourcePod("running", "healthy", corev1.PodRunning),
		overviewResourcePod("pending", "unhealthy", corev1.PodPending),
		overviewResourcePod("tainted", "tainted", corev1.PodRunning),
		overviewResourcePod("unscheduled", "", corev1.PodPending),
		overviewResourcePod("completed", "healthy", corev1.PodSucceeded),
		overviewResourcePod("failed", "healthy", corev1.PodFailed),
	}
	pods[0].Spec.InitContainers = []corev1.Container{{Resources: corev1.ResourceRequirements{Requests: corev1.ResourceList{
		corev1.ResourceCPU: resource.MustParse("1"), corev1.ResourceMemory: resource.MustParse("512Mi"),
	}}}}
	pods[0].Spec.Overhead = corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m"), corev1.ResourceMemory: resource.MustParse("64Mi")}
	action, calls, closeServer := overviewClusterAction(t, nodes, pods, false)
	defer closeServer()
	got, err := action.GetClusterInfo(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "kubernetes", got.ResourceSource)
	assert.Equal(t, "rainbond_workloads", got.ResourceScope)
	assert.Equal(t, 12, got.CapCPU)
	assert.Equal(t, 3*8192, got.CapMem)
	assert.InDelta(t, 2.1, got.ReqCPU, .00001)
	assert.Equal(t, 1088, got.ReqMem)
	assert.InDelta(t, 1.6, got.HealthReqCPU, .00001)
	assert.InDelta(t, .5, got.UnhealthReqCPU, .00001)
	assert.Equal(t, int32(2), got.NodeReady)
	assert.Equal(t, 3, got.RunPodNumber)
	assert.Equal(t, 2, got.RunningPodNumber)
	require.NotNil(t, got.MaxAllocatableMemoryNodeResource)
	assert.Equal(t, int64(8192-576)*1024*1024, got.MaxAllocatableMemoryNodeResource.Memory)
	assert.Equal(t, int64(2900), got.MaxAllocatableMemoryNodeResource.MilliCPU)
	assert.Equal(t, int32(1), calls.nodes.Load())
	assert.Equal(t, int32(1), calls.pods.Load())
	assert.Empty(t, action.prometheusCli.(*recordingPrometheus).queries)
	_, err = action.GetClusterInfo(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int32(1), calls.pods.Load(), "cached reads must not relist Pods")
}

func TestOverviewClusterResourcesReportZeroOnlyAfterSuccessfulReads(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "real zero", true: "API error is not zero"}[fail], func(t *testing.T) {
			action, _, closeServer := overviewClusterAction(t, nil, nil, fail)
			defer closeServer()
			got, err := action.GetClusterInfo(context.Background())
			if fail {
				require.Error(t, err)
				assert.Nil(t, got)
				assert.Nil(t, action.clusterInfoCache)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, "kubernetes", got.ResourceSource)
			assert.Zero(t, got.ReqCPU)
			assert.Zero(t, got.ReqMem)
			assert.Zero(t, got.RunPodNumber)
			assert.Zero(t, got.RunningPodNumber)
		})
	}
}

func TestOverviewClusterResourcesCoalesceConcurrentRefresh(t *testing.T) {
	action, calls, closeServer := overviewClusterAction(t, []corev1.Node{overviewResourceNode("worker", true)}, nil, false)
	defer closeServer()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := action.GetClusterInfo(context.Background())
			assert.NoError(t, err)
		}()
	}
	wg.Wait()
	assert.Equal(t, int32(1), calls.nodes.Load())
	assert.Equal(t, int32(1), calls.pods.Load())
	action.cacheTime = time.Now().Add(-time.Minute)
	_, err := action.GetClusterInfo(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int32(2), calls.pods.Load())
}

func TestOverviewClusterResourceRequestsIncludeInitSidecarsAndOverhead(t *testing.T) {
	pod := overviewResourcePod("resource-accounting", "worker", corev1.PodRunning)
	always := corev1.ContainerRestartPolicyAlways
	pod.Spec.InitContainers = []corev1.Container{
		{RestartPolicy: &always, Resources: corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m"), corev1.ResourceMemory: resource.MustParse("64Mi")}}},
		{Resources: corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("2"), corev1.ResourceMemory: resource.MustParse("512Mi")}}},
	}
	pod.Spec.Overhead = corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("50m"), corev1.ResourceMemory: resource.MustParse("32Mi")}
	got := podResourceInformationFromPod(&pod)
	assert.Equal(t, int64(2150), got.CPU)
	assert.Equal(t, int64(608*1024*1024), got.Memory)
}

func TestOverviewClusterResourcesPreserveFractionalCapacityAndScheduledScope(t *testing.T) {
	node := overviewResourceNode("worker", true)
	node.Status.Allocatable[corev1.ResourceCPU] = resource.MustParse("7900m")
	pods := []corev1.Pod{
		overviewResourcePod("active", "worker", corev1.PodUnknown),
		overviewResourcePod("not-scheduled", "", corev1.PodRunning),
		overviewResourcePod("missing-node", "removed", corev1.PodRunning),
		overviewResourcePod("not-rainbond", "worker", corev1.PodRunning),
	}
	pods[3].Labels = map[string]string{"creator": "other"}
	got := buildClusterResourceSnapshot([]*corev1.Node{&node}, pods)
	assert.Equal(t, int64(7900), got.CapCPUMillicores)
	assert.Equal(t, 8, got.CapCPU, "legacy integer field stays compatible")
	assert.Equal(t, 1, got.RunPodNumber)
	assert.Zero(t, got.RunningPodNumber)
	assert.InDelta(t, .5, got.ReqCPU, .00001)
}

func TestOverviewClusterResourcesIncludeEmptySchedulableNodes(t *testing.T) {
	node := overviewResourceNode("empty-worker", true)
	got := buildClusterResourceSnapshot([]*corev1.Node{&node}, nil)
	require.NotNil(t, got.MaxAllocatableMemoryNodeResource)
	assert.Equal(t, int64(8192*1024*1024), got.MaxAllocatableMemoryNodeResource.Memory)
	node.Spec.Unschedulable = true
	got = buildClusterResourceSnapshot([]*corev1.Node{&node}, nil)
	assert.Nil(t, got.MaxAllocatableMemoryNodeResource)
	assert.Equal(t, int32(1), got.NodeReady, "cordon does not mean unready")
	assert.Equal(t, int64(4000), got.CapCPUMillicores)
}

func TestOverviewClusterResourceRefreshWaitRespectsCancellation(t *testing.T) {
	action := &clusterAction{infoRefresh: &clusterInfoRefresh{done: make(chan struct{})}}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	started := time.Now()
	got, err := action.GetClusterInfo(ctx)
	assert.Nil(t, got)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Less(t, time.Since(started), time.Second)
}

func TestOverviewClusterResourceOptionalVersionCannotBlockResources(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/nodes":
			_, _ = w.Write([]byte(`{"items":[]}`))
		case "/api/v1/pods":
			_, _ = w.Write([]byte(`{"items":[]}`))
		case "/version":
			<-r.Context().Done()
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	cli, err := kubernetes.NewForConfig(&rest.Config{Host: server.URL})
	require.NoError(t, err)
	action := &clusterAction{clientset: cli}
	started := time.Now()
	got, err := action.GetClusterInfo(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "kubernetes", got.ResourceSource)
	assert.Empty(t, got.K8sVersion)
	assert.Less(t, time.Since(started), time.Second)
}

// capability_id: rainbond.overview.nonblocking-local-disk
func TestOverviewClusterResourceCannotBeBlockedByLocalDisk(t *testing.T) {
	action, calls, closeServer := overviewClusterAction(t, []corev1.Node{overviewResourceNode("worker", true)}, nil, false)
	defer closeServer()
	started := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	defer releaseOnce.Do(func() { close(release) })
	var diskCalls atomic.Int32
	action.diskUsage = func(string) (*disk.UsageStat, error) {
		if diskCalls.Add(1) == 1 {
			close(started)
		}
		<-release
		return &disk.UsageStat{Total: 1000, Used: 750}, nil
	}
	type response struct {
		value *model.ClusterResource
		err   error
	}
	returned := make(chan response, 1)
	go func() {
		value, err := action.GetClusterInfo(context.Background())
		returned <- response{value, err}
	}()
	var first *model.ClusterResource
	select {
	case result := <-returned:
		require.NoError(t, result.err)
		first = result.value
		assert.Equal(t, "unavailable", result.value.LocalDiskStatsStatus)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("local statfs blocked the successful Kubernetes resource response")
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("background sample did not start")
	}
	for i := 0; i < 20; i++ {
		got, err := action.GetClusterInfo(context.Background())
		require.NoError(t, err)
		assert.Equal(t, "kubernetes", got.ResourceSource)
		assert.Equal(t, "unavailable", got.LocalDiskStatsStatus)
	}
	assert.Equal(t, int32(1), diskCalls.Load(), "a hung statfs must not spawn per-request goroutines")
	releaseOnce.Do(func() { close(release) })
	require.Eventually(t, func() bool {
		got, err := action.GetClusterInfo(context.Background())
		return err == nil && got.LocalDiskStatsStatus == "ok" && got.CapDisk == 1000 && got.ReqDisk == 750
	}, time.Second, 10*time.Millisecond)
	assert.Equal(t, int32(1), calls.pods.Load(), "disk refresh must not invalidate Kubernetes resources")
	assert.Equal(t, "unavailable", first.LocalDiskStatsStatus, "background completion must not mutate a returned response")
	assert.Zero(t, first.CapDisk)
}

func TestOverviewLocalDiskFailureUsesCooldownAndPreservesStaleSample(t *testing.T) {
	for _, tc := range []struct {
		name   string
		stale  bool
		nilNil bool
	}{{name: "failure"}, {name: "nil nil result", nilNil: true}, {name: "stale success survives failure", stale: true}} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			action := &clusterAction{
				clusterInfoCache: &model.ClusterResource{ResourceSource: "kubernetes"},
				cacheTime:        time.Now(),
				diskUsage: func(string) (*disk.UsageStat, error) {
					calls.Add(1)
					if tc.nilNil {
						return nil, nil
					}
					return nil, errors.New("filesystem unavailable")
				},
			}
			if tc.stale {
				action.localDisk = clusterLocalDiskSample{
					capacity: 1000, used: 750, available: true,
					sampledAt: time.Now().Add(-time.Minute),
				}
			}
			_, err := action.GetClusterInfo(context.Background())
			require.NoError(t, err)
			require.Eventually(t, func() bool {
				action.cacheMu.Lock()
				defer action.cacheMu.Unlock()
				return action.localDisk.failed && !action.localDisk.refreshing
			}, time.Second, 10*time.Millisecond)
			for i := 0; i < 20; i++ {
				got, err := action.GetClusterInfo(context.Background())
				require.NoError(t, err)
				if tc.stale {
					assert.Equal(t, "stale", got.LocalDiskStatsStatus)
					assert.Equal(t, uint64(1000), got.CapDisk)
					assert.Equal(t, uint64(750), got.ReqDisk)
				} else {
					assert.Equal(t, "unavailable", got.LocalDiskStatsStatus)
				}
			}
			assert.Equal(t, int32(1), calls.Load(), "failures must have a 30 second retry cooldown")
			action.cacheMu.Lock()
			action.localDisk.lastAttempt = time.Now().Add(-time.Minute)
			action.cacheMu.Unlock()
			_, err = action.GetClusterInfo(context.Background())
			require.NoError(t, err)
			require.Eventually(t, func() bool { return calls.Load() == 2 }, time.Second, 10*time.Millisecond)
		})
	}
}

// capability_id: rainbond.overview.empty-installed-plugins
func TestOverviewEmptyInstalledPluginsAreAnArray(t *testing.T) {
	action := &clusterAction{rainbondClient: rainbondfake.NewSimpleClientset()}
	for _, official := range []bool{false, true} {
		got, err := action.ListPlugins(official)
		require.NoError(t, err)
		encoded, err := json.Marshal(got)
		require.NoError(t, err)
		assert.Equal(t, "[]", string(encoded))
	}
}

type overviewResourceCalls struct{ nodes, pods atomic.Int32 }

func overviewClusterAction(t *testing.T, nodes []corev1.Node, pods []corev1.Pod, fail bool) (*clusterAction, *overviewResourceCalls, func()) {
	t.Helper()
	calls := &overviewResourceCalls{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/nodes":
			calls.nodes.Add(1)
			require.NoError(t, json.NewEncoder(w).Encode(&corev1.NodeList{Items: nodes}))
		case "/api/v1/pods":
			calls.pods.Add(1)
			assert.Equal(t, "creator=Rainbond", r.URL.Query().Get("labelSelector"))
			if fail {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			require.NoError(t, json.NewEncoder(w).Encode(&corev1.PodList{Items: pods}))
		case "/version":
			_, _ = w.Write([]byte(`{"gitVersion":"v1.30.4"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	cli, err := kubernetes.NewForConfig(&rest.Config{Host: server.URL})
	require.NoError(t, err)
	return &clusterAction{clientset: cli, prometheusCli: &recordingPrometheus{}}, calls, server.Close
}

func overviewResourceNode(name string, ready bool) corev1.Node {
	node := newResourceQueryNode(name)
	node.Status.Allocatable = node.Status.Capacity.DeepCopy()
	node.Status.Allocatable[corev1.ResourcePods] = resource.MustParse("110")
	condition := corev1.ConditionFalse
	if ready {
		condition = corev1.ConditionTrue
	}
	node.Status.Conditions = []corev1.NodeCondition{{Type: corev1.NodeReady, Status: condition}}
	return node
}

func overviewResourcePod(name, node string, phase corev1.PodPhase) corev1.Pod {
	pod := newResourceQueryPod(name, node, "service-"+name)
	pod.Status.Phase = phase
	return pod
}
