package handler

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubevirtv1 "kubevirt.io/api/core/v1"
	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

// capability_id: rainbond.vm-template-import.status-restoring
func TestResolveVMTransitionStatusReturnsStartingForDataVolumeImportWithoutRestoreContext(t *testing.T) {
	vm := &kubevirtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo-vm",
			Namespace: "demo-ns",
		},
		Status: kubevirtv1.VirtualMachineStatus{
			PrintableStatus: kubevirtv1.VirtualMachineStatusStopped,
		},
	}
	vmi := &kubevirtv1.VirtualMachineInstance{
		Status: kubevirtv1.VirtualMachineInstanceStatus{
			Phase: kubevirtv1.Pending,
			Conditions: []kubevirtv1.VirtualMachineInstanceCondition{
				{
					Type:   kubevirtv1.VirtualMachineInstanceProvisioning,
					Status: "True",
				},
				{
					Type:   kubevirtv1.VirtualMachineInstanceDataVolumesReady,
					Status: "False",
				},
			},
		},
	}

	got, ok := resolveVMTransitionStatus(vm, vmi)
	if !ok {
		t.Fatal("expected importing data volumes to override vm closed status")
	}
	if got != "starting" {
		t.Fatalf("expected importing data volumes without restore context to map to %q, got %q", "starting", got)
	}
}

// capability_id: rainbond.vm-template-import.status-restoring
func TestResolveVMServiceRuntimeStatusReturnsRestoringWhenArtifactDataVolumeImportsBeforeVMIExists(t *testing.T) {
	action := &ServiceAction{
		getVirtualMachineByServiceIDHook: func(serviceID string) (*kubevirtv1.VirtualMachine, error) {
			return &kubevirtv1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "demo-vm",
					Namespace: "demo-ns",
				},
				Spec: kubevirtv1.VirtualMachineSpec{
					DataVolumeTemplates: []kubevirtv1.DataVolumeTemplateSpec{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "manual133",
								Annotations: map[string]string{
									"rainbond.com/vm-artifact-image":   "goodrain.me/team/windows-root:v1",
									"rainbond.com/vm-artifact-service": "vm-artifact-manual133",
								},
							},
						},
					},
				},
				Status: kubevirtv1.VirtualMachineStatus{
					PrintableStatus: kubevirtv1.VirtualMachineStatusStopped,
				},
			}, nil
		},
		getVirtualMachineInstanceByServiceIDHook: func(serviceID string) (*kubevirtv1.VirtualMachineInstance, error) {
			return nil, nil
		},
		getDataVolumePhasesByNamesHook: func(namespace string, names []string) (map[string]string, error) {
			if namespace != "demo-ns" {
				t.Fatalf("expected namespace %q, got %q", "demo-ns", namespace)
			}
			if len(names) != 1 || names[0] != "manual133" {
				t.Fatalf("expected data volume names [manual133], got %#v", names)
			}
			return map[string]string{
				"manual133": "ImportInProgress",
			}, nil
		},
	}

	got, ok := action.resolveVMServiceRuntimeStatus("service-a")
	if !ok {
		t.Fatal("expected importing data volume to override vm status before vmi exists")
	}
	if got != "restoring" {
		t.Fatalf("expected importing data volume to map to %q, got %q", "restoring", got)
	}
}

// capability_id: rainbond.vm-template-import.status-restoring
func TestResolveVMServiceRuntimeStatusReturnsStartingForInitialBlankDataVolume(t *testing.T) {
	action := &ServiceAction{
		getVirtualMachineByServiceIDHook: func(serviceID string) (*kubevirtv1.VirtualMachine, error) {
			return &kubevirtv1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "demo-vm",
					Namespace: "demo-ns",
				},
				Spec: kubevirtv1.VirtualMachineSpec{
					DataVolumeTemplates: []kubevirtv1.DataVolumeTemplateSpec{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "manual133",
							},
							Spec: cdiv1.DataVolumeSpec{
								Source: &cdiv1.DataVolumeSource{
									Blank: &cdiv1.DataVolumeBlankImage{},
								},
							},
						},
					},
				},
				Status: kubevirtv1.VirtualMachineStatus{
					PrintableStatus: kubevirtv1.VirtualMachineStatusProvisioning,
				},
			}, nil
		},
		getVirtualMachineInstanceByServiceIDHook: func(serviceID string) (*kubevirtv1.VirtualMachineInstance, error) {
			return nil, nil
		},
		getDataVolumePhasesByNamesHook: func(namespace string, names []string) (map[string]string, error) {
			t.Fatalf("blank data volumes should not be queried as restore volumes")
			return nil, nil
		},
	}

	got, ok := action.resolveVMServiceRuntimeStatus("service-a")
	if !ok {
		t.Fatal("expected provisioning vm to map to starting")
	}
	if got != "starting" {
		t.Fatalf("expected initial blank data volume to map to %q, got %q", "starting", got)
	}
}

// capability_id: rainbond.vm-template-import.status-restoring
func TestResolveVMServiceRuntimeStatusReturnsStartingForInitialHTTPDataVolume(t *testing.T) {
	action := &ServiceAction{
		getVirtualMachineByServiceIDHook: func(serviceID string) (*kubevirtv1.VirtualMachine, error) {
			return &kubevirtv1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "demo-vm",
					Namespace: "demo-ns",
				},
				Spec: kubevirtv1.VirtualMachineSpec{
					DataVolumeTemplates: []kubevirtv1.DataVolumeTemplateSpec{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "manual133",
							},
							Spec: cdiv1.DataVolumeSpec{
								Source: &cdiv1.DataVolumeSource{
									HTTP: &cdiv1.DataVolumeSourceHTTP{URL: "https://download/root.qcow2"},
								},
							},
						},
					},
				},
				Status: kubevirtv1.VirtualMachineStatus{
					PrintableStatus: kubevirtv1.VirtualMachineStatusProvisioning,
				},
			}, nil
		},
		getVirtualMachineInstanceByServiceIDHook: func(serviceID string) (*kubevirtv1.VirtualMachineInstance, error) {
			return nil, nil
		},
		getDataVolumePhasesByNamesHook: func(namespace string, names []string) (map[string]string, error) {
			t.Fatalf("initial http data volumes should not be queried as restore volumes")
			return nil, nil
		},
	}

	got, ok := action.resolveVMServiceRuntimeStatus("service-a")
	if !ok {
		t.Fatal("expected provisioning vm to map to starting")
	}
	if got != "starting" {
		t.Fatalf("expected initial http data volume to map to %q, got %q", "starting", got)
	}
}

// capability_id: rainbond.vm-template-import.restore-progress
func TestResolveVMRestoreStatusIncludesDataVolumeProgress(t *testing.T) {
	status := resolveVMDataVolumeRestoreStatus("demo-ns", []vmDataVolumeDetail{
		{
			Name:     "manual133",
			Phase:    "ImportInProgress",
			Progress: "11.34%",
			Message:  "copying disk",
		},
		{
			Name:     "manual134",
			Phase:    "Succeeded",
			Progress: "100.0%",
		},
	})

	if status == nil {
		t.Fatal("expected restore status")
	}
	if status.Status != "restoring" {
		t.Fatalf("expected restore status %q, got %q", "restoring", status.Status)
	}
	if status.Progress != "11.34%" {
		t.Fatalf("expected aggregate progress %q, got %q", "11.34%", status.Progress)
	}
	if len(status.DataVolumes) != 2 {
		t.Fatalf("expected two data volumes, got %#v", status.DataVolumes)
	}
	if len(status.ImporterPods) != 1 || status.ImporterPods[0].Name != "importer-manual133" {
		t.Fatalf("expected importer pod for importing volume, got %#v", status.ImporterPods)
	}
	if status.Message != "manual133: copying disk" {
		t.Fatalf("expected restore message from importing volume, got %q", status.Message)
	}
}

// capability_id: rainbond.vm-template-import.restore-progress
func TestResolveVMRestoreStatusMarksAllDataVolumesSucceeded(t *testing.T) {
	status := resolveVMDataVolumeRestoreStatus("demo-ns", []vmDataVolumeDetail{
		{
			Name:     "manual133",
			Phase:    "Succeeded",
			Progress: "100.0%",
		},
	})

	if status == nil {
		t.Fatal("expected restore status")
	}
	if status.Status != "success" {
		t.Fatalf("expected restore status %q, got %q", "success", status.Status)
	}
	if status.Progress != "100.0%" {
		t.Fatalf("expected progress 100.0%%, got %q", status.Progress)
	}
	if len(status.ImporterPods) != 0 {
		t.Fatalf("expected no importer pods after success, got %#v", status.ImporterPods)
	}
}

// capability_id: rainbond.vm-template-import.restore-progress
func TestResolveVMDataVolumeRestoreIgnoresInitialBlankDataVolumes(t *testing.T) {
	action := &ServiceAction{
		getVirtualMachineByServiceIDHook: func(serviceID string) (*kubevirtv1.VirtualMachine, error) {
			return &kubevirtv1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "demo-vm",
					Namespace: "demo-ns",
				},
				Spec: kubevirtv1.VirtualMachineSpec{
					DataVolumeTemplates: []kubevirtv1.DataVolumeTemplateSpec{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "manual133",
							},
							Spec: cdiv1.DataVolumeSpec{
								Source: &cdiv1.DataVolumeSource{
									Blank: &cdiv1.DataVolumeBlankImage{},
								},
							},
						},
					},
				},
			}, nil
		},
		getDataVolumeDetailsByNamesHook: func(namespace string, names []string) ([]vmDataVolumeDetail, error) {
			t.Fatalf("blank data volumes should not be queried as restore details")
			return nil, nil
		},
	}

	if got := action.resolveVMDataVolumeRestore("service-a"); got != nil {
		t.Fatalf("expected no restore status for initial blank data volume, got %#v", got)
	}
}

func TestResolveVMTransitionStatusReturnsStartingForPendingProvisioningVMI(t *testing.T) {
	vm := &kubevirtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo-vm",
			Namespace: "demo-ns",
		},
		Status: kubevirtv1.VirtualMachineStatus{
			PrintableStatus: kubevirtv1.VirtualMachineStatusStopped,
		},
	}
	vmi := &kubevirtv1.VirtualMachineInstance{
		Status: kubevirtv1.VirtualMachineInstanceStatus{
			Phase: kubevirtv1.Pending,
			Conditions: []kubevirtv1.VirtualMachineInstanceCondition{
				{
					Type:   kubevirtv1.VirtualMachineInstanceProvisioning,
					Status: "True",
				},
			},
		},
	}

	got, ok := resolveVMTransitionStatus(vm, vmi)
	if !ok {
		t.Fatal("expected pending provisioning vmi to override vm closed status")
	}
	if got != "starting" {
		t.Fatalf("expected pending provisioning vmi to map to %q, got %q", "starting", got)
	}
}

func TestResolveVMTransitionStatusKeepsStoppedVMClosedWhenNoVMIExists(t *testing.T) {
	vm := &kubevirtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo-vm",
			Namespace: "demo-ns",
		},
		Status: kubevirtv1.VirtualMachineStatus{
			PrintableStatus: kubevirtv1.VirtualMachineStatusStopped,
		},
	}

	if got, ok := resolveVMTransitionStatus(vm, nil); ok {
		t.Fatalf("expected no override for stopped vm without vmi, got status %q", got)
	}
}

func TestResolveVMTransitionStatusReturnsAbnormalForDataVolumeError(t *testing.T) {
	vm := &kubevirtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo-vm",
			Namespace: "demo-ns",
		},
		Status: kubevirtv1.VirtualMachineStatus{
			PrintableStatus: kubevirtv1.VirtualMachineStatusDataVolumeError,
		},
	}

	got, ok := resolveVMTransitionStatus(vm, nil)
	if !ok {
		t.Fatal("expected data volume error to override vm status")
	}
	if got != "abnormal" {
		t.Fatalf("expected data volume error to map to %q, got %q", "abnormal", got)
	}
}
