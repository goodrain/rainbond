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

	promcli "github.com/goodrain/rainbond/api/client/prometheus"
	apimodel "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/db"
	dbdao "github.com/goodrain/rainbond/db/dao"
	dbmodel "github.com/goodrain/rainbond/db/model"
	workerclient "github.com/goodrain/rainbond/worker/client"
	"github.com/goodrain/rainbond/worker/server/pb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type tenantResourcesLimitsDAO struct {
	dbdao.TenantDao
	limits map[string]*dbmodel.Tenants
	ids    []string
	err    error
}

func (d *tenantResourcesLimitsDAO) GetTenantLimitsByNames([]string) (map[string]*dbmodel.Tenants, []string, error) {
	return d.limits, d.ids, d.err
}

type tenantResourcesServicesDAO struct {
	dbdao.TenantServiceDao
	services []*dbmodel.TenantServices
	err      error
	requests [][]string
}

func (d *tenantResourcesServicesDAO) GetServicesByTenantIDs(ids []string) ([]*dbmodel.TenantServices, error) {
	d.requests = append(d.requests, append([]string(nil), ids...))
	return d.services, d.err
}

type tenantResourcesTestManager struct {
	db.Manager
	limits   *tenantResourcesLimitsDAO
	services *tenantResourcesServicesDAO
}

func (m tenantResourcesTestManager) TenantDao() dbdao.TenantDao { return m.limits }

func (m tenantResourcesTestManager) TenantServiceDao() dbdao.TenantServiceDao { return m.services }

type tenantResourcesRuntime struct {
	pb.AppRuntimeSyncClient
	mu       sync.Mutex
	requests []string
}

func (r *tenantResourcesRuntime) GetTenantResource(_ context.Context, request *pb.TenantRequest,
	_ ...grpc.CallOption) (*pb.TenantResource, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.requests = append(r.requests, request.TenantId)
	return &pb.TenantResource{CpuLimit: 250, MemoryLimit: 128, RunningAppNum: 2, RunningApplications: 1}, nil
}

type tenantResourcesFixture struct {
	action    *TenantAction
	limits    *tenantResourcesLimitsDAO
	services  *tenantResourcesServicesDAO
	runtime   *tenantResourcesRuntime
	prom      *recordingPrometheus
	nodeCalls atomic.Int32
	podCalls  atomic.Int32
	nodeError atomic.Bool
}

func newTenantResourcesFixture(t *testing.T, limits map[string]*dbmodel.Tenants, ids []string) *tenantResourcesFixture {
	t.Helper()
	f := &tenantResourcesFixture{
		limits:   &tenantResourcesLimitsDAO{limits: limits, ids: ids},
		services: &tenantResourcesServicesDAO{services: []*dbmodel.TenantServices{{TenantID: "tenant-a"}}},
		runtime:  &tenantResourcesRuntime{},
		prom:     &recordingPrometheus{metrics: nil},
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/nodes":
			f.nodeCalls.Add(1)
			if f.nodeError.Load() {
				http.Error(w, "nodes unavailable", http.StatusServiceUnavailable)
				return
			}
			if err := json.NewEncoder(w).Encode(&corev1.NodeList{
				TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "NodeList"},
				Items: []corev1.Node{{Status: corev1.NodeStatus{Allocatable: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("4"), corev1.ResourceMemory: resource.MustParse("8Gi"),
				}}}},
			}); err != nil {
				t.Error(err)
			}
		case "/api/v1/pods":
			f.podCalls.Add(1)
			if err := json.NewEncoder(w).Encode(&corev1.PodList{
				TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "PodList"},
			}); err != nil {
				t.Error(err)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	kubeClient, err := kubernetes.NewForConfig(&rest.Config{Host: server.URL})
	require.NoError(t, err)
	f.action = &TenantAction{
		kubeClient:    kubeClient,
		statusCli:     &workerclient.AppRuntimeSyncClient{AppRuntimeSyncClient: f.runtime},
		prometheusCli: f.prom,
	}
	previous := db.GetManager()
	db.SetTestManager(tenantResourcesTestManager{limits: f.limits, services: f.services})
	t.Cleanup(func() { db.SetTestManager(previous) })
	return f
}

// capability_id: rainbond.tenant-resources.skip-unused-cluster-scans
func TestGetTenantsResourcesWithLimitsSkipsClusterReads(t *testing.T) {
	for _, limit := range []int{0, 1024} {
		t.Run(map[bool]string{true: "zero-limit", false: "configured-limit"}[limit == 0], func(t *testing.T) {
			f := newTenantResourcesFixture(t, map[string]*dbmodel.Tenants{
				"tenant-a": {LimitCPU: limit, LimitMemory: limit, LimitStorage: limit},
			}, []string{"tenant-a"})
			f.prom.metrics = map[string]promcli.Metric{
				"app_resource_appfs": vectorMetric(metricValue(map[string]string{"tenant_id": "tenant-a"}, 2048)),
			}

			result, err := f.action.GetTenantsResources(context.Background(), &apimodel.TenantResources{})

			require.NoError(t, err)
			assert.Zero(t, f.nodeCalls.Load())
			assert.Zero(t, f.podCalls.Load())
			assert.Nil(t, f.action.cacheClusterResourceStats)
			assert.Equal(t, [][]string{{"tenant-a"}}, f.services.requests)
			assert.Equal(t, []string{"tenant-a"}, f.runtime.requests)
			assert.Len(t, f.prom.queries, 1)
			assert.Equal(t, map[string]map[string]interface{}{"tenant-a": {
				"tenant_id": "tenant-a", "limit_cpu": int64(limit), "limit_memory": int64(limit),
				"limit_storage": int64(limit), "service_total_num": 1, "disk": 2,
				"service_running_num": int64(2), "cpu": int64(250), "memory": int64(128), "app_running_num": int64(1),
			}}, result)
		})
	}
}

// capability_id: rainbond.tenant-resources.skip-unused-cluster-scans
func TestGetTenantsResourcesWithLimitsDoesNotWaitForClusterCache(t *testing.T) {
	f := newTenantResourcesFixture(t, map[string]*dbmodel.Tenants{"tenant-a": {}}, []string{"tenant-a"})
	f.action.cacheMu.Lock()
	finished := make(chan error, 1)
	go func() {
		_, err := f.action.GetTenantsResources(context.Background(), &apimodel.TenantResources{})
		finished <- err
	}()
	select {
	case err := <-finished:
		f.action.cacheMu.Unlock()
		require.NoError(t, err)
	case <-time.After(time.Second):
		f.action.cacheMu.Unlock()
		<-finished
		t.Fatal("complete tenant limits must not wait for the cluster resource cache lock")
	}
	assert.Zero(t, f.nodeCalls.Load())
	assert.Zero(t, f.podCalls.Load())
}

// capability_id: rainbond.tenant-resources.skip-unused-cluster-scans
func TestGetTenantsResourcesMissingLimitsUsesOneClusterFallback(t *testing.T) {
	f := newTenantResourcesFixture(t, map[string]*dbmodel.Tenants{
		"tenant-a": {}, "tenant-b": nil,
	}, []string{"tenant-a", "tenant-b", "tenant-c"})

	result, err := f.action.GetTenantsResources(context.Background(), &apimodel.TenantResources{})

	require.NoError(t, err)
	assert.Equal(t, int32(1), f.nodeCalls.Load())
	assert.Equal(t, int32(1), f.podCalls.Load())
	assert.Equal(t, int64(0), result["tenant-a"]["limit_cpu"])
	assert.Equal(t, int64(0), result["tenant-a"]["limit_memory"])
	for _, tenantID := range []string{"tenant-b", "tenant-c"} {
		assert.Equal(t, int64(4000), result[tenantID]["limit_cpu"])
		assert.Equal(t, int64(8192), result[tenantID]["limit_memory"])
		assert.Equal(t, int64(0), result[tenantID]["limit_storage"])
	}
	assert.Len(t, f.runtime.requests, 3)
	assert.Len(t, f.prom.queries, 1)
}

// capability_id: rainbond.tenant-resources.skip-unused-cluster-scans
func TestGetTenantsResourcesEmptyIDsSkipsDownstreamCalls(t *testing.T) {
	f := newTenantResourcesFixture(t, nil, nil)

	result, err := f.action.GetTenantsResources(context.Background(), &apimodel.TenantResources{})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Empty(t, result)
	assert.Empty(t, f.services.requests)
	assert.Empty(t, f.runtime.requests)
	assert.Empty(t, f.prom.queries)
	assert.Zero(t, f.nodeCalls.Load())
	assert.Zero(t, f.podCalls.Load())
}

// capability_id: rainbond.tenant-resources.skip-unused-cluster-scans
func TestGetTenantsResourcesPreservesDAOErrors(t *testing.T) {
	for _, failingDAO := range []string{"limits", "services"} {
		t.Run(failingDAO, func(t *testing.T) {
			f := newTenantResourcesFixture(t, nil, []string{"tenant-a"})
			failure := errors.New("database unavailable")
			if failingDAO == "limits" {
				f.limits.err = failure
			} else {
				f.services.err = failure
			}

			result, err := f.action.GetTenantsResources(context.Background(), &apimodel.TenantResources{})

			assert.ErrorIs(t, err, failure)
			assert.Nil(t, result)
			assert.Empty(t, f.runtime.requests)
			assert.Empty(t, f.prom.queries)
			assert.Zero(t, f.nodeCalls.Load())
			assert.Zero(t, f.podCalls.Load())
		})
	}
}

// capability_id: rainbond.tenant-resources.skip-unused-cluster-scans
func TestGetTenantsResourcesPreservesZeroFallbackAfterKubernetesFailure(t *testing.T) {
	f := newTenantResourcesFixture(t, nil, []string{"tenant-a"})
	f.nodeError.Store(true)

	result, err := f.action.GetTenantsResources(context.Background(), &apimodel.TenantResources{})

	require.NoError(t, err)
	assert.Equal(t, int32(1), f.nodeCalls.Load())
	assert.Zero(t, f.podCalls.Load())
	assert.Equal(t, int64(0), result["tenant-a"]["limit_cpu"])
	assert.Equal(t, int64(0), result["tenant-a"]["limit_memory"])
	assert.Equal(t, int64(0), result["tenant-a"]["limit_storage"])
	assert.Equal(t, int64(250), result["tenant-a"]["cpu"])
}
