package handler

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	workermodel "github.com/goodrain/rainbond/worker/discover/model"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	kubefake "k8s.io/client-go/kubernetes/fake"
	v1 "kubevirt.io/api/core/v1"
	kubecli "kubevirt.io/client-go/kubecli"
)

func TestServiceVerticalVMLiveUpdatePatchesRunningVM(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	serviceDao := &resourceSyncTenantServiceDao{
		service: &dbmodel.TenantServices{
			ServiceID:       "service-vm",
			ServiceAlias:    "service-vm",
			ExtendMethod:    "vm",
			ContainerCPU:    6000,
			ContainerMemory: 12288,
			ContainerGPU:    0,
		},
	}
	eventDao := &resourceSyncEventDao{}
	db.SetTestManager(resourceSyncTestManager{
		serviceDao: serviceDao,
		eventDao:   eventDao,
	})
	defer db.SetTestManager(nil)

	mockClient := kubecli.NewMockKubevirtClient(ctrl)
	mockVMList := kubecli.NewMockVirtualMachineInterface(ctrl)
	mockVMPatch := kubecli.NewMockVirtualMachineInterface(ctrl)
	mockVMIList := kubecli.NewMockVirtualMachineInstanceInterface(ctrl)
	mockKV := kubecli.NewMockKubeVirtInterface(ctrl)

	mockClient.EXPECT().VirtualMachine("").Return(mockVMList)
	mockVMList.EXPECT().List(gomock.Any(), metav1.ListOptions{LabelSelector: "service_id=service-vm"}).Return(&v1.VirtualMachineList{
		Items: []v1.VirtualMachine{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "demo-vm", Namespace: "demo-ns"},
				Spec: v1.VirtualMachineSpec{
					Template: &v1.VirtualMachineInstanceTemplateSpec{
						Spec: v1.VirtualMachineInstanceSpec{
							Domain: v1.DomainSpec{
								CPU: &v1.CPU{
									Sockets:    6,
									Cores:      1,
									Threads:    1,
									MaxSockets: 8,
								},
								Memory: &v1.Memory{
									Guest:    quantityPtr("12Gi"),
									MaxGuest: quantityPtr("16Gi"),
								},
							},
						},
					},
				},
				Status: v1.VirtualMachineStatus{PrintableStatus: v1.VirtualMachineStatusRunning},
			},
		},
	}, nil)

	mockClient.EXPECT().VirtualMachineInstance("").Return(mockVMIList)
	mockVMIList.EXPECT().List(gomock.Any(), metav1.ListOptions{LabelSelector: "service_id=service-vm"}).Return(&v1.VirtualMachineInstanceList{
		Items: []v1.VirtualMachineInstance{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "demo-vm", Namespace: "demo-ns"},
				Status: v1.VirtualMachineInstanceStatus{
					Phase: v1.Running,
					Conditions: []v1.VirtualMachineInstanceCondition{
						{Type: v1.VirtualMachineInstanceIsMigratable, Status: "True"},
					},
				},
			},
		},
	}, nil)

	mockClient.EXPECT().KubeVirt("").Return(mockKV)
	mockKV.EXPECT().List(gomock.Any(), gomock.Any()).Return(&v1.KubeVirtList{
		Items: []v1.KubeVirt{
			{
				Spec: v1.KubeVirtSpec{
					WorkloadUpdateStrategy: v1.KubeVirtWorkloadUpdateStrategy{
						WorkloadUpdateMethods: []v1.WorkloadUpdateMethod{v1.WorkloadUpdateMethodLiveMigrate},
					},
					Configuration: v1.KubeVirtConfiguration{
						VMRolloutStrategy: func() *v1.VMRolloutStrategy {
							strategy := v1.VMRolloutStrategyLiveUpdate
							return &strategy
						}(),
					},
				},
			},
		},
	}, nil)

	mockClient.EXPECT().VirtualMachine("demo-ns").Return(mockVMPatch)
	mockVMPatch.EXPECT().Patch(gomock.Any(), "demo-vm", types.JSONPatchType, gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, _ string, _ types.PatchType, data []byte, _ metav1.PatchOptions, _ ...string) (*v1.VirtualMachine, error) {
			payload := string(data)
			if !strings.Contains(payload, "/spec/template/spec/domain/cpu/sockets") {
				t.Fatalf("expected cpu sockets patch, got %s", payload)
			}
			return &v1.VirtualMachine{}, nil
		},
	)

	action := &ServiceAction{
		MQClient:       &noopMQClient{},
		kubevirtClient: mockClient,
	}
	newCPU := 7000
	memory := 12288
	err := action.ServiceVertical(context.Background(), &workermodel.VerticalScalingTaskBody{
		ServiceID:       "service-vm",
		ContainerCPU:    &newCPU,
		ContainerMemory: &memory,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if serviceDao.updatedService == nil || serviceDao.updatedService.ContainerCPU != 7000 {
		t.Fatalf("expected updated cpu persisted, got %#v", serviceDao.updatedService)
	}
}

func TestServiceVerticalVMStoppedFallsBackToSpecSync(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	serviceDao := &resourceSyncTenantServiceDao{
		service: &dbmodel.TenantServices{
			ServiceID:       "service-vm",
			ServiceAlias:    "service-vm",
			ExtendMethod:    "vm",
			ContainerCPU:    4000,
			ContainerMemory: 8192,
			ContainerGPU:    0,
		},
	}
	db.SetTestManager(resourceSyncTestManager{
		serviceDao: serviceDao,
		eventDao:   &resourceSyncEventDao{},
	})
	defer db.SetTestManager(nil)

	mockClient := kubecli.NewMockKubevirtClient(ctrl)
	mockVMList := kubecli.NewMockVirtualMachineInterface(ctrl)

	mockClient.EXPECT().VirtualMachine("").Return(mockVMList)
	mockVMList.EXPECT().List(gomock.Any(), metav1.ListOptions{LabelSelector: "service_id=service-vm"}).Return(&v1.VirtualMachineList{
		Items: []v1.VirtualMachine{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "demo-vm", Namespace: "demo-ns"},
				Status:     v1.VirtualMachineStatus{PrintableStatus: v1.VirtualMachineStatusStopped},
			},
		},
	}, nil)

	syncedServiceID := ""
	action := &ServiceAction{
		MQClient:       &noopMQClient{},
		kubevirtClient: mockClient,
		syncVirtualMachineSpecHook: func(serviceID string) error {
			syncedServiceID = serviceID
			return nil
		},
	}
	newCPU := 5000
	newMemory := 10240
	err := action.ServiceVertical(context.Background(), &workermodel.VerticalScalingTaskBody{
		ServiceID:       "service-vm",
		ContainerCPU:    &newCPU,
		ContainerMemory: &newMemory,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if syncedServiceID != "service-vm" {
		t.Fatalf("expected vm spec sync for stopped VM, got %q", syncedServiceID)
	}
}

// capability_id: rainbond.vm-live-update.unsupported-auto-restart
func TestServiceVerticalVMNonMigratableFallsBackToSpecSyncAndRestart(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	serviceDao := &resourceSyncTenantServiceDao{
		service: &dbmodel.TenantServices{
			ServiceID:       "service-vm",
			ServiceAlias:    "service-vm",
			ExtendMethod:    "vm",
			ContainerCPU:    4000,
			ContainerMemory: 8192,
			ContainerGPU:    0,
		},
	}
	eventDao := &resourceSyncEventDao{}
	db.SetTestManager(resourceSyncTestManager{
		serviceDao: serviceDao,
		eventDao:   eventDao,
	})
	defer db.SetTestManager(nil)

	mockClient := kubecli.NewMockKubevirtClient(ctrl)
	mockVMInterface := kubecli.NewMockVirtualMachineInterface(ctrl)

	syncedServiceID := ""
	action := &ServiceAction{
		MQClient:       &noopMQClient{},
		kubevirtClient: mockClient,
		syncVirtualMachineSpecHook: func(serviceID string) error {
			syncedServiceID = serviceID
			return nil
		},
	}
	action.getVirtualMachineByServiceIDHook = func(serviceID string) (*v1.VirtualMachine, error) {
		return &v1.VirtualMachine{
			ObjectMeta: metav1.ObjectMeta{Name: "demo-vm", Namespace: "demo-ns"},
			Status:     v1.VirtualMachineStatus{PrintableStatus: v1.VirtualMachineStatusRunning},
		}, nil
	}
	action.getVirtualMachineInstanceByServiceIDHook = func(serviceID string) (*v1.VirtualMachineInstance, error) {
		return &v1.VirtualMachineInstance{Status: v1.VirtualMachineInstanceStatus{
			Phase: v1.Running,
			Conditions: []v1.VirtualMachineInstanceCondition{
				{
					Type:    v1.VirtualMachineInstanceIsMigratable,
					Status:  "False",
					Message: "PVC manual82 is not shared",
				},
			},
		}}, nil
	}

	mockClient.EXPECT().VirtualMachine("demo-ns").Return(mockVMInterface)
	mockVMInterface.EXPECT().Restart(gomock.Any(), "demo-vm", gomock.Any()).Return(nil)

	newMemory := 10240
	err := action.ServiceVertical(context.Background(), &workermodel.VerticalScalingTaskBody{
		ServiceID:       "service-vm",
		ContainerMemory: &newMemory,
	})
	if err != nil {
		t.Fatalf("expected non-migratable running VM to restart instead of failing live update, got %v", err)
	}
	if syncedServiceID != "service-vm" {
		t.Fatalf("expected vm spec sync before restart, got %q", syncedServiceID)
	}
	if serviceDao.updatedService == nil || serviceDao.updatedService.ContainerMemory != 10240 {
		t.Fatalf("expected updated memory to persist, got %#v", serviceDao.updatedService)
	}
	if len(eventDao.statuses) == 0 || eventDao.statuses[len(eventDao.statuses)-1] != dbmodel.EventStatusSuccess {
		t.Fatalf("expected success event status, got %#v", eventDao.statuses)
	}
}

// capability_id: rainbond.vm-live-update.unsupported-auto-restart
func TestServiceVerticalVMPatchMigrationErrorFallsBackToSpecSyncAndRestart(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	serviceDao := &resourceSyncTenantServiceDao{
		service: &dbmodel.TenantServices{
			ServiceID:       "service-vm",
			ServiceAlias:    "service-vm",
			ExtendMethod:    "vm",
			ContainerCPU:    4000,
			ContainerMemory: 8192,
			ContainerGPU:    0,
		},
	}
	eventDao := &resourceSyncEventDao{}
	db.SetTestManager(resourceSyncTestManager{
		serviceDao: serviceDao,
		eventDao:   eventDao,
	})
	defer db.SetTestManager(nil)

	mockClient := kubecli.NewMockKubevirtClient(ctrl)
	mockVMInterface := kubecli.NewMockVirtualMachineInterface(ctrl)

	syncedServiceID := ""
	action := &ServiceAction{
		MQClient:       &noopMQClient{},
		kubevirtClient: mockClient,
		syncVirtualMachineSpecHook: func(serviceID string) error {
			syncedServiceID = serviceID
			return nil
		},
	}
	action.isVMLiveUpdateClusterConfiguredHook = func(ctx context.Context) bool { return true }
	action.getVirtualMachineByServiceIDHook = func(serviceID string) (*v1.VirtualMachine, error) {
		return &v1.VirtualMachine{
			ObjectMeta: metav1.ObjectMeta{Name: "demo-vm", Namespace: "demo-ns"},
			Spec: v1.VirtualMachineSpec{
				Template: &v1.VirtualMachineInstanceTemplateSpec{
					Spec: v1.VirtualMachineInstanceSpec{
						Domain: v1.DomainSpec{
							Memory: &v1.Memory{
								Guest:    quantityPtr("8Gi"),
								MaxGuest: quantityPtr("16Gi"),
							},
						},
					},
				},
			},
			Status: v1.VirtualMachineStatus{PrintableStatus: v1.VirtualMachineStatusRunning},
		}, nil
	}
	action.getVirtualMachineInstanceByServiceIDHook = func(serviceID string) (*v1.VirtualMachineInstance, error) {
		return &v1.VirtualMachineInstance{Status: v1.VirtualMachineInstanceStatus{
			Phase: v1.Running,
			Conditions: []v1.VirtualMachineInstanceCondition{
				{Type: v1.VirtualMachineInstanceIsMigratable, Status: "True"},
			},
		}}, nil
	}

	mockClient.EXPECT().VirtualMachine("demo-ns").Return(mockVMInterface).Times(2)
	mockVMInterface.EXPECT().Patch(gomock.Any(), "demo-vm", types.JSONPatchType, gomock.Any(), gomock.Any()).
		Return(nil, errors.New("cannot migrate VMI: PVC manual82 is not shared, live migration requires that all PVCs must be shared (using ReadWriteMany access mode)"))
	mockVMInterface.EXPECT().Restart(gomock.Any(), "demo-vm", gomock.Any()).Return(nil)

	newMemory := 10240
	err := action.ServiceVertical(context.Background(), &workermodel.VerticalScalingTaskBody{
		ServiceID:       "service-vm",
		ContainerMemory: &newMemory,
	})
	if err != nil {
		t.Fatalf("expected live migration pvc error to restart instead of failing vertical update, got %v", err)
	}
	if syncedServiceID != "service-vm" {
		t.Fatalf("expected vm spec sync before restart, got %q", syncedServiceID)
	}
	if serviceDao.updatedService == nil || serviceDao.updatedService.ContainerMemory != 10240 {
		t.Fatalf("expected updated memory to persist, got %#v", serviceDao.updatedService)
	}
	if len(eventDao.statuses) == 0 || eventDao.statuses[len(eventDao.statuses)-1] != dbmodel.EventStatusSuccess {
		t.Fatalf("expected success event status, got %#v", eventDao.statuses)
	}
}

// capability_id: rainbond.vm-live-update.cpu-memory-combined-rejected
func TestServiceVerticalVMLiveUpdateRejectsCombinedCPUAndMemoryChange(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	serviceDao := &resourceSyncTenantServiceDao{
		service: &dbmodel.TenantServices{
			ServiceID:       "service-vm",
			ServiceAlias:    "service-vm",
			ExtendMethod:    "vm",
			ContainerCPU:    8000,
			ContainerMemory: 16384,
			ContainerGPU:    0,
		},
	}
	eventDao := &resourceSyncEventDao{}
	db.SetTestManager(resourceSyncTestManager{
		serviceDao: serviceDao,
		eventDao:   eventDao,
	})
	defer db.SetTestManager(nil)

	syncedServiceID := ""
	action := &ServiceAction{
		MQClient:       &noopMQClient{},
		kubevirtClient: kubecli.NewMockKubevirtClient(ctrl),
		syncVirtualMachineSpecHook: func(serviceID string) error {
			syncedServiceID = serviceID
			return nil
		},
	}
	action.isVMLiveUpdateClusterConfiguredHook = func(ctx context.Context) bool { return true }
	action.getVirtualMachineByServiceIDHook = func(serviceID string) (*v1.VirtualMachine, error) {
		return &v1.VirtualMachine{
			ObjectMeta: metav1.ObjectMeta{Name: "demo-vm", Namespace: "demo-ns"},
			Spec: v1.VirtualMachineSpec{
				Template: &v1.VirtualMachineInstanceTemplateSpec{
					Spec: v1.VirtualMachineInstanceSpec{
						Domain: v1.DomainSpec{
							CPU: &v1.CPU{
								Sockets:    8,
								Cores:      1,
								Threads:    1,
								MaxSockets: 16,
							},
							Memory: &v1.Memory{
								Guest:    quantityPtr("16Gi"),
								MaxGuest: quantityPtr("64Gi"),
							},
						},
					},
				},
			},
			Status: v1.VirtualMachineStatus{PrintableStatus: v1.VirtualMachineStatusRunning},
		}, nil
	}
	action.getVirtualMachineInstanceByServiceIDHook = func(serviceID string) (*v1.VirtualMachineInstance, error) {
		return &v1.VirtualMachineInstance{Status: v1.VirtualMachineInstanceStatus{
			Phase: v1.Running,
			Conditions: []v1.VirtualMachineInstanceCondition{
				{Type: v1.VirtualMachineInstanceIsMigratable, Status: "True"},
			},
		}}, nil
	}

	newCPU := 10000
	newMemory := 24576
	err := action.ServiceVertical(context.Background(), &workermodel.VerticalScalingTaskBody{
		ServiceID:       "service-vm",
		ContainerCPU:    &newCPU,
		ContainerMemory: &newMemory,
	})
	if err == nil {
		t.Fatal("expected combined cpu and memory live update to be rejected")
	}
	if err.Error() != "运行中虚拟机 CPU 和内存热更新请分两次操作，不支持一次同时修改。" {
		t.Fatalf("expected combined resource rejection message, got %q", err.Error())
	}
	if syncedServiceID != "service-vm" {
		t.Fatalf("expected rollback vm spec sync, got %q", syncedServiceID)
	}
	if serviceDao.service == nil || serviceDao.service.ContainerCPU != 8000 || serviceDao.service.ContainerMemory != 16384 {
		t.Fatalf("expected resources rollback to original values, got %#v", serviceDao.service)
	}
	if len(eventDao.statuses) == 0 || eventDao.statuses[len(eventDao.statuses)-1] != dbmodel.EventStatusFailure {
		t.Fatalf("expected failure event status, got %#v", eventDao.statuses)
	}
}

// capability_id: rainbond.vm-live-update.running-cpu-shrink-rejected
func TestServiceVerticalVMLiveUpdateRejectsRunningVMCPUShrink(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	serviceDao := &resourceSyncTenantServiceDao{
		service: &dbmodel.TenantServices{
			ServiceID:       "service-vm",
			ServiceAlias:    "service-vm",
			ExtendMethod:    "vm",
			ContainerCPU:    6000,
			ContainerMemory: 12288,
			ContainerGPU:    0,
		},
	}
	eventDao := &resourceSyncEventDao{}
	db.SetTestManager(resourceSyncTestManager{
		serviceDao: serviceDao,
		eventDao:   eventDao,
	})
	defer db.SetTestManager(nil)

	syncedServiceID := ""
	action := &ServiceAction{
		MQClient:       &noopMQClient{},
		kubevirtClient: kubecli.NewMockKubevirtClient(ctrl),
		syncVirtualMachineSpecHook: func(serviceID string) error {
			syncedServiceID = serviceID
			return nil
		},
	}
	action.isVMLiveUpdateClusterConfiguredHook = func(ctx context.Context) bool { return true }
	action.getVirtualMachineByServiceIDHook = func(serviceID string) (*v1.VirtualMachine, error) {
		return &v1.VirtualMachine{
			Status: v1.VirtualMachineStatus{PrintableStatus: v1.VirtualMachineStatusRunning},
		}, nil
	}
	action.getVirtualMachineInstanceByServiceIDHook = func(serviceID string) (*v1.VirtualMachineInstance, error) {
		return &v1.VirtualMachineInstance{Status: v1.VirtualMachineInstanceStatus{
			Phase: v1.Running,
			Conditions: []v1.VirtualMachineInstanceCondition{
				{Type: v1.VirtualMachineInstanceIsMigratable, Status: "True"},
			},
		}}, nil
	}

	newCPU := 4000
	err := action.ServiceVertical(context.Background(), &workermodel.VerticalScalingTaskBody{
		ServiceID:    "service-vm",
		ContainerCPU: &newCPU,
	})
	if err == nil {
		t.Fatal("expected running vm cpu shrink to be rejected")
	}
	if err.Error() != "虚拟机 CPU 热更新仅支持扩容，不支持缩容，请停机后再修改规格。" {
		t.Fatalf("expected localized rejection message, got %q", err.Error())
	}
	if syncedServiceID != "service-vm" {
		t.Fatalf("expected rollback vm spec sync, got %q", syncedServiceID)
	}
	if serviceDao.service == nil || serviceDao.service.ContainerCPU != 6000 {
		t.Fatalf("expected cpu rollback to original value, got %#v", serviceDao.service)
	}
	if len(eventDao.statuses) == 0 || eventDao.statuses[len(eventDao.statuses)-1] != dbmodel.EventStatusFailure {
		t.Fatalf("expected failure event status, got %#v", eventDao.statuses)
	}
}

// capability_id: rainbond.vm-live-update.running-memory-shrink-rejected
func TestServiceVerticalVMLiveUpdateRejectsRunningVMMemoryShrink(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	serviceDao := &resourceSyncTenantServiceDao{
		service: &dbmodel.TenantServices{
			ServiceID:       "service-vm",
			ServiceAlias:    "service-vm",
			ExtendMethod:    "vm",
			ContainerCPU:    6000,
			ContainerMemory: 12288,
			ContainerGPU:    0,
		},
	}
	eventDao := &resourceSyncEventDao{}
	db.SetTestManager(resourceSyncTestManager{
		serviceDao: serviceDao,
		eventDao:   eventDao,
	})
	defer db.SetTestManager(nil)

	syncedServiceID := ""
	action := &ServiceAction{
		MQClient:       &noopMQClient{},
		kubevirtClient: kubecli.NewMockKubevirtClient(ctrl),
		syncVirtualMachineSpecHook: func(serviceID string) error {
			syncedServiceID = serviceID
			return nil
		},
	}
	action.isVMLiveUpdateClusterConfiguredHook = func(ctx context.Context) bool { return true }
	action.getVirtualMachineByServiceIDHook = func(serviceID string) (*v1.VirtualMachine, error) {
		return &v1.VirtualMachine{
			Status: v1.VirtualMachineStatus{PrintableStatus: v1.VirtualMachineStatusRunning},
		}, nil
	}
	action.getVirtualMachineInstanceByServiceIDHook = func(serviceID string) (*v1.VirtualMachineInstance, error) {
		return &v1.VirtualMachineInstance{Status: v1.VirtualMachineInstanceStatus{
			Phase: v1.Running,
			Conditions: []v1.VirtualMachineInstanceCondition{
				{Type: v1.VirtualMachineInstanceIsMigratable, Status: "True"},
			},
		}}, nil
	}

	newMemory := 8192
	err := action.ServiceVertical(context.Background(), &workermodel.VerticalScalingTaskBody{
		ServiceID:       "service-vm",
		ContainerMemory: &newMemory,
	})
	if err == nil {
		t.Fatal("expected running vm memory shrink to be rejected")
	}
	if err.Error() != "虚拟机内存热更新仅支持扩容，不支持缩容，请停机后再修改规格。" {
		t.Fatalf("expected localized rejection message, got %q", err.Error())
	}
	if syncedServiceID != "service-vm" {
		t.Fatalf("expected rollback vm spec sync, got %q", syncedServiceID)
	}
	if serviceDao.service == nil || serviceDao.service.ContainerMemory != 12288 {
		t.Fatalf("expected memory rollback to original value, got %#v", serviceDao.service)
	}
	if len(eventDao.statuses) == 0 || eventDao.statuses[len(eventDao.statuses)-1] != dbmodel.EventStatusFailure {
		t.Fatalf("expected failure event status, got %#v", eventDao.statuses)
	}
}

// capability_id: rainbond.vm-live-update.memory-target-below-max-guest
func TestServiceVerticalVMLiveUpdateRejectsRunningVMMemoryAtMaxGuest(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	serviceDao := &resourceSyncTenantServiceDao{
		service: &dbmodel.TenantServices{
			ServiceID:       "service-vm",
			ServiceAlias:    "service-vm",
			ExtendMethod:    "vm",
			ContainerCPU:    4000,
			ContainerMemory: 8192,
			ContainerGPU:    0,
		},
	}
	eventDao := &resourceSyncEventDao{}
	db.SetTestManager(resourceSyncTestManager{
		serviceDao: serviceDao,
		eventDao:   eventDao,
	})
	defer db.SetTestManager(nil)

	syncedServiceID := ""
	action := &ServiceAction{
		MQClient:       &noopMQClient{},
		kubevirtClient: kubecli.NewMockKubevirtClient(ctrl),
		syncVirtualMachineSpecHook: func(serviceID string) error {
			syncedServiceID = serviceID
			return nil
		},
	}
	action.isVMLiveUpdateClusterConfiguredHook = func(ctx context.Context) bool { return true }
	action.getVirtualMachineByServiceIDHook = func(serviceID string) (*v1.VirtualMachine, error) {
		return &v1.VirtualMachine{
			ObjectMeta: metav1.ObjectMeta{Name: "demo-vm", Namespace: "demo-ns"},
			Spec: v1.VirtualMachineSpec{
				Template: &v1.VirtualMachineInstanceTemplateSpec{
					Spec: v1.VirtualMachineInstanceSpec{
						Domain: v1.DomainSpec{
							Memory: &v1.Memory{
								Guest:    quantityPtr("8Gi"),
								MaxGuest: quantityPtr("16Gi"),
							},
						},
					},
				},
			},
			Status: v1.VirtualMachineStatus{PrintableStatus: v1.VirtualMachineStatusRunning},
		}, nil
	}
	action.getVirtualMachineInstanceByServiceIDHook = func(serviceID string) (*v1.VirtualMachineInstance, error) {
		return &v1.VirtualMachineInstance{Status: v1.VirtualMachineInstanceStatus{
			Phase: v1.Running,
			Conditions: []v1.VirtualMachineInstanceCondition{
				{Type: v1.VirtualMachineInstanceIsMigratable, Status: "True"},
			},
		}}, nil
	}

	newMemory := 16384
	err := action.ServiceVertical(context.Background(), &workermodel.VerticalScalingTaskBody{
		ServiceID:       "service-vm",
		ContainerMemory: &newMemory,
	})
	if err == nil {
		t.Fatal("expected running vm memory target at maxGuest to be rejected")
	}
	if err.Error() != "vm memory live update target must be lower than maxGuest" {
		t.Fatalf("expected maxGuest headroom rejection message, got %q", err.Error())
	}
	if syncedServiceID != "service-vm" {
		t.Fatalf("expected rollback vm spec sync, got %q", syncedServiceID)
	}
	if serviceDao.service == nil || serviceDao.service.ContainerMemory != 8192 {
		t.Fatalf("expected memory rollback to original value, got %#v", serviceDao.service)
	}
	if len(eventDao.statuses) == 0 || eventDao.statuses[len(eventDao.statuses)-1] != dbmodel.EventStatusFailure {
		t.Fatalf("expected failure event status, got %#v", eventDao.statuses)
	}
}

func TestGetVMLiveUpdateCapabilityIgnoresRemovedNetworkFields(t *testing.T) {
	serviceDao := &resourceSyncTenantServiceDao{
		service: &dbmodel.TenantServices{
			ServiceID:       "service-vm",
			ServiceAlias:    "service-vm",
			ExtendMethod:    "vm",
			ContainerCPU:    6000,
			ContainerMemory: 12288,
		},
	}
	db.SetTestManager(resourceSyncTestManager{
		serviceDao: serviceDao,
		eventDao:   &resourceSyncEventDao{},
	})
	defer db.SetTestManager(nil)

	action := &ServiceAction{
		syncVirtualMachineSpecHook: func(serviceID string) error { return nil },
	}
	action.loadVMRuntimeSpecExtensionSetHook = func(componentID string) (map[string]string, error) {
		return map[string]string{"vm_network_mode": "fixed"}, nil
	}
	action.loadVMRuntimeDeviceExtensionSetHook = func(componentID string) (map[string]string, error) {
		return map[string]string{}, nil
	}
	action.isVMLiveUpdateClusterConfiguredHook = func(ctx context.Context) bool { return true }
	action.getVirtualMachineByServiceIDHook = func(serviceID string) (*v1.VirtualMachine, error) {
		return &v1.VirtualMachine{Status: v1.VirtualMachineStatus{PrintableStatus: v1.VirtualMachineStatusRunning}}, nil
	}
	action.getVirtualMachineInstanceByServiceIDHook = func(serviceID string) (*v1.VirtualMachineInstance, error) {
		return &v1.VirtualMachineInstance{Status: v1.VirtualMachineInstanceStatus{
			Phase: v1.Running,
			Conditions: []v1.VirtualMachineInstanceCondition{
				{Type: v1.VirtualMachineInstanceIsMigratable, Status: "True"},
			},
		}}, nil
	}

	capability := action.GetVMLiveUpdateCapability("service-vm")
	if !capability.CPUHotUpdateSupported || !capability.MemoryHotUpdateSupported {
		t.Fatalf("expected removed vm network fields to no longer block hot update, got %#v", capability)
	}
	if capability.HotUpdateReason != "" {
		t.Fatalf("did not expect removed vm network fields to set hot update reason, got %#v", capability)
	}
}

func TestGetVMLiveUpdateCapabilityRejectsGPUVM(t *testing.T) {
	serviceDao := &resourceSyncTenantServiceDao{
		service: &dbmodel.TenantServices{
			ServiceID:       "service-vm",
			ServiceAlias:    "service-vm",
			ExtendMethod:    "vm",
			ContainerCPU:    6000,
			ContainerMemory: 12288,
		},
	}
	db.SetTestManager(resourceSyncTestManager{
		serviceDao: serviceDao,
		eventDao:   &resourceSyncEventDao{},
	})
	defer db.SetTestManager(nil)

	action := &ServiceAction{}
	action.loadVMRuntimeSpecExtensionSetHook = func(componentID string) (map[string]string, error) {
		return map[string]string{}, nil
	}
	action.loadVMRuntimeDeviceExtensionSetHook = func(componentID string) (map[string]string, error) {
		return map[string]string{"vm_gpu_enabled": "true"}, nil
	}
	action.isVMLiveUpdateClusterConfiguredHook = func(ctx context.Context) bool { return true }
	action.getVirtualMachineByServiceIDHook = func(serviceID string) (*v1.VirtualMachine, error) {
		return &v1.VirtualMachine{Status: v1.VirtualMachineStatus{PrintableStatus: v1.VirtualMachineStatusRunning}}, nil
	}
	action.getVirtualMachineInstanceByServiceIDHook = func(serviceID string) (*v1.VirtualMachineInstance, error) {
		return &v1.VirtualMachineInstance{Status: v1.VirtualMachineInstanceStatus{
			Phase: v1.Running,
			Conditions: []v1.VirtualMachineInstanceCondition{
				{Type: v1.VirtualMachineInstanceIsMigratable, Status: "True"},
			},
		}}, nil
	}

	capability := action.GetVMLiveUpdateCapability("service-vm")
	if capability.CPUHotUpdateSupported || capability.MemoryHotUpdateSupported {
		t.Fatalf("expected gpu vm to be unsupported, got %#v", capability)
	}
	if capability.HotUpdateReason == "" {
		t.Fatalf("expected gpu reason, got %#v", capability)
	}
}

// capability_id: rainbond.vm-live-update.capability-requires-installer-media-removal
func TestGetVMLiveUpdateCapabilityRejectsWhenInstallerMediaStillAttached(t *testing.T) {
	serviceDao := &resourceSyncTenantServiceDao{
		service: &dbmodel.TenantServices{
			ServiceID:       "service-vm",
			ServiceAlias:    "service-vm",
			ExtendMethod:    "vm",
			ContainerCPU:    6000,
			ContainerMemory: 12288,
		},
	}
	db.SetTestManager(resourceSyncTestManager{
		serviceDao: serviceDao,
		eventDao:   &resourceSyncEventDao{},
	})
	defer db.SetTestManager(nil)

	action := &ServiceAction{}
	action.loadVMRuntimeSpecExtensionSetHook = func(componentID string) (map[string]string, error) {
		return map[string]string{}, nil
	}
	action.loadVMRuntimeDeviceExtensionSetHook = func(componentID string) (map[string]string, error) {
		return map[string]string{}, nil
	}
	action.isVMLiveUpdateClusterConfiguredHook = func(ctx context.Context) bool { return true }
	action.getVirtualMachineByServiceIDHook = func(serviceID string) (*v1.VirtualMachine, error) {
		return &v1.VirtualMachine{
			Status: v1.VirtualMachineStatus{PrintableStatus: v1.VirtualMachineStatusRunning},
			Spec: v1.VirtualMachineSpec{
				Template: &v1.VirtualMachineInstanceTemplateSpec{
					Spec: v1.VirtualMachineInstanceSpec{
						Domain: v1.DomainSpec{
							Devices: v1.Devices{
								Disks: []v1.Disk{
									{
										Name: "vmimage",
										DiskDevice: v1.DiskDevice{
											CDRom: &v1.CDRomTarget{Bus: v1.DiskBusSATA},
										},
									},
								},
							},
						},
					},
				},
			},
		}, nil
	}
	action.getVirtualMachineInstanceByServiceIDHook = func(serviceID string) (*v1.VirtualMachineInstance, error) {
		return &v1.VirtualMachineInstance{Status: v1.VirtualMachineInstanceStatus{
			Phase: v1.Running,
			Conditions: []v1.VirtualMachineInstanceCondition{
				{Type: v1.VirtualMachineInstanceIsMigratable, Status: "True"},
			},
		}}, nil
	}

	capability := action.GetVMLiveUpdateCapability("service-vm")
	if capability.CPUHotUpdateSupported || capability.MemoryHotUpdateSupported {
		t.Fatalf("expected installer media to block live update capability, got %#v", capability)
	}
	if !strings.Contains(capability.HotUpdateReason, "初始化安装光盘") {
		t.Fatalf("expected installer media reason, got %#v", capability)
	}
}

// capability_id: rainbond.vm-live-update.installer-media-removal-required
func TestServiceVerticalVMLiveUpdateRejectsWhenInstallerMediaStillAttached(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	serviceDao := &resourceSyncTenantServiceDao{
		service: &dbmodel.TenantServices{
			ServiceID:       "service-vm",
			ServiceAlias:    "service-vm",
			ExtendMethod:    "vm",
			ContainerCPU:    4000,
			ContainerMemory: 8192,
			ContainerGPU:    0,
		},
	}
	eventDao := &resourceSyncEventDao{}
	db.SetTestManager(resourceSyncTestManager{
		serviceDao: serviceDao,
		eventDao:   eventDao,
	})
	defer db.SetTestManager(nil)

	syncedServiceID := ""
	action := &ServiceAction{
		MQClient:       &noopMQClient{},
		kubevirtClient: kubecli.NewMockKubevirtClient(ctrl),
		syncVirtualMachineSpecHook: func(serviceID string) error {
			syncedServiceID = serviceID
			return nil
		},
	}
	action.isVMLiveUpdateClusterConfiguredHook = func(ctx context.Context) bool { return true }
	action.getVirtualMachineByServiceIDHook = func(serviceID string) (*v1.VirtualMachine, error) {
		return &v1.VirtualMachine{
			Status: v1.VirtualMachineStatus{PrintableStatus: v1.VirtualMachineStatusRunning},
			Spec: v1.VirtualMachineSpec{
				Template: &v1.VirtualMachineInstanceTemplateSpec{
					Spec: v1.VirtualMachineInstanceSpec{
						Domain: v1.DomainSpec{
							Devices: v1.Devices{
								Disks: []v1.Disk{
									{
										Name: "vmimage",
										DiskDevice: v1.DiskDevice{
											CDRom: &v1.CDRomTarget{Bus: v1.DiskBusSATA},
										},
									},
								},
							},
						},
					},
				},
			},
		}, nil
	}
	action.getVirtualMachineInstanceByServiceIDHook = func(serviceID string) (*v1.VirtualMachineInstance, error) {
		return &v1.VirtualMachineInstance{Status: v1.VirtualMachineInstanceStatus{
			Phase: v1.Running,
			Conditions: []v1.VirtualMachineInstanceCondition{
				{Type: v1.VirtualMachineInstanceIsMigratable, Status: "True"},
			},
		}}, nil
	}

	newMemory := 10240
	err := action.ServiceVertical(context.Background(), &workermodel.VerticalScalingTaskBody{
		ServiceID:       "service-vm",
		ContainerMemory: &newMemory,
	})
	if err == nil {
		t.Fatal("expected live update to be rejected when installer media is still attached")
	}
	if !strings.Contains(err.Error(), "初始化安装光盘") {
		t.Fatalf("expected installer media rejection, got %q", err.Error())
	}
	if syncedServiceID != "service-vm" {
		t.Fatalf("expected rollback vm spec sync, got %q", syncedServiceID)
	}
	if serviceDao.service == nil || serviceDao.service.ContainerMemory != 8192 {
		t.Fatalf("expected memory rollback to original value, got %#v", serviceDao.service)
	}
	if len(eventDao.statuses) == 0 || eventDao.statuses[len(eventDao.statuses)-1] != dbmodel.EventStatusFailure {
		t.Fatalf("expected failure event status, got %#v", eventDao.statuses)
	}
}

// capability_id: rainbond.vm-live-update.migration-target-missing-auto-restart
func TestServiceVerticalVMLiveUpdateRestartsWhenNoMigrationTargetNode(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	serviceDao := &resourceSyncTenantServiceDao{
		service: &dbmodel.TenantServices{
			ServiceID:       "service-vm",
			ServiceAlias:    "service-vm",
			ExtendMethod:    "vm",
			ContainerCPU:    4000,
			ContainerMemory: 8192,
			ContainerGPU:    0,
		},
	}
	eventDao := &resourceSyncEventDao{}
	db.SetTestManager(resourceSyncTestManager{
		serviceDao: serviceDao,
		eventDao:   eventDao,
	})
	defer db.SetTestManager(nil)

	kubeClient := kubefake.NewSimpleClientset(
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "virt-launcher-service-vm",
				Namespace: "default",
				Labels: map[string]string{
					"service_id":  "service-vm",
					"kubevirt.io": "virt-launcher",
				},
			},
			Spec: corev1.PodSpec{
				NodeName: "node-a",
				NodeSelector: map[string]string{
					"kubevirt.io/schedulable":           "true",
					"cpu-vendor.node.kubevirt.io/Intel": "true",
				},
			},
			Status: corev1.PodStatus{Phase: corev1.PodRunning},
		},
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node-a",
				Labels: map[string]string{
					"kubevirt.io/schedulable":           "true",
					"cpu-vendor.node.kubevirt.io/Intel": "true",
				},
			},
		},
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node-b",
				Labels: map[string]string{
					"kubevirt.io/schedulable": "true",
				},
			},
		},
	)

	syncedServiceID := ""
	mockClient := kubecli.NewMockKubevirtClient(ctrl)
	action := &ServiceAction{
		MQClient:       &noopMQClient{},
		kubeClient:     kubeClient,
		kubevirtClient: mockClient,
		syncVirtualMachineSpecHook: func(serviceID string) error {
			syncedServiceID = serviceID
			return nil
		},
	}
	action.isVMLiveUpdateClusterConfiguredHook = func(ctx context.Context) bool { return true }
	action.getVirtualMachineByServiceIDHook = func(serviceID string) (*v1.VirtualMachine, error) {
		return &v1.VirtualMachine{
			ObjectMeta: metav1.ObjectMeta{Name: "demo-vm", Namespace: "default"},
			Status:     v1.VirtualMachineStatus{PrintableStatus: v1.VirtualMachineStatusRunning},
		}, nil
	}
	action.getVirtualMachineInstanceByServiceIDHook = func(serviceID string) (*v1.VirtualMachineInstance, error) {
		return &v1.VirtualMachineInstance{Status: v1.VirtualMachineInstanceStatus{
			Phase: v1.Running,
			Conditions: []v1.VirtualMachineInstanceCondition{
				{Type: v1.VirtualMachineInstanceIsMigratable, Status: "True"},
			},
		}}, nil
	}

	mockVMInterface := kubecli.NewMockVirtualMachineInterface(ctrl)
	mockClient.EXPECT().VirtualMachine("default").Return(mockVMInterface)
	mockVMInterface.EXPECT().Restart(gomock.Any(), "demo-vm", gomock.Any()).Return(nil)

	newMemory := 10240
	err := action.ServiceVertical(context.Background(), &workermodel.VerticalScalingTaskBody{
		ServiceID:       "service-vm",
		ContainerMemory: &newMemory,
	})
	if err != nil {
		t.Fatalf("expected missing migration target to restart instead of failing live update, got %v", err)
	}
	if syncedServiceID != "service-vm" {
		t.Fatalf("expected vm spec sync before restart, got %q", syncedServiceID)
	}
	if serviceDao.service == nil || serviceDao.service.ContainerMemory != 10240 {
		t.Fatalf("expected updated memory to persist, got %#v", serviceDao.service)
	}
	if len(eventDao.statuses) == 0 || eventDao.statuses[len(eventDao.statuses)-1] != dbmodel.EventStatusSuccess {
		t.Fatalf("expected success event status, got %#v", eventDao.statuses)
	}
}

// capability_id: rainbond.vm-live-update.capability-requires-migration-target
func TestGetVMLiveUpdateCapabilityRejectsWhenNoMigrationTargetNode(t *testing.T) {
	serviceDao := &resourceSyncTenantServiceDao{
		service: &dbmodel.TenantServices{
			ServiceID:       "service-vm",
			ServiceAlias:    "service-vm",
			ExtendMethod:    "vm",
			ContainerCPU:    6000,
			ContainerMemory: 12288,
		},
	}
	db.SetTestManager(resourceSyncTestManager{
		serviceDao: serviceDao,
		eventDao:   &resourceSyncEventDao{},
	})
	defer db.SetTestManager(nil)

	kubeClient := kubefake.NewSimpleClientset(
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "virt-launcher-service-vm",
				Namespace: "default",
				Labels: map[string]string{
					"service_id":  "service-vm",
					"kubevirt.io": "virt-launcher",
				},
			},
			Spec: corev1.PodSpec{
				NodeName: "node-a",
				NodeSelector: map[string]string{
					"kubevirt.io/schedulable":           "true",
					"cpu-vendor.node.kubevirt.io/Intel": "true",
				},
			},
			Status: corev1.PodStatus{Phase: corev1.PodRunning},
		},
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node-a",
				Labels: map[string]string{
					"kubevirt.io/schedulable":           "true",
					"cpu-vendor.node.kubevirt.io/Intel": "true",
				},
			},
		},
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node-b",
				Labels: map[string]string{
					"kubevirt.io/schedulable": "true",
				},
			},
		},
	)

	action := &ServiceAction{
		kubeClient: kubeClient,
	}
	action.loadVMRuntimeSpecExtensionSetHook = func(componentID string) (map[string]string, error) {
		return map[string]string{}, nil
	}
	action.loadVMRuntimeDeviceExtensionSetHook = func(componentID string) (map[string]string, error) {
		return map[string]string{}, nil
	}
	action.isVMLiveUpdateClusterConfiguredHook = func(ctx context.Context) bool { return true }
	action.getVirtualMachineByServiceIDHook = func(serviceID string) (*v1.VirtualMachine, error) {
		return &v1.VirtualMachine{
			ObjectMeta: metav1.ObjectMeta{Name: "demo-vm", Namespace: "default"},
			Status:     v1.VirtualMachineStatus{PrintableStatus: v1.VirtualMachineStatusRunning},
		}, nil
	}
	action.getVirtualMachineInstanceByServiceIDHook = func(serviceID string) (*v1.VirtualMachineInstance, error) {
		return &v1.VirtualMachineInstance{Status: v1.VirtualMachineInstanceStatus{
			Phase: v1.Running,
			Conditions: []v1.VirtualMachineInstanceCondition{
				{Type: v1.VirtualMachineInstanceIsMigratable, Status: "True"},
			},
		}}, nil
	}

	capability := action.GetVMLiveUpdateCapability("service-vm")
	if capability.CPUHotUpdateSupported || capability.MemoryHotUpdateSupported {
		t.Fatalf("expected live update capability to be blocked, got %#v", capability)
	}
	if !strings.Contains(capability.HotUpdateReason, "没有可用的迁移目标节点") {
		t.Fatalf("expected migration target reason, got %#v", capability)
	}
}

func quantityPtr(raw string) *resource.Quantity {
	q := resource.MustParse(raw)
	return &q
}
