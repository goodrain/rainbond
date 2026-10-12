package controller

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/goodrain/rainbond/db/model"
	guard "github.com/goodrain/rainbond/pkg/cleanup"
	"github.com/jinzhu/gorm"
)

// capability_id: rainbond.cleanup.upload-use-lifetime
func TestUploadUsePersistsAcrossStorageWorkAndUncertainResults(t *testing.T) {
	for _, outcome := range []string{"complete", "uncertain", "crashed", "blocked"} {
		t.Run(outcome, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "use.db")
			database, err := gorm.Open("sqlite3", path)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { database.Close() }()
			if err := database.AutoMigrate(&model.CleanupStorage{}, &model.CleanupOperation{}, &model.UploadSession{}, &model.PackageUploadUse{}).Error; err != nil {
				t.Fatal(err)
			}
			if err := database.Create(&model.UploadSession{ID: "session", EventID: "event", Status: "uploading", ExpiresAt: time.Now().Add(time.Hour)}).Error; err != nil {
				t.Fatal(err)
			}
			binding := guard.StorageRegistration{StorageID: "owned", Generation: "one", VolumeUID: "isolated-volume", RootPath: "/owned"}
			if err := guard.RegisterStorage(database, binding); err != nil {
				t.Fatal(err)
			}
			// Isolated coordinator state fixture, not production readiness certification.
			mode := "ready"
			if outcome == "blocked" {
				mode = "draining"
			}
			if err := database.Model(&model.CleanupStorage{}).Where("storage_id = ?", "owned").UpdateColumn("mode", mode).Error; err != nil {
				t.Fatal(err)
			}
			finish, err := admitUploadUse(database, "event", "session", "chunk")
			if outcome == "blocked" {
				if !errors.Is(err, guard.ErrCoordinationBusy) {
					t.Fatal("maintenance allowed upload", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			deletion := guard.CoordinationRequest{StorageID: "owned", Generation: "one", OperationID: "delete", Owner: "isolated-test", Kind: "delete", Scope: "selected", Fingerprint: "selected", Target: "owned-target"}
			if _, err := guard.AcquireOperation(database, deletion); !errors.Is(err, guard.ErrCoordinationBusy) {
				t.Fatal("in-flight upload not protected", err)
			}
			if outcome != "crashed" {
				if err := finish(outcome == "complete"); err != nil {
					t.Fatal(err)
				}
			}
			database.Close()
			database, err = gorm.Open("sqlite3", path)
			if err != nil {
				t.Fatal(err)
			}
			_, err = guard.AcquireOperation(database, deletion)
			if outcome == "complete" {
				if err != nil {
					t.Fatal("confirmed completion did not release", err)
				}
			} else if !errors.Is(err, guard.ErrCoordinationBusy) {
				t.Fatal("restart released uncertain/in-flight upload", err)
			}
		})
	}
}

func TestUploadMaintenanceStopsNativeChunkWriteCancelAndExpiry(t *testing.T) {
	admissions, cleanups := 0, 0
	session := &model.UploadSession{ID: "owned", EventID: "event", Status: "uploading", ExpiresAt: time.Now().Add(time.Hour), TotalChunks: 1}
	manager := &ChunkUploadManager{sessionCache: map[string]*model.UploadSession{"owned": session},
		admitUse: func(*model.UploadSession, string) (func(bool) error, error) {
			admissions++
			return nil, guard.ErrCoordinationBusy
		},
		cleanupChunks: func(string) error { cleanups++; return nil },
	}
	if err := manager.SaveChunk("owned", 0, nil); !errors.Is(err, guard.ErrCoordinationBusy) {
		t.Fatal("chunk write bypassed admission", err)
	}
	if err := manager.CancelUpload("owned"); !errors.Is(err, guard.ErrCoordinationBusy) {
		t.Fatal("cancel bypassed admission", err)
	}
	session.ExpiresAt = time.Now().Add(-time.Hour)
	if err := manager.cleanExpiredSessions(); !errors.Is(err, guard.ErrCoordinationBusy) {
		t.Fatal("expiration bypassed admission", err)
	}
	if admissions != 3 || cleanups != 0 || manager.sessionCache["owned"] == nil {
		t.Fatal("blocked mutation touched upload storage or recovery cache")
	}
}

// capability_id: rainbond.cleanup.package-upload-exclusive-close
func TestNativeUploadCancelCannotOverlapAnActivePart(t *testing.T) {
	database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "native.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := database.AutoMigrate(&model.CleanupStorage{}, &model.CleanupOperation{}, &model.UploadSession{}, &model.PackageUploadUse{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Create(&model.UploadSession{ID: "session", EventID: "event", Status: "uploading", ExpiresAt: time.Now().Add(time.Hour)}).Error; err != nil {
		t.Fatal(err)
	}
	part, err := admitUploadUse(database, "event", "session", "chunk")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := admitUploadUse(database, "event", "session", "cancel"); !errors.Is(err, guard.ErrCoordinationBusy) {
		t.Fatal("native cancel overlapped active storage write", err)
	}
	if err := part(true); err != nil {
		t.Fatal(err)
	}
	cancel, err := admitUploadUse(database, "event", "session", "cancel")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := admitUploadUse(database, "event", "session", "chunk"); !errors.Is(err, guard.ErrCoordinationBusy) {
		t.Fatal("part entered cancellation", err)
	}
	if err := cancel(false); err != nil {
		t.Fatal(err)
	}
	if _, err := admitUploadUse(database, "event", "session", "expire"); !errors.Is(err, guard.ErrCoordinationUncertain) {
		t.Fatal("uncertain native request lost protection", err)
	}
}
