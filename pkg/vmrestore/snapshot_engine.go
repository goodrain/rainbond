package vmrestore

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"time"

	"github.com/google/uuid"
	csi "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	native "kubevirt.io/api/snapshot/v1beta1"
)

// SnapshotContext is a safe current-resource preflight, not a stored manifest.
type SnapshotContext struct {
	SourceVM   ObjectRef
	WasRunning bool
	Stopped    bool
	Config     SnapshotConfig
	Disks      []SnapshotDisk
}

// SnapshotObservation contains backend reads only, never a public raw response.
type SnapshotObservation struct {
	Snapshot        *native.VirtualMachineSnapshot
	Content         *native.VirtualMachineSnapshotContent
	Volumes         map[string]*csi.VolumeSnapshot
	Contents        map[string]*csi.VolumeSnapshotContent
	Resources       []ResourceRef
	SourceStopped   bool
	SourceRunning   bool
	SourceLockClear bool
	CleanupComplete bool
}

// SnapshotBackend performs one bounded action. Context/Observe are read-only.
// Delete only receives server-observed identities and must use UID preconditions.
type SnapshotBackend interface {
	Context(context.Context, Scope) (SnapshotContext, error)
	Exists(context.Context, Scope, string) (bool, error)
	Observe(context.Context, Scope, SnapshotIntent, []ResourceRef) (SnapshotObservation, error)
	Halt(context.Context, Scope, SnapshotIntent) error
	Create(context.Context, Scope, SnapshotIntent, string, string) (ObjectRef, error)
	Resume(context.Context, Scope, SnapshotIntent) error
	Delete(context.Context, Scope, []ResourceRef) error
	DeleteReferences(context.Context, Scope, []ResourceRef) error
}

// SnapshotEngine applies bounded Region commands; Console owns the total workflow.
type SnapshotEngine struct {
	store    *Store
	backend  SnapshotBackend
	Identity *SnapshotFirmwareMaterializer
}

// NewSnapshotEngine uses the same Region store as the shared volume resolver.
func NewSnapshotEngine(store *Store, backend SnapshotBackend) *SnapshotEngine {
	return &SnapshotEngine{store: store, backend: backend}
}

// SnapshotOperationDTO adds observed power flags without exposing admission tokens.
type SnapshotOperationDTO struct {
	ManifestID               string      `json:"manifest_id,omitempty"`
	ManifestDocumentHash     string      `json:"manifest_document_hash,omitempty"`
	FinalizedControlRevision uint64      `json:"finalized_control_revision,omitempty"`
	FenceOwned               *bool       `json:"fence_owned,omitempty"`
	FenceCleared             *bool       `json:"fence_cleared,omitempty"`
	NativeProofHash          string      `json:"native_proof_hash,omitempty"`
	SnapshotResources        []ObjectRef `json:"snapshot_resources"`
	EventID                  string      `json:"event_id"`
	Deleted                  bool        `json:"deleted"`
	SnapshotDTO
	WasRunning      bool   `json:"was_running"`
	PowerAfter      string `json:"power_after"`
	SourceStopped   bool   `json:"source_stopped"`
	SourceRunning   bool   `json:"source_running"`
	CleanupComplete bool   `json:"cleanup_complete"`
	ManifestACKed   bool   `json:"manifest_acked"`
	Finalized       bool   `json:"finalized"`
	Cancelled       bool   `json:"cancelled"`
}

func resourceOnce(resources []ResourceRef, next ResourceRef) []ResourceRef {
	for _, ref := range resources {
		if sameObject(ref.ObjectRef, next.ObjectRef) && ref.Purpose == next.Purpose {
			return resources
		}
	}
	return append(resources, next)
}
func engineRequestHash(action string, request SnapshotActionRequest) (string, error) {
	return snapshotDigest(struct {
		Action  string                `json:"action"`
		Request SnapshotActionRequest `json:"request"`
	}{action, request})
}
func requireSnapshotOwner(current Current, operationID, coreHash string) error {
	if current.Receipt == nil || current.Receipt.Snapshot == nil || current.Receipt.OperationID != operationID || current.Receipt.ManifestHash != coreHash {
		return ErrConflict
	}
	if current.State.Fence == nil || current.State.Fence.Kind != "snapshot" || current.State.Fence.OperationID != operationID || current.State.Fence.ManifestHash != coreHash {
		return ErrBusy
	}
	return nil
}
func (e *SnapshotEngine) mutate(scope Scope, operationID, step, hash string, change func(Current) (Mutation, error)) (Current, error) {
	for attempts := 0; attempts < 3; attempts++ {
		current, err := e.store.Read(scope, operationID)
		if err != nil {
			return Current{}, err
		}
		if current.Receipt == nil {
			return Current{}, ErrConflict
		}
		expected := Expectation{ControlRevision: current.State.ControlRevision, BindingGeneration: current.State.BindingGeneration, OperationID: operationID, ManifestHash: current.Receipt.ManifestHash, Step: step, RequestHash: hash}
		result, err := e.store.Mutate(scope, expected, change)
		if !errors.Is(err, ErrConflict) {
			return result, err
		}
	}
	return Current{}, ErrConflict
}
func (e *SnapshotEngine) receipt(scope Scope, operationID string) (Current, error) {
	current, err := e.store.ReadOnly(scope, operationID)
	if err != nil {
		return current, err
	}
	if current.Receipt == nil || current.Receipt.Snapshot == nil {
		return current, ErrNotFound
	}
	return current, nil
}
func proofFor(receipt *Receipt) SnapshotProof {
	proof := SnapshotProof{Sealed: receipt.ConsoleAckHash != "", VolumeSnapshotUIDs: map[string]string{}}
	for _, ref := range receipt.Resources {
		switch ref.Kind {
		case "VirtualMachineSnapshot":
			proof.SnapshotUID = ref.UID
		case "VirtualMachineSnapshotContent":
			proof.ContentUID = ref.UID
		case "VolumeSnapshot":
			proof.VolumeSnapshotUIDs[ref.Name] = ref.UID
		}
	}
	return proof
}

func nativeSnapshotProof(dto SnapshotDTO, observed SnapshotObservation) (string, error) {
	if !dto.ReadyToUse || observed.Snapshot == nil || observed.Content == nil {
		return "", ErrSnapshotPending
	}
	type diskProof struct {
		Disk    SnapshotBackup `json:"disk"`
		Content ObjectRef      `json:"content"`
	}
	disks := []diskProof{}
	for _, disk := range dto.Disks {
		volume := observed.Volumes[disk.VolumeSnapshot.Name]
		if volume == nil || volume.Status == nil || volume.Status.BoundVolumeSnapshotContentName == nil {
			return "", ErrSnapshotPending
		}
		content := observed.Contents[*volume.Status.BoundVolumeSnapshotContentName]
		if content == nil || content.Status == nil || content.Status.ReadyToUse == nil || !*content.Status.ReadyToUse {
			return "", ErrSnapshotPending
		}
		ref := content.Spec.VolumeSnapshotRef
		if content.Name != *volume.Status.BoundVolumeSnapshotContentName || !validUID(string(content.UID)) || content.DeletionTimestamp != nil || content.Status.Error != nil || ref.APIVersion != csi.SchemeGroupVersion.String() || ref.Kind != "VolumeSnapshot" || ref.Namespace != volume.Namespace || ref.Name != volume.Name || ref.UID != volume.UID {
			return "", ErrConflict
		}
		disks = append(disks, diskProof{disk, refOf("VolumeSnapshotContent", content)})
	}
	sort.Slice(disks, func(i, j int) bool { return disks[i].Disk.VolumeID < disks[j].Disk.VolumeID })
	resources := []ObjectRef{}
	seen := map[string]bool{}
	for _, resource := range observed.Resources {
		ref := resource.ObjectRef
		key := ref.Kind + "/" + ref.Namespace + "/" + ref.Name
		if seen[key] || !validUID(ref.UID) {
			return "", ErrConflict
		}
		seen[key] = true
		resources = append(resources, ref)
	}
	sort.Slice(resources, func(i, j int) bool {
		a, b := resources[i], resources[j]
		return a.Kind+"/"+a.Namespace+"/"+a.Name < b.Kind+"/"+b.Namespace+"/"+b.Name
	})
	return snapshotDigest(struct {
		Snapshot  ObjectRef   `json:"snapshot"`
		Content   ObjectRef   `json:"content"`
		Resources []ObjectRef `json:"resources"`
		Disks     []diskProof `json:"disks"`
	}{refOf("VirtualMachineSnapshot", observed.Snapshot), refOf("VirtualMachineSnapshotContent", observed.Content), resources, disks})
}

// Prepare accepts Running and only reserves an operation. The worker requests
// graceful shutdown next; user-facing flows never require manual pre-shutdown.
func (e *SnapshotEngine) Prepare(ctx context.Context, scope Scope, request SnapshotCreateRequest) (SnapshotOperationDTO, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if request.RequestID == "" {
		request.RequestID = uuid.NewString()
	}
	if request.ManifestID == "" {
		request.ManifestID = request.RequestID
	}
	if request.PowerAfter == "" {
		request.PowerAfter = "previous"
	}
	if len(request.Operator) > 40 || !idPattern.MatchString(request.RequestID) || !idPattern.MatchString(request.ManifestID) {
		return SnapshotOperationDTO{}, ErrSnapshotInvalid
	}
	current, err := e.store.Read(scope, request.RequestID)
	if err != nil {
		return SnapshotOperationDTO{}, err
	}
	if current.Receipt != nil {
		saved := current.Receipt.Snapshot
		if saved == nil || saved.Name != request.Name || saved.Description != request.Description || saved.PowerAfter != request.PowerAfter || current.Receipt.ManifestID != request.ManifestID {
			return SnapshotOperationDTO{}, ErrConflict
		}
		if err := e.audit(scope, request.RequestID, request.Operator, ""); err != nil {
			return SnapshotOperationDTO{}, err
		}
		return e.Get(ctx, scope, request.RequestID)
	}
	if current.State.Fence != nil || current.State.Admission != nil {
		return SnapshotOperationDTO{}, ErrBusy
	}
	metadata, err := e.backend.Context(ctx, scope)
	if err != nil {
		return SnapshotOperationDTO{}, err
	}
	configHash, err := snapshotDigest(metadata.Config)
	if err != nil {
		return SnapshotOperationDTO{}, err
	}
	intent := SnapshotIntent{OperationID: request.RequestID, Name: request.Name, Description: request.Description, SourceVM: metadata.SourceVM, WasRunning: metadata.WasRunning, PowerAfter: request.PowerAfter, ConfigHash: configHash, Disks: metadata.Disks}
	if err := validateSnapshotIntent(scope, intent); err != nil {
		return SnapshotOperationDTO{}, err
	}
	coreHash, err := SnapshotCoreHash(scope, intent)
	if err != nil {
		return SnapshotOperationDTO{}, err
	}
	exists, err := e.backend.Exists(ctx, scope, intent.Name)
	if err != nil {
		return SnapshotOperationDTO{}, err
	}
	if exists {
		return SnapshotOperationDTO{}, ErrConflict
	}
	hash, err := snapshotDigest(request)
	if err != nil {
		return SnapshotOperationDTO{}, err
	}
	expected := Expectation{ControlRevision: current.State.ControlRevision, BindingGeneration: current.State.BindingGeneration, OperationID: request.RequestID, ManifestHash: coreHash, Step: "snapshot-prepare", RequestHash: hash}
	_, err = e.store.Mutate(scope, expected, func(c Current) (Mutation, error) {
		if c.State.Fence != nil || c.State.Admission != nil {
			return Mutation{}, ErrBusy
		}
		c.State.Fence = &Fence{Kind: "snapshot", Phase: "prepared", OperationID: request.RequestID, ManifestHash: coreHash, TargetVMUID: intent.SourceVM.UID}
		receipt := NewReceipt(scope, request.RequestID, coreHash)
		receipt.ManifestID = request.ManifestID
		receipt.Snapshot = &intent
		receipt.Resources = append(receipt.Resources, ResourceRef{ObjectRef: intent.SourceVM, Purpose: "snapshot-source-vm"})
		for _, disk := range intent.Disks {
			receipt.Resources = resourceOnce(receipt.Resources, ResourceRef{ObjectRef: disk.SourcePVC, Purpose: "snapshot-source-pvc"})
			if disk.SourceDV != nil {
				receipt.Resources = resourceOnce(receipt.Resources, ResourceRef{ObjectRef: *disk.SourceDV, Purpose: "snapshot-source-dv"})
			}
		}
		return Mutation{State: c.State, Receipt: &receipt}, nil
	})
	if err != nil {
		return SnapshotOperationDTO{}, err
	}
	if err := e.audit(scope, request.RequestID, request.Operator, ""); err != nil {
		return SnapshotOperationDTO{}, err
	}
	return e.Get(ctx, scope, request.RequestID)
}

// Get performs no commit, ACK, source unlock, start or cleanup side effect.
func (e *SnapshotEngine) Get(ctx context.Context, scope Scope, operationID string) (SnapshotOperationDTO, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	current, err := e.receipt(scope, operationID)
	if err != nil {
		return SnapshotOperationDTO{}, err
	}
	receipt := current.Receipt
	intent := *receipt.Snapshot
	observed, err := e.backend.Observe(ctx, scope, intent, receipt.Resources)
	if err != nil {
		return SnapshotOperationDTO{}, err
	}
	// Native observation may outlive a ledger transition. Project fence and ACK
	// facts from a fresh validated read, never the earlier pre-observation view.
	current, err = e.receipt(scope, operationID)
	if err != nil {
		return SnapshotOperationDTO{}, err
	}
	return e.project(current, observed)
}

func (e *SnapshotEngine) project(current Current, observed SnapshotObservation) (SnapshotOperationDTO, error) {
	receipt := current.Receipt
	intent := *receipt.Snapshot
	dto, err := ProjectSnapshot(receipt.Scope, intent, receipt.OperationID, receipt.ManifestHash, observed.Snapshot, observed.Content, observed.Volumes, proofFor(receipt))
	if err != nil {
		return SnapshotOperationDTO{}, err
	}
	dto.SourceLockClear = observed.SourceLockClear
	result := SnapshotOperationDTO{EventID: snapshotEventID(receipt.Scope, receipt.OperationID), SnapshotDTO: dto, WasRunning: intent.WasRunning, PowerAfter: intent.PowerAfter, SourceStopped: observed.SourceStopped, SourceRunning: observed.SourceRunning, CleanupComplete: observed.CleanupComplete, ManifestACKed: receipt.ConsoleAckHash != ""}
	result.ManifestID, result.ManifestDocumentHash = receipt.ManifestID, receipt.ConsoleAckHash
	owned := current.State.Fence != nil && current.State.Fence.OperationID == receipt.OperationID && current.State.Fence.ManifestHash == receipt.ManifestHash
	finalized := receipt.Steps["snapshot-finalize"]
	if finalized.Applied {
		result.FinalizedControlRevision = finalized.ControlRevision
	}
	// A successful finalize step proves historical release. Current release
	// additionally requires no fence (including another operation's) or effect.
	cleared := finalized.Applied && current.State.Fence == nil && snapshotPendingEffect(receipt) == ""
	result.FenceOwned, result.FenceCleared = &owned, &cleared
	if dto.ReadyToUse {
		if err := checkSnapshotFirmware(receipt.SnapshotIdentity, observed.Content); err != nil {
			if errors.Is(err, ErrSnapshotPending) {
				result.ReadyToUse = false
				result.Coverage.Complete = false
				result.ErrorCode = "vm_snapshot_identity_unverified"
				return result, nil
			}
			return SnapshotOperationDTO{}, err
		}
		result.NativeProofHash, err = nativeSnapshotProof(dto, observed)
		if errors.Is(err, ErrSnapshotPending) {
			result.ReadyToUse, result.Coverage.Complete, result.Phase = false, false, "creating"
		} else if err != nil {
			return SnapshotOperationDTO{}, err
		}
	}
	for _, ref := range observed.Resources {
		result.SnapshotResources = append(result.SnapshotResources, ref.ObjectRef)
	}
	_, result.Finalized = receipt.Steps["snapshot-finalize"]
	_, result.Cancelled = receipt.Steps["snapshot-cancelled"]
	_, result.Deleted = receipt.Steps["snapshot-deleted"]
	if result.Deleted {
		result.Phase = "deleted"
	}
	if current.State.Fence != nil && current.State.Fence.OperationID == receipt.OperationID && !result.ReadyToUse {
		result.Phase = current.State.Fence.Phase
	}
	if reason := snapshotPendingEffect(receipt); reason != "" {
		result.ErrorCode, result.Cancelled = reason, false
		if current.State.Fence != nil {
			result.Phase = current.State.Fence.Phase
		}
	}
	if snapshotCleanupSelected(receipt) && !snapshotStep(receipt, "snapshot-cancelled") {
		result.ReadyToUse, result.Coverage.Complete, result.Phase = false, false, "cancelling"
	}
	if snapshotKeepStopped(receipt) && !result.Finalized {
		result.Phase = "stopping"
	}
	return result, nil
}
func (e *SnapshotEngine) action(ctx context.Context, scope Scope, request SnapshotActionRequest) (Current, SnapshotObservation, error) {
	current, err := e.receipt(scope, request.OperationID)
	if err != nil {
		return current, SnapshotObservation{}, err
	}
	if err := requireSnapshotOwner(current, request.OperationID, request.CoreHash); err != nil {
		return current, SnapshotObservation{}, err
	}
	observation, err := e.backend.Observe(ctx, scope, *current.Receipt.Snapshot, current.Receipt.Resources)
	return current, observation, err
}
func unchangedSource(expected SnapshotIntent, current SnapshotContext) error {
	hash, err := snapshotDigest(current.Config)
	if err != nil || !sameObject(expected.SourceVM, current.SourceVM) || hash != expected.ConfigHash || !reflect.DeepEqual(expected.Disks, current.Disks) {
		return ErrConflict
	}
	return nil
}

// Halt claims one graceful Stop and never retries an unknown side effect.
func (e *SnapshotEngine) Halt(ctx context.Context, scope Scope, request SnapshotActionRequest) (SnapshotOperationDTO, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	current, observed, err := e.action(ctx, scope, request)
	if err != nil {
		return SnapshotOperationDTO{}, err
	}
	if snapshotCleanupSelected(current.Receipt) || snapshotKeepStopped(current.Receipt) {
		return SnapshotOperationDTO{}, ErrConflict
	}
	if e.Identity != nil {
		if err := e.Identity.Pin(ctx, scope, request.OperationID); err != nil {
			if errors.Is(err, ErrRestorePending) || errors.Is(err, ErrRestoreUnknown) {
				dto, readErr := e.Get(ctx, scope, request.OperationID)
				if readErr != nil {
					return dto, readErr
				}
				dto.ErrorCode = "vm_snapshot_identity_pending"
				if errors.Is(err, ErrRestoreUnknown) {
					dto.ErrorCode = "vm_snapshot_identity_outcome_unknown"
				}
				dto.ReadyToUse = false
				dto.Coverage.Complete = false
				return dto, nil
			}
			if errors.Is(err, ErrRestoreCapability) {
				return SnapshotOperationDTO{}, ErrSnapshotConfig
			}
			return SnapshotOperationDTO{}, err
		}
	}
	metadata, err := e.backend.Context(ctx, scope)
	if err != nil {
		return SnapshotOperationDTO{}, err
	}
	if err := unchangedSource(*current.Receipt.Snapshot, metadata); err != nil {
		return SnapshotOperationDTO{}, err
	}
	hash, err := engineRequestHash("halt", request)
	if err != nil {
		return SnapshotOperationDTO{}, err
	}
	claimed, err := e.mutate(scope, request.OperationID, "snapshot-halt-intent", hash, func(c Current) (Mutation, error) {
		if err := requireSnapshotOwner(c, request.OperationID, request.CoreHash); err != nil {
			return Mutation{}, err
		}
		if snapshotCleanupSelected(c.Receipt) || snapshotKeepStopped(c.Receipt) {
			return Mutation{}, ErrConflict
		}
		c.State.Fence.Phase = "stopping"
		return Mutation{State: c.State, Receipt: c.Receipt}, nil
	})
	if err != nil {
		return SnapshotOperationDTO{}, err
	}
	if err := requireSnapshotOwner(claimed, request.OperationID, request.CoreHash); err != nil {
		return SnapshotOperationDTO{}, err
	}
	if claimed.Replayed {
		if snapshotEffectUnsettled(claimed.Receipt, "snapshot-halt-intent", "snapshot-halt-requested", "snapshot-halt-suppressed") {
			return e.pendingEffect(ctx, scope, request.OperationID, ErrSnapshotPending)
		}
		return e.Get(ctx, scope, request.OperationID)
	}
	latest, err := e.receipt(scope, request.OperationID)
	if err != nil {
		return SnapshotOperationDTO{}, err
	}
	step := "snapshot-halt-requested"
	if snapshotCleanupSelected(latest.Receipt) || snapshotKeepStopped(latest.Receipt) {
		step = "snapshot-halt-suppressed"
	} else if !observed.SourceStopped {
		if err := e.backend.Halt(ctx, scope, *current.Receipt.Snapshot); err != nil {
			return e.pendingEffect(ctx, scope, request.OperationID, err)
		}
	}
	_, err = e.mutate(scope, request.OperationID, step, hash, func(c Current) (Mutation, error) {
		if err := requireSnapshotOwner(c, request.OperationID, request.CoreHash); err != nil {
			return Mutation{}, err
		}
		c.State.Fence.Phase = snapshotPhase(c.Receipt, "stopping")
		return Mutation{State: c.State, Receipt: c.Receipt}, nil
	})
	if err != nil {
		return SnapshotOperationDTO{}, err
	}
	return e.Get(ctx, scope, request.OperationID)
}

// Create owns one native RPC claim; cancellation waits for its exact UID to settle.
func (e *SnapshotEngine) Create(ctx context.Context, scope Scope, request SnapshotActionRequest) (SnapshotOperationDTO, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	prior, err := e.receipt(scope, request.OperationID)
	if err != nil {
		return SnapshotOperationDTO{}, err
	}
	hash, err := engineRequestHash("create", request)
	if err != nil {
		return SnapshotOperationDTO{}, err
	}
	if step, done := prior.Receipt.Steps["snapshot-created"]; done {
		if step.RequestHash != hash {
			return SnapshotOperationDTO{}, ErrConflict
		}
		return e.Get(ctx, scope, request.OperationID)
	}
	current, observed, err := e.action(ctx, scope, request)
	if err != nil {
		return SnapshotOperationDTO{}, err
	}
	if snapshotCleanupSelected(current.Receipt) || snapshotKeepStopped(current.Receipt) {
		return SnapshotOperationDTO{}, ErrConflict
	}
	if !observed.SourceStopped || !observed.SourceLockClear && observed.Snapshot == nil {
		return SnapshotOperationDTO{}, ErrSnapshotPending
	}
	metadata, err := e.backend.Context(ctx, scope)
	if err != nil {
		return SnapshotOperationDTO{}, err
	}
	if err := unchangedSource(*current.Receipt.Snapshot, metadata); err != nil {
		return SnapshotOperationDTO{}, err
	}
	claimed, err := e.mutate(scope, request.OperationID, "snapshot-create-intent", hash, func(c Current) (Mutation, error) {
		if err := requireSnapshotOwner(c, request.OperationID, request.CoreHash); err != nil {
			return Mutation{}, err
		}
		if snapshotCleanupSelected(c.Receipt) || snapshotKeepStopped(c.Receipt) {
			return Mutation{}, ErrConflict
		}
		if snapshotEffectUnsettled(c.Receipt, "snapshot-halt-intent", "snapshot-halt-requested", "snapshot-halt-suppressed") {
			return Mutation{}, ErrSnapshotPending
		}
		c.State.Fence.Phase = "creating"
		return Mutation{State: c.State, Receipt: c.Receipt}, nil
	})
	if err != nil {
		return SnapshotOperationDTO{}, err
	}
	if err := requireSnapshotOwner(claimed, request.OperationID, request.CoreHash); err != nil {
		return SnapshotOperationDTO{}, err
	}
	if claimed.Replayed {
		return e.pendingEffect(ctx, scope, request.OperationID, ErrSnapshotPending)
	}
	latest, err := e.receipt(scope, request.OperationID)
	if err != nil {
		return SnapshotOperationDTO{}, err
	}
	if snapshotCleanupSelected(latest.Receipt) || snapshotKeepStopped(latest.Receipt) {
		_, err = e.mutate(scope, request.OperationID, "snapshot-create-suppressed", hash, func(c Current) (Mutation, error) {
			if err := requireSnapshotOwner(c, request.OperationID, request.CoreHash); err != nil {
				return Mutation{}, err
			}
			c.State.Fence.Phase = snapshotPhase(c.Receipt, "creating")
			return Mutation{State: c.State, Receipt: c.Receipt}, nil
		})
		if err != nil {
			return SnapshotOperationDTO{}, err
		}
		return e.Get(ctx, scope, request.OperationID)
	}
	ref, err := e.backend.Create(ctx, scope, *current.Receipt.Snapshot, request.OperationID, request.CoreHash)
	if err != nil {
		return e.pendingEffect(ctx, scope, request.OperationID, err)
	}
	if ref.Kind != "VirtualMachineSnapshot" || ref.Namespace != scope.Namespace || ref.Name != current.Receipt.Snapshot.Name || !validUID(ref.UID) {
		return e.pendingEffect(ctx, scope, request.OperationID, ErrConflict)
	}
	_, err = e.mutate(scope, request.OperationID, "snapshot-created", hash, func(c Current) (Mutation, error) {
		if err := requireSnapshotOwner(c, request.OperationID, request.CoreHash); err != nil {
			return Mutation{}, err
		}
		c.Receipt.Resources = resourceOnce(c.Receipt.Resources, ResourceRef{ObjectRef: ref, Purpose: "snapshot"})
		c.State.Fence.Phase = snapshotPhase(c.Receipt, "snapshotting")
		return Mutation{State: c.State, Receipt: c.Receipt}, nil
	})
	if err != nil {
		return SnapshotOperationDTO{}, err
	}
	return e.Get(ctx, scope, request.OperationID)
}

func snapshotACKMatches(current Current, observed SnapshotOperationDTO, request SnapshotActionRequest) error {
	if !hashPattern.MatchString(request.DocumentHash) || request.ManifestID != current.Receipt.ManifestID || request.CoreHash != current.Receipt.ManifestHash {
		return ErrConflict
	}
	if request.AckKind == "ready" {
		if !observed.ReadyToUse || request.SnapshotUID != observed.SnapshotUID || request.ContentUID != observed.ContentUID {
			return ErrSnapshotPending
		}
		if !hashPattern.MatchString(request.NativeProofHash) || request.NativeProofHash != observed.NativeProofHash {
			return ErrConflict
		}
	} else if request.AckKind == "cancelled" {
		if !observed.Cancelled || !observed.CleanupComplete {
			return ErrSnapshotPending
		}
	} else {
		return ErrSnapshotInvalid
	}
	return nil
}

// ACK is an authenticated Console-worker statement after its DB commit; it is
// never constructed by blindly forwarding a UI body. Native identities and all
// coverage are independently checked here rather than trusting its hash alone.
func (e *SnapshotEngine) acknowledge(ctx context.Context, scope Scope, request SnapshotActionRequest) (Current, error) {
	current, observed, err := e.action(ctx, scope, request)
	if err != nil {
		return current, err
	}
	dto, err := e.project(current, observed)
	if err != nil {
		return current, err
	}
	if err := snapshotACKMatches(current, dto, request); err != nil {
		return current, err
	}
	ackRequest := request
	ackRequest.RequestID = ""
	hash, err := engineRequestHash("ack", ackRequest)
	if err != nil {
		return current, err
	}
	accepted, err := e.mutate(scope, request.OperationID, "snapshot-ack-"+request.AckKind, hash, func(c Current) (Mutation, error) {
		if err := requireSnapshotOwner(c, request.OperationID, request.CoreHash); err != nil {
			return Mutation{}, err
		}
		if request.AckKind == "ready" && snapshotCleanupSelected(c.Receipt) {
			return Mutation{}, ErrConflict
		}
		if request.AckKind == "ready" && !snapshotStep(c.Receipt, "snapshot-created") {
			return Mutation{}, ErrSnapshotPending
		}
		if c.State.ControlRevision != current.State.ControlRevision || snapshotPendingEffect(c.Receipt) != "" {
			return Mutation{}, ErrSnapshotPending
		}
		if err := snapshotACKMatches(c, dto, request); err != nil {
			return Mutation{}, err
		}
		if c.Receipt.ConsoleAckHash != "" && c.Receipt.ConsoleAckHash != request.DocumentHash {
			return Mutation{}, ErrConflict
		}
		c.Receipt.ConsoleAckHash = request.DocumentHash
		for _, ref := range observed.Resources {
			c.Receipt.Resources = resourceOnce(c.Receipt.Resources, ref)
		}
		c.State.Fence.Phase = "manifest-acked"
		return Mutation{State: c.State, Receipt: c.Receipt}, nil
	})
	if err != nil {
		return accepted, err
	}
	// Historical step replay skips its callback, so recheck the immutable choice and lease.
	if err := requireSnapshotOwner(accepted, request.OperationID, request.CoreHash); err != nil {
		return accepted, err
	}
	if request.AckKind == "ready" && snapshotCleanupSelected(accepted.Receipt) {
		return accepted, ErrConflict
	}
	if snapshotPendingEffect(accepted.Receipt) != "" {
		return accepted, ErrSnapshotPending
	}
	return accepted, nil
}

func resumeUnsettled(receipt *Receipt) bool {
	return snapshotEffectUnsettled(receipt, "snapshot-resume-intent", "snapshot-resume-requested", "snapshot-resume-suppressed")
}

func snapshotStep(receipt *Receipt, name string) bool {
	_, exists := receipt.Steps[name]
	return exists
}

func snapshotEffectUnsettled(receipt *Receipt, claim, settled, suppressed string) bool {
	return snapshotStep(receipt, claim) && !snapshotStep(receipt, settled) && (suppressed == "" || !snapshotStep(receipt, suppressed))
}

func snapshotPendingEffect(receipt *Receipt) string {
	for _, effect := range []struct{ claim, settled, suppressed, reason string }{
		{"snapshot-create-intent", "snapshot-created", "snapshot-create-suppressed", "vm_snapshot_create_outcome_pending"},
		{"snapshot-halt-intent", "snapshot-halt-requested", "snapshot-halt-suppressed", "vm_snapshot_halt_outcome_pending"},
		{"snapshot-cleanup-delete-intent", "snapshot-cleanup-delete-requested", "", "vm_snapshot_cleanup_outcome_pending"},
		{"snapshot-cancel-halt-intent", "snapshot-cancel-halt-requested", "", "vm_snapshot_halt_outcome_pending"},
		{"snapshot-delete-rpc-intent", "snapshot-delete-rpc-requested", "", "vm_snapshot_delete_outcome_pending"},
		{"snapshot-resume-intent", "snapshot-resume-requested", "snapshot-resume-suppressed", "vm_snapshot_resume_outcome_pending"},
	} {
		if snapshotEffectUnsettled(receipt, effect.claim, effect.settled, effect.suppressed) {
			return effect.reason
		}
	}
	return ""
}

// The shared choice and ready-ACK steps serialize cancellation mode without a second ledger.
func snapshotCleanupSelected(receipt *Receipt) bool {
	return !snapshotStep(receipt, "snapshot-ack-ready") && (snapshotStep(receipt, "snapshot-cancel-intent") || snapshotStep(receipt, "snapshot-cancel-choice"))
}

func snapshotKeepStopped(receipt *Receipt) bool {
	return snapshotStep(receipt, "snapshot-keep-stopped") || snapshotStep(receipt, "snapshot-cancel-choice") && snapshotStep(receipt, "snapshot-ack-ready")
}

func snapshotPhase(receipt *Receipt, otherwise string) string {
	if snapshotKeepStopped(receipt) {
		return "stopping"
	}
	if snapshotCleanupSelected(receipt) && !snapshotStep(receipt, "snapshot-cancelled") {
		return "cancelling"
	}
	return otherwise
}

func (e *SnapshotEngine) pendingEffect(ctx context.Context, scope Scope, operationID string, cause error) (SnapshotOperationDTO, error) {
	dto, err := e.Get(ctx, scope, operationID)
	if err != nil {
		return dto, err
	}
	return dto, cause
}

// Resume claims one Start outside the transaction. Unknown outcomes remain fenced.
func (e *SnapshotEngine) Resume(ctx context.Context, scope Scope, request SnapshotActionRequest) (SnapshotOperationDTO, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	prior, err := e.receipt(scope, request.OperationID)
	if err != nil {
		return SnapshotOperationDTO{}, err
	}
	if snapshotKeepStopped(prior.Receipt) {
		return SnapshotOperationDTO{}, ErrConflict
	}
	if snapshotPendingEffect(prior.Receipt) != "" {
		return e.pendingEffect(ctx, scope, request.OperationID, ErrSnapshotPending)
	}
	current, err := e.acknowledge(ctx, scope, request)
	if err != nil {
		if errors.Is(err, ErrSnapshotPending) {
			return e.pendingEffect(ctx, scope, request.OperationID, err)
		}
		return SnapshotOperationDTO{}, err
	}
	intent := *current.Receipt.Snapshot
	if !intent.WasRunning || intent.PowerAfter != "previous" {
		return SnapshotOperationDTO{}, ErrSnapshotInvalid
	}
	observed, err := e.backend.Observe(ctx, scope, intent, current.Receipt.Resources)
	if err != nil {
		return SnapshotOperationDTO{}, err
	}
	if !observed.SourceLockClear {
		return SnapshotOperationDTO{}, ErrSnapshotPending
	}
	dto, err := e.project(current, observed)
	if err != nil {
		return SnapshotOperationDTO{}, err
	}
	if err := snapshotACKMatches(current, dto, request); err != nil {
		return SnapshotOperationDTO{}, err
	}
	hash, err := engineRequestHash("resume", request)
	if err != nil {
		return SnapshotOperationDTO{}, err
	}
	claimed, err := e.mutate(scope, request.OperationID, "snapshot-resume-intent", hash, func(c Current) (Mutation, error) {
		if err := requireSnapshotOwner(c, request.OperationID, request.CoreHash); err != nil {
			return Mutation{}, err
		}
		if snapshotKeepStopped(c.Receipt) {
			return Mutation{}, ErrConflict
		}
		c.State.Fence.Phase = "resuming"
		return Mutation{State: c.State, Receipt: c.Receipt}, nil
	})
	if err != nil {
		return SnapshotOperationDTO{}, err
	}
	if claimed.Replayed {
		dto, err := e.Get(ctx, scope, request.OperationID)
		if err == nil && resumeUnsettled(claimed.Receipt) {
			err = ErrSnapshotPending
		}
		return dto, err
	}
	// A cancellation after this read still cannot finalize until this claim settles.
	latest, err := e.receipt(scope, request.OperationID)
	if err != nil {
		return SnapshotOperationDTO{}, err
	}
	step := "snapshot-resume-requested"
	if snapshotKeepStopped(latest.Receipt) {
		step = "snapshot-resume-suppressed"
	} else if !observed.SourceRunning {
		if err := e.backend.Resume(ctx, scope, intent); err != nil {
			dto, readErr := e.Get(ctx, scope, request.OperationID)
			if readErr != nil {
				return dto, readErr
			}
			return dto, err
		}
	}
	_, err = e.mutate(scope, request.OperationID, step, hash, func(c Current) (Mutation, error) {
		if err := requireSnapshotOwner(c, request.OperationID, request.CoreHash); err != nil {
			return Mutation{}, err
		}
		if snapshotKeepStopped(c.Receipt) {
			c.State.Fence.Phase = "stopping"
		} else {
			c.State.Fence.Phase = "resuming"
		}
		return Mutation{State: c.State, Receipt: c.Receipt}, nil
	})
	if err != nil {
		return SnapshotOperationDTO{}, err
	}
	return e.Get(ctx, scope, request.OperationID)
}

// Finalize releases the fence only after ACK, actual desired power and native unlock.
func (e *SnapshotEngine) Finalize(ctx context.Context, scope Scope, request SnapshotActionRequest) (SnapshotOperationDTO, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	prior, err := e.receipt(scope, request.OperationID)
	if err != nil {
		return SnapshotOperationDTO{}, err
	}
	hash, err := engineRequestHash("finalize", request)
	if err != nil {
		return SnapshotOperationDTO{}, err
	}
	if done, ok := prior.Receipt.Steps["snapshot-finalize"]; ok {
		if done.RequestHash != hash {
			return SnapshotOperationDTO{}, ErrConflict
		}
		status := "success"
		if request.AckKind == "cancelled" {
			status = "failure"
		}
		if err := e.audit(scope, request.OperationID, "", status); err != nil {
			return SnapshotOperationDTO{}, err
		}
		return e.Get(ctx, scope, request.OperationID)
	}
	if snapshotPendingEffect(prior.Receipt) != "" {
		dto, readErr := e.Get(ctx, scope, request.OperationID)
		if readErr != nil {
			return dto, readErr
		}
		return dto, ErrSnapshotPending
	}
	current, err := e.acknowledge(ctx, scope, request)
	if err != nil {
		if errors.Is(err, ErrSnapshotPending) {
			return e.pendingEffect(ctx, scope, request.OperationID, err)
		}
		return SnapshotOperationDTO{}, err
	}
	observedRevision, observedGeneration := current.State.ControlRevision, current.State.BindingGeneration
	observed, err := e.backend.Observe(ctx, scope, *current.Receipt.Snapshot, current.Receipt.Resources)
	if err != nil {
		return SnapshotOperationDTO{}, err
	}
	if !observed.SourceLockClear {
		return SnapshotOperationDTO{}, ErrSnapshotPending
	}
	dto, err := e.project(current, observed)
	if err != nil {
		return SnapshotOperationDTO{}, err
	}
	if err := snapshotACKMatches(current, dto, request); err != nil {
		return SnapshotOperationDTO{}, err
	}
	keepStopped := snapshotKeepStopped(current.Receipt)
	wantRunning := current.Receipt.Snapshot.WasRunning && current.Receipt.Snapshot.PowerAfter == "previous" && !keepStopped
	if wantRunning && !observed.SourceRunning || !wantRunning && !observed.SourceStopped {
		return SnapshotOperationDTO{}, ErrSnapshotPending
	}
	_, err = e.mutate(scope, request.OperationID, "snapshot-finalize", hash, func(c Current) (Mutation, error) {
		if err := requireSnapshotOwner(c, request.OperationID, request.CoreHash); err != nil {
			return Mutation{}, err
		}
		// Every ledger mutation advances this revision. A newer claim or power
		// decision needs a fresh observation, never a CAS retry with the old one.
		if c.State.ControlRevision != observedRevision || c.State.BindingGeneration != observedGeneration || snapshotPendingEffect(c.Receipt) != "" {
			return Mutation{}, ErrSnapshotPending
		}
		latestKeepStopped := snapshotKeepStopped(c.Receipt)
		latestWantRunning := c.Receipt.Snapshot.WasRunning && c.Receipt.Snapshot.PowerAfter == "previous" && !latestKeepStopped
		if latestWantRunning != wantRunning || c.Receipt.ConsoleAckHash != request.DocumentHash {
			return Mutation{}, ErrSnapshotPending
		}
		if err := snapshotACKMatches(c, dto, request); err != nil {
			return Mutation{}, err
		}
		c.State.Fence = nil
		return Mutation{State: c.State, Receipt: c.Receipt}, nil
	})
	if err != nil {
		return SnapshotOperationDTO{}, err
	}
	status := "success"
	if request.AckKind == "cancelled" {
		status = "failure"
	}
	if err := e.audit(scope, request.OperationID, "", status); err != nil {
		return SnapshotOperationDTO{}, err
	}
	return e.Get(ctx, scope, request.OperationID)
}

// Cancel atomically selects cleanup or keep-stopped before any cancellation effect.
func (e *SnapshotEngine) Cancel(ctx context.Context, scope Scope, request SnapshotActionRequest) (SnapshotOperationDTO, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	current, _, err := e.action(ctx, scope, request)
	if err != nil {
		return SnapshotOperationDTO{}, err
	}
	hash, err := engineRequestHash("cancel", request)
	if err != nil {
		return SnapshotOperationDTO{}, err
	}
	if done, ok := current.Receipt.Steps["snapshot-cancelled"]; ok {
		if done.RequestHash != hash {
			return SnapshotOperationDTO{}, ErrConflict
		}
		return e.Get(ctx, scope, request.OperationID)
	}
	chosen, err := e.mutate(scope, request.OperationID, "snapshot-cancel-choice", hash, func(c Current) (Mutation, error) {
		if err := requireSnapshotOwner(c, request.OperationID, request.CoreHash); err != nil {
			return Mutation{}, err
		}
		if snapshotStep(c.Receipt, "snapshot-ack-ready") {
			c.State.Fence.Phase = "stopping"
		} else {
			c.State.Fence.Phase = "cancelling"
		}
		return Mutation{State: c.State, Receipt: c.Receipt}, nil
	})
	if err != nil {
		return SnapshotOperationDTO{}, err
	}
	if err := requireSnapshotOwner(chosen, request.OperationID, request.CoreHash); err != nil {
		return SnapshotOperationDTO{}, err
	}
	marker := "snapshot-cancel-intent"
	if snapshotKeepStopped(chosen.Receipt) {
		marker = "snapshot-keep-stopped"
	}
	_, err = e.mutate(scope, request.OperationID, marker, hash, func(c Current) (Mutation, error) {
		if err := requireSnapshotOwner(c, request.OperationID, request.CoreHash); err != nil {
			return Mutation{}, err
		}
		if (marker == "snapshot-keep-stopped") != snapshotKeepStopped(c.Receipt) {
			return Mutation{}, ErrConflict
		}
		c.State.Fence.Phase = snapshotPhase(c.Receipt, "cancelling")
		return Mutation{State: c.State, Receipt: c.Receipt}, nil
	})
	if err != nil {
		return SnapshotOperationDTO{}, err
	}
	current, observed, err := e.action(ctx, scope, request)
	if err != nil {
		return SnapshotOperationDTO{}, err
	}
	if snapshotStep(current.Receipt, "snapshot-cancelled") {
		return e.Get(ctx, scope, request.OperationID)
	}
	if reason := snapshotPendingEffect(current.Receipt); reason != "" {
		cause := ErrSnapshotPending
		if reason == "vm_snapshot_create_outcome_pending" {
			cause = ErrBusy
		}
		return e.pendingEffect(ctx, scope, request.OperationID, cause)
	}
	if snapshotKeepStopped(current.Receipt) {
		return e.keepSnapshotStopped(ctx, scope, request, hash, current, observed)
	}
	if !snapshotCleanupSelected(current.Receipt) {
		return SnapshotOperationDTO{}, ErrConflict
	}
	if !observed.CleanupComplete {
		claimed, err := e.mutate(scope, request.OperationID, "snapshot-cleanup-delete-intent", hash, func(c Current) (Mutation, error) {
			if err := requireSnapshotOwner(c, request.OperationID, request.CoreHash); err != nil {
				return Mutation{}, err
			}
			if !snapshotCleanupSelected(c.Receipt) || snapshotStep(c.Receipt, "snapshot-ack-ready") {
				return Mutation{}, ErrConflict
			}
			if c.State.ControlRevision != current.State.ControlRevision || snapshotPendingEffect(c.Receipt) != "" {
				return Mutation{}, ErrSnapshotPending
			}
			for _, ref := range observed.Resources {
				c.Receipt.Resources = resourceOnce(c.Receipt.Resources, ref)
			}
			c.State.Fence.Phase = "cancelling"
			return Mutation{State: c.State, Receipt: c.Receipt}, nil
		})
		if err != nil {
			return SnapshotOperationDTO{}, err
		}
		if err := requireSnapshotOwner(claimed, request.OperationID, request.CoreHash); err != nil {
			return SnapshotOperationDTO{}, err
		}
		if !snapshotCleanupSelected(claimed.Receipt) {
			return SnapshotOperationDTO{}, ErrConflict
		}
		if claimed.Replayed {
			return e.Get(ctx, scope, request.OperationID)
		}
		if err := e.backend.Delete(ctx, scope, observed.Resources); err != nil {
			return e.pendingEffect(ctx, scope, request.OperationID, err)
		}
		_, err = e.mutate(scope, request.OperationID, "snapshot-cleanup-delete-requested", hash, func(c Current) (Mutation, error) {
			if err := requireSnapshotOwner(c, request.OperationID, request.CoreHash); err != nil {
				return Mutation{}, err
			}
			if !snapshotCleanupSelected(c.Receipt) {
				return Mutation{}, ErrConflict
			}
			c.State.Fence.Phase = "cancelling"
			return Mutation{State: c.State, Receipt: c.Receipt}, nil
		})
		if err != nil {
			return SnapshotOperationDTO{}, err
		}
		return e.Get(ctx, scope, request.OperationID)
	}
	if !observed.SourceLockClear || snapshotStep(current.Receipt, "snapshot-halt-requested") && !observed.SourceStopped {
		return e.pendingEffect(ctx, scope, request.OperationID, ErrSnapshotPending)
	}
	_, err = e.mutate(scope, request.OperationID, "snapshot-cancelled", hash, func(c Current) (Mutation, error) {
		if err := requireSnapshotOwner(c, request.OperationID, request.CoreHash); err != nil {
			return Mutation{}, err
		}
		if !snapshotCleanupSelected(c.Receipt) || snapshotStep(c.Receipt, "snapshot-ack-ready") {
			return Mutation{}, ErrConflict
		}
		if c.State.ControlRevision != current.State.ControlRevision || c.State.BindingGeneration != current.State.BindingGeneration || snapshotPendingEffect(c.Receipt) != "" {
			return Mutation{}, ErrSnapshotPending
		}
		c.State.Fence.Phase = "cancelled"
		return Mutation{State: c.State, Receipt: c.Receipt}, nil
	})
	if err != nil {
		return SnapshotOperationDTO{}, err
	}
	return e.Get(ctx, scope, request.OperationID)
}

func (e *SnapshotEngine) keepSnapshotStopped(ctx context.Context, scope Scope, request SnapshotActionRequest, hash string, current Current, observed SnapshotObservation) (SnapshotOperationDTO, error) {
	if observed.SourceStopped {
		return e.Get(ctx, scope, request.OperationID)
	}
	claimed, err := e.mutate(scope, request.OperationID, "snapshot-cancel-halt-intent", hash, func(c Current) (Mutation, error) {
		if err := requireSnapshotOwner(c, request.OperationID, request.CoreHash); err != nil {
			return Mutation{}, err
		}
		if !snapshotKeepStopped(c.Receipt) {
			return Mutation{}, ErrConflict
		}
		if c.State.ControlRevision != current.State.ControlRevision || snapshotPendingEffect(c.Receipt) != "" {
			return Mutation{}, ErrSnapshotPending
		}
		c.State.Fence.Phase = "stopping"
		return Mutation{State: c.State, Receipt: c.Receipt}, nil
	})
	if err != nil {
		return SnapshotOperationDTO{}, err
	}
	if err := requireSnapshotOwner(claimed, request.OperationID, request.CoreHash); err != nil {
		return SnapshotOperationDTO{}, err
	}
	if !snapshotKeepStopped(claimed.Receipt) {
		return SnapshotOperationDTO{}, ErrConflict
	}
	if claimed.Replayed {
		return e.Get(ctx, scope, request.OperationID)
	}
	if err := e.backend.Halt(ctx, scope, *current.Receipt.Snapshot); err != nil {
		return e.pendingEffect(ctx, scope, request.OperationID, err)
	}
	_, err = e.mutate(scope, request.OperationID, "snapshot-cancel-halt-requested", hash, func(c Current) (Mutation, error) {
		if err := requireSnapshotOwner(c, request.OperationID, request.CoreHash); err != nil {
			return Mutation{}, err
		}
		if !snapshotKeepStopped(c.Receipt) {
			return Mutation{}, ErrConflict
		}
		c.State.Fence.Phase = "stopping"
		return Mutation{State: c.State, Receipt: c.Receipt}, nil
	})
	if err != nil {
		return SnapshotOperationDTO{}, err
	}
	return e.Get(ctx, scope, request.OperationID)
}
