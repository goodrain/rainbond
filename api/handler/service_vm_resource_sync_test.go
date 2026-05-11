package handler

import (
	"context"
	"testing"

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
	serviceDao dbdao.TenantServiceDao
	eventDao   dbdao.EventDao
}

func (m resourceSyncTestManager) TenantServiceDao() dbdao.TenantServiceDao {
	return m.serviceDao
}

func (m resourceSyncTestManager) ServiceEventDao() dbdao.EventDao {
	return m.eventDao
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
