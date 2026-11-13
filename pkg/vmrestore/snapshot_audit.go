package vmrestore

import (
	"time"

	dbmodel "github.com/goodrain/rainbond/db/model"
)

// snapshotEventID makes retries reuse one existing-platform event, not duplicate
// checking rows or an early SYNEVENTTYPE success merely because HTTP was202.
func snapshotEventID(scope Scope, operationID string) string {
	hash, _ := snapshotDigest(struct {
		Scope       Scope
		OperationID string
	}{scope, operationID})
	return hash[:32]
}
func (e *SnapshotEngine) audit(scope Scope, operationID, operator, status string) error {
	if len(operator) > 40 {
		return ErrSnapshotInvalid
	}
	tx := e.store.db.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer tx.Rollback()
	locked, err := e.store.loadLocked(tx, scope, operationID)
	if err != nil {
		return err
	}
	if locked.current.Receipt == nil || locked.current.Receipt.Snapshot == nil {
		return ErrConflict
	}
	eventID := snapshotEventID(scope, operationID)
	var rows []dbmodel.ServiceEvent
	if err := lockedQuery(tx).Where("event_id = ?", eventID).Find(&rows).Error; err != nil {
		return err
	}
	if len(rows) > 1 {
		return ErrCorrupt
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if len(rows) == 0 {
		event := dbmodel.ServiceEvent{EventID: eventID, TenantID: scope.TenantID, ServiceID: scope.ServiceID, Target: dbmodel.TargetTypeService, TargetID: scope.ServiceID, UserName: operator, CreateTime: now, StartTime: now, SynType: dbmodel.ASYNEVENTTYPE, OptType: "snapshot-vm", Status: "checking"}
		if status != "" {
			event.Status = status
			event.FinalStatus = "complete"
			event.EndTime = now
		}
		if err := tx.Create(&event).Error; err != nil {
			return err
		}
	} else {
		event := rows[0]
		if event.TenantID != scope.TenantID || event.TargetID != scope.ServiceID || event.OptType != "snapshot-vm" {
			return ErrConflict
		}
		if status != "" {
			if err := tx.Model(&dbmodel.ServiceEvent{}).Where("ID = ? AND event_id = ?", event.ID, eventID).Updates(map[string]interface{}{"status": status, "final_status": "complete", "end_time": now}).Error; err != nil {
				return err
			}
		}
	}
	return tx.Commit().Error
}
