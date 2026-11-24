package vmrestore

import (
	"context"
	"sort"
	"time"
)

// SnapshotListDTO keeps the existing Region response envelope's bean shape.
type SnapshotListDTO struct {
	Items    []SnapshotOperationDTO `json:"items"`
	Page     int                    `json:"page"`
	PageSize int                    `json:"page_size"`
	Total    int                    `json:"total"`
}

func (e *SnapshotEngine) records(scope Scope) ([]Receipt, error) {
	current, err := e.store.Read(scope, "")
	if err != nil {
		return nil, err
	}
	receipts, err := (&Resolver{store: e.store}).receipts(scope, current.State)
	if err != nil {
		return nil, err
	}
	records := []Receipt{}
	for _, receipt := range receipts {
		if receipt.Snapshot != nil {
			records = append(records, receipt)
		}
	}
	// The resolver validates history in key order. Operation creation order comes
	// from its first successful control revision, independently of request IDs.
	sort.SliceStable(records, func(i, j int) bool {
		return firstSnapshotRevision(records[i]) > firstSnapshotRevision(records[j])
	})
	return records, nil
}

func firstSnapshotRevision(receipt Receipt) uint64 {
	var first uint64
	for _, step := range receipt.Steps {
		if step.Applied && (first == 0 || step.ControlRevision < first) {
			first = step.ControlRevision
		}
	}
	return first
}

// List returns managed records only; native success never fabricates a Console manifest.
func (e *SnapshotEngine) List(ctx context.Context, scope Scope, page, pageSize int) (SnapshotListDTO, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	result := SnapshotListDTO{Items: []SnapshotOperationDTO{}, Page: page, PageSize: pageSize}
	if page < 1 || pageSize < 1 || pageSize > 100 {
		return result, ErrSnapshotInvalid
	}
	records, err := e.records(scope)
	if err != nil {
		return result, err
	}
	active := []Receipt{}
	names := map[string]bool{}
	for _, receipt := range records {
		if _, deleted := receipt.Steps["snapshot-deleted"]; deleted {
			continue
		}
		if _, cancelled := receipt.Steps["snapshot-cancelled"]; cancelled {
			continue
		}
		if names[receipt.Snapshot.Name] {
			continue
		}
		names[receipt.Snapshot.Name] = true
		active = append(active, receipt)
	}
	result.Total = len(active)
	start := (page - 1) * pageSize
	if start < 0 || start >= len(active) {
		return result, nil
	}
	end := start + pageSize
	if end > len(active) {
		end = len(active)
	}
	for _, receipt := range active[start:end] {
		dto, err := e.Get(ctx, scope, receipt.OperationID)
		if err != nil {
			return result, err
		}
		result.Items = append(result.Items, dto)
	}
	sort.SliceStable(result.Items, func(i, j int) bool { return result.Items[i].CreationTime > result.Items[j].CreationTime })
	return result, nil
}

// ByName resolves only server-owned receipt metadata, not a UI supplied op map.
func (e *SnapshotEngine) ByName(ctx context.Context, scope Scope, name string) (SnapshotOperationDTO, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	records, err := e.records(scope)
	if err != nil {
		return SnapshotOperationDTO{}, err
	}
	for _, record := range records {
		if record.Snapshot.Name == name {
			return e.Get(ctx, scope, record.OperationID)
		}
	}
	return SnapshotOperationDTO{}, ErrNotFound
}
func (e *SnapshotEngine) snapshotDeleteReferences(ctx context.Context, scope Scope, owner Receipt, observation SnapshotObservation) error {
	current, err := e.store.Read(scope, "")
	if err != nil {
		return err
	}
	records, err := (&Resolver{store: e.store}).receipts(scope, current.State)
	if err != nil {
		return err
	}
	resources := append([]ResourceRef(nil), owner.Resources...)
	for _, ref := range observation.Resources {
		resources = resourceOnce(resources, ref)
	}
	for _, record := range records {
		if record.OperationID == owner.OperationID || record.SnapshotReferenceReleased {
			continue
		}
		for _, ref := range record.Resources {
			if ref.Kind == "VirtualMachineSnapshot" {
				for _, owned := range resources {
					if owned.Kind == ref.Kind && sameObject(owned.ObjectRef, ref.ObjectRef) {
						return ErrBusy
					}
				}
			}
		}
	}
	// Read-only snapshot-backed PVCs remain a real physical dependency; provenance
	// in an active independent binding is deliberately not consulted here.
	return e.backend.DeleteReferences(ctx, scope, resources)
}

func requireSnapshotDeleteOwner(current Current, operationID, coreHash string) error {
	if current.Receipt == nil || current.Receipt.Snapshot == nil || current.Receipt.OperationID != operationID || current.Receipt.ManifestHash != coreHash {
		return ErrConflict
	}
	fence := current.State.Fence
	if fence == nil || fence.Kind != "delete" || fence.OperationID != operationID || fence.ManifestHash != coreHash || !fence.TargetVMAbsent {
		return ErrBusy
	}
	return nil
}

// Delete serializes its lease, one RPC claim and fresh terminal cleanup evidence.
func (e *SnapshotEngine) Delete(ctx context.Context, scope Scope, request SnapshotActionRequest) (SnapshotOperationDTO, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	current, err := e.receipt(scope, request.OperationID)
	if err != nil {
		return SnapshotOperationDTO{}, err
	}
	hash, err := engineRequestHash("delete", request)
	if err != nil {
		return SnapshotOperationDTO{}, err
	}
	if step, done := current.Receipt.Steps["snapshot-deleted"]; done {
		if step.RequestHash != hash {
			return SnapshotOperationDTO{}, ErrConflict
		}
		return e.Get(ctx, scope, request.OperationID)
	}
	if request.CoreHash != current.Receipt.ManifestHash || !validUID(request.SnapshotUID) || proofFor(current.Receipt).SnapshotUID != request.SnapshotUID {
		return SnapshotOperationDTO{}, ErrConflict
	}
	if !snapshotStep(current.Receipt, "snapshot-finalize") {
		return SnapshotOperationDTO{}, ErrBusy
	}
	if current.State.Fence == nil {
		observed, err := e.backend.Observe(ctx, scope, *current.Receipt.Snapshot, current.Receipt.Resources)
		if err != nil {
			return SnapshotOperationDTO{}, err
		}
		if err := e.snapshotDeleteReferences(ctx, scope, *current.Receipt, observed); err != nil {
			return SnapshotOperationDTO{}, err
		}
		_, err = e.mutate(scope, request.OperationID, "snapshot-delete-intent", hash, func(c Current) (Mutation, error) {
			if c.State.Fence != nil || c.State.Admission != nil {
				return Mutation{}, ErrBusy
			}
			if c.State.ControlRevision != current.State.ControlRevision || c.State.BindingGeneration != current.State.BindingGeneration {
				return Mutation{}, ErrSnapshotPending
			}
			c.State.Fence = &Fence{Kind: "delete", Phase: "deleting-snapshot", OperationID: request.OperationID, ManifestHash: request.CoreHash, TargetVMAbsent: true}
			for _, ref := range observed.Resources {
				c.Receipt.Resources = resourceOnce(c.Receipt.Resources, ref)
			}
			return Mutation{State: c.State, Receipt: c.Receipt}, nil
		})
		if err != nil {
			return SnapshotOperationDTO{}, err
		}
	}
	current, err = e.receipt(scope, request.OperationID)
	if err != nil {
		return SnapshotOperationDTO{}, err
	}
	if err := requireSnapshotDeleteOwner(current, request.OperationID, request.CoreHash); err != nil {
		return SnapshotOperationDTO{}, err
	}
	if snapshotPendingEffect(current.Receipt) != "" {
		return e.pendingEffect(ctx, scope, request.OperationID, ErrSnapshotPending)
	}
	observed, err := e.backend.Observe(ctx, scope, *current.Receipt.Snapshot, current.Receipt.Resources)
	if err != nil {
		return SnapshotOperationDTO{}, err
	}
	if err := e.snapshotDeleteReferences(ctx, scope, *current.Receipt, observed); err != nil {
		return SnapshotOperationDTO{}, err
	}
	if !observed.CleanupComplete {
		claimed, err := e.mutate(scope, request.OperationID, "snapshot-delete-rpc-intent", hash, func(c Current) (Mutation, error) {
			if err := requireSnapshotDeleteOwner(c, request.OperationID, request.CoreHash); err != nil {
				return Mutation{}, err
			}
			if c.State.ControlRevision != current.State.ControlRevision || snapshotPendingEffect(c.Receipt) != "" {
				return Mutation{}, ErrSnapshotPending
			}
			for _, ref := range observed.Resources {
				c.Receipt.Resources = resourceOnce(c.Receipt.Resources, ref)
			}
			return Mutation{State: c.State, Receipt: c.Receipt}, nil
		})
		if err != nil {
			return SnapshotOperationDTO{}, err
		}
		if err := requireSnapshotDeleteOwner(claimed, request.OperationID, request.CoreHash); err != nil {
			return SnapshotOperationDTO{}, err
		}
		if claimed.Replayed {
			return e.Get(ctx, scope, request.OperationID)
		}
		if err := e.backend.Delete(ctx, scope, observed.Resources); err != nil {
			return e.pendingEffect(ctx, scope, request.OperationID, err)
		}
		_, err = e.mutate(scope, request.OperationID, "snapshot-delete-rpc-requested", hash, func(c Current) (Mutation, error) {
			if err := requireSnapshotDeleteOwner(c, request.OperationID, request.CoreHash); err != nil {
				return Mutation{}, err
			}
			return Mutation{State: c.State, Receipt: c.Receipt}, nil
		})
		if err != nil {
			return SnapshotOperationDTO{}, err
		}
		return e.Get(ctx, scope, request.OperationID)
	}
	if !observed.SourceLockClear {
		return SnapshotOperationDTO{}, ErrSnapshotPending
	}
	_, err = e.mutate(scope, request.OperationID, "snapshot-deleted", hash, func(c Current) (Mutation, error) {
		if err := requireSnapshotDeleteOwner(c, request.OperationID, request.CoreHash); err != nil {
			return Mutation{}, err
		}
		if c.State.ControlRevision != current.State.ControlRevision || c.State.BindingGeneration != current.State.BindingGeneration || snapshotPendingEffect(c.Receipt) != "" {
			return Mutation{}, ErrSnapshotPending
		}
		c.State.Fence = nil
		return Mutation{State: c.State, Receipt: c.Receipt}, nil
	})
	if err != nil {
		return SnapshotOperationDTO{}, err
	}
	return e.Get(ctx, scope, request.OperationID)
}
