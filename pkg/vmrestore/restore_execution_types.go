package vmrestore

import (
	"context"
	"errors"

	"github.com/goodrain/rainbond/pkg/vmrestore/restoreplan"
	"github.com/google/uuid"
)

var (
	// ErrRestorePending means the exact resource is not yet observed ready.
	ErrRestorePending = errors.New("restore_resource_pending")
	// ErrRestoreUnknown retains the fence until the exact intent is reconciled.
	ErrRestoreUnknown = errors.New("restore_resource_outcome_unknown")
	// ErrRestoreCapability rejects an unqualified provider path.
	ErrRestoreCapability = errors.New("restore_provider_not_certified")
)

// RestoreObjectIntent survives an API timeout; its nonce and exact desired hash
// permit recovery only of that attempted create, never a same-name replacement.
type RestoreObjectIntent struct {
	Key             string     `json:"key"`
	Nonce           string     `json:"nonce"`
	DesiredHash     string     `json:"desired_hash"`
	Object          ObjectRef  `json:"object"`
	Purpose         string     `json:"purpose"`
	DeleteRequested bool       `json:"delete_requested"`
	Deleted         bool       `json:"deleted"`
	BoundPV         *ObjectRef `json:"bound_pv,omitempty"`
}

// RestoreStartupPolicy contains identity/provenance only, never bootstrap contents.
// Both A and B are pinned: B can itself be an earlier restored generation.
type RestoreStartupPolicy struct {
	FirmwareUUID      string                  `json:"firmware_uuid"`
	BootstrapMode     string                  `json:"bootstrap_mode"`
	NoCloudVolumeName string                  `json:"nocloud_volume_name"`
	NoCloudDiskName   string                  `json:"nocloud_disk_name"`
	DataSourceKind    string                  `json:"datasource_kind"`
	ProducerUID       string                  `json:"producer_uid"`
	RestoreID         string                  `json:"restore_id,omitempty"`
	SnapshotUID       string                  `json:"snapshot_uid,omitempty"`
	GeneratedMedia    []RestoreGeneratedMedia `json:"generated_media"`
}

// RestoreGeneratedMedia pins a trusted producer media identity without contents.
type RestoreGeneratedMedia struct {
	VolumeName  string    `json:"volume_name"`
	DiskName    string    `json:"disk_name"`
	Source      ObjectRef `json:"source"`
	ProducerUID string    `json:"producer_uid"`
}

func (p RestoreStartupPolicy) validate(scope Scope) error {
	if _, err := uuid.Parse(p.FirmwareUUID); err != nil {
		return ErrRestoreCapability
	}
	knownSource := (p.DataSourceKind == "CloudInitNoCloud" && validName(p.NoCloudVolumeName) && validName(p.NoCloudDiskName)) || (p.DataSourceKind == "None" && p.NoCloudVolumeName == "" && p.NoCloudDiskName == "")
	if (p.BootstrapMode != "platform" && p.BootstrapMode != "restored") || !knownSource || !validUID(p.ProducerUID) || (p.RestoreID == "") != (p.SnapshotUID == "") || (p.RestoreID != "" && (!idPattern.MatchString(p.RestoreID) || !validUID(p.SnapshotUID))) || len(p.GeneratedMedia) > 128 {
		return ErrRestoreCapability
	}
	seen := map[string]bool{}
	for _, m := range p.GeneratedMedia {
		if seen[m.VolumeName] || !validName(m.VolumeName) || !validName(m.DiskName) || !validUID(m.ProducerUID) || m.Source.Namespace != scope.Namespace || !validName(m.Source.Name) || !validUID(m.Source.UID) || (m.Source.Kind != "ConfigMap" && m.Source.Kind != "Secret") {
			return ErrRestoreCapability
		}
		seen[m.VolumeName] = true
	}
	return nil
}

// RestoreExecution is a Region resource receipt; Console owns workflow scheduling.
type RestoreExecution struct {
	ConfigAfter   *RestoreConfig                      `json:"config_after,omitempty"`
	BeforeStartup RestoreStartupPolicy                `json:"before_startup"`
	SourceStartup RestoreStartupPolicy                `json:"source_startup"`
	Frame         RestoreResourcePlan                 `json:"frame"`
	Intents       map[string]RestoreObjectIntent      `json:"intents"`
	Evidence      map[string]restoreplan.CopyEvidence `json:"evidence"`
	NetworkHash   string                              `json:"network_hash"`
	IdentityHash  string                              `json:"identity_hash"`
}

func (e RestoreExecution) validate(scope Scope, operationID, coreHash string) error {
	if e.ConfigAfter != nil && e.ConfigAfter.validate(scope) != nil {
		return ErrCorrupt
	}
	identityHash, _ := snapshotDigest(e.SourceStartup)
	if e.IdentityHash != identityHash {
		return ErrCorrupt
	}
	if e.BeforeStartup.validate(scope) != nil || e.SourceStartup.validate(scope) != nil || e.SourceStartup.BootstrapMode != "restored" || e.Frame.validate() != nil || e.Frame.Scope != scope || e.Frame.OperationID != operationID || e.Frame.CoreHash != coreHash || !hashPattern.MatchString(e.NetworkHash) || !hashPattern.MatchString(e.IdentityHash) || e.Intents == nil || e.Evidence == nil || len(e.Intents) > 2048 || len(e.Evidence) > 3 {
		return ErrCorrupt
	}
	for key, v := range e.Intents {
		if _, ok := restoreGVR[v.Object.Kind]; !ok {
			return ErrCorrupt
		}
		if key != v.Key || !idPattern.MatchString(key) || !validUID(v.Nonce) || !hashPattern.MatchString(v.DesiredHash) || v.Object.Namespace != scope.Namespace || !validName(v.Object.Name) || v.Purpose == "" || v.Object.Kind == "" || (v.Object.UID != "" && !validUID(v.Object.UID)) || (v.Deleted && !v.DeleteRequested) {
			return ErrCorrupt
		}
		if v.BoundPV != nil && (v.BoundPV.Kind != "PersistentVolume" || v.BoundPV.Namespace != "" || !validName(v.BoundPV.Name) || !validUID(v.BoundPV.UID)) {
			return ErrCorrupt
		}
	}
	for mode, v := range e.Evidence {
		if mode != "inspect" && mode != "copy" && mode != "verify" {
			return ErrCorrupt
		}
		if v.Identity.Namespace != scope.Namespace || !hashPattern.MatchString(v.LogSHA256) || v.LogBytes < 0 {
			return ErrCorrupt
		}
	}
	return nil
}

// RestoreRuntime is implemented by the normal lifecycle group, using the same
// Resolver and admission through final asynchronous callbacks.
type RestoreRuntime interface {
	Observe(context.Context, RestoreRuntimeRequest) (RestoreRuntimeObservation, error)
	Apply(context.Context, RestoreRuntimeRequest) (RestoreRuntimeObservation, error)
}

// RestoreRuntimeRequest is an internal persisted lifecycle action frame.
type RestoreRuntimeRequest struct {
	Startup                                 RestoreStartupPolicy
	Scope                                   Scope
	OperationID, CoreHash, Action, IntentID string
	BindingGeneration                       uint64
	VM                                      ObjectRef
	ConfigHash                              string
}

// RestoreRuntimeObservation carries safe exact-identity readiness evidence.
type RestoreRuntimeObservation struct {
	VM                                            ObjectRef
	VMResourceVersion                             string
	VMGeneration, ObservedGeneration              int64
	BindingGeneration                             uint64
	Halted, Running, WritersClear, DesiredMatches bool
	NetworkHash, IdentityHash                     string
}
