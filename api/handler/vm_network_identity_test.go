package handler

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	apimodel "github.com/goodrain/rainbond/api/model"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	v1 "kubevirt.io/api/core/v1"
	"kubevirt.io/client-go/kubecli"
)

func networkIdentityVM(mac string, cores uint32) *v1.VirtualMachine {
	return &v1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{Name: "network-vm", Namespace: "network-team", UID: types.UID("network-vm-uid"), ResourceVersion: "7"},
		Spec: v1.VirtualMachineSpec{Template: &v1.VirtualMachineInstanceTemplateSpec{
			Spec: v1.VirtualMachineInstanceSpec{Domain: v1.DomainSpec{
				CPU: &v1.CPU{Cores: cores},
				Devices: v1.Devices{Interfaces: []v1.Interface{{Name: "default", MacAddress: mac, Model: "virtio",
					InterfaceBindingMethod: v1.InterfaceBindingMethod{Bridge: &v1.InterfaceBridge{}}}}},
			}},
		}},
	}
}

// capability_id: rainbond.vm-network.stable-interface-identity
func TestApplyVirtualMachineSpecPreservesInterfaceMAC(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		existingMAC, nextMAC string
		cores                uint32
		wantMAC              string
		changed              bool
	}{
		{"omitted MAC remains stable", "02:11:22:33:44:55", "", 1, "02:11:22:33:44:55", false},
		{"new default does not rotate an existing NIC", "02:11:22:33:44:55", "02:aa:bb:cc:dd:ee", 1, "02:11:22:33:44:55", false},
		{"CPU update keeps the same NIC", "02:11:22:33:44:55", "02:aa:bb:cc:dd:ee", 4, "02:11:22:33:44:55", true},
		{"previously unset MAC accepts a fixed identity", "", "02:aa:bb:cc:dd:ee", 1, "02:aa:bb:cc:dd:ee", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := networkIdentityVM(tc.existingMAC, 1)
			desired := networkIdentityVM(tc.nextMAC, tc.cores)
			beforeCopy, desiredCopy := before.DeepCopy(), desired.DeepCopy()
			got, changed := applyVirtualMachineSpec(before, desired)
			if changed != tc.changed || got.Spec.Template.Spec.Domain.Devices.Interfaces[0].MacAddress != tc.wantMAC {
				t.Fatal("routine spec synchronization changed the guest network identity")
			}
			if got.Spec.Template.Spec.Domain.CPU.Cores != tc.cores {
				t.Fatal("preserving the MAC discarded the requested CPU update")
			}
			if !reflect.DeepEqual(before, beforeCopy) || !reflect.DeepEqual(desired, desiredCopy) {
				t.Fatal("spec synchronization mutated an input object")
			}
		})
	}
}

// capability_id: rainbond.vm-network.stable-interface-identity
func TestApplyVirtualMachineSpecDoesNotBorrowAnotherInterfaceMAC(t *testing.T) {
	before := networkIdentityVM("02:11:22:33:44:55", 1)
	desired := networkIdentityVM("02:aa:bb:cc:dd:ee", 1)
	desired.Spec.Template.Spec.Domain.Devices.Interfaces[0].Name = "other-network"
	got, changed := applyVirtualMachineSpec(before, desired)
	if !changed || got.Spec.Template.Spec.Domain.Devices.Interfaces[0].MacAddress != "02:aa:bb:cc:dd:ee" {
		t.Fatal("an unrelated interface inherited the old NIC identity")
	}
}

func networkIdentityVMI(vm *v1.VirtualMachine, mac string) *v1.VirtualMachineInstance {
	controller := true
	return &v1.VirtualMachineInstance{
		ObjectMeta: metav1.ObjectMeta{Name: vm.Name, Namespace: vm.Namespace, UID: types.UID("network-instance-uid"),
			OwnerReferences: []metav1.OwnerReference{{APIVersion: "kubevirt.io/v1", Kind: "VirtualMachine", Name: vm.Name, UID: vm.UID, Controller: &controller}}},
		Status: v1.VirtualMachineInstanceStatus{Interfaces: []v1.VirtualMachineInstanceNetworkInterface{{Name: "default", MAC: mac}}},
	}
}

// capability_id: rainbond.vm-network.stable-interface-identity
func TestObservedVMInterfaceMACCompatibility(t *testing.T) {
	for _, tc := range []struct {
		name       string
		currentMAC string
		observed   string
		mutate     func(*v1.VirtualMachineInstance)
		wantMAC    string
		changed    bool
		wantError  bool
	}{
		{name: "pin legacy running NIC", observed: "02:AA:BB:CC:DD:EE", wantMAC: "02:aa:bb:cc:dd:ee", changed: true},
		{name: "keep configured NIC", currentMAC: "02:11:22:33:44:55", observed: "02:aa:bb:cc:dd:ee", wantMAC: "02:11:22:33:44:55"},
		{name: "not yet observed", observed: ""},
		{name: "invalid observation", observed: "invalid", wantError: true},
		{name: "multicast observation", observed: "01:11:22:33:44:55", wantError: true},
		{name: "EUI64 observation", observed: "02:11:22:33:44:55:66:77", wantError: true},
		{name: "wrong VM owner", observed: "02:aa:bb:cc:dd:ee", mutate: func(i *v1.VirtualMachineInstance) { i.OwnerReferences[0].UID = "another-vm" }, wantError: true},
		{name: "different namespace", observed: "02:aa:bb:cc:dd:ee", mutate: func(i *v1.VirtualMachineInstance) { i.Namespace = "another-team" }, wantError: true},
		{name: "different name", observed: "02:aa:bb:cc:dd:ee", mutate: func(i *v1.VirtualMachineInstance) { i.Name = "another-vm" }, wantError: true},
		{name: "unrelated interface", observed: "02:aa:bb:cc:dd:ee", mutate: func(i *v1.VirtualMachineInstance) { i.Status.Interfaces[0].Name = "other" }},
		{name: "ambiguous interface", observed: "02:aa:bb:cc:dd:ee", mutate: func(i *v1.VirtualMachineInstance) {
			i.Status.Interfaces = append(i.Status.Interfaces, v1.VirtualMachineInstanceNetworkInterface{Name: "default", MAC: "02:11:22:33:44:55"})
		}, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			vm := networkIdentityVM(tc.currentMAC, 1)
			instance := networkIdentityVMI(vm, tc.observed)
			if tc.mutate != nil {
				tc.mutate(instance)
			}
			vmBefore, instanceBefore := vm.DeepCopy(), instance.DeepCopy()
			got, changed, err := applyObservedVMInterfaceMACs(vm, instance)
			if (err != nil) != tc.wantError {
				t.Fatal("unexpected network identity observation result")
			}
			if !reflect.DeepEqual(vm, vmBefore) || !reflect.DeepEqual(instance, instanceBefore) {
				t.Fatal("observing the live NIC mutated its source objects")
			}
			if err != nil {
				return
			}
			if changed != tc.changed || got.Spec.Template.Spec.Domain.Devices.Interfaces[0].MacAddress != tc.wantMAC {
				t.Fatal("the stored network identity does not match the owned live NIC")
			}
		})
	}
}

// capability_id: rainbond.vm-network.stable-interface-identity
func TestVMNetworkIdentityPinnedBeforePowerOperation(t *testing.T) {
	for _, operation := range []string{"stop", "restart"} {
		t.Run(operation, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			client := kubecli.NewMockKubevirtClient(ctrl)
			machines := kubecli.NewMockVirtualMachineInterface(ctrl)
			instances := kubecli.NewMockVirtualMachineInstanceInterface(ctrl)
			vm := networkIdentityVM("", 1)
			vm.Status.PrintableStatus = v1.VirtualMachineStatusRunning
			client.EXPECT().VirtualMachine(vm.Namespace).Return(machines).AnyTimes()
			client.EXPECT().VirtualMachineInstance(vm.Namespace).Return(instances)
			machines.EXPECT().Get(gomock.Any(), vm.Name, gomock.Any()).Return(vm.DeepCopy(), nil)
			instances.EXPECT().Get(gomock.Any(), vm.Name, gomock.Any()).Return(networkIdentityVMI(vm, "02:11:22:33:44:55"), nil)
			pinned := false
			machines.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
				func(_ context.Context, updated *v1.VirtualMachine, _ metav1.UpdateOptions) (*v1.VirtualMachine, error) {
					pinned = updated.Spec.Template.Spec.Domain.Devices.Interfaces[0].MacAddress == "02:11:22:33:44:55"
					return updated, nil
				})
			if operation == "stop" {
				machines.EXPECT().Stop(gomock.Any(), vm.Name, gomock.Any()).DoAndReturn(func(context.Context, string, *v1.StopOptions) error {
					if !pinned {
						t.Fatal("stop discarded the last observable NIC before saving its identity")
					}
					return nil
				})
			} else {
				machines.EXPECT().Restart(gomock.Any(), vm.Name, gomock.Any()).DoAndReturn(func(context.Context, string, *v1.RestartOptions) error {
					if !pinned {
						t.Fatal("restart discarded the last observable NIC before saving its identity")
					}
					return nil
				})
			}
			action := &ServiceAction{kubevirtClient: client, getVirtualMachineByServiceIDHook: func(string) (*v1.VirtualMachine, error) { return vm, nil }}
			var err error
			if operation == "stop" {
				err = action.StopVM(context.Background(), "network-service")
			} else {
				err = action.RestartVM(context.Background(), &apimodel.StartStopStruct{ServiceID: "network-service"}, "version")
			}
			if err != nil {
				t.Fatal("the normal VM power operation failed")
			}
		})
	}
}

// capability_id: rainbond.vm-network.stable-interface-identity
func TestPinVMNetworkIdentityFailureAndRetry(t *testing.T) {
	for _, scenario := range []string{"VM read failure", "VM replaced", "instance absent", "instance read failure", "instance owner changed", "observation pending", "sensitive update error", "conflict keeps concurrent MAC"} {
		t.Run(scenario, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			client := kubecli.NewMockKubevirtClient(ctrl)
			machines := kubecli.NewMockVirtualMachineInterface(ctrl)
			instances := kubecli.NewMockVirtualMachineInstanceInterface(ctrl)
			vm := networkIdentityVM("", 1)
			client.EXPECT().VirtualMachine(vm.Namespace).Return(machines).AnyTimes()
			wantError := false
			switch scenario {
			case "VM read failure":
				machines.EXPECT().Get(gomock.Any(), vm.Name, gomock.Any()).Return(nil, errors.New("read failed"))
				wantError = true
			case "VM replaced":
				replacement := vm.DeepCopy()
				replacement.UID = "replacement-vm-uid"
				machines.EXPECT().Get(gomock.Any(), vm.Name, gomock.Any()).Return(replacement, nil)
				wantError = true
			default:
				machines.EXPECT().Get(gomock.Any(), vm.Name, gomock.Any()).Return(vm.DeepCopy(), nil)
				client.EXPECT().VirtualMachineInstance(vm.Namespace).Return(instances)
				instance := networkIdentityVMI(vm, "02:11:22:33:44:55")
				switch scenario {
				case "instance absent":
					instances.EXPECT().Get(gomock.Any(), vm.Name, gomock.Any()).Return(nil, k8serrors.NewNotFound(schema.GroupResource{Resource: "virtualmachineinstances"}, vm.Name))
				case "instance read failure":
					instances.EXPECT().Get(gomock.Any(), vm.Name, gomock.Any()).Return(nil, errors.New("read failed"))
					wantError = true
				case "instance owner changed":
					instance.OwnerReferences[0].UID = "another-vm-uid"
					instances.EXPECT().Get(gomock.Any(), vm.Name, gomock.Any()).Return(instance, nil)
					wantError = true
				case "observation pending":
					instance.Status.Interfaces = nil
					instances.EXPECT().Get(gomock.Any(), vm.Name, gomock.Any()).Return(instance, nil)
				case "sensitive update error":
					instances.EXPECT().Get(gomock.Any(), vm.Name, gomock.Any()).Return(instance, nil)
					machines.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("opaque-login-verifier"))
					wantError = true
				case "conflict keeps concurrent MAC":
					instances.EXPECT().Get(gomock.Any(), vm.Name, gomock.Any()).Return(instance, nil)
					machines.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, k8serrors.NewConflict(schema.GroupResource{Resource: "virtualmachines"}, vm.Name, errors.New("changed")))
					concurrent := networkIdentityVM("02:55:66:77:88:99", 8)
					concurrent.ResourceVersion = "8"
					machines.EXPECT().Get(gomock.Any(), vm.Name, gomock.Any()).Return(concurrent, nil)
				}
			}
			action := &ServiceAction{kubevirtClient: client}
			got, err := action.pinRunningVMNetworkIdentity(vm)
			if (err != nil) != wantError {
				t.Fatal("unexpected identity preservation result")
			}
			if err != nil && strings.Contains(err.Error(), "opaque-login-verifier") {
				t.Fatal("an admission failure exposed guest initialization material")
			}
			if scenario == "conflict keeps concurrent MAC" && (got.Spec.Template.Spec.Domain.CPU.Cores != 8 || got.Spec.Template.Spec.Domain.Devices.Interfaces[0].MacAddress != "02:55:66:77:88:99") {
				t.Fatal("retry overwrote a concurrent hardware or NIC identity update")
			}
		})
	}
}
