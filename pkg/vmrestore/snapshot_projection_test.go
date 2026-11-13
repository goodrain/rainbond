package vmrestore

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	csi "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	kv "kubevirt.io/api/core/v1"
	native "kubevirt.io/api/snapshot/v1beta1"
)

type snapshotFixture struct {
	scope    Scope
	intent   SnapshotIntent
	op       string
	core     string
	snapshot *native.VirtualMachineSnapshot
	content  *native.VirtualMachineSnapshotContent
	volumes  map[string]*csi.VolumeSnapshot
}

func snapshotProjectionFixture(t *testing.T) snapshotFixture {
	t.Helper()
	scope := Scope{TenantID: "tenant", ServiceID: "service", Namespace: "team"}
	op := "operation-a"
	ready := true
	order := uint(1)
	quantity := resource.MustParse("2Gi")
	storage := "cephfs-vm-snapshot"
	claim := "active-pvc"
	vsName := "volume-snapshot-a"
	contentName := "vmsnapshot-content-snapshot-uid"
	sourceUID := types.UID("source-vm-uid")
	labels := map[string]string{"tenant_id": scope.TenantID, "service_id": scope.ServiceID}
	vm := &kv.VirtualMachine{ObjectMeta: metav1.ObjectMeta{Name: "vm-a", Namespace: scope.Namespace, UID: sourceUID, Labels: labels}, Spec: kv.VirtualMachineSpec{Template: &kv.VirtualMachineInstanceTemplateSpec{Spec: kv.VirtualMachineInstanceSpec{Domain: kv.DomainSpec{CPU: &kv.CPU{Sockets: 2, Cores: 1, Threads: 1}, Memory: &kv.Memory{Guest: &quantity}, Devices: kv.Devices{Disks: []kv.Disk{{Name: "manual7", BootOrder: &order, DiskDevice: kv.DiskDevice{Disk: &kv.DiskTarget{Bus: kv.DiskBusVirtio}}}}}}, Volumes: []kv.Volume{{Name: "manual7", VolumeSource: kv.VolumeSource{PersistentVolumeClaim: &kv.PersistentVolumeClaimVolumeSource{PersistentVolumeClaimVolumeSource: corev1.PersistentVolumeClaimVolumeSource{ClaimName: claim}}}}, {Name: "cloud-init", VolumeSource: kv.VolumeSource{CloudInitNoCloud: &kv.CloudInitNoCloudSource{UserData: "NON_SECRET_METADATA_SENTINEL"}}}}}}}}
	disk := SnapshotDisk{VolumeID: 7, DiskKey: "disk", CanonicalName: "manual7", Role: "root", SourcePVC: ObjectRef{Kind: "PersistentVolumeClaim", Namespace: scope.Namespace, Name: claim, UID: "pvc-uid"}, StorageClass: storage, RequestedBytes: 20 << 30}
	config, err := SnapshotSafeConfig(vm, []SnapshotDisk{disk})
	if err != nil {
		t.Fatal("fixture config invalid")
	}
	configHash, err := snapshotDigest(config)
	if err != nil {
		t.Fatal(err)
	}
	intent := SnapshotIntent{OperationID: op, Name: "snapshot-a", Description: "before update", SourceVM: ObjectRef{Kind: "VirtualMachine", Namespace: scope.Namespace, Name: vm.Name, UID: string(vm.UID)}, WasRunning: true, PowerAfter: "previous", ConfigHash: configHash, Disks: []SnapshotDisk{disk}}
	core, err := SnapshotCoreHash(scope, intent)
	if err != nil {
		t.Fatal(err)
	}
	apiGroup := kv.VirtualMachineGroupVersionKind.Group
	snap := &native.VirtualMachineSnapshot{ObjectMeta: metav1.ObjectMeta{Name: intent.Name, Namespace: scope.Namespace, UID: "snapshot-uid", Labels: map[string]string{"service_id": scope.ServiceID, "tenant_id": scope.TenantID, SnapshotOperationLabel: op}, Annotations: map[string]string{SnapshotCoreAnnotation: core, SnapshotSourceUIDAnnotation: string(sourceUID)}}, Spec: native.VirtualMachineSnapshotSpec{Source: corev1.TypedLocalObjectReference{APIGroup: &apiGroup, Kind: "VirtualMachine", Name: vm.Name}}, Status: &native.VirtualMachineSnapshotStatus{SourceUID: &sourceUID, VirtualMachineSnapshotContentName: &contentName, ReadyToUse: &ready, Phase: native.Succeeded, SnapshotVolumes: &native.SnapshotVolumesLists{IncludedVolumes: []string{"manual7"}}}}
	content := &native.VirtualMachineSnapshotContent{ObjectMeta: metav1.ObjectMeta{Name: contentName, Namespace: scope.Namespace, UID: "content-uid"}, Spec: native.VirtualMachineSnapshotContentSpec{VirtualMachineSnapshotName: &intent.Name, Source: native.SourceSpec{VirtualMachine: &native.VirtualMachine{ObjectMeta: vm.ObjectMeta, Spec: vm.Spec}}, VolumeBackups: []native.VolumeBackup{{VolumeName: "manual7", PersistentVolumeClaim: native.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: claim, Namespace: scope.Namespace, UID: "pvc-uid"}, Spec: corev1.PersistentVolumeClaimSpec{StorageClassName: &storage, Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("20Gi")}}}}, VolumeSnapshotName: &vsName}}}, Status: &native.VirtualMachineSnapshotContentStatus{ReadyToUse: &ready}}
	volume := &csi.VolumeSnapshot{ObjectMeta: metav1.ObjectMeta{Name: vsName, Namespace: scope.Namespace, UID: "volume-snapshot-uid", OwnerReferences: []metav1.OwnerReference{{APIVersion: native.SchemeGroupVersion.String(), Kind: "VirtualMachineSnapshotContent", Name: contentName, UID: content.UID}}}, Spec: csi.VolumeSnapshotSpec{Source: csi.VolumeSnapshotSource{PersistentVolumeClaimName: &claim}}, Status: &csi.VolumeSnapshotStatus{ReadyToUse: &ready}}
	return snapshotFixture{scope, intent, op, core, snap, content, map[string]*csi.VolumeSnapshot{vsName: volume}}
}
func (f snapshotFixture) project(proof SnapshotProof) (SnapshotDTO, error) {
	return ProjectSnapshot(f.scope, f.intent, f.op, f.core, f.snapshot, f.content, f.volumes, proof)
}

// capability_id: rainbond.vm-snapshot.safe-metadata-projection
func TestSnapshotProjectionUsesApprovedFieldsAndCompleteCoverage(t *testing.T) {
	f := snapshotProjectionFixture(t)
	dto, err := f.project(SnapshotProof{})
	if err != nil {
		t.Fatal(err)
	}
	if !dto.ReadyToUse || !dto.Coverage.Complete || dto.Config.CPUMilli != 2000 || dto.Config.MemoryMiB != 2048 || len(dto.Disks) != 1 {
		t.Fatal("valid full snapshot was not represented accurately")
	}
	data, err := json.Marshal(dto)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "NON_SECRET_METADATA_SENTINEL") || strings.Contains(string(data), "cloudInit") || strings.Contains(string(data), "spec") {
		t.Fatal("safe projection leaked native initialization metadata")
	}
}

func TestNativeReadyCannotHideMissingWritableDisk(t *testing.T) {
	f := snapshotProjectionFixture(t)
	other := f.intent.Disks[0]
	other.VolumeID = 8
	other.DiskKey = "data"
	other.CanonicalName = "manual8"
	other.Role = "data"
	other.SourcePVC.Name = "data-pvc"
	other.SourcePVC.UID = "data-uid"
	f.intent.Disks = append(f.intent.Disks, other)
	vm := f.content.Spec.Source.VirtualMachine
	vm.Spec.Template.Spec.Domain.Devices.Disks = append(vm.Spec.Template.Spec.Domain.Devices.Disks, kv.Disk{Name: "manual8", DiskDevice: kv.DiskDevice{Disk: &kv.DiskTarget{Bus: kv.DiskBusVirtio}}})
	vm.Spec.Template.Spec.Volumes = append(vm.Spec.Template.Spec.Volumes, kv.Volume{Name: "manual8", VolumeSource: kv.VolumeSource{PersistentVolumeClaim: &kv.PersistentVolumeClaimVolumeSource{PersistentVolumeClaimVolumeSource: corev1.PersistentVolumeClaimVolumeSource{ClaimName: "data-pvc"}}}})
	current := &kv.VirtualMachine{ObjectMeta: vm.ObjectMeta, Spec: vm.Spec}
	config, err := SnapshotSafeConfig(current, f.intent.Disks)
	if err != nil {
		t.Fatal(err)
	}
	f.intent.ConfigHash, _ = snapshotDigest(config)
	f.core, _ = SnapshotCoreHash(f.scope, f.intent)
	f.snapshot.Annotations[SnapshotCoreAnnotation] = f.core
	dto, err := f.project(SnapshotProof{})
	if err != nil {
		t.Fatal(err)
	}
	if dto.ReadyToUse || dto.Coverage.ExpectedWritableDisks != 2 || dto.Coverage.IncludedDisks != 1 || len(dto.Coverage.MissingDiskKeys) != 1 || dto.Coverage.MissingDiskKeys[0] != "data" {
		t.Fatal("native Ready masked a missing writable disk")
	}
}

func TestSnapshotProjectionRejectsReplacedResourcesAndUnsealedSource(t *testing.T) {
	for _, what := range []string{"snapshot", "content", "volume", "scope", "source"} {
		t.Run(what, func(t *testing.T) {
			f := snapshotProjectionFixture(t)
			proof := SnapshotProof{Sealed: true, SnapshotUID: string(f.snapshot.UID), ContentUID: string(f.content.UID), VolumeSnapshotUIDs: map[string]string{"volume-snapshot-a": "volume-snapshot-uid"}}
			switch what {
			case "snapshot":
				f.snapshot.UID = "replacement"
			case "content":
				f.content.UID = "replacement"
			case "volume":
				f.volumes["volume-snapshot-a"].UID = "replacement"
			case "scope":
				f.content.Namespace = "other-team"
			case "source":
				other := types.UID("new-vm")
				f.snapshot.Status.SourceUID = &other
				proof = SnapshotProof{}
			}
			if _, err := f.project(proof); !errors.Is(err, ErrConflict) {
				t.Fatal("replacement or cross-scope source was accepted")
			}
		})
	}
}

func TestSealedHistoricalSnapshotDoesNotUseCurrentVMUIDAsOrigin(t *testing.T) {
	f := snapshotProjectionFixture(t)
	newUID := types.UID("rebuilt-current-vm")
	f.snapshot.Status.SourceUID = &newUID
	proof := SnapshotProof{Sealed: true, SnapshotUID: string(f.snapshot.UID), ContentUID: string(f.content.UID), VolumeSnapshotUIDs: map[string]string{"volume-snapshot-a": "volume-snapshot-uid"}}
	dto, err := f.project(proof)
	if err != nil || !dto.ReadyToUse || dto.SourceVMUID != f.intent.SourceVM.UID {
		t.Fatal("historical source was replaced with current VM identity")
	}
}

func TestUnknownWritableVolumeIsNotSilentlyExcluded(t *testing.T) {
	f := snapshotProjectionFixture(t)
	vm := f.content.Spec.Source.VirtualMachine
	vm.Spec.Template.Spec.Volumes = append(vm.Spec.Template.Spec.Volumes, kv.Volume{Name: "unmanaged", VolumeSource: kv.VolumeSource{PersistentVolumeClaim: &kv.PersistentVolumeClaimVolumeSource{PersistentVolumeClaimVolumeSource: corev1.PersistentVolumeClaimVolumeSource{ClaimName: "other-pvc"}}}})
	if _, err := f.project(SnapshotProof{}); !errors.Is(err, ErrSnapshotIncomplete) {
		t.Fatal("unknown writable storage was silently omitted")
	}
}

func TestSnapshotProjectionBlocksEveryUnmanagedStorageSource(t *testing.T) {
	for name, source := range map[string]kv.VolumeSource{
		"pvc":            {PersistentVolumeClaim: &kv.PersistentVolumeClaimVolumeSource{}},
		"data-volume":    {DataVolume: &kv.DataVolumeSource{Name: "unmanaged"}},
		"host-disk":      {HostDisk: &kv.HostDisk{}},
		"ephemeral":      {Ephemeral: &kv.EphemeralVolumeSource{}},
		"empty-disk":     {EmptyDisk: &kv.EmptyDiskSource{}},
		"container-disk": {ContainerDisk: &kv.ContainerDiskSource{}},
		"memory-dump":    {MemoryDump: &kv.MemoryDumpVolumeSource{}},
		"unknown":        {},
	} {
		t.Run(name, func(t *testing.T) {
			f := snapshotProjectionFixture(t)
			vm := f.content.Spec.Source.VirtualMachine
			vm.Spec.Template.Spec.Volumes = append(vm.Spec.Template.Spec.Volumes, kv.Volume{Name: "unmanaged", VolumeSource: source})
			if _, err := f.project(SnapshotProof{}); !errors.Is(err, ErrSnapshotIncomplete) {
				t.Fatalf("unmanaged storage was omitted: %v", err)
			}
		})
	}
}

func TestSnapshotSafeConfigRequiresExactManagedVolumeSource(t *testing.T) {
	for _, name := range []string{"missing", "other-pvc", "other-dv", "duplicate", "empty-disk"} {
		t.Run(name, func(t *testing.T) {
			f := snapshotProjectionFixture(t)
			source := f.content.Spec.Source.VirtualMachine
			volumes := &source.Spec.Template.Spec.Volumes
			switch name {
			case "missing":
				*volumes = (*volumes)[1:]
			case "other-pvc":
				(*volumes)[0].PersistentVolumeClaim.ClaimName = "other"
			case "other-dv":
				(*volumes)[0].VolumeSource = kv.VolumeSource{DataVolume: &kv.DataVolumeSource{Name: "other"}}
			case "duplicate":
				*volumes = append(*volumes, (*volumes)[0])
			case "empty-disk":
				(*volumes)[0].VolumeSource = kv.VolumeSource{EmptyDisk: &kv.EmptyDiskSource{}}
			}
			vm := &kv.VirtualMachine{ObjectMeta: source.ObjectMeta, Spec: source.Spec}
			if _, err := SnapshotSafeConfig(vm, f.intent.Disks); err == nil {
				t.Fatal("managed layout accepted an unmatched storage source")
			}
		})
	}
}

func TestSealedHistoricalSnapshotSurvivesMissingMutableSourceStatus(t *testing.T) {
	f := snapshotProjectionFixture(t)
	f.snapshot.Status.SourceUID = nil
	proof := SnapshotProof{Sealed: true, SnapshotUID: string(f.snapshot.UID), ContentUID: string(f.content.UID), VolumeSnapshotUIDs: map[string]string{"volume-snapshot-a": "volume-snapshot-uid"}}
	dto, err := f.project(proof)
	if err != nil || !dto.ReadyToUse || dto.SourceVMUID != f.intent.SourceVM.UID {
		t.Fatal("sealed historical content was invalidated by mutable status")
	}
}

func TestSnapshotProjectionRejectsWrongNativeSourceAndCSIBinding(t *testing.T) {
	for _, name := range []string{"source-group", "volume-name", "owner-group"} {
		t.Run(name, func(t *testing.T) {
			f := snapshotProjectionFixture(t)
			switch name {
			case "source-group":
				wrong := "other.example"
				f.snapshot.Spec.Source.APIGroup = &wrong
			case "volume-name":
				f.volumes["volume-snapshot-a"].Name = "different-volume-snapshot"
			case "owner-group":
				f.volumes["volume-snapshot-a"].OwnerReferences[0].APIVersion = "other.example/v1"
			}
			if _, err := f.project(SnapshotProof{}); !errors.Is(err, ErrConflict) {
				t.Fatalf("mismatched native chain was accepted: %v", err)
			}
		})
	}
}

func TestSnapshotProjectionNeverMarksTerminatingChildrenReady(t *testing.T) {
	for _, name := range []string{"content", "csi"} {
		t.Run(name, func(t *testing.T) {
			f := snapshotProjectionFixture(t)
			now := metav1.Now()
			if name == "content" {
				f.content.DeletionTimestamp = &now
			} else {
				f.volumes["volume-snapshot-a"].DeletionTimestamp = &now
			}
			dto, err := f.project(SnapshotProof{})
			if err != nil || dto.ReadyToUse {
				t.Fatal("terminating child was marked ready")
			}
		})
	}
}

func TestSnapshotSafeConfigExcludesGeneratedInitializationMedia(t *testing.T) {
	for name, source := range map[string]kv.VolumeSource{
		"no-cloud":         {CloudInitNoCloud: &kv.CloudInitNoCloudSource{}},
		"config-drive":     {CloudInitConfigDrive: &kv.CloudInitConfigDriveSource{}},
		"sysprep":          {Sysprep: &kv.SysprepSource{}},
		"config-map":       {ConfigMap: &kv.ConfigMapVolumeSource{}},
		"secret-reference": {Secret: &kv.SecretVolumeSource{}},
		"downward-api":     {DownwardAPI: &kv.DownwardAPIVolumeSource{}},
		"service-account":  {ServiceAccount: &kv.ServiceAccountVolumeSource{}},
		"downward-metrics": {DownwardMetrics: &kv.DownwardMetricsVolumeSource{}},
	} {
		t.Run(name, func(t *testing.T) {
			f := snapshotProjectionFixture(t)
			vm := f.content.Spec.Source.VirtualMachine
			vm.Spec.Template.Spec.Volumes = append(vm.Spec.Template.Spec.Volumes, kv.Volume{Name: "generated", VolumeSource: source})
			vm.Spec.Template.Spec.Domain.Devices.Disks = append(vm.Spec.Template.Spec.Domain.Devices.Disks, kv.Disk{Name: "generated"})
			dto, err := f.project(SnapshotProof{})
			if err != nil || !dto.ReadyToUse || len(dto.Config.DiskLayout) != 1 {
				t.Fatalf("generated initialization media altered approved layout: %v", err)
			}
			// Combining initialization with a writable source must fail closed.
			vm.Spec.Template.Spec.Volumes[len(vm.Spec.Template.Spec.Volumes)-1].EmptyDisk = &kv.EmptyDiskSource{}
			if _, err := f.project(SnapshotProof{}); !errors.Is(err, ErrSnapshotIncomplete) {
				t.Fatal("initialization source hid writable storage")
			}
		})
	}
}

func TestSnapshotSafeConfigRejectsUnsupportedConfiguration(t *testing.T) {
	for _, name := range []string{"nil-template", "nil-cpu", "cpu-overflow", "total-cpu-overflow", "nil-memory", "fractional-mib", "zero-memory", "secure-boot", "duplicate-device", "read-only-mismatch", "no-device", "no-bus", "unknown-bus", "missing-device"} {
		t.Run(name, func(t *testing.T) {
			f := snapshotProjectionFixture(t)
			source := f.content.Spec.Source.VirtualMachine
			vm := &kv.VirtualMachine{ObjectMeta: source.ObjectMeta, Spec: source.Spec}
			domain := &vm.Spec.Template.Spec.Domain
			switch name {
			case "nil-template":
				vm.Spec.Template = nil
			case "nil-cpu":
				domain.CPU = nil
			case "cpu-overflow":
				domain.CPU.Sockets = 4097
			case "total-cpu-overflow":
				domain.CPU.Sockets, domain.CPU.Cores = 4096, 2
			case "nil-memory":
				domain.Memory = nil
			case "fractional-mib":
				q := resource.MustParse("1025Ki")
				domain.Memory.Guest = &q
			case "zero-memory":
				q := resource.MustParse("0")
				domain.Memory.Guest = &q
			case "secure-boot":
				yes := true
				domain.Firmware = &kv.Firmware{Bootloader: &kv.Bootloader{EFI: &kv.EFI{SecureBoot: &yes}}}
			case "duplicate-device":
				domain.Devices.Disks = append(domain.Devices.Disks, domain.Devices.Disks[0])
			case "read-only-mismatch":
				domain.Devices.Disks[0].Disk.ReadOnly = true
			case "no-device":
				domain.Devices.Disks[0].DiskDevice = kv.DiskDevice{}
			case "no-bus":
				domain.Devices.Disks[0].Disk.Bus = ""
			case "unknown-bus":
				domain.Devices.Disks[0].Disk.Bus = "unsupported"
			case "missing-device":
				domain.Devices.Disks = nil
			}
			if _, err := SnapshotSafeConfig(vm, f.intent.Disks); !errors.Is(err, ErrSnapshotConfig) {
				t.Fatalf("unsupported VM configuration was accepted: %v", err)
			}
		})
	}
}

func TestSnapshotSafeConfigKeepsApprovedUnitsBootAndDeviceLayout(t *testing.T) {
	f := snapshotProjectionFixture(t)
	source := f.content.Spec.Source.VirtualMachine
	vm := &kv.VirtualMachine{ObjectMeta: source.ObjectMeta, Spec: source.Spec}
	domain := &vm.Spec.Template.Spec.Domain
	domain.CPU = &kv.CPU{}
	no := false
	domain.Firmware = &kv.Firmware{Bootloader: &kv.Bootloader{EFI: &kv.EFI{SecureBoot: &no}}}
	dv := ObjectRef{Kind: "DataVolume", Name: "source-dv", Namespace: f.scope.Namespace, UID: "dv-uid"}
	f.intent.Disks[0].SourceDV = &dv
	vm.Spec.Template.Spec.Volumes[0].VolumeSource = kv.VolumeSource{DataVolume: &kv.DataVolumeSource{Name: dv.Name}}
	for _, device := range []kv.DiskDevice{{Disk: &kv.DiskTarget{Bus: kv.DiskBusVirtio}}, {LUN: &kv.LunTarget{Bus: kv.DiskBusSCSI}}, {CDRom: &kv.CDRomTarget{Bus: kv.DiskBusSATA, ReadOnly: &no}}} {
		domain.Devices.Disks[0].DiskDevice = device
		config, err := SnapshotSafeConfig(vm, f.intent.Disks)
		if err != nil || config.CPUMilli != 1000 || config.MemoryMiB != 2048 || config.BootMode != "uefi" || config.DiskLayout[0].BootOrder != 1 {
			t.Fatalf("approved CPU, memory, boot or layout was lost: %v", err)
		}
	}
}

func TestSnapshotSafeConfigRequiresRepresentableFirmwareState(t *testing.T) {
	no, yes := false, true
	for _, tc := range []struct {
		name        string
		efi         *kv.EFI
		tpm         *kv.TPMDevice
		unsupported bool
		bootMode    string
	}{
		{name: "bios", bootMode: "bios"},
		{name: "uefi-secure-boot-off", efi: &kv.EFI{SecureBoot: &no}, bootMode: "uefi"},
		{name: "uefi-nonpersistent", efi: &kv.EFI{SecureBoot: &no, Persistent: &no}, bootMode: "uefi"},
		{name: "tpm-nonpersistent-default", tpm: &kv.TPMDevice{}, bootMode: "bios"},
		{name: "tpm-nonpersistent-explicit", tpm: &kv.TPMDevice{Persistent: &no}, bootMode: "bios"},
		{name: "uefi-secure-boot-default-on", efi: &kv.EFI{}, unsupported: true},
		{name: "uefi-secure-boot-explicit-on", efi: &kv.EFI{SecureBoot: &yes}, unsupported: true},
		{name: "uefi-persistent", efi: &kv.EFI{SecureBoot: &no, Persistent: &yes}, unsupported: true},
		{name: "tpm-persistent", tpm: &kv.TPMDevice{Persistent: &yes}, unsupported: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := snapshotProjectionFixture(t)
			source := f.content.Spec.Source.VirtualMachine
			vm := &kv.VirtualMachine{ObjectMeta: source.ObjectMeta, Spec: source.Spec}
			domain := &vm.Spec.Template.Spec.Domain
			if tc.efi != nil {
				domain.Firmware = &kv.Firmware{Bootloader: &kv.Bootloader{EFI: tc.efi}}
			}
			domain.Devices.TPM = tc.tpm
			config, err := SnapshotSafeConfig(vm, f.intent.Disks)
			if tc.unsupported {
				if !errors.Is(err, ErrSnapshotConfig) {
					t.Fatalf("unrepresented firmware state was accepted: config=%+v err=%v", config, err)
				}
				return
			}
			if err != nil || config.BootMode != tc.bootMode || config.CPUMilli != 2000 || config.MemoryMiB != 2048 || len(config.DiskLayout) != 1 {
				t.Fatalf("supported firmware configuration was lost: config=%+v err=%v", config, err)
			}
		})
	}
}

func TestSnapshotProjectionRejectsUnrepresentedHistoricalFirmwareState(t *testing.T) {
	for _, name := range []string{"uefi-secure-boot-default-on", "uefi-secure-boot-explicit-on", "uefi-persistent", "tpm-persistent"} {
		for _, sealed := range []bool{false, true} {
			proofName := "unsealed"
			if sealed {
				proofName = "sealed"
			}
			t.Run(name+"/"+proofName, func(t *testing.T) {
				f := snapshotProjectionFixture(t)
				originalConfigHash, originalCoreHash := f.intent.ConfigHash, f.core
				domain := &f.content.Spec.Source.VirtualMachine.Spec.Template.Spec.Domain
				no, yes := false, true
				switch name {
				case "uefi-secure-boot-default-on":
					domain.Firmware = &kv.Firmware{Bootloader: &kv.Bootloader{EFI: &kv.EFI{}}}
				case "uefi-secure-boot-explicit-on":
					domain.Firmware = &kv.Firmware{Bootloader: &kv.Bootloader{EFI: &kv.EFI{SecureBoot: &yes}}}
				case "uefi-persistent":
					domain.Firmware = &kv.Firmware{Bootloader: &kv.Bootloader{EFI: &kv.EFI{SecureBoot: &no, Persistent: &yes}}}
				case "tpm-persistent":
					domain.Devices.TPM = &kv.TPMDevice{Persistent: &yes}
				}
				proof := SnapshotProof{}
				if sealed {
					proof = SnapshotProof{Sealed: true, SnapshotUID: string(f.snapshot.UID), ContentUID: string(f.content.UID), VolumeSnapshotUIDs: map[string]string{"volume-snapshot-a": "volume-snapshot-uid"}}
				}
				// Keep the original intent and receipt hashes: ignored backend state must
				// not inherit the disk-only snapshot's readiness, even with a sealed proof.
				dto, err := f.project(proof)
				if !errors.Is(err, ErrSnapshotConfig) || dto.ReadyToUse || dto.Coverage.Complete || dto.Config != nil {
					t.Fatalf("unsupported historical state became usable: err=%v ready=%v complete=%v config=%+v", err, dto.ReadyToUse, dto.Coverage.Complete, dto.Config)
				}
				if f.intent.ConfigHash != originalConfigHash || f.core != originalCoreHash {
					t.Fatal("firmware mutation changed the original receipt fingerprint")
				}
			})
		}
	}
}

func TestSnapshotProjectionKeepsPendingAndFailureSafe(t *testing.T) {
	for _, name := range []string{"no-snapshot", "deleting", "no-status", "no-content", "native-failure", "no-volume", "no-content-status"} {
		t.Run(name, func(t *testing.T) {
			f := snapshotProjectionFixture(t)
			switch name {
			case "no-snapshot":
				f.snapshot = nil
			case "deleting":
				now := metav1.Now()
				f.snapshot.DeletionTimestamp = &now
			case "no-status":
				f.snapshot.Status = nil
			case "no-content":
				f.content = nil
			case "native-failure":
				message := "NON_SECRET_NATIVE_FAILURE_SENTINEL"
				f.snapshot.Status.Error = &native.Error{Message: &message}
			case "no-volume":
				f.volumes = nil
			case "no-content-status":
				f.content.Status = nil
			}
			dto, err := f.project(SnapshotProof{})
			if err != nil || dto.ReadyToUse {
				t.Fatalf("pending or failed native state became ready: %v", err)
			}
			encoded, err := json.Marshal(dto)
			if err != nil || strings.Contains(string(encoded), "NON_SECRET_NATIVE_FAILURE_SENTINEL") {
				t.Fatal("native failure text escaped safe DTO")
			}
			if name == "native-failure" && (dto.ErrorCode != "vm_snapshot_native_failed" || dto.Phase != "failed") {
				t.Fatal("native failure did not use fixed domain code")
			}
		})
	}
}
