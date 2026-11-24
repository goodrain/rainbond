package handler

import (
	"errors"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kv "kubevirt.io/api/core/v1"
)

// capability_id: rainbond.vm-runtime.stopped-import-status
func TestStoppedImportDoesNotRemainStartingWithoutVMI(t *testing.T) {
	for _, status := range []kv.VirtualMachinePrintableStatus{kv.VirtualMachineStatusProvisioning, kv.VirtualMachineStatusDataVolumeError, kv.VirtualMachineStatusStarting, kv.VirtualMachineStatusWaitingForVolumeBinding} {
		t.Run(string(status), func(t *testing.T) {
			halted := kv.RunStrategyHalted
			vm := &kv.VirtualMachine{ObjectMeta: metav1.ObjectMeta{Namespace: "team"}, Spec: kv.VirtualMachineSpec{RunStrategy: &halted, Template: &kv.VirtualMachineInstanceTemplateSpec{Spec: kv.VirtualMachineInstanceSpec{Volumes: []kv.Volume{{Name: "import-root", VolumeSource: kv.VolumeSource{DataVolume: &kv.DataVolumeSource{Name: "import-root"}}}}}}}, Status: kv.VirtualMachineStatus{PrintableStatus: status}}
			action := &ServiceAction{
				getVirtualMachineByServiceIDHook:         func(string) (*kv.VirtualMachine, error) { return vm, nil },
				getVirtualMachineInstanceByServiceIDHook: func(string) (*kv.VirtualMachineInstance, error) { return nil, nil },
				getDataVolumePhasesByNamesHook: func(string, []string) (map[string]string, error) {
					t.Fatal("a stopped guest must not take its power state from image imports")
					return nil, nil
				},
			}
			actual, ok, _ := action.resolveVMServiceRuntimeStatusWithGuest("test-vm")
			if !ok || actual != "closed" {
				t.Fatalf("explicitly stopped %s VM was reported as %q, observed=%v", status, actual, ok)
			}
		})
	}
}

// capability_id: rainbond.vm-runtime.stopped-import-status
func TestStoppedImportRequiresConfirmedPowerAndAbsentVMI(t *testing.T) {
	for _, mode := range []string{"running-requested", "vmi-exists", "vmi-read-failed", "vm-read-failed", "legacy-stopped"} {
		t.Run(mode, func(t *testing.T) {
			strategy := kv.RunStrategyHalted
			vm := &kv.VirtualMachine{Spec: kv.VirtualMachineSpec{RunStrategy: &strategy}, Status: kv.VirtualMachineStatus{PrintableStatus: kv.VirtualMachineStatusProvisioning}}
			var vmi *kv.VirtualMachineInstance
			var vmErr, vmiErr error
			switch mode {
			case "running-requested":
				strategy = kv.RunStrategyAlways
			case "vmi-exists":
				vmi = &kv.VirtualMachineInstance{Status: kv.VirtualMachineInstanceStatus{Phase: kv.Running}}
			case "vmi-read-failed":
				vmiErr = errors.New("read unavailable")
			case "vm-read-failed":
				vmErr = errors.New("read unavailable")
			case "legacy-stopped":
				stopped := false
				vm.Spec.RunStrategy = nil
				vm.Spec.Running = &stopped
			}
			action := &ServiceAction{
				getVirtualMachineByServiceIDHook:         func(string) (*kv.VirtualMachine, error) { return vm, vmErr },
				getVirtualMachineInstanceByServiceIDHook: func(string) (*kv.VirtualMachineInstance, error) { return vmi, vmiErr },
			}
			actual, _, _ := action.resolveVMServiceRuntimeStatusWithGuest("test-vm")
			if (actual == "closed") != (mode == "legacy-stopped") {
				t.Fatalf("%s reported as %q", mode, actual)
			}
		})
	}
}
