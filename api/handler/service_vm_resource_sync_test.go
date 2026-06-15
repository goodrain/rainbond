package handler

import (
	"context"
	"testing"

	apimodel "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/db"
	dbdao "github.com/goodrain/rainbond/db/dao"
	dbmodel "github.com/goodrain/rainbond/db/model"
	mqpb "github.com/goodrain/rainbond/mq/api/grpc/pb"
	gclient "github.com/goodrain/rainbond/mq/client"
	workermodel "github.com/goodrain/rainbond/worker/discover/model"
	"google.golang.org/grpc"
)

type resourceSyncTestManager struct {
	db.Manager
	serviceDao      dbdao.TenantServiceDao
	eventDao        dbdao.EventDao
	serviceProbeDao dbdao.ServiceProbeDao
	relationDao     dbdao.TenantServiceRelationDao
	attributeDao    dbdao.ComponentK8sAttributeDao
}

func (m resourceSyncTestManager) TenantServiceDao() dbdao.TenantServiceDao {
	return m.serviceDao
}

func (m resourceSyncTestManager) ServiceEventDao() dbdao.EventDao {
	return m.eventDao
}

func (m resourceSyncTestManager) ServiceProbeDao() dbdao.ServiceProbeDao {
	return m.serviceProbeDao
}

func (m resourceSyncTestManager) TenantServiceRelationDao() dbdao.TenantServiceRelationDao {
	return m.relationDao
}

func (m resourceSyncTestManager) ComponentK8sAttributeDao() dbdao.ComponentK8sAttributeDao {
	return m.attributeDao
}

type resourceSyncTenantServiceDao struct {
	dbdao.TenantServiceDao
	service        *dbmodel.TenantServices
	updatedService *dbmodel.TenantServices
}

func (d *resourceSyncTenantServiceDao) GetServiceByID(serviceID string) (*dbmodel.TenantServices, error) {
	return d.service, nil
}

func (d *resourceSyncTenantServiceDao) IsK8sComponentNameDuplicate(appID, serviceID, k8sComponentName string) bool {
	return false
}

func (d *resourceSyncTenantServiceDao) UpdateModel(arg dbmodel.Interface) error {
	service, ok := arg.(*dbmodel.TenantServices)
	if !ok {
		return nil
	}
	copied := *service
	d.updatedService = &copied
	d.service = &copied
	return nil
}

type resourceSyncEventDao struct {
	dbdao.EventDao
	statuses []dbmodel.EventStatus
}

func (d *resourceSyncEventDao) SetEventStatus(_ context.Context, status dbmodel.EventStatus) error {
	d.statuses = append(d.statuses, status)
	return nil
}

type resourceSyncServiceProbeDao struct {
	dbdao.ServiceProbeDao
	added   int
	updated int
	deleted int
}

func (d *resourceSyncServiceProbeDao) AddModel(dbmodel.Interface) error {
	d.added++
	return nil
}

func (d *resourceSyncServiceProbeDao) UpdateModel(dbmodel.Interface) error {
	d.updated++
	return nil
}

func (d *resourceSyncServiceProbeDao) DeleteModel(string, ...interface{}) error {
	d.deleted++
	return nil
}

type resourceSyncRelationDao struct {
	dbdao.TenantServiceRelationDao
	added   int
	deleted int
}

func (d *resourceSyncRelationDao) AddModel(dbmodel.Interface) error {
	d.added++
	return nil
}

func (d *resourceSyncRelationDao) DeleteRelationByDepID(serviceID, depID string) error {
	d.deleted++
	return nil
}

type resourceSyncComponentK8sAttributeDao struct {
	dbdao.ComponentK8sAttributeDao
	attributes map[string]*dbmodel.ComponentK8sAttributes
	deleted    []string
}

func (d *resourceSyncComponentK8sAttributeDao) CreateOrUpdateAttributesInBatch(attributes []*dbmodel.ComponentK8sAttributes) error {
	if d.attributes == nil {
		d.attributes = map[string]*dbmodel.ComponentK8sAttributes{}
	}
	for _, attr := range attributes {
		copied := *attr
		d.attributes[attr.Name] = &copied
	}
	return nil
}

func (d *resourceSyncComponentK8sAttributeDao) DeleteByComponentIDAndName(componentID, name string) error {
	d.deleted = append(d.deleted, componentID+"/"+name)
	if d.attributes != nil {
		delete(d.attributes, name)
	}
	return nil
}

type noopMQClient struct{}

func (m *noopMQClient) Enqueue(context.Context, *mqpb.EnqueueRequest, ...grpc.CallOption) (*mqpb.TaskReply, error) {
	return nil, nil
}

func (m *noopMQClient) Topics(context.Context, *mqpb.TopicRequest, ...grpc.CallOption) (*mqpb.TaskReply, error) {
	return nil, nil
}

func (m *noopMQClient) Dequeue(context.Context, *mqpb.DequeueRequest, ...grpc.CallOption) (*mqpb.TaskMessage, error) {
	return nil, nil
}

func (m *noopMQClient) Close() {}

func (m *noopMQClient) SendBuilderTopic(gclient.TaskStruct) error {
	return nil
}

// capability_id: rainbond.vm-probe-change-syncs-spec
func TestServiceProbeSyncsVirtualMachineSpecWhenVMProbeChanges(t *testing.T) {
	for _, tc := range []struct {
		name       string
		action     string
		wantAdd    int
		wantUpdate int
		wantDelete int
	}{
		{name: "add", action: "add", wantAdd: 1},
		{name: "update", action: "update", wantUpdate: 1},
		{name: "delete", action: "delete", wantDelete: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			serviceDao := &resourceSyncTenantServiceDao{
				service: &dbmodel.TenantServices{
					ServiceID:    "service-vm",
					ServiceAlias: "service-vm",
					ExtendMethod: "vm",
				},
			}
			probeDao := &resourceSyncServiceProbeDao{}
			db.SetTestManager(resourceSyncTestManager{
				serviceDao:      serviceDao,
				eventDao:        &resourceSyncEventDao{},
				serviceProbeDao: probeDao,
			})
			defer db.SetTestManager(nil)

			syncedServiceID := ""
			action := &ServiceAction{
				syncVirtualMachineSpecHook: func(serviceID string) error {
					syncedServiceID = serviceID
					return nil
				},
			}

			err := action.ServiceProbe(&dbmodel.TenantServiceProbe{
				ServiceID: "service-vm",
				ProbeID:   "probe-vm",
				Mode:      "readiness",
			}, tc.action)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if syncedServiceID != "service-vm" {
				t.Fatalf("expected vm spec sync for service-vm, got %q", syncedServiceID)
			}
			if probeDao.added != tc.wantAdd || probeDao.updated != tc.wantUpdate || probeDao.deleted != tc.wantDelete {
				t.Fatalf("unexpected probe dao calls: add=%d update=%d delete=%d", probeDao.added, probeDao.updated, probeDao.deleted)
			}
		})
	}
}

func TestServiceUpdateSyncsVirtualMachineSpecWhenVMResourcesChange(t *testing.T) {
	serviceDao := &resourceSyncTenantServiceDao{
		service: &dbmodel.TenantServices{
			ServiceID:        "service-vm",
			ServiceAlias:     "service-vm",
			ExtendMethod:     "vm",
			ContainerCPU:     500,
			ContainerMemory:  512,
			ContainerGPU:     0,
			K8sComponentName: "demo-vm",
		},
	}
	db.SetTestManager(resourceSyncTestManager{
		serviceDao: serviceDao,
		eventDao:   &resourceSyncEventDao{},
	})
	defer db.SetTestManager(nil)

	syncedServiceID := ""
	action := &ServiceAction{
		syncVirtualMachineSpecHook: func(serviceID string) error {
			syncedServiceID = serviceID
			return nil
		},
	}

	err := action.ServiceUpdate(map[string]interface{}{
		"service_id":         "service-vm",
		"container_cpu":      1000,
		"container_memory":   1024,
		"k8s_component_name": "demo-vm",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if syncedServiceID != "service-vm" {
		t.Fatalf("expected vm spec sync for service-vm, got %q", syncedServiceID)
	}
	if serviceDao.updatedService == nil {
		t.Fatal("expected updated service to be persisted")
	}
	if serviceDao.updatedService.ContainerCPU != 1000 || serviceDao.updatedService.ContainerMemory != 1024 {
		t.Fatalf("expected updated resources to be persisted, got cpu=%d mem=%d", serviceDao.updatedService.ContainerCPU, serviceDao.updatedService.ContainerMemory)
	}
}

func TestServiceVerticalSyncsVirtualMachineSpecWhenVMResourcesChange(t *testing.T) {
	serviceDao := &resourceSyncTenantServiceDao{
		service: &dbmodel.TenantServices{
			ServiceID:       "service-vm",
			ServiceAlias:    "service-vm",
			ExtendMethod:    "vm",
			ContainerCPU:    500,
			ContainerMemory: 512,
			ContainerGPU:    0,
		},
	}
	db.SetTestManager(resourceSyncTestManager{
		serviceDao: serviceDao,
		eventDao:   &resourceSyncEventDao{},
	})
	defer db.SetTestManager(nil)

	syncedServiceID := ""
	cpu := 1000
	memory := 1024
	action := &ServiceAction{
		MQClient: &noopMQClient{},
		syncVirtualMachineSpecHook: func(serviceID string) error {
			syncedServiceID = serviceID
			return nil
		},
	}

	err := action.ServiceVertical(context.Background(), &workermodel.VerticalScalingTaskBody{
		ServiceID:       "service-vm",
		ContainerCPU:    &cpu,
		ContainerMemory: &memory,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if syncedServiceID != "service-vm" {
		t.Fatalf("expected vm spec sync for service-vm, got %q", syncedServiceID)
	}
	if serviceDao.updatedService == nil {
		t.Fatal("expected updated service to be persisted")
	}
	if serviceDao.updatedService.ContainerCPU != 1000 || serviceDao.updatedService.ContainerMemory != 1024 {
		t.Fatalf("expected updated resources to be persisted, got cpu=%d mem=%d", serviceDao.updatedService.ContainerCPU, serviceDao.updatedService.ContainerMemory)
	}
}

func TestServiceDependSyncsVirtualMachineSpecWhenVMDependencyChanges(t *testing.T) {
	for _, tc := range []struct {
		name   string
		action string
	}{
		{name: "add", action: "add"},
		{name: "delete", action: "delete"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			serviceDao := &resourceSyncTenantServiceDao{
				service: &dbmodel.TenantServices{
					ServiceID:    "service-vm",
					ServiceAlias: "service-vm",
					ExtendMethod: "vm",
				},
			}
			db.SetTestManager(resourceSyncTestManager{
				serviceDao: serviceDao,
				eventDao:   &resourceSyncEventDao{},
				relationDao: &resourceSyncRelationDao{},
			})
			defer db.SetTestManager(nil)

			syncedServiceID := ""
			action := &ServiceAction{
				syncVirtualMachineSpecHook: func(serviceID string) error {
					syncedServiceID = serviceID
					return nil
				},
			}

			err := action.ServiceDepend(tc.action, &apimodel.DependService{
				TenantID:     "tenant-1",
				ServiceID:    "service-vm",
				DepServiceID: "dep-service",
			})
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if syncedServiceID != "service-vm" {
				t.Fatalf("expected vm spec sync for service-vm, got %q", syncedServiceID)
			}
		})
	}
}
