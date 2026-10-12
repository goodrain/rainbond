package cleanup

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
)

// capability_id: rainbond.cleanup.package-upload-exclusive-close
func TestPackageUploadCloseDrainsPartsAndRetainsUnknownAcrossRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.db")
	database, err := gorm.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { database.Close() }()
	if err := database.AutoMigrate(&model.UploadSession{}, &model.PackageUploadUse{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Create(&model.UploadSession{ID: "session", EventID: "event", Status: "uploading", ExpiresAt: time.Now().Add(time.Hour)}).Error; err != nil {
		t.Fatal(err)
	}
	part := PackageUploadRequest{OperationID: "part", SessionID: "session", EventID: "event", Action: "chunk"}
	closing := PackageUploadRequest{OperationID: "close", SessionID: "session", EventID: "event", Action: "complete"}
	if _, err := AcquirePackageUploadRequest(database, part); err != nil {
		t.Fatal(err)
	}
	second := part
	second.OperationID = "part-two"
	if _, err := AcquirePackageUploadRequest(database, second); err != nil {
		t.Fatal("parallel part rejected", err)
	}
	if _, err := AcquirePackageUploadRequest(database, closing); !errors.Is(err, ErrCoordinationBusy) {
		t.Fatal("closed with in-flight part", err)
	}
	if err := FinishPackageUploadRequest(database, part, true); err != nil {
		t.Fatal(err)
	}
	if _, err := AcquirePackageUploadRequest(database, part); !errors.Is(err, ErrCoordinationChanged) {
		t.Fatal("replayed completed storage request", err)
	}
	if _, err := AcquirePackageUploadRequest(database, closing); !errors.Is(err, ErrCoordinationBusy) {
		t.Fatal("closing ignored second active part", err)
	}
	if err := FinishPackageUploadRequest(database, second, true); err != nil {
		t.Fatal(err)
	}
	if _, err := AcquirePackageUploadRequest(database, closing); err != nil {
		t.Fatal(err)
	}
	part.OperationID = "late"
	if _, err := AcquirePackageUploadRequest(database, part); !errors.Is(err, ErrCoordinationBusy) {
		t.Fatal("late part entered closing session", err)
	}
	if err := FinishPackageUploadRequest(database, closing, false); err != nil {
		t.Fatal(err)
	}
	database.Close()
	database, err = gorm.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AcquirePackageUploadRequest(database, part); !errors.Is(err, ErrCoordinationUncertain) {
		t.Fatal("restart lost uncertainty", err)
	}
	if err := FinishPackageUploadRequest(database, closing, true); !errors.Is(err, ErrCoordinationUncertain) {
		t.Fatal("unknown request silently released", err)
	}
}

func TestPackageUploadAdmissionChecksFreshSessionAndOriginalRequest(t *testing.T) {
	database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "fresh.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := database.AutoMigrate(&model.UploadSession{}, &model.PackageUploadUse{}).Error; err != nil {
		t.Fatal(err)
	}
	row := model.UploadSession{ID: "session", EventID: "event", Status: "uploading", ExpiresAt: time.Now().Add(-time.Hour)}
	if err := database.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	request := PackageUploadRequest{OperationID: "part", SessionID: "session", EventID: "event", Action: "chunk"}
	if _, err := AcquirePackageUploadRequest(database, request); !errors.Is(err, ErrCoordinationBusy) {
		t.Fatal("expired session accepted new data", err)
	}
	request.Action = "cancel"
	foreign := request
	foreign.EventID = "foreign"
	if _, err := AcquirePackageUploadRequest(database, foreign); !errors.Is(err, ErrCoordinationChanged) {
		t.Fatal("foreign event accepted", err)
	}
	if _, err := AcquirePackageUploadRequest(database, request); err != nil {
		t.Fatal(err)
	}
	if err := FinishPackageUploadRequest(database, foreign, true); !errors.Is(err, ErrCoordinationChanged) {
		t.Fatal("foreign completion released request", err)
	}
	if err := database.Where("id = ?", "session").Delete(&model.UploadSession{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := FinishPackageUploadRequest(database, request, true); err != nil {
		t.Fatal("confirmed cancellation could not finish after session removal", err)
	}
	if err := FinishPackageUploadRequest(database, request, true); err != nil {
		t.Fatal("completion acknowledgement not idempotent", err)
	}
	if _, err := AcquirePackageUploadRequest(database, request); err == nil {
		t.Fatal("deleted session resumed")
	}
}
