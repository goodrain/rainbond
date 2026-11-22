package handler

import (
	"context"
	"reflect"
	"testing"

	"github.com/golang/mock/gomock"
	apimodel "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	workermodel "github.com/goodrain/rainbond/worker/discover/model"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	kv "kubevirt.io/api/core/v1"
	"kubevirt.io/client-go/kubecli"
)

// capability_id: rainbond.vm-network.preserve-binding-on-edit
func TestHardwareSaveUpdatesResourcesWithoutReplacingStoredNetwork(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	service := &dbmodel.TenantServices{ServiceID: "edited-vm", ExtendMethod: "vm", ContainerCPU: 1000, ContainerMemory: 1024}
	services := &resourceSyncTenantServiceDao{service: service}
	db.SetTestManager(resourceSyncTestManager{serviceDao: services, eventDao: &resourceSyncEventDao{}})
	defer db.SetTestManager(nil)
	client := kubecli.NewMockKubevirtClient(ctrl)
	machines := kubecli.NewMockVirtualMachineInterface(ctrl)
	secureBoot := false
	vm := &kv.VirtualMachine{ObjectMeta: metav1.ObjectMeta{Name: "edited-vm", Namespace: "team", UID: types.UID("same-vm")},
		Spec: kv.VirtualMachineSpec{Template: &kv.VirtualMachineInstanceTemplateSpec{Spec: kv.VirtualMachineInstanceSpec{
			Domain: kv.DomainSpec{
				CPU: &kv.CPU{Cores: 1}, Memory: &kv.Memory{Guest: quantityPtr("1Gi")},
				Firmware: &kv.Firmware{UUID: types.UID("same-firmware"), Bootloader: &kv.Bootloader{EFI: &kv.EFI{SecureBoot: &secureBoot}}},
				Devices:  kv.Devices{Interfaces: []kv.Interface{{Name: "default", Model: "virtio", MacAddress: "82:20:94:04:c7:5a", InterfaceBindingMethod: kv.InterfaceBindingMethod{Masquerade: &kv.InterfaceMasquerade{}}}}},
			},
			Networks: []kv.Network{{Name: "default", NetworkSource: kv.NetworkSource{Pod: &kv.PodNetwork{}}}},
		}}}, Status: kv.VirtualMachineStatus{PrintableStatus: kv.VirtualMachineStatusStopped}}
	attachRawRuntimeObservation(t, client, vm)
	original := vm.DeepCopy()
	updates := 0
	client.EXPECT().VirtualMachine(vm.Namespace).Return(machines).AnyTimes()
	machines.EXPECT().Get(gomock.Any(), vm.Name, gomock.Any()).DoAndReturn(func(context.Context, string, metav1.GetOptions) (*kv.VirtualMachine, error) {
		return vm.DeepCopy(), nil
	}).Times(2)
	machines.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, next *kv.VirtualMachine, _ metav1.UpdateOptions) (*kv.VirtualMachine, error) {
		updates++
		vm = next.DeepCopy()
		return next, nil
	})
	action := &ServiceAction{MQClient: &noopMQClient{}, kubevirtClient: client,
		getVirtualMachineByServiceIDHook: func(string) (*kv.VirtualMachine, error) { return vm.DeepCopy(), nil },
	}
	action.syncVirtualMachineSpecHook = func(string) error {
		// The render boundary reads the just-saved DB values but uses the old default bridge.
		desired := vm.DeepCopy()
		desired.Spec.Template.Spec.Domain.CPU = &kv.CPU{Cores: uint32(services.service.ContainerCPU / 1000)}
		memory := resource.MustParse("2Gi")
		desired.Spec.Template.Spec.Domain.Memory = &kv.Memory{Guest: &memory}
		desired.Spec.Template.Spec.Domain.Devices.Interfaces = []kv.Interface{{Name: "default", Model: "virtio", InterfaceBindingMethod: kv.InterfaceBindingMethod{Bridge: &kv.InterfaceBridge{}}}}
		desired.Spec.Template.Spec.Domain.Firmware = nil
		return action.syncVirtualMachineSpec(vm.DeepCopy(), desired)
	}
	cpu, memory := 2000, 2048
	if err := action.ServiceVertical(context.Background(), &workermodel.VerticalScalingTaskBody{ServiceID: service.ServiceID, ContainerCPU: &cpu, ContainerMemory: &memory}); err != nil {
		t.Fatal(err)
	}
	if updates != 1 || services.updatedService == nil || services.updatedService.ContainerCPU != cpu || services.updatedService.ContainerMemory != memory {
		t.Fatal("hardware save did not persist and synchronize the explicit resource edit")
	}
	if vm.Spec.Template.Spec.Domain.CPU.Cores != 2 || vm.Spec.Template.Spec.Domain.Memory.Guest.Cmp(resource.MustParse("2Gi")) != 0 {
		t.Fatal("preserving runtime identity discarded the requested CPU/memory edit")
	}
	if !reflect.DeepEqual(vm.Spec.Template.Spec.Networks, original.Spec.Template.Spec.Networks) ||
		!reflect.DeepEqual(vm.Spec.Template.Spec.Domain.Devices.Interfaces, original.Spec.Template.Spec.Domain.Devices.Interfaces) ||
		!reflect.DeepEqual(vm.Spec.Template.Spec.Domain.Firmware, original.Spec.Template.Spec.Domain.Firmware) {
		t.Fatal("saving CPU/memory replaced the unrelated stored network or firmware identity")
	}
	saved := vm.DeepCopy()
	machines.EXPECT().Start(gomock.Any(), vm.Name, gomock.Any()).DoAndReturn(func(context.Context, string, *kv.StartOptions) error {
		if !reflect.DeepEqual(vm.Spec, saved.Spec) {
			t.Fatal("power-on discarded the hardware saved before this request")
		}
		return nil
	})
	if err := action.StartOrCreateVM(context.Background(), &apimodel.StartStopStruct{ServiceID: service.ServiceID}, "saved-version"); err != nil {
		t.Fatal(err)
	}
}

// capability_id: rainbond.vm-network.preserve-binding-on-edit
func TestSpecMergeRetainsExplicitDiskAndDeviceChanges(t *testing.T) {
	existing := &kv.VirtualMachine{Spec: kv.VirtualMachineSpec{Template: &kv.VirtualMachineInstanceTemplateSpec{Spec: kv.VirtualMachineInstanceSpec{
		Domain:   kv.DomainSpec{Devices: kv.Devices{Interfaces: []kv.Interface{{Name: "default", MacAddress: "82:20:94:04:c7:5a", InterfaceBindingMethod: kv.InterfaceBindingMethod{Masquerade: &kv.InterfaceMasquerade{}}}}}},
		Networks: []kv.Network{{Name: "default", NetworkSource: kv.NetworkSource{Pod: &kv.PodNetwork{}}}},
	}}}}
	desired := existing.DeepCopy()
	desired.Spec.Template.Spec.Domain.Devices.Disks = []kv.Disk{{Name: "extra", DiskDevice: kv.DiskDevice{Disk: &kv.DiskTarget{Bus: kv.DiskBusVirtio}}}}
	desired.Spec.Template.Spec.Volumes = []kv.Volume{{Name: "extra", VolumeSource: kv.VolumeSource{DataVolume: &kv.DataVolumeSource{Name: "selected-data-volume"}}}}
	desired.Spec.Template.Spec.NodeSelector = map[string]string{"hardware": "requested-node"}
	desired.Spec.Template.Spec.Domain.Firmware = &kv.Firmware{Bootloader: &kv.Bootloader{BIOS: &kv.BIOS{}}}
	desired.Spec.Template.Spec.Domain.Devices.Interfaces[0].MacAddress = ""
	desired.Spec.Template.Spec.Domain.Devices.Interfaces[0].Masquerade = nil
	desired.Spec.Template.Spec.Domain.Devices.Interfaces[0].Bridge = &kv.InterfaceBridge{}
	before, requested := existing.DeepCopy(), desired.DeepCopy()
	merged, changed := applyVirtualMachineSpec(existing, desired)
	if !changed || !reflect.DeepEqual(merged.Spec.Template.Spec.Volumes, desired.Spec.Template.Spec.Volumes) || !reflect.DeepEqual(merged.Spec.Template.Spec.Domain.Devices.Disks, desired.Spec.Template.Spec.Domain.Devices.Disks) {
		t.Fatal("stored network preservation suppressed the explicitly selected disk/device change")
	}
	if !reflect.DeepEqual(merged.Spec.Template.Spec.Domain.Devices.Interfaces, existing.Spec.Template.Spec.Domain.Devices.Interfaces) {
		t.Fatal("disk edit replaced the stored network interface")
	}
	if !reflect.DeepEqual(merged.Spec.Template.Spec.NodeSelector, desired.Spec.Template.Spec.NodeSelector) || !reflect.DeepEqual(merged.Spec.Template.Spec.Domain.Firmware, desired.Spec.Template.Spec.Domain.Firmware) {
		t.Fatal("preserving network identity ignored explicit scheduling or boot configuration")
	}
	if !reflect.DeepEqual(existing, before) || !reflect.DeepEqual(desired, requested) {
		t.Fatal("merging VM edits mutated an input specification")
	}
}

// capability_id: rainbond.vm-network.preserve-binding-on-edit
func TestSpecMergeKeepsCompleteNetworkBinding(t *testing.T) {
	before := networkIdentityVM("82:20:94:04:c7:5a", 1)
	before.Spec.Template.Spec.Domain.Devices.Interfaces[0].Bridge = nil
	before.Spec.Template.Spec.Domain.Devices.Interfaces[0].Masquerade = &kv.InterfaceMasquerade{}
	before.Spec.Template.Spec.Domain.Devices.Interfaces[0].Ports = []kv.Port{{Name: "web", Port: 80}}
	before.Spec.Template.Spec.Networks = []kv.Network{{Name: "default", NetworkSource: kv.NetworkSource{Pod: &kv.PodNetwork{VMNetworkCIDR: "10.0.2.0/24"}}}, {Name: "secondary", NetworkSource: kv.NetworkSource{Multus: &kv.MultusNetwork{NetworkName: "team/network"}}}}
	before.Spec.Template.Spec.Domain.Devices.Interfaces = append(before.Spec.Template.Spec.Domain.Devices.Interfaces, kv.Interface{Name: "secondary", MacAddress: "02:22:33:44:55:66", InterfaceBindingMethod: kv.InterfaceBindingMethod{Bridge: &kv.InterfaceBridge{}}})
	desired := networkIdentityVM("02:aa:bb:cc:dd:ee", 4)
	desired.Spec.Template.Spec.Networks = []kv.Network{{Name: "default", NetworkSource: kv.NetworkSource{Pod: &kv.PodNetwork{}}}}
	oldCopy, desiredCopy := before.DeepCopy(), desired.DeepCopy()
	merged, changed := applyVirtualMachineSpec(before, desired)
	if !changed || merged.Spec.Template.Spec.Domain.CPU.Cores != 4 {
		t.Fatal("network preservation discarded requested hardware")
	}
	if !reflect.DeepEqual(merged.Spec.Template.Spec.Networks, before.Spec.Template.Spec.Networks) || !reflect.DeepEqual(merged.Spec.Template.Spec.Domain.Devices.Interfaces, before.Spec.Template.Spec.Domain.Devices.Interfaces) {
		t.Fatal("resource rendering replaced stored network binding, model, ports, MAC, or secondary network")
	}
	if !reflect.DeepEqual(before, oldCopy) || !reflect.DeepEqual(desired, desiredCopy) {
		t.Fatal("merge modified an input VM")
	}
	merged.Spec.Template.Spec.Domain.Devices.Interfaces[0].MacAddress = "02:00:00:00:00:01"
	merged.Spec.Template.Spec.Networks[0].Pod.VMNetworkCIDR = "10.99.0.0/24"
	if !reflect.DeepEqual(before, oldCopy) || !reflect.DeepEqual(desired, desiredCopy) {
		t.Fatal("merged network aliases an input VM")
	}
}

// capability_id: rainbond.vm-network.preserve-binding-on-edit
func TestSpecMergeFirmwareDefaultsAndExplicitUUID(t *testing.T) {
	for _, storedUUID := range []types.UID{"", "stored-firmware"} {
		t.Run(string(storedUUID), func(t *testing.T) {
			secure := false
			before := networkIdentityVM("02:11:22:33:44:55", 1)
			before.Spec.Template.Spec.Domain.Firmware = &kv.Firmware{UUID: storedUUID, Bootloader: &kv.Bootloader{EFI: &kv.EFI{SecureBoot: &secure}}}
			desired := before.DeepCopy()
			desired.Spec.Template.Spec.Domain.Firmware = nil
			merged, changed := applyVirtualMachineSpec(before, desired)
			if changed || !reflect.DeepEqual(merged.Spec.Template.Spec.Domain.Firmware, before.Spec.Template.Spec.Domain.Firmware) {
				t.Fatal("omitted firmware lost the stored EFI/secure-boot configuration")
			}
			desired = before.DeepCopy()
			desired.Spec.Template.Spec.Domain.Firmware = &kv.Firmware{UUID: "explicit-firmware", Bootloader: &kv.Bootloader{BIOS: &kv.BIOS{}}}
			merged, changed = applyVirtualMachineSpec(before, desired)
			if !changed || !reflect.DeepEqual(merged.Spec.Template.Spec.Domain.Firmware, desired.Spec.Template.Spec.Domain.Firmware) {
				t.Fatal("preservation ignored the existing explicit UUID/bootloader semantics")
			}
			desired.Spec.Template.Spec.Domain.Firmware.UUID = ""
			merged, _ = applyVirtualMachineSpec(before, desired)
			if merged.Spec.Template.Spec.Domain.Firmware.UUID != storedUUID || merged.Spec.Template.Spec.Domain.Firmware.Bootloader.BIOS == nil {
				t.Fatal("existing implicit UUID fallback or explicit bootloader change regressed")
			}
		})
	}
}
