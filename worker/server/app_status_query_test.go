package server

import (
	"context"
	"errors"
	"testing"

	"github.com/goodrain/rainbond/db"
	dbdao "github.com/goodrain/rainbond/db/dao"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/pkg/apis/rainbond/v1alpha1"
	"github.com/goodrain/rainbond/worker/appm/store"
	"github.com/goodrain/rainbond/worker/server/pb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/labels"
)

type singleAppStatusApplicationDAO struct {
	dbdao.ApplicationDao
	app *dbmodel.Application
	err error
	ids []string
}

func (d *singleAppStatusApplicationDAO) GetAppByID(appID string) (*dbmodel.Application, error) {
	d.ids = append(d.ids, appID)
	return d.app, d.err
}

type unusedAppStatusServiceDAO struct {
	dbdao.TenantServiceDao
	listCalls int
}

func (d *unusedAppStatusServiceDAO) ListByAppIDs(_ []string) ([]*dbmodel.TenantServices, error) {
	d.listCalls++
	return nil, errors.New("unused component query failed")
}

type singleAppStatusTenantDAO struct {
	dbdao.TenantDao
	tenant *dbmodel.Tenants
	err    error
	ids    []string
}

func (d *singleAppStatusTenantDAO) GetTenantByUUID(tenantID string) (*dbmodel.Tenants, error) {
	d.ids = append(d.ids, tenantID)
	return d.tenant, d.err
}

type singleAppStatusManager struct {
	batchAppStatusManager
	tenantDAO dbdao.TenantDao
}

func (m singleAppStatusManager) TenantDao() dbdao.TenantDao {
	return m.tenantDAO
}

type singleAppStatusStore struct {
	store.Storer
	status        pb.AppStatus_Status
	statusErr     error
	statusIDs     []string
	cpu           int64
	memory        int64
	resourceErr   error
	resourceIDs   []string
	helmApp       *v1alpha1.HelmApp
	helmErr       error
	helmRefs      [][2]string
	pods          []*corev1.Pod
	podErr        error
	podNamespaces []string
	podSelectors  []labels.Selector
}

func (s *singleAppStatusStore) GetAppStatus(appID string) (pb.AppStatus_Status, error) {
	s.statusIDs = append(s.statusIDs, appID)
	return s.status, s.statusErr
}

func (s *singleAppStatusStore) GetAppResources(appID string) (int64, int64, error) {
	s.resourceIDs = append(s.resourceIDs, appID)
	return s.cpu, s.memory, s.resourceErr
}

func (s *singleAppStatusStore) GetHelmApp(namespace, name string) (*v1alpha1.HelmApp, error) {
	s.helmRefs = append(s.helmRefs, [2]string{namespace, name})
	return s.helmApp, s.helmErr
}

func (s *singleAppStatusStore) ListPods(namespace string, selector labels.Selector) ([]*corev1.Pod, error) {
	s.podNamespaces = append(s.podNamespaces, namespace)
	s.podSelectors = append(s.podSelectors, selector)
	return s.pods, s.podErr
}

// capability_id: rainbond.worker.app-status-no-unused-component-query
func TestGetAppStatusSkipsUnusedComponentRead(t *testing.T) {
	lookupErr := errors.New("application lookup failed")
	statusErr := errors.New("application status failed")
	resourceErr := errors.New("application resources failed")
	for _, tc := range []struct {
		name          string
		status        pb.AppStatus_Status
		cpu           int64
		memory        int64
		lookupErr     error
		statusErr     error
		resourceErr   error
		wantErr       error
		statusCalls   int
		resourceCalls int
	}{
		{name: "running with resources", status: pb.AppStatus_RUNNING, cpu: 125, memory: 256, statusCalls: 1, resourceCalls: 1},
		{name: "empty retains explicit zero resource fields", status: pb.AppStatus_NIL, statusCalls: 1, resourceCalls: 1},
		{name: "closed retains resource values", status: pb.AppStatus_CLOSED, cpu: 250, memory: 512, statusCalls: 1, resourceCalls: 1},
		{name: "lookup error short circuits", lookupErr: lookupErr, wantErr: lookupErr},
		{name: "status error short circuits resources", statusErr: statusErr, wantErr: statusErr, statusCalls: 1},
		{name: "resource error preserved", resourceErr: resourceErr, wantErr: resourceErr, statusCalls: 1, resourceCalls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			applicationDAO := &singleAppStatusApplicationDAO{
				app: &dbmodel.Application{AppID: "app-a", AppName: "application-a"}, err: tc.lookupErr,
			}
			serviceDAO := &unusedAppStatusServiceDAO{}
			db.SetTestManager(batchAppStatusManager{applicationDAO: applicationDAO, serviceDAO: serviceDAO})
			t.Cleanup(func() { db.SetTestManager(nil) })
			statusStore := &singleAppStatusStore{
				status: tc.status, statusErr: tc.statusErr, cpu: tc.cpu, memory: tc.memory, resourceErr: tc.resourceErr,
			}

			got, err := (&RuntimeServer{store: statusStore}).GetAppStatus(context.Background(), &pb.AppStatusReq{AppId: "app-a"})

			assert.Equal(t, []string{"app-a"}, applicationDAO.ids)
			assert.Zero(t, serviceDAO.listCalls, "single application status must not issue an unused component query")
			assert.Len(t, statusStore.statusIDs, tc.statusCalls)
			assert.Len(t, statusStore.resourceIDs, tc.resourceCalls)
			for _, id := range append(statusStore.statusIDs, statusStore.resourceIDs...) {
				assert.Equal(t, "app-a", id)
			}
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				assert.Nil(t, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, &pb.AppStatus{
				Status: tc.status.String(), SetCPU: true, Cpu: tc.cpu, SetMemory: true, Memory: tc.memory,
				AppId: "app-a", AppName: "application-a",
			}, got)
			assert.Empty(t, statusStore.helmRefs)
			assert.Empty(t, statusStore.podNamespaces)
		})
	}
}

// capability_id: rainbond.worker.app-status-no-unused-component-query
func TestGetAppStatusPreservesHelmStatusAndErrors(t *testing.T) {
	tenantErr := errors.New("tenant lookup failed")
	helmErr := errors.New("helm lookup failed")
	podErr := errors.New("helm pods failed")
	for _, tc := range []struct {
		name      string
		phase     v1alpha1.HelmAppStatusPhase
		tenantErr error
		helmErr   error
		podErr    error
		wantErr   error
		helmCalls int
		podCalls  int
	}{
		{name: "installed helm retains fields", phase: v1alpha1.HelmAppStatusPhaseInstalled, helmCalls: 1, podCalls: 1},
		{name: "empty phase retains detecting default", helmCalls: 1, podCalls: 1},
		{name: "tenant error", tenantErr: tenantErr, wantErr: tenantErr},
		{name: "helm error", helmErr: helmErr, wantErr: helmErr, helmCalls: 1},
		{name: "pod error", podErr: podErr, wantErr: podErr, helmCalls: 1, podCalls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			applicationDAO := &singleAppStatusApplicationDAO{
				app: &dbmodel.Application{AppID: "helm-a", AppName: "helm-release", TenantID: "tenant-a", AppType: dbmodel.AppTypeHelm},
			}
			serviceDAO := &unusedAppStatusServiceDAO{}
			tenantDAO := &singleAppStatusTenantDAO{tenant: &dbmodel.Tenants{Namespace: "helm-ns"}, err: tc.tenantErr}
			db.SetTestManager(singleAppStatusManager{
				batchAppStatusManager: batchAppStatusManager{applicationDAO: applicationDAO, serviceDAO: serviceDAO}, tenantDAO: tenantDAO,
			})
			t.Cleanup(func() { db.SetTestManager(nil) })
			statusStore := &singleAppStatusStore{
				helmApp: &v1alpha1.HelmApp{Status: v1alpha1.HelmAppStatus{
					Phase: tc.phase, Status: v1alpha1.HelmAppStatusDeployed, CurrentVersion: "1.2.3", Overrides: []string{"replicas=2"},
					Conditions: []v1alpha1.HelmAppCondition{{Type: v1alpha1.HelmAppInstalled, Status: corev1.ConditionTrue, Reason: "ready", Message: "installed"}},
				}},
				helmErr: tc.helmErr, podErr: tc.podErr,
				pods: []*corev1.Pod{{Spec: corev1.PodSpec{Containers: []corev1.Container{{Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m"), corev1.ResourceMemory: resource.MustParse("64Mi")},
				}}}}}},
			}

			got, err := (&RuntimeServer{store: statusStore}).GetAppStatus(context.Background(), &pb.AppStatusReq{AppId: "helm-a"})

			assert.Equal(t, []string{"helm-a"}, applicationDAO.ids)
			assert.Equal(t, []string{"tenant-a"}, tenantDAO.ids)
			assert.Zero(t, serviceDAO.listCalls)
			assert.Empty(t, statusStore.statusIDs)
			assert.Empty(t, statusStore.resourceIDs)
			assert.Len(t, statusStore.helmRefs, tc.helmCalls)
			assert.Len(t, statusStore.podNamespaces, tc.podCalls)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				assert.Nil(t, got)
				return
			}
			require.NoError(t, err)
			phase := tc.phase
			if phase == "" {
				phase = v1alpha1.HelmAppStatusPhaseDetecting
			}
			assert.Equal(t, &pb.AppStatus{
				Status: "deployed", Phase: string(phase), Cpu: 100, SetCPU: true, Memory: 64, SetMemory: true,
				Version: "1.2.3", Overrides: []string{"replicas=2"}, AppId: "helm-a", AppName: "helm-release",
				Conditions: []*pb.AppStatusCondition{{Type: "HelmAppInstalled", Status: true, Reason: "ready", Message: "installed"}},
			}, got)
			assert.Equal(t, [][2]string{{"helm-ns", "helm-release"}}, statusStore.helmRefs)
			assert.Equal(t, []string{"helm-ns"}, statusStore.podNamespaces)
			require.Len(t, statusStore.podSelectors, 1)
			assert.True(t, statusStore.podSelectors[0].Matches(labels.Set{"app": "helm-release", "app.kubernetes.io/managed-by": "Helm"}))
			assert.False(t, statusStore.podSelectors[0].Matches(labels.Set{"app": "other", "app.kubernetes.io/managed-by": "Helm"}))
		})
	}
}
