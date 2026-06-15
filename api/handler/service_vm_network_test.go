package handler

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/worker/server/pb"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "kubevirt.io/api/core/v1"
	kubecli "kubevirt.io/client-go/kubecli"
)

func TestSetVMFixedPodIPEnablesCurrentRunningPodIPAndRestarts(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	serviceDao := &resourceSyncTenantServiceDao{
		service: &dbmodel.TenantServices{
			ServiceID:    "service-vm",
			ServiceAlias: "service-vm",
			TenantID:     "tenant-1",
			ExtendMethod: "vm",
		},
	}
	attrDao := &resourceSyncComponentK8sAttributeDao{}
	db.SetTestManager(resourceSyncTestManager{
		serviceDao:   serviceDao,
		attributeDao: attrDao,
	})
	defer db.SetTestManager(nil)

	mockClient := kubecli.NewMockKubevirtClient(ctrl)
	mockVMInterface := kubecli.NewMockVirtualMachineInterface(ctrl)

	syncedServiceID := ""
	action := &ServiceAction{
		kubevirtClient: mockClient,
		syncVirtualMachineSpecHook: func(serviceID string) error {
			syncedServiceID = serviceID
			return nil
		},
		getServicePodsHook: func(serviceID string) (*pb.ServiceAppPodList, error) {
			return &pb.ServiceAppPodList{
				NewPods: []*pb.ServiceAppPod{
					{PodName: "vm-launcher", PodIp: "10.42.247.130", PodStatus: "RUNNING"},
				},
			}, nil
		},
		getVirtualMachineByServiceIDHook: func(serviceID string) (*v1.VirtualMachine, error) {
			return &v1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{Name: "demo-vm", Namespace: "demo-ns"},
				Status:     v1.VirtualMachineStatus{PrintableStatus: v1.VirtualMachineStatusRunning},
			}, nil
		},
	}

	mockClient.EXPECT().VirtualMachine("demo-ns").Return(mockVMInterface)
	mockVMInterface.EXPECT().Restart(gomock.Any(), "demo-vm", gomock.Any()).Return(nil)

	result, err := action.SetVMFixedPodIP(context.Background(), "service-vm", true)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.FixedIP != "10.42.247.130" || !result.FixedIPEnabled || !result.Restarted {
		t.Fatalf("unexpected result: %#v", result)
	}
	if syncedServiceID != "service-vm" {
		t.Fatalf("expected vm spec sync before restart, got %q", syncedServiceID)
	}
	if attrDao.attributes["vm_fixed_ip_enabled"].AttributeValue != "true" {
		t.Fatalf("expected fixed ip enabled attr, got %#v", attrDao.attributes["vm_fixed_ip_enabled"])
	}
	if attrDao.attributes["vm_fixed_ip"].AttributeValue != "10.42.247.130" {
		t.Fatalf("expected fixed ip attr, got %#v", attrDao.attributes["vm_fixed_ip"])
	}
}

func TestSetVMFixedPodIPDisablesAndRestarts(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	serviceDao := &resourceSyncTenantServiceDao{
		service: &dbmodel.TenantServices{
			ServiceID:    "service-vm",
			ServiceAlias: "service-vm",
			TenantID:     "tenant-1",
			ExtendMethod: "vm",
		},
	}
	attrDao := &resourceSyncComponentK8sAttributeDao{
		attributes: map[string]*dbmodel.ComponentK8sAttributes{
			"vm_fixed_ip_enabled": {Name: "vm_fixed_ip_enabled", AttributeValue: "true"},
			"vm_fixed_ip":         {Name: "vm_fixed_ip", AttributeValue: "10.42.247.130"},
		},
	}
	db.SetTestManager(resourceSyncTestManager{
		serviceDao:   serviceDao,
		attributeDao: attrDao,
	})
	defer db.SetTestManager(nil)

	mockClient := kubecli.NewMockKubevirtClient(ctrl)
	mockVMInterface := kubecli.NewMockVirtualMachineInterface(ctrl)

	syncedServiceID := ""
	action := &ServiceAction{
		kubevirtClient: mockClient,
		syncVirtualMachineSpecHook: func(serviceID string) error {
			syncedServiceID = serviceID
			return nil
		},
		getVirtualMachineByServiceIDHook: func(serviceID string) (*v1.VirtualMachine, error) {
			return &v1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{Name: "demo-vm", Namespace: "demo-ns"},
				Status:     v1.VirtualMachineStatus{PrintableStatus: v1.VirtualMachineStatusRunning},
			}, nil
		},
	}

	mockClient.EXPECT().VirtualMachine("demo-ns").Return(mockVMInterface)
	mockVMInterface.EXPECT().Restart(gomock.Any(), "demo-vm", gomock.Any()).Return(nil)

	result, err := action.SetVMFixedPodIP(context.Background(), "service-vm", false)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.FixedIP != "" || result.FixedIPEnabled || !result.Restarted {
		t.Fatalf("unexpected result: %#v", result)
	}
	if syncedServiceID != "service-vm" {
		t.Fatalf("expected vm spec sync before restart, got %q", syncedServiceID)
	}
	if attrDao.attributes["vm_fixed_ip_enabled"].AttributeValue != "false" {
		t.Fatalf("expected fixed ip disabled attr, got %#v", attrDao.attributes["vm_fixed_ip_enabled"])
	}
	if _, ok := attrDao.attributes["vm_fixed_ip"]; ok {
		t.Fatalf("expected fixed ip attr to be deleted, got %#v", attrDao.attributes["vm_fixed_ip"])
	}
}
