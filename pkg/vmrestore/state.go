// Package vmrestore contains the Region-owned VM restore control and bindings.
// It does not run a restore workflow or store the Console snapshot manifest.
package vmrestore

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation"
)

// SchemaVersion and the following keys define the bounded persisted records.
const (
	SchemaVersion     = 1
	StateKey          = "vm_restore_state"
	ReceiptPrefix     = "vm_restore_receipt_"
	LegacyBindingsKey = "vm_volume_bindings"
	MaxRecordBytes    = 512 << 10
)

// ErrConflict and the following errors distinguish rejected state transitions.
var (
	ErrConflict          = errors.New("VM restore state conflict")
	ErrCorrupt           = errors.New("VM restore state is invalid")
	ErrDetached          = errors.New("VM disk is intentionally detached")
	ErrBusy              = errors.New("VM operation is in progress")
	ErrNotFound          = errors.New("VM component does not exist")
	ErrInternalAttribute = errors.New("VM restore internal attributes are not public")
	idPattern            = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)
	hashPattern          = regexp.MustCompile(`^[a-f0-9]{64}$`)
)

// Scope is derived from authenticated component context, never from a UI body.
type Scope struct {
	TenantID  string `json:"tenant_id"`
	ServiceID string `json:"service_id"`
	Namespace string `json:"namespace"`
}

// ObjectRef records a server-observed Kubernetes identity. UIDs are opaque.
type ObjectRef struct {
	Kind      string `json:"kind"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	UID       string `json:"uid"`
}

// Binding retains canonical device identity independently of the active PVC name.
type Binding struct {
	Role          string     `json:"role"`
	Attached      bool       `json:"attached"`
	ActiveDV      *ObjectRef `json:"active_dv,omitempty"`
	PreviousDV    *ObjectRef `json:"previous_dv,omitempty"`
	VolumeID      uint       `json:"volume_id"`
	DiskKey       string     `json:"disk_key"`
	CanonicalName string     `json:"canonical_name"`
	ActivePVC     ObjectRef  `json:"active_pvc"`
	PreviousPVC   *ObjectRef `json:"previous_pvc,omitempty"`
	RestoreID     string     `json:"restore_id"`
	SnapshotUID   string     `json:"snapshot_uid"`
}

// Fence cannot expire into permission to start or delete the VM.
type Fence struct {
	Kind           string `json:"kind"`
	Phase          string `json:"phase"`
	TargetVMAbsent bool   `json:"target_vm_absent,omitempty"`
	OperationID    string `json:"operation_id"`
	ManifestHash   string `json:"manifest_hash"`
	TargetVMUID    string `json:"target_vm_uid"`
}

// Admission owns an ordinary operation through its final asynchronous callback.
type Admission struct {
	Token          string `json:"token"`
	Action         string `json:"action"`
	OutcomeUnknown bool   `json:"outcome_unknown,omitempty"`
}

// State is one durable row; idle rows retain their revision to avoid ABA.
type State struct {
	StartupOperationID string             `json:"startup_operation_id,omitempty"`
	SchemaVersion      int                `json:"schema_version"`
	Scope              Scope              `json:"scope"`
	ControlRevision    uint64             `json:"control_revision"`
	BindingGeneration  uint64             `json:"binding_generation"`
	Bindings           map[string]Binding `json:"bindings"`
	Fence              *Fence             `json:"fence,omitempty"`
	Admission          *Admission         `json:"admission,omitempty"`
}

// ResourceRef records an observed identity and its Region-owned purpose.
type ResourceRef struct {
	ObjectRef
	Purpose string `json:"purpose"`
}

// StepResult records an applied Region-side effect, not running resource readiness.
type StepResult struct {
	RequestHash       string        `json:"request_hash"`
	ControlRevision   uint64        `json:"control_revision"`
	BindingGeneration uint64        `json:"binding_generation"`
	Resources         []ResourceRef `json:"resources,omitempty"`
	Applied           bool          `json:"applied"`
	ErrorCode         string        `json:"error_code,omitempty"`
}

// Receipt is Region resource ownership and step idempotency, not the total workflow.
type Receipt struct {
	NativeRestore             *NativeRestoreControl     `json:"native_restore,omitempty"`
	Startup                   *VMStartupProof           `json:"startup,omitempty"`
	SnapshotIdentity          *SnapshotFirmwareIdentity `json:"snapshot_identity,omitempty"`
	PowerEventID              string                    `json:"power_event_id,omitempty"`
	Execution                 *RestoreExecution         `json:"execution,omitempty"`
	Restore                   *RestoreIntent            `json:"restore,omitempty"`
	RollbackConfigHash        string                    `json:"rollback_config_hash,omitempty"`
	RollbackAckHash           string                    `json:"rollback_ack_hash,omitempty"`
	Snapshot                  *SnapshotIntent           `json:"snapshot,omitempty"`
	ManifestID                string                    `json:"manifest_id,omitempty"`
	ConfigCommitHash          string                    `json:"config_commit_hash,omitempty"`
	ConsoleAckHash            string                    `json:"console_ack_hash,omitempty"`
	SchemaVersion             int                       `json:"schema_version"`
	Scope                     Scope                     `json:"scope"`
	OperationID               string                    `json:"operation_id"`
	ManifestHash              string                    `json:"manifest_hash"`
	OldBindings               map[string]Binding        `json:"old_bindings"`
	NewBindings               map[string]Binding        `json:"new_bindings"`
	Resources                 []ResourceRef             `json:"resources,omitempty"`
	Steps                     map[string]StepResult     `json:"steps"`
	RetainOldPVCs             bool                      `json:"retain_old_pvcs"`
	SnapshotReferenceReleased bool                      `json:"snapshot_reference_released"`
}

// Expectation protects an entire short Region step.
type Expectation struct {
	// AdmissionOwner carries an already-held server-side owner for state-only
	// updates. It is never persisted or accepted from a public request DTO.
	AdmissionOwner    string
	ControlRevision   uint64
	BindingGeneration uint64
	OperationID       string
	ManifestHash      string
	Step              string
	RequestHash       string
}

// Current is returned by one consistent Region read.
type Current struct {
	State    State
	Receipt  *Receipt
	Replayed bool
}

// Mutation must contain only database changes; callbacks must not perform I/O.
type Mutation struct {
	State   State
	Receipt *Receipt
}

// NewState is an empty binding map for a component which has never restored.
func NewState(scope Scope) State {
	return State{SchemaVersion: SchemaVersion, Scope: scope, Bindings: map[string]Binding{}}
}

// IsInternalAttribute reserves all restore keys, including future receipt names.
func IsInternalAttribute(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	return name == LegacyBindingsKey || strings.HasPrefix(name, "vm_restore_")
}

// NewReceipt initializes the bounded per-operation resource receipt.
func NewReceipt(scope Scope, operationID, manifestHash string) Receipt {
	return Receipt{SchemaVersion: SchemaVersion, Scope: scope, OperationID: operationID, ManifestHash: manifestHash, OldBindings: map[string]Binding{}, NewBindings: map[string]Binding{}, Steps: map[string]StepResult{}}
}

// CheckPublicAttribute prevents both reading and writing internal state via CRUD.
func CheckPublicAttribute(name string) error {
	if IsInternalAttribute(name) {
		return ErrInternalAttribute
	}
	return nil
}

// ReceiptKey accepts backend-generated bounded IDs, not arbitrary attribute paths.
func ReceiptKey(operationID string) (string, error) {
	if !idPattern.MatchString(operationID) {
		return "", ErrCorrupt
	}
	return ReceiptPrefix + operationID, nil
}
func validUID(uid string) bool {
	return len(uid) > 0 && len(uid) <= 128 && strings.TrimSpace(uid) == uid && !strings.ContainsAny(uid, "\r\n\x00")
}
func validScope(s Scope) bool {
	return validUID(s.TenantID) && validUID(s.ServiceID) && validNamespace(s.Namespace)
}
func validName(name string) bool {
	return len(validation.IsDNS1123Subdomain(name)) == 0
}
func validNamespace(namespace string) bool {
	return len(validation.IsDNS1123Label(namespace)) == 0
}
func validRef(r ObjectRef) bool {
	return r.Kind == "PersistentVolumeClaim" && validNamespace(r.Namespace) && validName(r.Name) && validUID(r.UID)
}
func validResourceRef(r ResourceRef, scope Scope) bool {
	if !validName(r.Name) || !validUID(r.UID) || !idPattern.MatchString(r.Purpose) {
		return false
	}
	switch r.Kind {
	case "PersistentVolumeClaim", "DataVolume", "VirtualMachine", "VirtualMachineInstance", "Pod", "Job", "ConfigMap", "VirtualMachineSnapshot", "VirtualMachineRestore", "VirtualMachineSnapshotContent", "VolumeSnapshot":
		return r.Namespace == scope.Namespace
	case "PersistentVolume", "StorageClass", "VolumeSnapshotContent":
		return r.Namespace == ""
	default:
		return false
	}
}
func validateBindings(bindings map[string]Binding) error {
	if bindings == nil {
		return ErrCorrupt
	}
	for key, b := range bindings {
		parsed, err := strconv.ParseUint(key, 10, 32)
		if err != nil || parsed == 0 || key != strconv.FormatUint(parsed, 10) || uint(parsed) != b.VolumeID || b.CanonicalName != fmt.Sprintf("manual%d", b.VolumeID) || b.DiskKey == "" || !validRef(b.ActivePVC) {
			return ErrCorrupt
		}
		if (b.Role != "root" && b.Role != "data") || (b.Role == "root" && !b.Attached) {
			return ErrCorrupt
		}
		for _, dv := range []*ObjectRef{b.ActiveDV, b.PreviousDV} {
			if dv != nil && (dv.Kind != "DataVolume" || dv.Namespace != b.ActivePVC.Namespace || !validName(dv.Name) || !validUID(dv.UID)) {
				return ErrCorrupt
			}
		}
		if b.PreviousDV != nil && b.PreviousPVC == nil {
			return ErrCorrupt
		}
		if b.PreviousPVC != nil && (!validRef(*b.PreviousPVC) || b.PreviousPVC.Namespace != b.ActivePVC.Namespace || b.PreviousPVC.UID == b.ActivePVC.UID) {
			return ErrCorrupt
		}
		if (b.RestoreID == "") != (b.SnapshotUID == "") || (b.RestoreID != "" && (!idPattern.MatchString(b.RestoreID) || !validUID(b.SnapshotUID))) {
			return ErrCorrupt
		}
	}
	return nil
}

// Validate rejects damaged or cross-component state rather than selecting B.
func (s State) Validate(scope Scope) error {
	if s.StartupOperationID != "" && !idPattern.MatchString(s.StartupOperationID) {
		return ErrCorrupt
	}
	if s.SchemaVersion != SchemaVersion || s.Scope != scope || !validScope(scope) {
		return ErrCorrupt
	}
	if err := validateBindings(s.Bindings); err != nil {
		return err
	}
	for _, binding := range s.Bindings {
		if binding.ActivePVC.Namespace != scope.Namespace {
			return ErrCorrupt
		}
	}
	if s.Fence != nil && (!idPattern.MatchString(s.Fence.OperationID) || !hashPattern.MatchString(s.Fence.ManifestHash) || (s.Fence.Kind != "snapshot" && s.Fence.Kind != "restore" && s.Fence.Kind != "delete") || s.Fence.Phase == "" || (!s.Fence.TargetVMAbsent && !validUID(s.Fence.TargetVMUID)) || (s.Fence.TargetVMAbsent && s.Fence.TargetVMUID != "")) {
		return ErrCorrupt
	}
	if s.Admission != nil && (!idPattern.MatchString(s.Admission.Token) || s.Admission.Action == "") {
		return ErrCorrupt
	}
	if s.Fence != nil && s.Admission != nil {
		return ErrCorrupt
	}
	return nil
}

// Validate verifies a receipt belongs to the requested component and operation.
func (r Receipt) Validate(scope Scope, operationID string) error {
	if r.NativeRestore != nil && (r.Restore == nil || r.Execution != nil || r.NativeRestore.validate(scope, r.Restore) != nil) {
		return ErrCorrupt
	}
	if r.Startup != nil && r.Startup.validate(scope) != nil {
		return ErrCorrupt
	}
	if r.SnapshotIdentity != nil && (r.Snapshot == nil || r.SnapshotIdentity.validate(scope, r.Snapshot.SourceVM) != nil) {
		return ErrCorrupt
	}
	if r.PowerEventID != "" && (!idPattern.MatchString(r.PowerEventID) || len(r.PowerEventID) > 40) {
		return ErrCorrupt
	}
	if r.SchemaVersion != SchemaVersion || r.Scope != scope || !validScope(scope) || r.OperationID != operationID || !idPattern.MatchString(operationID) || !hashPattern.MatchString(r.ManifestHash) || r.Steps == nil {
		return ErrCorrupt
	}
	if r.Snapshot != nil {
		if r.Snapshot.OperationID != operationID {
			return ErrCorrupt
		}
		if err := validateSnapshotIntent(scope, *r.Snapshot); err != nil {
			return ErrCorrupt
		}
		core, err := SnapshotCoreHash(scope, *r.Snapshot)
		if err != nil || core != r.ManifestHash {
			return ErrCorrupt
		}
	}
	if err := validateBindings(r.OldBindings); err != nil {
		return err
	}
	if err := validateBindings(r.NewBindings); err != nil {
		return err
	}
	for _, bindings := range []map[string]Binding{r.OldBindings, r.NewBindings} {
		for _, binding := range bindings {
			if binding.ActivePVC.Namespace != scope.Namespace {
				return ErrCorrupt
			}
		}
	}
	for _, hash := range []string{r.ConfigCommitHash, r.ConsoleAckHash, r.RollbackConfigHash, r.RollbackAckHash} {
		if hash != "" && !hashPattern.MatchString(hash) {
			return ErrCorrupt
		}
	}
	if r.Restore != nil {
		if err := r.Restore.validate(scope); err != nil {
			return err
		}
	}
	if r.Execution != nil {
		if err := r.Execution.validate(scope, operationID, r.ManifestHash); err != nil {
			return err
		}
	}
	for _, resource := range r.Resources {
		if !validResourceRef(resource, scope) {
			return ErrCorrupt
		}
	}
	for step, result := range r.Steps {
		if !idPattern.MatchString(step) || !hashPattern.MatchString(result.RequestHash) {
			return ErrCorrupt
		}
		for _, resource := range result.Resources {
			if !validResourceRef(resource, scope) {
				return ErrCorrupt
			}
		}
	}
	return nil
}

// DecodeRecord is deliberately strict: unknown schemas cannot fall back to B.
func DecodeRecord(value string, target interface{}) error {
	if len(value) == 0 || len(value) > MaxRecordBytes {
		return ErrCorrupt
	}
	decoder := json.NewDecoder(bytes.NewBufferString(value))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return ErrCorrupt
	}
	if err := decoder.Decode(new(interface{})); err != io.EOF {
		return ErrCorrupt
	}
	return nil
}

// EncodeRecord enforces an explicit bound despite the existing longtext column.
func EncodeRecord(value interface{}) (string, error) {
	b, err := json.Marshal(value)
	if err != nil || len(b) > MaxRecordBytes {
		return "", ErrCorrupt
	}
	return string(b), nil
}

// ResolveClaim preserves manual<ID> as the device name while selecting A's PVC.
// This only resolves a persisted reference; the execution layer MUST check live UID.
func (s State) ResolveClaim(volumeID uint) (ObjectRef, bool, error) {
	if err := s.Validate(s.Scope); err != nil {
		return ObjectRef{}, false, err
	}
	b, ok := s.Bindings[strconv.FormatUint(uint64(volumeID), 10)]
	if !ok {
		return ObjectRef{}, false, nil
	}
	if !b.Attached {
		return ObjectRef{}, true, ErrDetached
	}
	return b.ActivePVC, true, nil
}

// CheckLivePVC is required at the execution layer immediately before use/delete.
func CheckLivePVC(expected ObjectRef, namespace, name, uid string) error {
	if !validRef(expected) || expected.Namespace != namespace || expected.Name != name || expected.UID != uid {
		return ErrConflict
	}
	return nil
}

// AdmitMutation is atomic only when invoked inside Store.MutateTx.
func (s *State) AdmitMutation(token, action string) error {
	if !idPattern.MatchString(token) || action == "" {
		return ErrCorrupt
	}
	if s.Fence != nil || s.Admission != nil {
		return ErrBusy
	}
	s.Admission = &Admission{Token: token, Action: action}
	return nil
}

// FinishMutation never releases another action's admission.
func (s *State) FinishMutation(token string, knownOutcome bool) error {
	if s.Admission == nil || s.Admission.Token != token {
		return ErrConflict
	}
	if !knownOutcome {
		s.Admission.OutcomeUnknown = true
		return nil
	}
	s.Admission = nil
	return nil
}

// Resolution is the only logical-to-physical binding interpretation.
type Resolution struct {
	Mapped        bool
	Attached      bool
	CanonicalName string
	PVC           *ObjectRef
	DV            *ObjectRef
	Role          string
}

// ResolveVolume must be shared by conversion, actual use, deletion and expansion.
func (s State) ResolveVolume(volumeID uint) (Resolution, error) {
	if err := s.Validate(s.Scope); err != nil {
		return Resolution{}, err
	}
	canonical := fmt.Sprintf("manual%d", volumeID)
	b, ok := s.Bindings[strconv.FormatUint(uint64(volumeID), 10)]
	if !ok {
		return Resolution{Mapped: false, Attached: true, CanonicalName: canonical}, nil
	}
	ref := b.ActivePVC
	var dv *ObjectRef
	if b.ActiveDV != nil {
		copy := *b.ActiveDV
		dv = &copy
	}
	return Resolution{Mapped: true, Attached: b.Attached, CanonicalName: b.CanonicalName, PVC: &ref, DV: dv, Role: b.Role}, nil
}
