package handler

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "kubevirt.io/api/core/v1"
	"testing"
)

func guestVMI(phase v1.VirtualMachineInstancePhase, agent corev1.ConditionStatus) *v1.VirtualMachineInstance {
	vmi := &v1.VirtualMachineInstance{Status: v1.VirtualMachineInstanceStatus{Phase: phase}}
	if agent != "" {
		vmi.Status.Conditions = append(vmi.Status.Conditions, v1.VirtualMachineInstanceCondition{Type: v1.VirtualMachineInstanceAgentConnected, Status: agent})
	}
	vmi.Status.GuestOSInfo = v1.VirtualMachineInstanceGuestOSInfo{Name: "Ubuntu", Version: "24.04", KernelRelease: "6.8"}
	return vmi
}

// capability_id: rainbond.vm-guest.status-observation
func TestResolveVMGuestStatus(t *testing.T) {
	paused := guestVMI(v1.Running, corev1.ConditionTrue)
	paused.Status.Conditions = append(paused.Status.Conditions, v1.VirtualMachineInstanceCondition{Type: v1.VirtualMachineInstancePaused, Status: corev1.ConditionTrue})
	stopped := &v1.VirtualMachine{Status: v1.VirtualMachineStatus{PrintableStatus: v1.VirtualMachineStatusStopped}}
	cases := []struct {
		name           string
		vm             *v1.VirtualMachine
		vmi            *v1.VirtualMachineInstance
		failed         bool
		status, reason string
		connected      *bool
		os             bool
	}{
		{"agent connected", nil, guestVMI(v1.Running, corev1.ConditionTrue), false, "responding", "agent_connected", guestBool(true), true},
		{"agent disconnected", nil, guestVMI(v1.Running, corev1.ConditionFalse), false, "unconfirmed", "agent_disconnected", guestBool(false), false},
		{"agent absent", nil, guestVMI(v1.Running, ""), false, "unconfirmed", "agent_not_reported", nil, false},
		{"agent unknown", nil, guestVMI(v1.Running, corev1.ConditionUnknown), false, "unconfirmed", "agent_not_reported", nil, false},
		{"paused ignores old agent", nil, paused, false, "paused", "vm_paused", nil, false},
		{"starting ignores old agent", nil, guestVMI(v1.Pending, corev1.ConditionTrue), false, "starting", "vm_starting", nil, false},
		{"stopped", stopped, nil, false, "powered_off", "vm_stopped", nil, false},
		{"not created", nil, nil, false, "unavailable", "instance_not_created", nil, false},
		{"read failed", stopped, guestVMI(v1.Running, corev1.ConditionTrue), true, "unavailable", "read_failed", nil, false},
		{"failed instance", nil, guestVMI(v1.Failed, corev1.ConditionTrue), false, "unavailable", "instance_not_running", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveVMGuestStatus(tc.vm, tc.vmi, tc.failed)
			if got.Status != tc.status || got.Reason != tc.reason {
				t.Fatalf("got %s/%s, want %s/%s", got.Status, got.Reason, tc.status, tc.reason)
			}
			if (got.AgentConnected == nil) != (tc.connected == nil) || got.AgentConnected != nil && *got.AgentConnected != *tc.connected {
				t.Fatalf("incorrect nullable agent connection")
			}
			if (got.OS != nil) != tc.os {
				t.Fatalf("stale or missing guest OS information")
			}
		})
	}
}

func guestBool(value bool) *bool { return &value }

// capability_id: rainbond.vm-guest.status-observation
func TestVMRuntimeGuestReadFailureDoesNotBecomeDisconnected(t *testing.T) {
	action := &ServiceAction{}
	_, _, guest := action.resolveVMServiceRuntimeStatusWithGuest("service-test")
	if guest.Status != "unavailable" || guest.AgentConnected != nil || guest.Reason != "read_failed" {
		t.Fatalf("missing client must report unknown observation")
	}
}

// capability_id: rainbond.vm-guest.status-observation
func TestVMRuntimeIncludesObservedGuest(t *testing.T) {
	action := &ServiceAction{
		getVirtualMachineByServiceIDHook: func(string) (*v1.VirtualMachine, error) {
			return &v1.VirtualMachine{ObjectMeta: metav1.ObjectMeta{Name: "vm", Namespace: "team"}}, nil
		},
		getVirtualMachineInstanceByServiceIDHook: func(string) (*v1.VirtualMachineInstance, error) {
			return guestVMI(v1.Running, corev1.ConditionTrue), nil
		},
	}
	_, _, guest := action.resolveVMServiceRuntimeStatusWithGuest("service-test")
	if guest.Status != "responding" || guest.OS.Name != "Ubuntu" {
		t.Fatal("actual guest observation missing")
	}
}
