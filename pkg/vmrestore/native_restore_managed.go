package vmrestore

import (
	"context"
	"errors"
	"reflect"
	"strconv"
	"time"

	"github.com/google/uuid"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	kv "kubevirt.io/api/core/v1"
	native "kubevirt.io/api/snapshot/v1beta1"
)

// ManagedNativeRestore progresses only on explicit idempotent POST commands.
// GET never reserves, repairs bindings, changes configuration or clears fences.
type ManagedNativeRestore struct {
	Store          *Store
	Resolver       *Resolver
	Snapshots      *SnapshotEngine
	Backend        NativeRestoreBackend
	Core           kubernetes.Interface
	Dynamic        dynamic.Interface
	CurrentStartup func(context.Context, Scope, *kv.VirtualMachine, string) (RestoreStartupPolicy, error)
	BuildPlatform  func(context.Context, Scope) (*kv.VirtualMachine, error)
}

func nativeExpectation(c Current, step string) Expectation {
	return Expectation{ControlRevision: c.State.ControlRevision, BindingGeneration: c.State.BindingGeneration, OperationID: c.Receipt.OperationID, ManifestHash: c.Receipt.ManifestHash, RequestHash: c.Receipt.ManifestHash, Step: step}
}

func (m *ManagedNativeRestore) available() bool {
	return m != nil && m.Store != nil && m.Resolver != nil && m.Snapshots != nil && m.Backend != nil && m.Core != nil && m.Dynamic != nil && m.CurrentStartup != nil && m.BuildPlatform != nil
}

func nativeOperation(scope Scope, request string) (string, error) {
	return nativeRestoreName(scope, request)
}

func (m *ManagedNativeRestore) read(scope Scope, name, request string) (Current, error) {
	op, err := nativeOperation(scope, request)
	if err != nil || !validName(name) {
		return Current{}, ErrSnapshotInvalid
	}
	c, err := m.Store.ReadOnly(scope, op)
	if err != nil {
		return c, err
	}
	if c.Receipt != nil && (c.Receipt.NativeRestore == nil || c.Receipt.Restore == nil || c.Receipt.Restore.SourceSnapshot.Name != name || c.Receipt.NativeRestore.RestoreName != op) {
		return c, ErrConflict
	}
	return c, nil
}

func managedNativeDTO(c Current, request string, actual *native.VirtualMachineRestore) NativeRestoreDTO {
	r := c.Receipt
	d := NativeRestoreDTO{RestoreName: r.NativeRestore.RestoreName, SnapshotName: r.Restore.SourceSnapshot.Name, RequestID: request, Phase: "pending", RequiresAdvance: true, Volumes: []NativeRestoreVolumeDTO{}}
	if r.Steps["native-storage-failed"].Applied {
		d.Phase, d.ErrorCode, d.RequiresAdvance = "failed", "vm_restore_native_failed", false
		return d
	}
	if actual != nil {
		d.Volumes = projectNativeRestore(actual).Volumes
		if status := projectNativeRestore(actual); status.Phase == "failed" {
			d.Phase = "failed"
			d.ErrorCode = status.ErrorCode
			d.RequiresAdvance = false
			return d
		}
	}
	if r.Steps["native-finalize"].Applied {
		d.Phase = "succeeded"
		d.Complete = true
		d.RequiresAdvance = false
	}
	return d
}

func (m *ManagedNativeRestore) observedRestore(ctx context.Context, scope Scope, c Current) (*kv.VirtualMachine, *native.VirtualMachineSnapshot, *native.VirtualMachineRestore, error) {
	vm, _, err := m.Backend.VirtualMachine(ctx, scope)
	if err != nil {
		return nil, nil, nil, ErrReferenceUnavailable
	}
	r := c.Receipt
	if vm == nil || !sameObject(r.Restore.CurrentVM, refOf("VirtualMachine", vm)) {
		return nil, nil, nil, ErrConflict
	}
	snap, err := m.Backend.Snapshot(ctx, scope, r.Restore.SourceSnapshot.Name)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, nil, nil, ErrNotFound
		}
		return nil, nil, nil, ErrReferenceUnavailable
	}
	if snap == nil || !sameObject(r.Restore.SourceSnapshot, refOf("VirtualMachineSnapshot", snap)) || validateNativeRestoreIdentity(scope, vm, snap) != nil {
		return nil, nil, nil, ErrConflict
	}
	expected := expectedNativeRestore(scope, vm, snap, r.NativeRestore.RestoreName, r.NativeRestore.RestoreName)
	// The native request annotation remains the public idempotency key, while
	// the private Store operation is the deterministic, scope-bound restore name.
	expected.Annotations[nativeRestoreRequestAnnotation] = r.NativeRestore.RequestID
	expected.Annotations["rainbond.io/vm-native-restore-attempt"] = r.NativeRestore.Nonce
	actual, err := m.Backend.Restore(ctx, scope, r.NativeRestore.RestoreName)
	if err != nil {
		return nil, nil, nil, ErrReferenceUnavailable
	}
	if actual != nil && (!sameNativeRestore(actual, expected) || !validUID(string(actual.UID)) || actual.Annotations["rainbond.io/vm-native-restore-attempt"] != r.NativeRestore.Nonce) {
		return nil, nil, nil, ErrConflict
	}
	for _, ref := range r.Resources {
		if ref.Purpose == "native-restore" && (actual == nil || !sameObject(ref.ObjectRef, refOf("VirtualMachineRestore", actual))) {
			return nil, nil, nil, ErrConflict
		}
	}
	return vm, snap, actual, nil
}

func (m *ManagedNativeRestore) Status(ctx context.Context, scope Scope, name, request string) (NativeRestoreDTO, error) {
	if !m.available() {
		return NativeRestoreDTO{}, ErrRestoreCapability
	}
	c, err := m.read(scope, name, request)
	if err != nil {
		return NativeRestoreDTO{}, err
	}
	if c.Receipt == nil {
		return NativeRestoreDTO{}, ErrNotFound
	}
	_, _, actual, err := m.observedRestore(ctx, scope, c)
	if err != nil {
		return NativeRestoreDTO{}, err
	}
	return managedNativeDTO(c, request, actual), nil
}

// nativeConfig uses the platform's sealed logical capacity and validates the
// native PVC's exact byte request can contain it; CDI filesystem overhead need
// not be a whole GiB. No storage-copy qualification is invented here.
func nativeConfig(scope Scope, dto SnapshotOperationDTO, content *native.VirtualMachineSnapshotContent, before RestoreConfig) (RestoreConfig, error) {
	if dto.Config == nil || !dto.Coverage.Complete || content == nil || string(content.UID) != dto.ContentUID {
		return RestoreConfig{}, ErrRestoreCapability
	}
	result := RestoreConfig{Hardware: *dto.Config, Volumes: []RestoreVolumeConfig{}}
	for _, layout := range result.Hardware.DiskLayout {
		var config *RestoreVolumeConfig
		for i := range before.Volumes {
			if before.Volumes[i].VolumeID == layout.VolumeID {
				copy := before.Volumes[i]
				config = &copy
			}
		}
		if config == nil || config.DiskKey != layout.DiskKey || config.ReadOnly != layout.ReadOnly {
			return result, ErrConflict
		}
		if layout.ReadOnly {
			result.Volumes = append(result.Volumes, *config)
			continue
		}
		var pvc *native.PersistentVolumeClaim
		for i := range content.Spec.VolumeBackups {
			b := &content.Spec.VolumeBackups[i]
			if b.VolumeName == layout.CanonicalName {
				if pvc != nil {
					return result, ErrConflict
				}
				pvc = &b.PersistentVolumeClaim
			}
		}
		if pvc == nil || pvc.Spec.StorageClassName == nil || *pvc.Spec.StorageClassName != config.StorageClass || len(pvc.Spec.AccessModes) != 1 || string(pvc.Spec.AccessModes[0]) != config.AccessMode || pvc.Spec.Resources.Requests.Storage().Value() < config.CapacityGiB<<30 {
			return result, ErrRestoreCapability
		}
		matched := false
		for _, d := range dto.Disks {
			if d.VolumeID == layout.VolumeID && d.SourcePVC.Name == pvc.Name && d.SourcePVC.UID == string(pvc.UID) {
				matched = true
			}
		}
		if !matched {
			return result, ErrConflict
		}
		result.Volumes = append(result.Volumes, *config)
	}
	return result, result.validate(scope)
}

func (m *ManagedNativeRestore) prepare(ctx context.Context, scope Scope, name, request string, c Current) (Current, error) {
	dto, err := m.Snapshots.ByName(ctx, scope, name)
	if err != nil {
		return c, err
	}
	if !dto.ReadyToUse || !dto.ManifestACKed || !dto.Finalized || dto.Cancelled || dto.Deleted || !dto.Coverage.Complete {
		return c, ErrRestoreCapability
	}
	source, err := m.Store.ReadOnly(scope, dto.OperationID)
	if err != nil {
		return c, err
	}
	if source.Receipt == nil || source.Receipt.SnapshotIdentity == nil || !source.Receipt.SnapshotIdentity.Pinned || source.Receipt.SnapshotIdentity.Startup == nil || source.Receipt.Snapshot == nil {
		return c, ErrRestoreCapability
	}
	observation, err := m.Snapshots.backend.Observe(ctx, scope, *source.Receipt.Snapshot, source.Receipt.Resources)
	if err != nil {
		return c, err
	}
	fresh, err := m.Snapshots.project(source, observation)
	if err != nil || !fresh.ReadyToUse || fresh.NativeProofHash != dto.NativeProofHash {
		return c, ErrConflict
	}
	vm, vmi, err := m.Backend.VirtualMachine(ctx, scope)
	if err != nil {
		return c, ErrReferenceUnavailable
	}
	if err = validateNativeRestoreSource(scope, vm, vmi, observation.Snapshot, ""); err != nil {
		return c, err
	}
	if err = m.checkAdvanced(ctx, scope, nativeRestoreSourceVM(observation.Content.Spec.Source.VirtualMachine), vm); err != nil {
		return c, err
	}
	view, err := m.Resolver.ReadBindingView(ctx, scope)
	if err != nil {
		return c, err
	}
	if view.Fence != nil || view.MutationInProgress {
		return c, ErrBusy
	}
	old := map[string]Binding{}
	for _, v := range view.Volumes {
		b, err := v.MaterializedBinding()
		if err != nil {
			return c, err
		}
		old[strconv.FormatUint(uint64(v.VolumeID), 10)] = b
	}
	// Native restore cannot promise retention of B-only disks or safely detach
	// them through KubeVirt's DeletedDataVolumes behavior. Require the same
	// attached logical set before issuing any restore mutation.
	selected := map[uint]bool{}
	for _, d := range source.Receipt.Snapshot.Disks {
		selected[d.VolumeID] = true
		b, ok := old[strconv.FormatUint(uint64(d.VolumeID), 10)]
		if !ok || !b.Attached || b.DiskKey != d.DiskKey {
			return c, ErrRestoreCapability
		}
		if d.ReadOnly && (!sameObject(b.ActivePVC, d.SourcePVC) || !reflect.DeepEqual(b.ActiveDV, d.SourceDV)) {
			return c, ErrRestoreCapability
		}
	}
	for _, v := range view.Volumes {
		if v.Attached && !selected[v.VolumeID] {
			return c, ErrRestoreCapability
		}
	}
	object, err := EncodeRuntimeVM(vm)
	if err != nil {
		return c, err
	}
	if err = runtimeUsesBindingView(object.Object, false, view); err != nil {
		return c, err
	}
	before, err := m.Store.selectedRuntimeConfig(scope)
	if err != nil {
		return c, err
	}
	after, err := nativeConfig(scope, fresh, observation.Content, before)
	if err != nil {
		return c, err
	}
	beforeHash, _ := RestoreConfigHash(before)
	afterHash, _ := RestoreConfigHash(after)
	if beforeHash != afterHash {
		return c, ErrNativeRestoreConfigurationChanged
	}
	firmware := ""
	if vm.Spec.Template.Spec.Domain.Firmware != nil {
		firmware = string(vm.Spec.Template.Spec.Domain.Firmware.UUID)
	}
	beforeStartup, err := m.CurrentStartup(ctx, scope, vm, firmware)
	if err != nil {
		return c, err
	}
	startup := *source.Receipt.SnapshotIdentity.Startup
	startup.BootstrapMode = "restored"
	startup.GeneratedMedia = []RestoreGeneratedMedia{}
	op, _ := nativeOperation(scope, request)
	hash, _ := RestoreConfigHash(after)
	intent := RestoreIntent{CurrentVM: refOf("VirtualMachine", vm), SourceSnapshot: refOf("VirtualMachineSnapshot", observation.Snapshot), SourceContent: refOf("VirtualMachineSnapshotContent", observation.Content), SourceVMUID: dto.SourceVMUID, SourceCoreHash: dto.CoreHash, SourceDocumentHash: source.Receipt.ConsoleAckHash, SourceGeneration: view.BindingGeneration, PowerAfter: "stopped", Before: before, ConfigAfterHash: hash}
	control := NativeRestoreControl{Config: after, BeforeStartup: beforeStartup, SourceStartup: startup, SnapshotOperationID: dto.OperationID, NativeProofHash: dto.NativeProofHash, RestoreName: op, RequestID: request, Nonce: uuid.NewString()}
	manifest, _ := snapshotDigest(struct {
		Intent  RestoreIntent
		Control NativeRestoreControl
	}{intent, control})
	if err = m.writersClear(ctx, scope, old); err != nil {
		return c, err
	}
	return m.Store.PrepareNativeRestoreDB(scope, Expectation{ControlRevision: view.ControlRevision, BindingGeneration: view.BindingGeneration, OperationID: op, ManifestHash: manifest}, intent, control, old)
}

func (m *ManagedNativeRestore) writersClear(ctx context.Context, scope Scope, bindings map[string]Binding) error {
	claims := map[string]bool{}
	for _, b := range bindings {
		if b.Attached {
			claims[b.ActivePVC.Name] = true
		}
	}
	pods, err := m.Core.CoreV1().Pods(scope.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return ErrReferenceUnavailable
	}
	for _, pod := range pods.Items {
		if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
			continue
		}
		for _, v := range pod.Spec.Volumes {
			if v.PersistentVolumeClaim != nil && claims[v.PersistentVolumeClaim.ClaimName] {
				return ErrBusy
			}
		}
	}
	return nil
}

func (m *ManagedNativeRestore) Advance(ctx context.Context, scope Scope, name, request string) (NativeRestoreDTO, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if !m.available() {
		return NativeRestoreDTO{}, ErrRestoreCapability
	}
	c, err := m.read(scope, name, request)
	if err != nil {
		return NativeRestoreDTO{}, err
	}
	if c.Receipt == nil {
		c, err = m.prepare(ctx, scope, name, request, c)
		if err != nil {
			return NativeRestoreDTO{}, err
		}
	}
	vm, snap, actual, err := m.observedRestore(ctx, scope, c)
	if err != nil {
		return NativeRestoreDTO{}, err
	}
	if c.Receipt.Steps["native-finalize"].Applied || c.Receipt.Steps["native-storage-failed"].Applied {
		return managedNativeDTO(c, request, actual), nil
	}
	if c.State.Fence == nil || c.State.Fence.OperationID != c.Receipt.OperationID || c.State.Fence.Kind != "restore" {
		return NativeRestoreDTO{}, ErrConflict
	}
	_, vmi, err := m.Backend.VirtualMachine(ctx, scope)
	if err != nil {
		return NativeRestoreDTO{}, ErrReferenceUnavailable
	}
	if err = validateNativeRestoreSource(scope, vm, vmi, snap, c.Receipt.NativeRestore.RestoreName); err != nil {
		return NativeRestoreDTO{}, err
	}
	source, err := m.Store.ReadOnly(scope, c.Receipt.NativeRestore.SnapshotOperationID)
	if err != nil {
		return NativeRestoreDTO{}, err
	}
	if source.Receipt == nil || source.Receipt.Snapshot == nil {
		return NativeRestoreDTO{}, ErrConflict
	}
	sourceObservation, err := m.Snapshots.backend.Observe(ctx, scope, *source.Receipt.Snapshot, source.Receipt.Resources)
	if err != nil {
		return NativeRestoreDTO{}, err
	}
	if sourceObservation.Content == nil || sourceObservation.Content.Spec.Source.VirtualMachine == nil || !sameObject(c.Receipt.Restore.SourceContent, refOf("VirtualMachineSnapshotContent", sourceObservation.Content)) {
		return NativeRestoreDTO{}, ErrConflict
	}
	if err = m.checkAdvanced(ctx, scope, nativeRestoreSourceVM(sourceObservation.Content.Spec.Source.VirtualMachine), vm); err != nil {
		return NativeRestoreDTO{}, err
	}
	if actual == nil {
		if c.Receipt.Steps["native-create-claimed"].Applied {
			return managedNativeDTO(c, request, nil), nil
		}
		if err = m.writersClear(ctx, scope, c.Receipt.OldBindings); err != nil {
			return NativeRestoreDTO{}, err
		}
		preserved, changed, err := m.preserveNativeSources(ctx, scope, c, vm)
		if err != nil {
			return NativeRestoreDTO{}, err
		}
		if changed {
			if err = m.updateVM(ctx, scope, c, vm, preserved); err != nil {
				return NativeRestoreDTO{}, err
			}
			return managedNativeDTO(c, request, nil), nil
		}
		e := nativeExpectation(c, "native-create-claimed")
		c, err = m.Store.Mutate(scope, e, func(now Current) (Mutation, error) {
			if nativeOwner(now, scope, e) != nil {
				return Mutation{}, ErrConflict
			}
			return Mutation{State: now.State, Receipt: now.Receipt}, nil
		})
		if err != nil {
			return NativeRestoreDTO{}, err
		}
		if c.Replayed {
			return managedNativeDTO(c, request, nil), nil
		}
		expected := expectedNativeRestore(scope, vm, snap, request, c.Receipt.NativeRestore.RestoreName)
		expected.Annotations["rainbond.io/vm-native-restore-attempt"] = c.Receipt.NativeRestore.Nonce
		actual, err = m.Backend.CreateRestore(ctx, scope, expected)
		if err != nil {
			return managedNativeDTO(c, request, nil), nil
		}
		if !sameNativeRestore(actual, expected) || !validUID(string(actual.UID)) || actual.Annotations["rainbond.io/vm-native-restore-attempt"] != c.Receipt.NativeRestore.Nonce {
			return NativeRestoreDTO{}, ErrConflict
		}
	}
	if !c.Receipt.Steps["native-created"].Applied {
		e := nativeExpectation(c, "native-created")
		c, err = m.Store.Mutate(scope, e, func(now Current) (Mutation, error) {
			if nativeOwner(now, scope, e) != nil {
				return Mutation{}, ErrConflict
			}
			now.Receipt.Resources = resourceOnce(now.Receipt.Resources, ResourceRef{ObjectRef: refOf("VirtualMachineRestore", actual), Purpose: "native-restore"})
			return Mutation{State: now.State, Receipt: now.Receipt}, nil
		})
		if err != nil {
			return NativeRestoreDTO{}, err
		}
	}
	if projectNativeRestore(actual).Phase == "failed" {
		return managedNativeDTO(c, request, actual), nil
	}
	if actual.Status == nil || actual.Status.Complete == nil || !*actual.Status.Complete {
		return managedNativeDTO(c, request, actual), nil
	}
	if vm.Status.RestoreInProgress != nil || !sourceUnlocked(vm) {
		return managedNativeDTO(c, request, actual), nil
	}
	if !c.Receipt.Steps[NativeRestoreCommitStep].Applied {
		bindings, err := m.restoredBindings(ctx, scope, c, vm, actual)
		if errors.Is(err, ErrSnapshotPending) {
			return managedNativeDTO(c, request, actual), nil
		}
		if errors.Is(err, errNativeCloneFailed) {
			c, err = m.Store.Mutate(scope, nativeExpectation(c, "native-storage-failed"), func(now Current) (Mutation, error) {
				if nativeOwner(now, scope, nativeExpectation(c, "native-storage-failed")) != nil || now.Receipt.Steps[NativeRestoreCommitStep].Applied {
					return Mutation{}, ErrConflict
				}
				now.State.Fence.Phase = "native-storage-failed"
				return Mutation{State: now.State, Receipt: now.Receipt}, nil
			})
			if err != nil {
				return NativeRestoreDTO{}, err
			}
			return managedNativeDTO(c, request, actual), nil
		}
		if err != nil {
			return NativeRestoreDTO{}, err
		}
		if err = m.writersClear(ctx, scope, bindings); err != nil {
			return NativeRestoreDTO{}, err
		}
		selection := RuntimeSelection{Scope: scope, BindingGeneration: c.State.BindingGeneration, Bindings: bindings, Config: c.Receipt.NativeRestore.Config, Startup: c.Receipt.NativeRestore.SourceStartup, BeforeStartup: c.Receipt.NativeRestore.BeforeStartup}
		projected, err := ProjectRestoredRuntime(ctx, m.Core, vm, selection)
		if err != nil {
			return NativeRestoreDTO{}, err
		}
		projected.Spec.Running = nil
		strategy := kv.RunStrategyHalted
		projected.Spec.RunStrategy = &strategy
		if !RuntimeVMMatches(vm, projected) {
			if err = m.updateVM(ctx, scope, c, vm, projected); err != nil {
				if errors.Is(err, ErrRestoreCapability) {
					return NativeRestoreDTO{}, err
				}
				return managedNativeDTO(c, request, actual), nil
			}
			return managedNativeDTO(c, request, actual), nil
		}
		if !c.Receipt.Steps["native-observed"].Applied {
			c, err = m.Store.SealNativeRestoreObservation(scope, nativeExpectation(c, "native-observed"), refOf("VirtualMachineRestore", actual), bindings)
			if err != nil {
				return NativeRestoreDTO{}, err
			}
		}
		c, err = m.Store.CommitNativeRestoreDB(scope, nativeExpectation(c, NativeRestoreCommitStep), RestoreCommitRequest{Config: c.Receipt.NativeRestore.Config, Bindings: bindings})
		if err != nil {
			return NativeRestoreDTO{}, err
		}
	}
	// A new live Resolver read proves normal platform renderers will select the
	// adopted PVC set. Native Complete alone never reaches the success state.
	view, err := m.Resolver.ReadBindingView(ctx, scope)
	if err != nil {
		return NativeRestoreDTO{}, err
	}
	object, err := EncodeRuntimeVM(vm)
	if err != nil {
		return NativeRestoreDTO{}, err
	}
	if err = runtimeUsesBindingView(object.Object, false, view); err != nil {
		return NativeRestoreDTO{}, err
	}
	if err = m.writersClear(ctx, scope, c.State.Bindings); err != nil {
		return NativeRestoreDTO{}, err
	}
	selected, err := m.Store.ReadRestoredRuntimeSelection(ctx, m.Resolver, scope)
	if err != nil {
		return NativeRestoreDTO{}, err
	}
	if selected == nil {
		return NativeRestoreDTO{}, ErrCorrupt
	}
	// The committed generation is stamped only after the atomic binding commit.
	// A timeout here retains the fence; the same POST re-reads and finishes it.
	vm, vmi, err = m.Backend.VirtualMachine(ctx, scope)
	if err != nil {
		return NativeRestoreDTO{}, ErrReferenceUnavailable
	}
	if err = validateNativeRestoreSource(scope, vm, vmi, snap, c.Receipt.NativeRestore.RestoreName); err != nil {
		return NativeRestoreDTO{}, err
	}
	if !sameObject(c.Receipt.Restore.CurrentVM, refOf("VirtualMachine", vm)) {
		return NativeRestoreDTO{}, ErrConflict
	}
	projected, err := ProjectRestoredRuntime(ctx, m.Core, vm, *selected)
	if err != nil {
		return NativeRestoreDTO{}, err
	}
	generation, err := markerGeneration(vm)
	if err != nil {
		return NativeRestoreDTO{}, err
	}
	if generation != selected.BindingGeneration || !RuntimeVMMatches(vm, projected) {
		if err = m.updateVM(ctx, scope, c, vm, projected); err != nil {
			if errors.Is(err, ErrRestoreCapability) {
				return NativeRestoreDTO{}, err
			}
			return managedNativeDTO(c, request, actual), nil
		}
		return managedNativeDTO(c, request, actual), nil
	}
	c, err = m.Store.FinalizeNativeRestoreDB(scope, nativeExpectation(c, "native-finalize"))
	if err != nil {
		return NativeRestoreDTO{}, err
	}
	return managedNativeDTO(c, request, actual), nil
}

// updateVM keeps real raw fields outside the exact owned differences, refuses
// unrepresentable replaced subtrees, and stamps only our own binding marker.
func (m *ManagedNativeRestore) updateVM(ctx context.Context, scope Scope, c Current, before, after *kv.VirtualMachine) error {
	raw, err := m.Dynamic.Resource(restoreVMGVR).Namespace(scope.Namespace).Get(ctx, before.Name, metav1.GetOptions{})
	if err != nil {
		return ErrReferenceUnavailable
	}
	updated, err := PreserveRuntimeVM(raw, before, after)
	if err != nil {
		return err
	}
	if err = MarkVMGeneration(updated, scope, c.State.BindingGeneration); err != nil {
		return err
	}
	current, err := m.Store.ReadOnly(scope, c.Receipt.OperationID)
	if err != nil {
		return err
	}
	if current.State.ControlRevision != c.State.ControlRevision || current.State.BindingGeneration != c.State.BindingGeneration || current.State.Fence == nil || current.State.Fence.OperationID != c.Receipt.OperationID || current.State.Fence.ManifestHash != c.Receipt.ManifestHash {
		return ErrConflict
	}
	_, err = m.Dynamic.Resource(restoreVMGVR).Namespace(scope.Namespace).Update(ctx, updated, metav1.UpdateOptions{})
	return err
}

func (m *ManagedNativeRestore) restoredBindings(ctx context.Context, scope Scope, c Current, vm *kv.VirtualMachine, actual *native.VirtualMachineRestore) (map[string]Binding, error) {
	source, err := m.Store.ReadOnly(scope, c.Receipt.NativeRestore.SnapshotOperationID)
	if err != nil {
		return nil, err
	}
	observed, err := m.Snapshots.backend.Observe(ctx, scope, *source.Receipt.Snapshot, source.Receipt.Resources)
	if err != nil {
		return nil, err
	}
	dto, err := m.Snapshots.project(source, observed)
	if err != nil || !dto.ReadyToUse || dto.NativeProofHash != c.Receipt.NativeRestore.NativeProofHash {
		return nil, ErrConflict
	}
	targets := map[uint]ObjectRef{}
	dvs := map[uint]ObjectRef{}
	seen := map[string]bool{}
	for _, layout := range c.Receipt.NativeRestore.Config.Hardware.DiskLayout {
		if layout.ReadOnly {
			b, ok := c.Receipt.OldBindings[strconv.FormatUint(uint64(layout.VolumeID), 10)]
			if !ok || !b.Attached {
				return nil, ErrConflict
			}
			pvc, err := m.Core.CoreV1().PersistentVolumeClaims(scope.Namespace).Get(ctx, b.ActivePVC.Name, metav1.GetOptions{})
			if err != nil {
				return nil, ErrReferenceUnavailable
			}
			if pvc.DeletionTimestamp != nil || pvc.Status.Phase != corev1.ClaimBound || !sameObject(b.ActivePVC, refOf("PersistentVolumeClaim", pvc)) {
				return nil, ErrConflict
			}
			matches := 0
			for _, v := range vm.Spec.Template.Spec.Volumes {
				if v.Name == layout.CanonicalName {
					if v.PersistentVolumeClaim != nil && v.PersistentVolumeClaim.ClaimName == b.ActivePVC.Name && v.PersistentVolumeClaim.ReadOnly {
						matches++
					}
					if v.DataVolume != nil && b.ActiveDV != nil && v.DataVolume.Name == b.ActiveDV.Name && nativeReadOnlyDevice(vm, v.Name) {
						dv, err := m.Resolver.dv(ctx, scope, b.ActiveDV.Name, b.ActiveDV)
						if err != nil || dv == nil {
							return nil, ErrConflict
						}
						matches++
					}
				}
			}
			if matches != 1 {
				return nil, ErrConflict
			}
			continue
		}
		var row *native.VolumeRestore
		for i := range actual.Status.Restores {
			v := &actual.Status.Restores[i]
			if v.VolumeName == layout.CanonicalName {
				if row != nil {
					return nil, ErrConflict
				}
				row = v
			}
		}
		if row == nil || seen[row.PersistentVolumeClaimName] {
			return nil, ErrConflict
		}
		seen[row.PersistentVolumeClaimName] = true
		var backup *SnapshotBackup
		for i := range dto.Disks {
			if dto.Disks[i].VolumeID == layout.VolumeID {
				backup = &dto.Disks[i]
			}
		}
		if backup == nil || row.VolumeSnapshotName != backup.VolumeSnapshot.Name {
			return nil, ErrConflict
		}
		pvc, err := m.Core.CoreV1().PersistentVolumeClaims(scope.Namespace).Get(ctx, row.PersistentVolumeClaimName, metav1.GetOptions{})
		if err != nil {
			return nil, ErrReferenceUnavailable
		}
		if !validUID(string(pvc.UID)) || pvc.DeletionTimestamp != nil || pvc.Status.Phase != corev1.ClaimBound || pvc.Spec.StorageClassName == nil || *pvc.Spec.StorageClassName != backup.StorageClass || (pvc.Spec.VolumeMode != nil && *pvc.Spec.VolumeMode != corev1.PersistentVolumeFilesystem) {
			return nil, ErrConflict
		}
		if tenant := pvc.Labels["tenant_id"]; tenant != "" && tenant != scope.TenantID {
			return nil, ErrConflict
		}
		if service := pvc.Labels["service_id"]; service != "" && service != scope.ServiceID {
			return nil, ErrConflict
		}
		if pvc.Spec.DataSource == nil || pvc.Spec.DataSource.APIGroup == nil || *pvc.Spec.DataSource.APIGroup != "snapshot.storage.k8s.io" || pvc.Spec.DataSource.Kind != "VolumeSnapshot" || pvc.Spec.DataSource.Name != row.VolumeSnapshotName {
			return nil, ErrConflict
		}
		if pvc.Spec.Resources.Requests.Storage().Value() < backup.RequestedBytes {
			return nil, ErrConflict
		}
		for _, v := range c.Receipt.NativeRestore.Config.Volumes {
			if v.VolumeID == layout.VolumeID && (pvc.Spec.Resources.Requests.Storage().Value() < v.CapacityGiB<<30 || len(pvc.Spec.AccessModes) != 1 || string(pvc.Spec.AccessModes[0]) != v.AccessMode) {
				return nil, ErrConflict
			}
		}
		pv, err := m.Core.CoreV1().PersistentVolumes().Get(ctx, pvc.Spec.VolumeName, metav1.GetOptions{})
		if err != nil {
			return nil, ErrReferenceUnavailable
		}
		class, err := m.Core.StorageV1().StorageClasses().Get(ctx, *pvc.Spec.StorageClassName, metav1.GetOptions{})
		if err != nil {
			return nil, ErrReferenceUnavailable
		}
		if pv.DeletionTimestamp != nil || pv.Spec.CSI == nil || pv.Spec.CSI.Driver != class.Provisioner || pv.Spec.ClaimRef == nil || pv.Spec.ClaimRef.UID != pvc.UID || pv.Spec.ClaimRef.Name != pvc.Name || pv.Spec.ClaimRef.Namespace != scope.Namespace {
			return nil, ErrConflict
		}
		if err := m.nativeCloneReady(ctx, pv, observed, *backup); err != nil {
			return nil, err
		}
		mounted := false
		var mountedDV string
		for _, v := range vm.Spec.Template.Spec.Volumes {
			if v.Name == layout.CanonicalName {
				if v.PersistentVolumeClaim != nil && v.PersistentVolumeClaim.ClaimName == pvc.Name {
					mounted = true
				}
				if v.DataVolume != nil {
					mountedDV = v.DataVolume.Name
				}
			}
		}
		var dvName string
		for _, owner := range pvc.OwnerReferences {
			if owner.Kind == "DataVolume" {
				if dvName != "" {
					return nil, ErrConflict
				}
				dvName = owner.Name
				dv, err := m.Dynamic.Resource(dataVolumeResource).Namespace(scope.Namespace).Get(ctx, owner.Name, metav1.GetOptions{})
				if err != nil {
					return nil, ErrReferenceUnavailable
				}
				claim, _, _ := unstructured.NestedString(dv.Object, "status", "claimName")
				if dv.GetUID() != owner.UID || claim != pvc.Name || dv.GetDeletionTimestamp() != nil || dv.GetLabels()["service_id"] != scope.ServiceID || dv.GetLabels()["tenant_id"] != "" && dv.GetLabels()["tenant_id"] != scope.TenantID {
					return nil, ErrConflict
				}
				dvs[layout.VolumeID] = refOf("DataVolume", dv)
			}
		}
		if mountedDV != "" && mountedDV == dvName {
			mounted = true
		}
		if !mounted || !pvcOwnedBy(scope, pvc, nil) && dvName == "" {
			return nil, ErrConflict
		}
		targets[layout.VolumeID] = refOf("PersistentVolumeClaim", pvc)
	}
	if len(targets) != len(actual.Status.Restores) {
		return nil, ErrConflict
	}
	bindings, err := BuildRestoreBindings(scope, c.Receipt.OperationID, c.Receipt.Restore.SourceSnapshot.UID, c.Receipt.OldBindings, c.Receipt.NativeRestore.Config, targets)
	if err != nil {
		return nil, err
	}
	for id, dv := range dvs {
		key := strconv.FormatUint(uint64(id), 10)
		b := bindings[key]
		copy := dv
		b.ActiveDV = &copy
		bindings[key] = b
	}
	return bindings, nil
}

func nativeReadOnlyDevice(vm *kv.VirtualMachine, name string) bool {
	count := 0
	readOnly := false
	for _, d := range vm.Spec.Template.Spec.Domain.Devices.Disks {
		if d.Name == name {
			count++
			readOnly = d.Disk != nil && d.Disk.ReadOnly || d.LUN != nil && d.LUN.ReadOnly || d.CDRom != nil && (d.CDRom.ReadOnly == nil || *d.CDRom.ReadOnly)
		}
	}
	return count == 1 && readOnly
}
