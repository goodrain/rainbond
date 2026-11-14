package handler

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	dbdao "github.com/goodrain/rainbond/db/dao"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/pkg/vmrestore"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubevirtv1 "kubevirt.io/api/core/v1"
	kubecli "kubevirt.io/client-go/kubecli"
)

type restoreAttributeRetryServiceDAO struct {
	dbdao.TenantServiceDao
	service *dbmodel.TenantServices
	err     error
	calls   int
}

func (d *restoreAttributeRetryServiceDAO) GetServiceByID(string) (*dbmodel.TenantServices, error) {
	d.calls++
	return d.service, d.err
}

type restoreAttributeRetryManager struct {
	restoreAttributePathManager
	services *restoreAttributeRetryServiceDAO
}

func (m restoreAttributeRetryManager) TenantServiceDao() dbdao.TenantServiceDao {
	return m.services
}

// capability_id: rainbond.vm-restore.public-attribute-delete-retry
func TestDeleteK8sAttributeRetriesRuntimeSyncAfterCommittedDelete(t *testing.T) {
	for _, name := range []string{"vm_gpu_enabled", "vm_usb_enabled"} {
		t.Run(name, func(t *testing.T) {
			_, database := restoreAttributePathFixture(t)
			row := insertRestoreAttribute(t, database, name, "component")
			services := &restoreAttributeRetryServiceDAO{service: &dbmodel.TenantServices{ServiceID: "component", ExtendMethod: string(dbmodel.ServiceTypeVM)}}
			manager := restoreAttributeRetryManager{restoreAttributePathManager: restoreAttributePathManager{database: database}, services: services}
			vm := &kubevirtv1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{Name: "vm", Namespace: "team"},
				Spec:       kubevirtv1.VirtualMachineSpec{Template: &kubevirtv1.VirtualMachineInstanceTemplateSpec{}},
			}
			if name == "vm_gpu_enabled" {
				vm.Spec.Template.Spec.Domain.Devices.GPUs = []kubevirtv1.GPU{{Name: "gpu-0", DeviceName: "gpu.example.com/test"}}
			} else {
				vm.Spec.Template.Spec.Domain.Devices.HostDevices = []kubevirtv1.HostDevice{{Name: "usb-0", DeviceName: "usb.example.com/test"}}
			}
			ctrl := gomock.NewController(t)
			client := kubecli.NewMockKubevirtClient(ctrl)
			vms := kubecli.NewMockVirtualMachineInterface(ctrl)
			attachRawRuntimeObservation(t, client, vm)
			client.EXPECT().VirtualMachine("team").Return(vms).AnyTimes()
			updateCalls := 0
			syncErr := errors.New("attribute_runtime_sync_fixed_code")
			vms.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, updated *kubevirtv1.VirtualMachine, _ metav1.UpdateOptions) (*kubevirtv1.VirtualMachine, error) {
				updateCalls++
				if updateCalls == 1 {
					return nil, syncErr
				}
				vm = updated.DeepCopy()
				return vm.DeepCopy(), nil
			}).AnyTimes()
			action := &ServiceAction{dbmanager: manager, kubevirtClient: client, getVirtualMachineByServiceIDHook: func(string) (*kubevirtv1.VirtualMachine, error) { return vm.DeepCopy(), nil }}
			if err := action.DeleteK8sAttribute("component", row.Name); !errors.Is(err, syncErr) || updateCalls != 1 {
				t.Fatal("first attribute deletion did not report the runtime synchronization failure")
			}
			var count int
			if err := database.Model(&dbmodel.ComponentK8sAttributes{}).Where("ID = ?", row.ID).Count(&count).Error; err != nil || count != 0 {
				t.Fatal("first attribute deletion did not commit its database change")
			}
			devices := vm.Spec.Template.Spec.Domain.Devices
			if len(devices.GPUs)+len(devices.HostDevices) != 1 {
				t.Fatal("failed runtime synchronization unexpectedly changed the VM")
			}
			if err := action.DeleteK8sAttribute("component", row.Name); err != nil || updateCalls != 2 || services.calls != 2 {
				t.Fatal("retry did not finish runtime synchronization after the attribute row was removed")
			}
			devices = vm.Spec.Template.Spec.Domain.Devices
			if len(devices.GPUs) != 0 || len(devices.HostDevices) != 0 {
				t.Fatal("successful deletion retry retained the old VM accelerator")
			}
		})
	}
}

func TestDeleteMissingK8sAttributeRetriesSpecSync(t *testing.T) {
	_, database := restoreAttributePathFixture(t)
	syncErr := errors.New("attribute_spec_sync_fixed_code")
	services := &restoreAttributeRetryServiceDAO{err: syncErr}
	action := &ServiceAction{dbmanager: restoreAttributeRetryManager{restoreAttributePathManager: restoreAttributePathManager{database: database}, services: services}}
	if err := action.DeleteK8sAttribute("component", "vm_disk_layout"); !errors.Is(err, syncErr) || services.calls != 1 {
		t.Fatal("missing public spec attribute suppressed runtime synchronization failure")
	}
	services.err = nil
	if err := action.DeleteK8sAttribute("component", "vm_disk_layout"); err != nil || services.calls != 2 {
		t.Fatal("missing public spec attribute did not retry runtime synchronization")
	}
}

func TestDeleteMissingPrivateOrOrdinaryK8sAttributeDoesNotRunRuntimeSync(t *testing.T) {
	_, database := restoreAttributePathFixture(t)
	services := &restoreAttributeRetryServiceDAO{err: errors.New("unexpected_runtime_sync_fixed_code")}
	action := &ServiceAction{dbmanager: restoreAttributeRetryManager{restoreAttributePathManager: restoreAttributePathManager{database: database}, services: services}}
	for _, name := range []string{vmrestore.StateKey, " VM_RESTORE_STATE ", vmrestore.ReceiptPrefix + "operation-a", vmrestore.LegacyBindingsKey} {
		if err := action.DeleteK8sAttribute("component", name); !errors.Is(err, vmrestore.ErrInternalAttribute) {
			t.Fatal("missing private attribute deletion was permitted")
		}
	}
	if err := action.DeleteK8sAttribute("component", "labels"); err != nil {
		t.Fatal("missing ordinary attribute deletion stopped being idempotent")
	}
	if services.calls != 0 {
		t.Fatal("missing private or ordinary attribute unexpectedly triggered VM synchronization")
	}
}
