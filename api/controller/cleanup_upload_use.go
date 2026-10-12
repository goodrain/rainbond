package controller

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"

	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/model"
	guard "github.com/goodrain/rainbond/pkg/cleanup"
	"github.com/jinzhu/gorm"
)

// admitUploadUse persists a producer before native storage work. Upload content
// may introduce image references during later parsing, so its unresolved scope
// follows the existing native builder's conservative all-store admission.
func admitUploadUse(database *gorm.DB, eventID, sessionID, action string) (func(bool) error, error) {
	if database == nil || !uploadInventoryEventID.MatchString(eventID) || !uploadInventoryEventID.MatchString(sessionID) {
		return nil, guard.ErrCoordinationChanged
	}
	if action != "chunk" && action != "complete" && action != "cancel" && action != "expire" {
		return nil, guard.ErrCoordinationChanged
	}
	stores, err := guard.DiscoverStores(database)
	if err != nil {
		return nil, err
	}
	id, err := guard.NewActivationRevision()
	if err != nil {
		return nil, err
	}
	fingerprint := sha256.Sum256([]byte(eventID + "\x00" + sessionID + "\x00" + action))
	requests := []guard.CoordinationRequest{}
	finish := func(confirmed bool) error {
		var first error
		for _, request := range requests {
			if err := guard.FinishOperation(database, request, confirmed); err != nil && first == nil {
				first = err
			}
		}
		return first
	}
	for _, store := range stores {
		identity := sha256.Sum256([]byte(id + "\x00" + store.StorageID))
		request := guard.CoordinationRequest{StorageID: store.StorageID, Generation: store.Generation, OperationID: hex.EncodeToString(identity[:]), Owner: "api-package-" + action, Kind: "producer", Scope: "*", Fingerprint: hex.EncodeToString(fingerprint[:])}
		created, err := guard.AcquireOperation(database, request)
		if err != nil || !created {
			_ = finish(true) // No storage work started; release only this call's acknowledged admissions.
			if err != nil {
				return nil, err
			}
			return nil, guard.ErrCoordinationChanged
		}
		requests = append(requests, request)
	}
	sessionRequest := guard.PackageUploadRequest{OperationID: id, SessionID: sessionID, EventID: eventID, Action: action}
	if _, err := guard.AcquirePackageUploadRequest(database, sessionRequest); err != nil {
		// Only explicit pre-admission rejection proves no session grant was committed.
		rejected := errors.Is(err, guard.ErrCoordinationBusy) || errors.Is(err, guard.ErrCoordinationChanged) || errors.Is(err, guard.ErrCoordinationUncertain)
		_ = finish(rejected)
		return nil, err
	}
	return func(confirmed bool) error {
		if err := guard.FinishPackageUploadRequest(database, sessionRequest, confirmed); err != nil {
			_ = finish(false)
			return err
		}
		return finish(confirmed)
	}, nil
}

func (m *ChunkUploadManager) admitSessionUse(session *model.UploadSession, action string) (func(bool) error, error) {
	if m.admitUse != nil {
		return m.admitUse(session, action)
	}
	return admitUploadUse(db.GetManager().DB(), session.EventID, session.ID, action)
}

func (m *ChunkUploadManager) expireSessionChunks(session *model.UploadSession) (resultErr error) {
	finish, err := m.admitSessionUse(session, "expire")
	if err != nil {
		return err
	}
	confirmed := false
	defer func() {
		if err := finish(confirmed); resultErr == nil {
			resultErr = err
		}
	}()
	if err := m.cleanupSessionChunks(session.ID); err != nil {
		return err
	}
	confirmed = true
	return nil
}
