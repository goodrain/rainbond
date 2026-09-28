package controller

import (
	"errors"
	"testing"
	"time"

	"github.com/goodrain/rainbond/db"
	dbdao "github.com/goodrain/rainbond/db/dao"
	"github.com/goodrain/rainbond/db/model"
)

type completionUploadDAO struct {
	cancelUploadDAO
	update func(*model.UploadSession) error
}

func (d *completionUploadDAO) UpdateModel(value model.Interface) error {
	return d.update(value.(*model.UploadSession))
}

type completionUploadDB struct {
	db.Manager
	uploads *completionUploadDAO
}

func (d completionUploadDB) UploadSessionDao() dbdao.UploadSessionDao { return d.uploads }

// capability_id: rainbond.cleanup.upload-completion-evidence
func TestUploadCompletionPreservesEvidenceBeforeChunkDeletion(t *testing.T) {
	for _, scenario := range []string{"success", "record-failed", "merge-failed", "cleanup-failed", "completed", "failed"} {
		t.Run(scenario, func(t *testing.T) {
			session := &model.UploadSession{ID: "owned", Status: "uploading", ExpiresAt: time.Now().Add(time.Hour), TotalChunks: 1, UploadedChunks: "0", StoragePath: "/owned/package.zip"}
			if scenario == "completed" || scenario == "failed" {
				session.Status = scenario
			}
			merges, cleanups, updates := 0, 0, 0
			persisted := false
			dao := &completionUploadDAO{update: func(value *model.UploadSession) error {
				updates++
				if scenario == "record-failed" {
					return errors.New("fixture database unavailable")
				}
				persisted = value.Status == "completed"
				return nil
			}}
			old := db.GetManager()
			db.SetTestManager(completionUploadDB{uploads: dao})
			defer db.SetTestManager(old)
			manager := &ChunkUploadManager{sessionCache: map[string]*model.UploadSession{"owned": session},
				mergeChunks: func(string, string, int) error {
					merges++
					if scenario == "merge-failed" {
						return errors.New("fixture merge failed")
					}
					return nil
				},
				cleanupChunks: func(string) error {
					cleanups++
					if !persisted {
						t.Error("deleted chunks before durable completion")
					}
					if scenario == "cleanup-failed" {
						return errors.New("fixture cleanup failed")
					}
					return nil
				},
			}
			path, err := manager.CompleteUpload("owned")
			success := scenario == "success" || scenario == "completed" || scenario == "cleanup-failed"
			if (err == nil) != success {
				t.Fatalf("unexpected completion result: %v", err)
			}
			if success && path != session.StoragePath {
				t.Fatal("lost completed package path")
			}
			if scenario == "completed" || scenario == "failed" {
				if merges != 0 || cleanups != 0 || updates != 0 {
					t.Fatal("terminal upload replayed storage operations")
				}
			}
			if scenario == "record-failed" {
				if cleanups != 0 || session.Status != "uploading" || manager.sessionCache["owned"] == nil {
					t.Fatal("failed persistence discarded recovery evidence")
				}
			}
			if (scenario == "success" || scenario == "cleanup-failed") && (merges != 1 || cleanups != 1 || updates != 1 || !persisted) {
				t.Fatal("completion did not persist and clean exactly once")
			}
			if scenario == "merge-failed" && cleanups != 0 {
				t.Fatal("failed merge deleted chunks")
			}
		})
	}
}
