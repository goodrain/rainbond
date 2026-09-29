package cleanup

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
)

// UploadSessionFingerprint binds the scanned session state, not caller-declared
// file size. Session status and chunk bookkeeping changes invalidate selection.
func UploadSessionFingerprint(session model.UploadSession) string {
	raw, _ := json.Marshal(struct {
		ID, EventID, Status, Chunks, Path string
		Created, Updated, Expires         time.Time
	}{session.ID, session.EventID, session.Status, session.UploadedChunks, session.StoragePath, session.CreatedAt.UTC(), session.UpdatedAt.UTC(), session.ExpiresAt.UTC()})
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// DeleteExpiredUploadChunks is an internal primitive for a verified caller.
// The caller must establish installed-writer coverage, storage binding and policy
// before calling. This primitive alone must never be used to advertise CanDelete.
// Removal runs once only after durable retirement; unknown effects are not replayed.
func DeleteExpiredUploadChunks(database *gorm.DB, operationID, eventID, sessionID, fingerprint string, remove func(string) error) (resultErr error) {
	if database == nil || remove == nil || !coordinationIdentity.MatchString(operationID) || !uploadReferenceID.MatchString(sessionID) || !uploadReferenceID.MatchString(eventID) || len(fingerprint) != 64 {
		return ErrCoordinationChanged
	}
	decoded, err := hex.DecodeString(fingerprint)
	if err != nil || hex.EncodeToString(decoded) != fingerprint {
		return ErrCoordinationChanged
	}
	fresh, err := retireExpiredUpload(database, operationID, eventID, sessionID, fingerprint)
	if err != nil || !fresh {
		return err
	}
	confirmed := false
	defer func() {
		state := "uncertain"
		if confirmed {
			state = "finished"
		}
		update := database.Model(&model.PackageUploadUse{}).Where("operation_id = ? AND session_id = ? AND event_id = ? AND action = ? AND fingerprint = ? AND state = ?", operationID, sessionID, eventID, "retire", fingerprint, "active").Update("state", state)
		if update.Error != nil || update.RowsAffected != 1 {
			resultErr = ErrCoordinationUncertain
		}
	}()
	if err := remove(sessionID); err != nil {
		return ErrCoordinationUncertain
	}
	confirmed = true
	return nil
}

func retireExpiredUpload(database *gorm.DB, operationID, eventID, sessionID, fingerprint string) (bool, error) {
	tx := database.Begin()
	if tx.Error != nil {
		return false, tx.Error
	}
	defer tx.Rollback()
	if err := tx.Model(&model.UploadSession{}).Where("id = ?", sessionID).UpdateColumn("status", gorm.Expr("status")).Error; err != nil {
		return false, err
	}
	var prior model.PackageUploadUse
	if err := tx.Where("operation_id = ?", operationID).First(&prior).Error; err == nil {
		if prior.SessionID != sessionID || prior.EventID != eventID || prior.Action != "retire" || prior.Fingerprint != fingerprint {
			return false, ErrCoordinationChanged
		}
		if prior.State == "finished" {
			return false, tx.Commit().Error
		}
		return false, ErrCoordinationUncertain
	} else if !gorm.IsRecordNotFoundError(err) {
		return false, err
	}
	var session model.UploadSession
	if err := tx.Where("id = ?", sessionID).First(&session).Error; err != nil {
		return false, ErrCoordinationChanged
	}
	if session.EventID != eventID || session.ExpiresAt.IsZero() || !session.ExpiresAt.Before(time.Now()) || UploadSessionFingerprint(session) != fingerprint {
		return false, ErrCoordinationChanged
	}
	if session.Status != "uploading" && session.Status != "completed" && session.Status != "failed" {
		return false, ErrCoordinationChanged
	}
	var outstanding int
	if err := tx.Model(&model.PackageUploadUse{}).Where("session_id = ? AND state <> ?", sessionID, "finished").Count(&outstanding).Error; err != nil {
		return false, err
	}
	if outstanding != 0 {
		return false, ErrCoordinationBusy
	}
	use := model.PackageUploadUse{OperationID: operationID, SessionID: sessionID, EventID: eventID, Action: "retire", State: "active", Fingerprint: fingerprint}
	if err := tx.Create(&use).Error; err != nil {
		return false, err
	}
	if err := tx.Model(&model.UploadSession{}).Where("id = ?", sessionID).UpdateColumn("status", "cleanup-retired").Error; err != nil {
		return false, err
	}
	return true, tx.Commit().Error
}
