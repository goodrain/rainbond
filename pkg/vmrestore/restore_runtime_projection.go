package vmrestore

import (
	"context"
	"encoding/json"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	kv "kubevirt.io/api/core/v1"
)

// RuntimeSelection is a render snapshot of the unique Region ledger. It is not
// persisted separately and never supplies fallback physical identities.
type RuntimeSelection struct {
	Scope                  Scope
	BindingGeneration      uint64
	Bindings               map[string]Binding
	Config                 RestoreConfig
	Startup, BeforeStartup RestoreStartupPolicy
}

// EncodeRuntimeVM uses the Kubernetes JSON decoder so uint disk boot orders
// become supported JSON integers for both real dynamic clients and fake clients.
func EncodeRuntimeVM(vm *kv.VirtualMachine) (*unstructured.Unstructured, error) {
	if vm == nil {
		return nil, ErrCorrupt
	}
	copy := vm.DeepCopy()
	copy.APIVersion = "kubevirt.io/v1"
	copy.Kind = "VirtualMachine"
	body, err := json.Marshal(copy)
	if err != nil {
		return nil, ErrCorrupt
	}
	object := &unstructured.Unstructured{}
	if err := object.UnmarshalJSON(body); err != nil {
		return nil, ErrCorrupt
	}
	object.SetAPIVersion("kubevirt.io/v1")
	object.SetKind("VirtualMachine")
	return object, nil
}

// ReadRestoredRuntimeSelection uses the sole live Resolver and its same Region
// ledger. A missing/mismatched source receipt never becomes canonical fallback.
func (s *Store) ReadRestoredRuntimeSelection(ctx context.Context, resolver *Resolver, scope Scope) (*RuntimeSelection, error) {
	if resolver == nil {
		return nil, ErrReferenceUnavailable
	}
	view, err := resolver.ReadBindingView(ctx, scope)
	if err != nil {
		return nil, err
	}
	current, err := s.Read(scope, "")
	if err != nil {
		return nil, err
	}
	if current.State.ControlRevision != view.ControlRevision || current.State.BindingGeneration != view.BindingGeneration {
		return nil, ErrConflict
	}
	var root *Binding
	for _, b := range current.State.Bindings {
		if b.Role == "root" {
			copy := b
			root = &copy
		}
	}
	if root == nil || root.RestoreID == "" {
		return nil, nil
	}
	c, err := s.Read(scope, root.RestoreID)
	if err != nil {
		return nil, err
	}
	if c.State.BindingGeneration != view.BindingGeneration || c.Receipt == nil || c.Receipt.Restore == nil || c.Receipt.ConfigCommitHash == "" {
		return nil, ErrCorrupt
	}
	var startup, beforeStartup RestoreStartupPolicy
	if c.Receipt.NativeRestore != nil {
		if !c.Receipt.Steps[NativeRestoreCommitStep].Applied {
			return nil, ErrCorrupt
		}
		startup, beforeStartup = c.Receipt.NativeRestore.SourceStartup, c.Receipt.NativeRestore.BeforeStartup
	} else {
		if c.Receipt.Execution == nil || !c.Receipt.Steps[RestoreCommitStep].Applied {
			return nil, ErrCorrupt
		}
		startup, beforeStartup = c.Receipt.Execution.SourceStartup, c.Receipt.Execution.BeforeStartup
	}
	config, err := s.selectedRuntimeConfig(scope)
	if err != nil {
		return nil, err
	}
	if err := resolver.stable(scope, current.State); err != nil {
		return nil, err
	}
	return &RuntimeSelection{Scope: scope, BindingGeneration: view.BindingGeneration, Bindings: cloneBindings(c.State.Bindings), Config: config, Startup: startup, BeforeStartup: beforeStartup}, nil
}

func (s *Store) selectedRuntimeConfig(scope Scope) (RestoreConfig, error) {
	return readSelectedRuntimeConfig(s.db, scope, false)
}

// ProjectRestoredRuntime replaces only ledger-owned disk/configuration and
// exact proven platform bootstrap media, preserving current network and users'
// advanced devices. It performs metadata reads only and never writes a guest.
func ProjectRestoredRuntime(ctx context.Context, core kubernetes.Interface, vm *kv.VirtualMachine, selection RuntimeSelection) (*kv.VirtualMachine, error) {
	if core == nil || vm == nil || vm.Spec.Template == nil || vm.Namespace != selection.Scope.Namespace || selection.Config.validate(selection.Scope) != nil || validateBindings(selection.Bindings) != nil || selection.Startup.validate(selection.Scope) != nil || selection.BeforeStartup.validate(selection.Scope) != nil {
		return nil, ErrRestoreCapability
	}
	remove := map[string]bool{}
	beforeMedia := selection.BeforeStartup.GeneratedMedia
	if selection.Startup.BootstrapMode != "restored" {
		beforeMedia = nil
	}
	for _, media := range beforeMedia {
		matched := false
		for _, v := range vm.Spec.Template.Spec.Volumes {
			if v.Name != media.VolumeName {
				continue
			}
			if media.Source.Kind == "ConfigMap" && v.ConfigMap != nil && v.ConfigMap.Name == media.Source.Name {
				object, err := core.CoreV1().ConfigMaps(selection.Scope.Namespace).Get(ctx, media.Source.Name, metav1.GetOptions{})
				if err != nil {
					return nil, ErrReferenceUnavailable
				}
				matched = string(object.UID) == media.Source.UID
			} else if media.Source.Kind == "Secret" && v.Secret != nil && v.Secret.SecretName == media.Source.Name {
				object, err := core.CoreV1().Secrets(selection.Scope.Namespace).Get(ctx, media.Source.Name, metav1.GetOptions{})
				if err != nil {
					return nil, ErrReferenceUnavailable
				}
				matched = string(object.UID) == media.Source.UID
			}
			if !matched {
				return nil, ErrConflict
			}
		}
		if matched {
			remove[media.VolumeName] = true
			remove[media.DiskName] = true
		}
	}
	if selection.Startup.BootstrapMode == "restored" && selection.BeforeStartup.DataSourceKind == "CloudInitNoCloud" {
		for _, v := range vm.Spec.Template.Spec.Volumes {
			if v.Name == selection.BeforeStartup.NoCloudVolumeName && v.CloudInitNoCloud == nil {
				return nil, ErrConflict
			}
		}
		remove[selection.BeforeStartup.NoCloudVolumeName] = true
		remove[selection.BeforeStartup.NoCloudDiskName] = true
	}
	managed := map[string]bool{}
	knownClaims := map[string]bool{}
	knownDVs := map[string]bool{}
	for _, b := range selection.Bindings {
		managed[b.CanonicalName] = true
		knownClaims[b.ActivePVC.Name] = true
		if b.PreviousPVC != nil {
			knownClaims[b.PreviousPVC.Name] = true
		}
		if b.ActiveDV != nil {
			knownDVs[b.ActiveDV.Name] = true
		}
		if b.PreviousDV != nil {
			knownDVs[b.PreviousDV.Name] = true
		}
	}
	result := vm.DeepCopy()
	volumes := []kv.Volume{}
	disks := []kv.Disk{}
	for _, v := range result.Spec.Template.Spec.Volumes {
		owns := managed[v.Name] || (v.PersistentVolumeClaim != nil && knownClaims[v.PersistentVolumeClaim.ClaimName]) || (v.DataVolume != nil && knownDVs[v.DataVolume.Name])
		if owns || remove[v.Name] {
			remove[v.Name] = true
			continue
		}
		volumes = append(volumes, v)
	}
	for _, d := range result.Spec.Template.Spec.Domain.Devices.Disks {
		if !remove[d.Name] && !managed[d.Name] {
			disks = append(disks, d)
		}
	}
	for _, layout := range selection.Config.Hardware.DiskLayout {
		b, exists := selection.Bindings[strconv.FormatUint(uint64(layout.VolumeID), 10)]
		if !exists || !b.Attached || b.DiskKey != layout.DiskKey || b.ActivePVC.Namespace != selection.Scope.Namespace {
			return nil, ErrConflict
		}
		volumes = append(volumes, kv.Volume{Name: b.CanonicalName, VolumeSource: kv.VolumeSource{PersistentVolumeClaim: &kv.PersistentVolumeClaimVolumeSource{PersistentVolumeClaimVolumeSource: corev1.PersistentVolumeClaimVolumeSource{ClaimName: b.ActivePVC.Name, ReadOnly: layout.ReadOnly}}}})
		disk := kv.Disk{Name: b.CanonicalName}
		bus := kv.DiskBus(layout.Bus)
		switch layout.Device {
		case "disk":
			disk.Disk = &kv.DiskTarget{Bus: bus}
		case "lun":
			disk.LUN = &kv.LunTarget{Bus: bus}
		case "cdrom":
			disk.CDRom = &kv.CDRomTarget{Bus: bus}
		default:
			return nil, ErrCorrupt
		}
		if layout.BootOrder > 0 {
			order := layout.BootOrder
			disk.BootOrder = &order
		}
		disks = append(disks, disk)
	}
	if selection.Startup.BootstrapMode == "restored" && selection.Startup.DataSourceKind == "CloudInitNoCloud" {
		for _, v := range volumes {
			if v.Name == selection.Startup.NoCloudVolumeName {
				return nil, ErrConflict
			}
		}
		volumes = append(volumes, kv.Volume{Name: selection.Startup.NoCloudVolumeName, VolumeSource: kv.VolumeSource{CloudInitNoCloud: &kv.CloudInitNoCloudSource{UserData: "#cloud-config\n{}\n"}}})
		disks = append(disks, kv.Disk{Name: selection.Startup.NoCloudDiskName, DiskDevice: kv.DiskDevice{Disk: &kv.DiskTarget{Bus: kv.DiskBusVirtio}}})
	}
	result.Spec.Template.Spec.Volumes = volumes
	result.Spec.Template.Spec.Domain.Devices.Disks = disks
	templates := []kv.DataVolumeTemplateSpec{}
	for _, dv := range result.Spec.DataVolumeTemplates {
		if !knownDVs[dv.Name] {
			templates = append(templates, dv)
		}
	}
	result.Spec.DataVolumeTemplates = templates
	domain := &result.Spec.Template.Spec.Domain
	if domain.CPU == nil {
		domain.CPU = &kv.CPU{}
	}
	domain.CPU.Sockets = 1
	domain.CPU.Cores = uint32(selection.Config.Hardware.CPUMilli / 1000)
	domain.CPU.Threads = 1
	guest := resource.NewQuantity(selection.Config.Hardware.MemoryMiB<<20, resource.BinarySI)
	domain.Memory = &kv.Memory{Guest: guest}
	if domain.Resources.Requests == nil {
		domain.Resources.Requests = corev1.ResourceList{}
	}
	domain.Resources.Requests[corev1.ResourceMemory] = *guest
	domain.Resources.Requests[corev1.ResourceCPU] = *resource.NewMilliQuantity(int64(selection.Config.Hardware.CPUMilli), resource.DecimalSI)
	if domain.Firmware == nil {
		domain.Firmware = &kv.Firmware{}
	}
	domain.Firmware.UUID = types.UID(selection.Startup.FirmwareUUID)
	domain.Firmware.Bootloader = &kv.Bootloader{}
	if selection.Config.Hardware.BootMode == "bios" {
		domain.Firmware.Bootloader.BIOS = &kv.BIOS{}
	} else {
		secure := false
		domain.Firmware.Bootloader.EFI = &kv.EFI{SecureBoot: &secure}
	}
	if err := MarkVMGeneration(result, selection.Scope, selection.BindingGeneration); err != nil {
		return nil, err
	}
	return result, nil
}
