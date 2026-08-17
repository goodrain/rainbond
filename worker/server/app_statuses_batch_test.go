package server

import (
	"context"
	"testing"

	"github.com/goodrain/rainbond/db"
	dbdao "github.com/goodrain/rainbond/db/dao"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/worker/appm/store"
	appv1 "github.com/goodrain/rainbond/worker/appm/types/v1"
	"github.com/goodrain/rainbond/worker/server/pb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

type batchAppStatusManager struct {
	db.Manager
	applicationDAO dbdao.ApplicationDao
	serviceDAO     dbdao.TenantServiceDao
}

func (m batchAppStatusManager) ApplicationDao() dbdao.ApplicationDao {
	return m.applicationDAO
}

func (m batchAppStatusManager) TenantServiceDao() dbdao.TenantServiceDao {
	return m.serviceDAO
}

type countingApplicationDAO struct {
	dbdao.ApplicationDao
	apps      []*dbmodel.Application
	listCalls int
}

func (d *countingApplicationDAO) ListByAppIDs(appIDs []string) ([]*dbmodel.Application, error) {
	d.listCalls++
	return d.apps, nil
}

type countingTenantServiceDAO struct {
	dbdao.TenantServiceDao
	services  []*dbmodel.TenantServices
	listCalls int
	appIDs    []string
}

func (d *countingTenantServiceDAO) ListByAppIDs(appIDs []string) ([]*dbmodel.TenantServices, error) {
	d.listCalls++
	d.appIDs = append([]string(nil), appIDs...)
	return d.services, nil
}

type countingPodLister struct {
	corelisters.PodLister
	listCalls int
}

func (l *countingPodLister) List(selector labels.Selector) ([]*corev1.Pod, error) {
	l.listCalls++
	return l.PodLister.List(selector)
}

type batchAppStatusStore struct {
	store.Storer
	lister               *store.Lister
	componentStatuses    map[string]string
	componentStatusCalls int
	componentIDs         []string
	legacyStatuses       map[string]pb.AppStatus_Status
	legacyResources      map[string][2]int64
}

func (s *batchAppStatusStore) GetAppServiceStatuses(serviceIDs []string) map[string]string {
	s.componentStatusCalls++
	s.componentIDs = append([]string(nil), serviceIDs...)
	return s.componentStatuses
}

func (s *batchAppStatusStore) Lister() *store.Lister {
	return s.lister
}

func (s *batchAppStatusStore) GetAppStatus(appID string) (pb.AppStatus_Status, error) {
	return s.legacyStatuses[appID], nil
}

func (s *batchAppStatusStore) GetAppResources(appID string) (int64, int64, error) {
	resources := s.legacyResources[appID]
	return resources[0], resources[1], nil
}

func TestListAppStatusesBatchesServicesStatusesAndPodSnapshot(t *testing.T) {
	apps := []*dbmodel.Application{
		{AppID: "app-a", AppName: "application-a"},
		{AppID: "app-b", AppName: "application-b"},
		{AppID: "app-c", AppName: "application-c"},
	}
	applicationDAO := &countingApplicationDAO{apps: apps}
	serviceDAO := &countingTenantServiceDAO{services: []*dbmodel.TenantServices{
		{AppID: "app-a", ServiceID: "service-a"},
		{AppID: "app-b", ServiceID: "service-b"},
	}}
	db.SetTestManager(batchAppStatusManager{
		applicationDAO: applicationDAO,
		serviceDAO:     serviceDAO,
	})
	defer db.SetTestManager(nil)

	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	for _, pod := range []*corev1.Pod{
		newResourceLimitPod("app-a-current", map[string]string{"app_id": "app-a", "service_id": "service-a"}, "100m", "100Mi"),
		newResourceLimitPod("app-a-legacy", map[string]string{"service_id": "service-a"}, "900m", "900Mi"),
		newResourceLimitPod("app-b-legacy", map[string]string{"service_id": "service-b"}, "200m", "200Mi"),
		newResourceLimitPod("unrelated", map[string]string{"app_id": "other-app", "service_id": "other-service"}, "10", "10Gi"),
	} {
		require.NoError(t, indexer.Add(pod))
	}
	podLister := &countingPodLister{PodLister: corelisters.NewPodLister(indexer)}
	statusStore := &batchAppStatusStore{
		lister: &store.Lister{Pod: podLister},
		componentStatuses: map[string]string{
			"service-a": appv1.RUNNING,
			"service-b": appv1.CLOSED,
		},
		legacyStatuses: map[string]pb.AppStatus_Status{
			"app-a": pb.AppStatus_RUNNING,
			"app-b": pb.AppStatus_CLOSED,
			"app-c": pb.AppStatus_NIL,
		},
		legacyResources: map[string][2]int64{
			"app-a": {100, 100},
			"app-b": {200, 200},
			"app-c": {0, 0},
		},
	}
	runtimeServer := &RuntimeServer{store: statusStore}

	result, err := runtimeServer.ListAppStatuses(context.Background(), &pb.AppStatusesReq{
		AppIds: []string{"app-a", "app-b", "app-c"},
	})

	require.NoError(t, err)
	require.Len(t, result.AppStatuses, 3)
	assert.Equal(t, 1, applicationDAO.listCalls)
	assert.Equal(t, 1, serviceDAO.listCalls)
	assert.Equal(t, []string{"app-a", "app-b", "app-c"}, serviceDAO.appIDs)
	assert.Equal(t, 1, statusStore.componentStatusCalls)
	assert.Equal(t, []string{"service-a", "service-b"}, statusStore.componentIDs)
	assert.Equal(t, 1, podLister.listCalls)
	assertAppStatus(t, result.AppStatuses[0], "app-a", pb.AppStatus_RUNNING, 100, 100)
	assertAppStatus(t, result.AppStatuses[1], "app-b", pb.AppStatus_CLOSED, 200, 200)
	assertAppStatus(t, result.AppStatuses[2], "app-c", pb.AppStatus_NIL, 0, 0)
}

func newResourceLimitPod(name string, podLabels map[string]string, cpu, memory string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "tenant-ns", Labels: podLabels},
		Spec: corev1.PodSpec{Containers: []corev1.Container{{
			Name: "main",
			Resources: corev1.ResourceRequirements{Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse(cpu),
				corev1.ResourceMemory: resource.MustParse(memory),
			}},
		}}},
	}
}

func assertAppStatus(t *testing.T, actual *pb.AppStatus, appID string, status pb.AppStatus_Status, cpu, memory int64) {
	t.Helper()
	require.NotNil(t, actual)
	assert.Equal(t, appID, actual.AppId)
	assert.Equal(t, status.String(), actual.Status)
	assert.Equal(t, cpu, actual.Cpu)
	assert.Equal(t, memory, actual.Memory)
	assert.True(t, actual.SetCPU)
	assert.True(t, actual.SetMemory)
}

var _ db.Manager = batchAppStatusManager{}
var _ dbdao.ApplicationDao = (*countingApplicationDAO)(nil)
var _ dbdao.TenantServiceDao = (*countingTenantServiceDAO)(nil)
var _ store.Storer = (*batchAppStatusStore)(nil)
var _ corelisters.PodLister = (*countingPodLister)(nil)
