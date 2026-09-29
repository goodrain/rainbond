package cleanup

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
)

// capability_id: rainbond.cleanup.expired-upload-retirement
func TestExpiredUploadDeletionRetiresSessionAndDoesNotReplay(t *testing.T) {
	database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "retire.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := database.AutoMigrate(&model.UploadSession{}, &model.PackageUploadUse{}).Error; err != nil {
		t.Fatal(err)
	}
	row := model.UploadSession{ID: "owned", EventID: "event", Status: "uploading", UploadedChunks: "0", CreatedAt: time.Now().Add(-48 * time.Hour), UpdatedAt: time.Now().Add(-25 * time.Hour), ExpiresAt: time.Now().Add(-time.Hour)}
	if err := database.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	selected, retained := filepath.Join(t.TempDir(), "selected"), filepath.Join(t.TempDir(), "retained")
	if err := os.WriteFile(selected, []byte("owned"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(retained, []byte("retained"), 0600); err != nil {
		t.Fatal(err)
	}
	fingerprint := UploadSessionFingerprint(row)
	writes := 0
	remove := func(id string) error {
		writes++
		if id != "owned" {
			t.Fatal("wrong deletion target")
		}
		var current model.UploadSession
		database.Where("id = ?", id).First(&current)
		if current.Status != "cleanup-retired" {
			t.Fatal("deleted before durable retirement")
		}
		if _, err := AcquirePackageUploadRequest(database, PackageUploadRequest{OperationID: "new", SessionID: id, EventID: "event", Action: "cancel"}); err == nil {
			t.Fatal("retired session still mutable")
		}
		return os.Remove(selected)
	}
	for i := 0; i < 2; i++ {
		if err := DeleteExpiredUploadChunks(database, "operation", "event", "owned", fingerprint, remove); err != nil {
			t.Fatal(err)
		}
	}
	if writes != 1 {
		t.Fatal("idempotent request replayed deletion")
	}
	if _, err := os.Stat(selected); !os.IsNotExist(err) {
		t.Fatal("selected chunks still exist")
	}
	if body, err := os.ReadFile(retained); err != nil || string(body) != "retained" {
		t.Fatal("unselected data changed")
	}
}

func TestExpiredUploadDeletionRejectsChangedOrActiveEvidence(t *testing.T) {
	for _, scenario := range []string{"changed", "not-expired", "active", "uncertain", "delete-failed"} {
		t.Run(scenario, func(t *testing.T) {
			database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "refuse.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer database.Close()
			database.AutoMigrate(&model.UploadSession{}, &model.PackageUploadUse{})
			row := model.UploadSession{ID: "owned", EventID: "event", Status: "uploading", ExpiresAt: time.Now().Add(-time.Hour)}
			if scenario == "not-expired" {
				row.ExpiresAt = time.Now().Add(time.Hour)
			}
			database.Create(&row)
			fingerprint := UploadSessionFingerprint(row)
			if scenario == "changed" {
				database.Model(&row).UpdateColumn("uploaded_chunks", "0")
			}
			if scenario == "active" || scenario == "uncertain" {
				database.Create(&model.PackageUploadUse{OperationID: "writer", SessionID: "owned", EventID: "event", Action: "chunk", State: scenario})
			}
			writes := 0
			remove := func(string) error { writes++; return errors.New("fixture storage unavailable") }
			if err := DeleteExpiredUploadChunks(database, "op", "event", "owned", fingerprint, remove); err == nil {
				t.Fatal("unsafe/failed deletion reported success")
			}
			expected := 0
			if scenario == "delete-failed" {
				expected = 1
			}
			if writes != expected {
				t.Fatal("invalid physical deletion count", writes)
			}
			if scenario == "delete-failed" {
				if err := DeleteExpiredUploadChunks(database, "op", "event", "owned", fingerprint, remove); !errors.Is(err, ErrCoordinationUncertain) || writes != 1 {
					t.Fatal("uncertain delete replayed", err)
				}
			}
		})
	}
}

func TestInterruptedUploadRetirementDoesNotReplayAfterRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "interrupted.db")
	database, err := gorm.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.AutoMigrate(&model.UploadSession{}, &model.PackageUploadUse{}).Error; err != nil {
		t.Fatal(err)
	}
	row := model.UploadSession{ID: "owned", EventID: "event", Status: "uploading", ExpiresAt: time.Now().Add(-time.Hour)}
	if err := database.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	fingerprint := UploadSessionFingerprint(row)
	if fresh, err := retireExpiredUpload(database, "operation", "event", "owned", fingerprint); err != nil || !fresh {
		t.Fatal(err)
	}
	database.Close()
	database, err = gorm.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	writes := 0
	if err := DeleteExpiredUploadChunks(database, "operation", "event", "owned", fingerprint, func(string) error { writes++; return nil }); !errors.Is(err, ErrCoordinationUncertain) || writes != 0 {
		t.Fatal("restart replayed uncertain physical operation", err)
	}
	if err := DeleteExpiredUploadChunks(database, "other", "event", "owned", fingerprint, func(string) error { writes++; return nil }); err == nil || writes != 0 {
		t.Fatal("new operation bypassed retired session")
	}
}

func TestInspectExpiredUploadDeletionUsesOnlyOriginalLedgerAndCurrentChunks(t *testing.T) {
	for _, scenario := range []struct {
		name, ledger, sessionStatus, want string
		objects                           int
		measureErr                        error
	}{
		{name: "finished-and-empty", ledger: "finished", sessionStatus: "cleanup-retired", want: "deleted"},
		{name: "finished-but-present", ledger: "finished", sessionStatus: "cleanup-retired", objects: 1, want: "unknown"},
		{name: "finished-measure-failed", ledger: "finished", sessionStatus: "cleanup-retired", measureErr: errors.New("unavailable"), want: "unknown"},
		{name: "uncertain", ledger: "uncertain", sessionStatus: "cleanup-retired", want: "unknown"},
		{name: "not-started", sessionStatus: "uploading", want: "present"},
		{name: "changed", sessionStatus: "failed", want: "unknown"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "inspect.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer database.Close()
			if err := database.AutoMigrate(&model.UploadSession{}, &model.PackageUploadUse{}).Error; err != nil {
				t.Fatal(err)
			}
			now := time.Now().UTC().Truncate(time.Second)
			original := model.UploadSession{ID: "owned", EventID: "event", Status: "uploading", CreatedAt: now.Add(-2 * time.Hour), UpdatedAt: now.Add(-2 * time.Hour), ExpiresAt: now.Add(-time.Hour)}
			fingerprint := UploadSessionFingerprint(original)
			current := original
			current.Status = scenario.sessionStatus
			if err := database.Create(&current).Error; err != nil {
				t.Fatal(err)
			}
			if scenario.ledger != "" {
				if err := database.Create(&model.PackageUploadUse{OperationID: "operation", SessionID: "owned", EventID: "event", Action: "retire", State: scenario.ledger, Fingerprint: fingerprint}).Error; err != nil {
					t.Fatal(err)
				}
			}
			measured := 0
			state, err := InspectExpiredUploadDeletion(database, "operation", "event", "owned", fingerprint, func(string) (int64, int, error) {
				measured++
				return int64(scenario.objects), scenario.objects, scenario.measureErr
			})
			if err != nil || state != scenario.want {
				t.Fatalf("state=%s err=%v", state, err)
			}
			if (scenario.ledger == "finished") != (measured == 1) {
				t.Fatalf("unexpected storage reads: %d", measured)
			}
		})
	}
}

func TestInspectExpiredUploadDeletionRejectsUnrelatedOperationIdentity(t *testing.T) {
	database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "inspect.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := database.AutoMigrate(&model.UploadSession{}, &model.PackageUploadUse{}).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	row := model.UploadSession{ID: "owned", EventID: "event", Status: "cleanup-retired", CreatedAt: now.Add(-2 * time.Hour), UpdatedAt: now.Add(-2 * time.Hour), ExpiresAt: now.Add(-time.Hour)}
	fingerprint := UploadSessionFingerprint(model.UploadSession{ID: "owned", EventID: "event", Status: "uploading", CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, ExpiresAt: row.ExpiresAt})
	database.Create(&row)
	database.Create(&model.PackageUploadUse{OperationID: "operation", SessionID: "other", EventID: "event", Action: "retire", State: "finished", Fingerprint: fingerprint})
	state, err := InspectExpiredUploadDeletion(database, "operation", "event", "owned", fingerprint, func(string) (int64, int, error) {
		t.Fatal("unrelated ledger reached storage")
		return 0, 0, nil
	})
	if err != nil || state != "unknown" {
		t.Fatal(state, err)
	}
}
