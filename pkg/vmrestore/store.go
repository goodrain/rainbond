package vmrestore

import (
	"encoding/json"
	"reflect"
	"strings"

	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
)

// Store persists Region control state using the existing component attribute table.
// Callers which already hold cleanup locks must acquire them before service locks.
type Store struct{ db *gorm.DB }

// NewStore does not change schema or create rows.
func NewStore(database *gorm.DB) *Store { return &Store{db: database} }
func lockedQuery(tx *gorm.DB) *gorm.DB {
	if tx.Dialect().GetName() == "sqlite3" {
		return tx
	}
	return tx.Set("gorm:query_option", "FOR UPDATE")
}
func beginSQLiteWrite(tx *gorm.DB) error {
	if tx.Dialect().GetName() != "sqlite3" {
		return nil
	}
	return tx.Exec("UPDATE tenant_services SET service_id=service_id WHERE 1=0").Error
}

type lockedState struct {
	current Current
	rows    map[string]dbmodel.ComponentK8sAttributes
}

func requiredRecordFields(value []byte, names ...string) (map[string]json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(value, &fields); err != nil {
		return nil, ErrCorrupt
	}
	for _, name := range names {
		raw, exists := fields[name]
		if !exists || strings.TrimSpace(string(raw)) == "null" {
			return nil, ErrCorrupt
		}
	}
	return fields, nil
}

func decodeStoredState(value string, scope Scope) (State, error) {
	var state State
	if err := DecodeRecord(value, &state); err != nil {
		return state, err
	}
	if _, err := requiredRecordFields([]byte(value), "schema_version", "scope", "control_revision", "binding_generation", "bindings"); err != nil {
		return state, err
	}
	if err := state.Validate(scope); err != nil {
		return state, err
	}
	// Only an absent ledger can have revision zero; every Store write advances it.
	if state.ControlRevision == 0 {
		return state, ErrCorrupt
	}
	return state, nil
}

func decodeStoredReceipt(row dbmodel.ComponentK8sAttributes, scope Scope, operationID string) (*Receipt, error) {
	key, err := ReceiptKey(operationID)
	if err != nil || row.TenantID != scope.TenantID || row.ComponentID != scope.ServiceID || row.Name != key {
		return nil, ErrCorrupt
	}
	var receipt Receipt
	if err := DecodeRecord(row.AttributeValue, &receipt); err != nil {
		return nil, err
	}
	fields, err := requiredRecordFields([]byte(row.AttributeValue), "schema_version", "scope", "operation_id", "manifest_hash", "old_bindings", "new_bindings", "steps", "retain_old_pvcs", "snapshot_reference_released")
	if err != nil {
		return nil, err
	}
	var steps map[string]json.RawMessage
	if err := json.Unmarshal(fields["steps"], &steps); err != nil {
		return nil, ErrCorrupt
	}
	for _, value := range steps {
		if _, err := requiredRecordFields(value, "request_hash", "control_revision", "binding_generation", "applied"); err != nil {
			return nil, err
		}
	}
	if err := receipt.Validate(scope, operationID); err != nil {
		return nil, err
	}
	return &receipt, nil
}

func validateSuccessfulSteps(receipt *Receipt, state State) error {
	if len(receipt.Steps) == 0 {
		return ErrCorrupt
	}
	revisions := make(map[uint64]bool, len(receipt.Steps))
	for _, step := range receipt.Steps {
		if !step.Applied || step.ErrorCode != "" || step.ControlRevision == 0 || step.ControlRevision > state.ControlRevision || step.BindingGeneration > state.BindingGeneration || step.BindingGeneration > step.ControlRevision {
			return ErrCorrupt
		}
		if step.ControlRevision == state.ControlRevision && step.BindingGeneration != state.BindingGeneration {
			return ErrCorrupt
		}
		if revisions[step.ControlRevision] {
			return ErrCorrupt
		}
		revisions[step.ControlRevision] = true
	}
	return nil
}

func (s *Store) requirePristineLedger(query *gorm.DB, scope Scope) error {
	// Read this component's metadata only and classify actual names in Go. SQL
	// collation and LIKE wildcards must not turn ordinary lookalikes into a ledger.
	var history []dbmodel.ComponentK8sAttributes
	if err := query.Select("ID, tenant_id, component_id, name").Where("component_id = ?", scope.ServiceID).Order("ID").Find(&history).Error; err != nil {
		return err
	}
	for _, row := range history {
		if IsInternalAttribute(row.Name) {
			return ErrCorrupt
		}
	}
	return nil
}

func (s *Store) validateFenceReceipt(query *gorm.DB, result lockedState, scope Scope) error {
	fence := result.current.State.Fence
	if fence == nil {
		return nil
	}
	key, err := ReceiptKey(fence.OperationID)
	if err != nil {
		return err
	}
	row, exists := result.rows[key]
	if !exists {
		var rows []dbmodel.ComponentK8sAttributes
		if err := query.Where("component_id = ? AND name = ?", scope.ServiceID, key).Order("ID").Find(&rows).Error; err != nil {
			return err
		}
		if len(rows) != 1 {
			return ErrCorrupt
		}
		row = rows[0]
	}
	receipt, err := decodeStoredReceipt(row, scope, fence.OperationID)
	if err != nil {
		return err
	}
	if receipt.ManifestHash != fence.ManifestHash {
		return ErrCorrupt
	}
	return validateSuccessfulSteps(receipt, result.current.State)
}

func (s *Store) loadLocked(tx *gorm.DB, scope Scope, operationID string) (lockedState, error) {
	return s.loadState(tx, scope, operationID, true)
}

func (s *Store) loadState(tx *gorm.DB, scope Scope, operationID string, lockRows bool) (lockedState, error) {
	result := lockedState{rows: map[string]dbmodel.ComponentK8sAttributes{}}
	if !validScope(scope) {
		return result, ErrCorrupt
	}
	query := tx
	if lockRows {
		if err := beginSQLiteWrite(tx); err != nil {
			return result, err
		}
		query = lockedQuery(tx)
	}
	// Read all matching service rows. There is no declared unique service UUID index.
	var services []dbmodel.TenantServices
	if err := query.Where("service_id = ?", scope.ServiceID).Order("ID").Find(&services).Error; err != nil {
		return result, err
	}
	if len(services) == 0 {
		return result, ErrNotFound
	}
	if len(services) != 1 || services[0].TenantID != scope.TenantID || !services[0].IsVM() {
		return result, ErrCorrupt
	}
	var tenants []dbmodel.Tenants
	if err := tx.Select("uuid, namespace").Where("uuid = ?", scope.TenantID).Find(&tenants).Error; err != nil {
		return result, err
	}
	if len(tenants) != 1 || tenants[0].Namespace != scope.Namespace {
		return result, ErrCorrupt
	}
	names := []string{StateKey, LegacyBindingsKey}
	if operationID != "" {
		key, err := ReceiptKey(operationID)
		if err != nil {
			return result, err
		}
		names = append(names, key)
	}
	var rows []dbmodel.ComponentK8sAttributes
	if err := query.Where("component_id = ? AND name IN (?)", scope.ServiceID, names).Order("name, ID").Find(&rows).Error; err != nil {
		return result, err
	}
	for _, row := range rows {
		if row.TenantID != scope.TenantID || row.ComponentID != scope.ServiceID {
			return result, ErrCorrupt
		}
		if _, exists := result.rows[row.Name]; exists {
			return result, ErrCorrupt
		}
		result.rows[row.Name] = row
		switch {
		case row.Name == StateKey:
			state, err := decodeStoredState(row.AttributeValue, scope)
			if err != nil {
				return result, err
			}
			result.current.State = state
		case row.Name == LegacyBindingsKey:
			// This feature has no deployed legacy ledger. Never silently ignore one.
			return result, ErrCorrupt
		case strings.HasPrefix(row.Name, ReceiptPrefix):
			receipt, err := decodeStoredReceipt(row, scope, operationID)
			if err != nil {
				return result, err
			}
			result.current.Receipt = receipt
		default:
			return result, ErrCorrupt
		}
	}
	if _, exists := result.rows[StateKey]; !exists {
		if result.current.Receipt != nil {
			return result, ErrCorrupt
		}
		if err := s.requirePristineLedger(query, scope); err != nil {
			return result, err
		}
		result.current.State = NewState(scope)
	}
	if result.current.Receipt != nil {
		if err := validateSuccessfulSteps(result.current.Receipt, result.current.State); err != nil {
			return result, err
		}
	}
	if err := s.validateFenceReceipt(query, result, scope); err != nil {
		return result, err
	}
	if err := s.validateStartupReceipt(query, result.current.State, scope); err != nil {
		return result, err
	}
	return result, nil
}

// Read obtains a consistent short transaction; no Kubernetes operation runs here.
func (s *Store) Read(scope Scope, operationID string) (Current, error) {
	return s.read(scope, operationID, true)
}

// ReadOnly validates the same ledger in a short transaction using only SELECTs.
// It neither acquires writer locks nor reserves authority for a later mutation.
func (s *Store) ReadOnly(scope Scope, operationID string) (Current, error) {
	return s.read(scope, operationID, false)
}

func (s *Store) read(scope Scope, operationID string, lockRows bool) (Current, error) {
	tx := s.db.Begin()
	if tx.Error != nil {
		return Current{}, tx.Error
	}
	defer tx.Rollback()
	locked, err := s.loadState(tx, scope, operationID, lockRows)
	if err != nil {
		return Current{}, err
	}
	if err := tx.Commit().Error; err != nil {
		return Current{}, err
	}
	return locked.current, nil
}

// Mutate owns a transaction. MutateTx joins its caller and never commits it.
func (s *Store) Mutate(scope Scope, expected Expectation, change func(Current) (Mutation, error)) (Current, error) {
	tx := s.db.Begin()
	if tx.Error != nil {
		return Current{}, tx.Error
	}
	defer tx.Rollback()
	result, err := s.MutateTx(tx, scope, expected, change)
	if err != nil {
		return Current{}, err
	}
	if err := tx.Commit().Error; err != nil {
		return Current{}, err
	}
	return result, nil
}

func (s *Store) saveRow(tx *gorm.DB, scope Scope, name, value string, old map[string]dbmodel.ComponentK8sAttributes) error {
	if row, exists := old[name]; exists {
		q := tx.Model(&dbmodel.ComponentK8sAttributes{}).Where("ID = ? AND tenant_id = ? AND component_id = ? AND name = ?", row.ID, scope.TenantID, scope.ServiceID, name).Updates(map[string]interface{}{"attribute_value": value, "save_type": "json"})
		if q.Error != nil {
			return q.Error
		}
		// A changed control revision guarantees an actual state-row change. A receipt
		// may remain byte-identical on an ordinary control update; verify it still exists.
		if q.RowsAffected == 0 {
			var count int
			if err := tx.Model(&dbmodel.ComponentK8sAttributes{}).Where("ID = ? AND tenant_id = ? AND component_id = ? AND name = ?", row.ID, scope.TenantID, scope.ServiceID, name).Count(&count).Error; err != nil {
				return err
			}
			if count != 1 {
				return ErrConflict
			}
		}
		return nil
	}
	return tx.Create(&dbmodel.ComponentK8sAttributes{TenantID: scope.TenantID, ComponentID: scope.ServiceID, Name: name, SaveType: "json", AttributeValue: value}).Error
}

// MutateTx only calls the supplied pure database-state function while locks are held.
func (s *Store) MutateTx(tx *gorm.DB, scope Scope, expected Expectation, change func(Current) (Mutation, error)) (Current, error) {
	if tx == nil || change == nil {
		return Current{}, ErrCorrupt
	}
	if expected.AdmissionOwner != "" && (expected.OperationID != "" || !idPattern.MatchString(expected.AdmissionOwner)) {
		return Current{}, ErrCorrupt
	}
	locked, err := s.loadLocked(tx, scope, expected.OperationID)
	if err != nil {
		return Current{}, err
	}
	before := locked.current
	if expected.OperationID != "" {
		if !hashPattern.MatchString(expected.ManifestHash) || !hashPattern.MatchString(expected.RequestHash) || !idPattern.MatchString(expected.Step) {
			return Current{}, ErrCorrupt
		}
		if before.Receipt != nil {
			if before.Receipt.ManifestHash != expected.ManifestHash {
				return Current{}, ErrConflict
			}
			if step, ok := before.Receipt.Steps[expected.Step]; ok {
				if step.RequestHash != expected.RequestHash {
					return Current{}, ErrConflict
				}
				before.Replayed = true
				return before, nil
			}
		}
	}
	// Historical successful steps may replay above. Other writes, including
	// ordinary callers without an operation ID, cannot bypass another owner's fence.
	if before.State.Fence != nil && before.State.Fence.OperationID != expected.OperationID {
		return Current{}, ErrBusy
	}
	owner := expected.OperationID
	if owner == "" {
		owner = expected.AdmissionOwner
	}
	if before.State.Admission != nil && before.State.Admission.Token != owner {
		return Current{}, ErrBusy
	}
	if before.State.ControlRevision != expected.ControlRevision || before.State.BindingGeneration != expected.BindingGeneration {
		return Current{}, ErrConflict
	}
	encoded, err := json.Marshal(before)
	if err != nil {
		return Current{}, ErrCorrupt
	}
	var copy Current
	if err = json.Unmarshal(encoded, &copy); err != nil {
		return Current{}, ErrCorrupt
	}
	update, err := change(copy)
	if err != nil {
		return Current{}, err
	}
	if update.State.ControlRevision != before.State.ControlRevision || update.State.ControlRevision == ^uint64(0) {
		return Current{}, ErrConflict
	}
	if update.State.BindingGeneration < before.State.BindingGeneration || update.State.BindingGeneration-before.State.BindingGeneration > 1 {
		return Current{}, ErrConflict
	}
	if !reflect.DeepEqual(update.State.Bindings, before.State.Bindings) && update.State.BindingGeneration != before.State.BindingGeneration+1 {
		return Current{}, ErrConflict
	}
	if before.State.StartupOperationID != "" && update.State.StartupOperationID != before.State.StartupOperationID {
		return Current{}, ErrConflict
	}
	update.State.ControlRevision++
	if err := update.State.Validate(scope); err != nil {
		return Current{}, err
	}
	if expected.OperationID == "" {
		if update.Receipt != nil {
			return Current{}, ErrCorrupt
		}
	} else {
		if update.Receipt == nil || update.Receipt.OperationID != expected.OperationID || update.Receipt.ManifestHash != expected.ManifestHash {
			return Current{}, ErrCorrupt
		}
		if err := update.Receipt.Validate(scope, expected.OperationID); err != nil {
			return Current{}, err
		}
		if before.Receipt != nil && !reflect.DeepEqual(update.Receipt.OldBindings, before.Receipt.OldBindings) {
			return Current{}, ErrConflict
		}
		if before.Receipt != nil && !sameRestoreIntent(before.Receipt.Restore, update.Receipt.Restore) {
			return Current{}, ErrConflict
		}
		if before.Receipt != nil && !reflect.DeepEqual(before.Receipt.NativeRestore, update.Receipt.NativeRestore) {
			return Current{}, ErrConflict
		}
		if before.Receipt != nil {
			old := before.Receipt
			next := update.Receipt
			if old.ConfigCommitHash != "" && (old.ConfigCommitHash != next.ConfigCommitHash || !reflect.DeepEqual(old.NewBindings, next.NewBindings)) {
				return Current{}, ErrConflict
			}
			for _, pair := range [][2]string{{old.ConsoleAckHash, next.ConsoleAckHash}, {old.RollbackConfigHash, next.RollbackConfigHash}, {old.RollbackAckHash, next.RollbackAckHash}} {
				if pair[0] != "" && pair[0] != pair[1] {
					return Current{}, ErrConflict
				}
			}
		}
		if before.Receipt != nil && !validExecutionTransition(before.Receipt.Execution, update.Receipt.Execution) {
			return Current{}, ErrConflict
		}
		if before.Receipt != nil && before.Receipt.PowerEventID != update.Receipt.PowerEventID {
			return Current{}, ErrConflict
		}
		if before.Receipt != nil && !reflect.DeepEqual(before.Receipt.Startup, update.Receipt.Startup) {
			return Current{}, ErrConflict
		}
		if before.Receipt != nil && !validSnapshotIdentityTransition(before.Receipt.SnapshotIdentity, update.Receipt.SnapshotIdentity) {
			return Current{}, ErrConflict
		}
		// The callback may neither rewrite existing history nor pre-seal any other
		// step. Store alone appends the successful step for this transaction.
		if before.Receipt == nil {
			if len(update.Receipt.Steps) != 0 {
				return Current{}, ErrConflict
			}
		} else if !reflect.DeepEqual(update.Receipt.Steps, before.Receipt.Steps) {
			return Current{}, ErrConflict
		}
		update.Receipt.Steps[expected.Step] = StepResult{RequestHash: expected.RequestHash, ControlRevision: update.State.ControlRevision, BindingGeneration: update.State.BindingGeneration, Resources: executionStepResources(update.Receipt, expected.Step), Applied: true}
		if err := update.Receipt.Validate(scope, expected.OperationID); err != nil {
			return Current{}, err
		}
		if err := validateSuccessfulSteps(update.Receipt, update.State); err != nil {
			return Current{}, err
		}
	}
	// Validate the prospective pair before either row is saved. A malformed
	// callback must not commit a ledger which loadLocked would reject next time.
	if fence := update.State.Fence; fence != nil {
		if update.Receipt == nil || expected.OperationID != fence.OperationID || expected.ManifestHash != fence.ManifestHash || update.Receipt.OperationID != fence.OperationID || update.Receipt.ManifestHash != fence.ManifestHash {
			return Current{}, ErrConflict
		}
	}
	stateValue, err := EncodeRecord(update.State)
	if err != nil {
		return Current{}, err
	}
	receiptKey, receiptValue := "", ""
	if update.Receipt != nil {
		receiptKey, _ = ReceiptKey(expected.OperationID)
		receiptValue, err = EncodeRecord(update.Receipt)
		if err != nil {
			return Current{}, err
		}
	}
	if err := s.saveRow(tx, scope, StateKey, stateValue, locked.rows); err != nil {
		return Current{}, err
	}
	if update.Receipt != nil {
		if err := s.saveRow(tx, scope, receiptKey, receiptValue, locked.rows); err != nil {
			return Current{}, err
		}
	}
	return Current{State: update.State, Receipt: update.Receipt}, nil
}
