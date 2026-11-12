package handler

import (
	apimodel "github.com/goodrain/rainbond/api/model"
	corev1 "k8s.io/api/core/v1"
	v1 "kubevirt.io/api/core/v1"
)

// resolveVMGuestStatus observes the agent channel, not application health.
// GuestOSInfo can outlive an agent connection and must not establish readiness.
func resolveVMGuestStatus(vm *v1.VirtualMachine, vmi *v1.VirtualMachineInstance, readFailed bool) *apimodel.VMGuestStatus {
	result := &apimodel.VMGuestStatus{Status: "unavailable", Reason: "read_failed"}
	if readFailed {
		return result
	}
	if vmi == nil {
		if vm != nil && vm.Status.PrintableStatus == v1.VirtualMachineStatusStopped {
			result.Status, result.Reason = "powered_off", "vm_stopped"
		} else {
			result.Reason = "instance_not_created"
		}
		return result
	}
	if isConditionTrue(vmi.Status.Conditions, v1.VirtualMachineInstancePaused) {
		result.Status, result.Reason = "paused", "vm_paused"
		return result
	}
	switch vmi.Status.Phase {
	case v1.Pending, v1.Scheduling, v1.Scheduled, v1.VmPhaseUnset:
		result.Status, result.Reason = "starting", "vm_starting"
		return result
	case v1.Running:
	default:
		result.Reason = "instance_not_running"
		return result
	}
	result.Status, result.Reason = "unconfirmed", "agent_not_reported"
	for _, condition := range vmi.Status.Conditions {
		if condition.Type != v1.VirtualMachineInstanceAgentConnected {
			continue
		}
		switch condition.Status {
		case corev1.ConditionTrue:
			connected := true
			result.AgentConnected = &connected
			result.Status, result.Reason = "responding", "agent_connected"
			info := vmi.Status.GuestOSInfo
			result.OS = &apimodel.VMGuestOSInfo{Name: info.Name, Version: info.Version, KernelRelease: info.KernelRelease}
		case corev1.ConditionFalse:
			connected := false
			result.AgentConnected = &connected
			result.Reason = "agent_disconnected"
		}
		break
	}
	return result
}
