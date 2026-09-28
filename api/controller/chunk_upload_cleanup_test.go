package controller

import (
	"errors"
	"testing"
	"time"

	"github.com/goodrain/rainbond/db"
	dbdao "github.com/goodrain/rainbond/db/dao"
	"github.com/goodrain/rainbond/db/model"
)

type cancelUploadDAO struct {
	bulkDeletes int
	candidate   *model.UploadSession
	adds        int
	dbdao.UploadSessionDao
	deletes int
	failure error
}

func (d *cancelUploadDAO) GetByEventIDAndFileName(string, string) (*model.UploadSession, error) {
	return d.candidate, nil
}
func (d *cancelUploadDAO) AddModel(model.Interface) error { d.adds++; return nil }
func (d *cancelUploadDAO) CleanExpiredSessions() error    { d.bulkDeletes++; return nil }
func (d *cancelUploadDAO) DeleteByID(string) error        { d.deletes++; return d.failure }

type cancelUploadDB struct {
	db.Manager
	uploads *cancelUploadDAO
}

func (d cancelUploadDB) UploadSessionDao() dbdao.UploadSessionDao { return d.uploads }

// capability_id: rainbond.cleanup.upload-cancel-evidence
func TestCancelUploadRetainsRecoveryEvidenceOnFailure(t *testing.T) {
	for _, scenario := range []string{"success", "storage-failed", "record-failed"} {
		t.Run(scenario, func(t *testing.T) {
			dao := &cancelUploadDAO{}
			if scenario == "record-failed" {
				dao.failure = errors.New("fixture database failure")
			}
			old := db.GetManager()
			db.SetTestManager(cancelUploadDB{uploads: dao})
			defer db.SetTestManager(old)
			cleanups := 0
			manager := &ChunkUploadManager{sessionCache: map[string]*model.UploadSession{"owned": {ID: "owned", Status: "uploading"}}, cleanupChunks: func(string) error {
				cleanups++
				if scenario == "storage-failed" {
					return errors.New("fixture storage failure")
				}
				return nil
			}}
			err := manager.CancelUpload("owned")
			if (err == nil) != (scenario == "success") || cleanups != 1 {
				t.Fatal("incorrect cleanup outcome", err, cleanups)
			}
			if scenario == "storage-failed" && dao.deletes != 0 {
				t.Fatal("failed storage cleanup discarded session")
			}
			_, cached := manager.sessionCache["owned"]
			if cached != (scenario != "success") {
				t.Fatal("failed cleanup discarded recovery cache")
			}
		})
	}
}

// capability_id: rainbond.cleanup.upload-expiration-evidence
func TestExpiredUploadCleanupPreservesRecoveryEvidence(t *testing.T) {
	for _, scenario := range []string{"uncached", "storage-failed", "success", "completed"} {
		t.Run(scenario, func(t *testing.T) {
			dao := &cancelUploadDAO{}
			if scenario == "record-failed" {
				dao.failure = errors.New("fixture db failure")
			}
			old := db.GetManager()
			db.SetTestManager(cancelUploadDB{uploads: dao})
			defer db.SetTestManager(old)
			calls := 0
			manager := &ChunkUploadManager{sessionCache: map[string]*model.UploadSession{}, cleanupChunks: func(string) error {
				calls++
				if scenario == "storage-failed" {
					return errors.New("fixture storage failure")
				}
				return nil
			}}
			if scenario != "uncached" {
				status := "uploading"
				if scenario == "completed" {
					status = "completed"
				}
				manager.sessionCache["owned"] = &model.UploadSession{ID: "owned", Status: status, ExpiresAt: time.Now().Add(-time.Hour)}
			}
			err := manager.cleanExpiredSessions()
			failed := scenario == "storage-failed"
			if (err != nil) != failed || dao.bulkDeletes != 0 {
				t.Fatal("incorrect expiration outcome", err, dao.bulkDeletes)
			}
			expectedCalls := 1
			if scenario == "uncached" {
				expectedCalls = 0
			}
			if calls != expectedCalls {
				t.Fatal("expanded automatic cleanup scope", calls)
			}
			if dao.deletes != 0 {
				t.Fatal("expiration discarded durable evidence", dao.deletes)
			}
			_, cached := manager.sessionCache["owned"]
			if cached != failed {
				t.Fatal("incorrect recovery cache state")
			}
		})
	}
}

// capability_id: rainbond.cleanup.expired-upload-writes
func TestExpiredUploadCannotContinueWriting(t *testing.T) {
	manager := &ChunkUploadManager{sessionCache: map[string]*model.UploadSession{"owned": {ID: "owned", Status: "uploading", ExpiresAt: time.Now().Add(-time.Hour), TotalChunks: 1, UploadedChunks: "0"}}}
	if err := manager.SaveChunk("owned", 0, nil); err == nil {
		t.Fatal("expired upload accepted a chunk")
	}
	if _, err := manager.CompleteUpload("owned"); err == nil {
		t.Fatal("expired upload accepted completion")
	}
}

// capability_id: rainbond.cleanup.expired-upload-reinitialize
func TestExpiredUploadStartsNewSessionInsteadOfResumingRemovedChunks(t *testing.T) {
	dao := &cancelUploadDAO{candidate: &model.UploadSession{ID: "expired", Status: "uploading", ExpiresAt: time.Now().Add(-time.Hour)}}
	old := db.GetManager()
	db.SetTestManager(cancelUploadDB{uploads: dao})
	defer db.SetTestManager(old)
	manager := &ChunkUploadManager{sessionCache: map[string]*model.UploadSession{}}
	session, err := manager.InitUploadSession("event", "fixture.zip", 1, "", MinChunkSize)
	if err != nil || session.ID == "expired" || dao.adds != 1 {
		t.Fatal("expired session resumed", err, dao.adds)
	}
}
