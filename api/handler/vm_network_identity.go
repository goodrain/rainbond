package handler

import (
	"context"
	"errors"
	"net"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	v1 "kubevirt.io/api/core/v1"
)

func vmNeedsInterfaceMAC(vm *v1.VirtualMachine) bool {
	if vm == nil || vm.Spec.Template == nil {
		return false
	}
	for _, iface := range vm.Spec.Template.Spec.Domain.Devices.Interfaces {
		if iface.Name != "" && iface.MacAddress == "" {
			return true
		}
	}
	return false
}

func applyObservedVMInterfaceMACs(vm *v1.VirtualMachine, instance *v1.VirtualMachineInstance) (*v1.VirtualMachine, bool, error) {
	if !vmNeedsInterfaceMAC(vm) || instance == nil {
		return vm, false, nil
	}
	if vm.UID == "" || instance.UID == "" || instance.Name != vm.Name || instance.Namespace != vm.Namespace || !metav1.IsControlledBy(instance, vm) {
		return nil, false, errors.New("VM interface observation belongs to a different instance owner")
	}
	wanted := make(map[string]bool)
	for _, iface := range vm.Spec.Template.Spec.Domain.Devices.Interfaces {
		if iface.MacAddress == "" && iface.Name != "" {
			wanted[iface.Name] = true
		}
	}
	observed := make(map[string]string)
	for _, iface := range instance.Status.Interfaces {
		if !wanted[iface.Name] || iface.MAC == "" {
			continue
		}
		address, err := net.ParseMAC(iface.MAC)
		if err != nil || len(address) != 6 || address[0]&1 != 0 || address.String() == "00:00:00:00:00:00" {
			return nil, false, errors.New("invalid VM interface MAC observation")
		}
		canonical := address.String()
		if previous := observed[iface.Name]; previous != "" && previous != canonical {
			return nil, false, errors.New("ambiguous VM interface MAC observation")
		}
		observed[iface.Name] = canonical
	}
	updated := vm.DeepCopy()
	changed := false
	for i := range updated.Spec.Template.Spec.Domain.Devices.Interfaces {
		iface := &updated.Spec.Template.Spec.Domain.Devices.Interfaces[i]
		if iface.MacAddress == "" && observed[iface.Name] != "" {
			iface.MacAddress = observed[iface.Name]
			changed = true
		}
	}
	return updated, changed, nil
}

// pinRunningVMNetworkIdentity saves a legacy VM's current NIC before its VMI
// disappears. It does not infer an old guest MAC when no instance exists.
func (s *ServiceAction) pinRunningVMNetworkIdentity(vm *v1.VirtualMachine) (*v1.VirtualMachine, error) {
	if s == nil || s.kubevirtClient == nil || !vmNeedsInterfaceMAC(vm) {
		return vm, nil
	}
	result := vm
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current, err := s.kubevirtClient.VirtualMachine(vm.Namespace).Get(context.Background(), vm.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if current.UID != vm.UID {
			return errors.New("VM identity changed before preserving its network interface")
		}
		result = current
		if !vmNeedsInterfaceMAC(current) {
			return nil
		}
		instance, err := s.kubevirtClient.VirtualMachineInstance(vm.Namespace).Get(context.Background(), vm.Name, metav1.GetOptions{})
		if k8serrors.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}
		updated, changed, err := applyObservedVMInterfaceMACs(current, instance)
		if err != nil || !changed {
			return err
		}
		result, err = s.kubevirtClient.VirtualMachine(vm.Namespace).Update(context.Background(), updated, metav1.UpdateOptions{})
		return err
	})
	if err != nil {
		// Admission errors may include the VM's cloud-init contents.
		return nil, errors.New("cannot preserve virtual machine network identity")
	}
	return result, nil
}
