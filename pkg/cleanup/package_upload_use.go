package cleanup

import (
	"time"

	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
)

// PackageUploadRequest binds a single execution to an existing native upload session.
type PackageUploadRequest struct{ OperationID, SessionID, EventID, Action string }

func (r PackageUploadRequest) valid() bool {
	return coordinationIdentity.MatchString(r.OperationID) && coordinationIdentity.MatchString(r.SessionID) && coordinationIdentity.MatchString(r.EventID) &&
		(r.Action == "chunk" || r.Action == "complete" || r.Action == "cancel" || r.Action == "expire")
}
func (r PackageUploadRequest) matches(use model.PackageUploadUse) bool {
	return use.OperationID == r.OperationID && use.SessionID == r.SessionID && use.EventID == r.EventID && use.Action == r.Action
}

// AcquirePackageUploadRequest returns fresh metadata only after durable admission.
// Parts may run concurrently; merging, cancellation and expiry require an idle
// session and block further parts. No transaction remains open during storage IO.
func AcquirePackageUploadRequest(database *gorm.DB, r PackageUploadRequest) (model.UploadSession, error) {
	var session model.UploadSession
	if database == nil || !r.valid() {
		return session, ErrCoordinationChanged
	}
	tx := database.Begin()
	if tx.Error != nil {
		return session, tx.Error
	}
	defer tx.Rollback()
	if err := tx.Model(&model.UploadSession{}).Where("id = ?", r.SessionID).UpdateColumn("status", gorm.Expr("status")).Error; err != nil {
		return session, err
	}
	if err := tx.Where("id = ?", r.SessionID).First(&session).Error; err != nil {
		return session, ErrCoordinationChanged
	}
	if session.EventID != r.EventID || session.ExpiresAt.IsZero() {
		return session, ErrCoordinationChanged
	}
	var previous model.PackageUploadUse
	if err := tx.Where("operation_id = ?", r.OperationID).First(&previous).Error; err == nil {
		return session, ErrCoordinationChanged
	} else if !gorm.IsRecordNotFoundError(err) {
		return session, err
	}
	uses := []model.PackageUploadUse{}
	if err := tx.Where("session_id = ? AND state <> ?", r.SessionID, "finished").Limit(1025).Find(&uses).Error; err != nil {
		return session, err
	}
	if len(uses) > 1024 {
		return session, ErrCoordinationBusy
	}
	for _, use := range uses {
		if use.State != "active" {
			return session, ErrCoordinationUncertain
		}
		if r.Action != "chunk" || use.Action != "chunk" {
			return session, ErrCoordinationBusy
		}
	}
	if len(uses) >= 1024 {
		return session, ErrCoordinationBusy
	}
	if session.Status != "uploading" && session.Status != "completed" && session.Status != "failed" {
		return session, ErrCoordinationChanged
	}
	if (r.Action == "chunk" || r.Action == "complete") && (session.Status != "uploading" || !session.ExpiresAt.After(time.Now())) {
		return session, ErrCoordinationBusy
	}
	if r.Action == "expire" && session.ExpiresAt.After(time.Now()) {
		return session, ErrCoordinationBusy
	}
	use := model.PackageUploadUse{OperationID: r.OperationID, SessionID: r.SessionID, EventID: r.EventID, Action: r.Action, State: "active"}
	if err := tx.Create(&use).Error; err != nil {
		return session, err
	}
	return session, tx.Commit().Error
}

// FinishPackageUploadRequest acknowledges only the original request. A missing
// session is allowed after confirmed native cancellation, which deletes its row.
// Unknown outcomes remain protective and cannot be promoted by a late retry.
func FinishPackageUploadRequest(database *gorm.DB, r PackageUploadRequest, confirmed bool) error {
	if database == nil || !r.valid() {
		return ErrCoordinationChanged
	}
	tx := database.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer tx.Rollback()
	if err := tx.Model(&model.PackageUploadUse{}).Where("operation_id = ?", r.OperationID).UpdateColumn("state", gorm.Expr("state")).Error; err != nil {
		return err
	}
	var use model.PackageUploadUse
	if err := tx.Where("operation_id = ?", r.OperationID).First(&use).Error; err != nil {
		return ErrCoordinationChanged
	}
	if !r.matches(use) {
		return ErrCoordinationChanged
	}
	if use.State == "finished" && confirmed {
		return tx.Commit().Error
	}
	if use.State == "uncertain" {
		if confirmed {
			return ErrCoordinationUncertain
		}
		return tx.Commit().Error
	}
	if use.State != "active" {
		return ErrCoordinationChanged
	}
	state := "uncertain"
	if confirmed {
		state = "finished"
	}
	if err := tx.Model(&model.PackageUploadUse{}).Where("operation_id = ?", r.OperationID).Update("state", state).Error; err != nil {
		return err
	}
	return tx.Commit().Error
}
