package vmrestore

import "context"

// RestoreStorageDependencies contains only server-owned execution prerequisites.
// The storage integration supplies real qualification and reservation adapters.
type RestoreStorageDependencies struct {
	Certification   CertifiedRestoreStorage
	Capacity        RestoreCapacityProvider
	DeadlineSeconds int64
	MaxDisks        int
}

// RestoreStorageProvider is the narrow storage-owner integration point. Load
// must use current verified configuration, not request flags or storage names.
type RestoreStorageProvider interface {
	Load(context.Context, Scope) (RestoreStorageDependencies, error)
}

// Ready rejects incomplete dependencies before any operation can acquire a fence.
func (d RestoreStorageDependencies) Ready() bool {
	return d.Capacity != nil && d.Certification.Capability.ROXVerified && d.Certification.Capability.SparsePersistenceVerified && d.Certification.DeleteReclaimVerified && d.DeadlineSeconds > 0 && d.DeadlineSeconds <= 86400 && d.MaxDisks > 0 && d.MaxDisks <= 128
}
