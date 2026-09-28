package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/goodrain/rainbond/api/middleware"
	"github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/pkg/component/storage"
	"github.com/jinzhu/gorm"
)

// capability_id: rainbond.cleanup.upload-inventory-api
func TestUploadInventoryRestrictsEventsAndPreservesUnknownSizes(t *testing.T) {
	database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "uploads.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := database.AutoMigrate(&model.UploadSession{}).Error; err != nil {
		t.Fatal(err)
	}
	for _, row := range []model.UploadSession{
		{ID: "known", EventID: "owned", Status: "uploading", FileName: "upload.zip", FileSize: 999999, StoragePath: "/private-path-never-return", FileMD5: "private-checksum-never-return", ExpiresAt: time.Now().Add(-time.Hour)},
		{ID: "unknown", EventID: "owned", Status: "completed", FileSize: 888888},
		{ID: "foreign", EventID: "other", Status: "uploading", FileName: "foreign-never-return"},
	} {
		if err := database.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	measured := 0
	packages := 0
	h := &CleanupCoordinationHandler{measureUploadEvent: func(ctx context.Context, eventID string) (storage.UploadChunkUsage, error) {
		packages++
		if eventID != "owned" {
			t.Fatal("measured foreign package event")
		}
		return storage.UploadChunkUsage{Bytes: 97, Objects: 3}, nil
	}, database: func() *gorm.DB { return database }, measureUploadChunks: func(ctx context.Context, id string) (storage.UploadChunkUsage, error) {
		measured++
		if id == "known" {
			return storage.UploadChunkUsage{Bytes: 17, Objects: 2}, nil
		}
		if id == "unknown" {
			return storage.UploadChunkUsage{}, errors.New("private-upstream-error")
		}
		t.Fatal("measured an unrequested event")
		return storage.UploadChunkUsage{}, nil
	}}
	t.Setenv("TOKEN", "isolated-upload-inventory-fixture")
	post := func(body string, auth bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, "/uploads/inventory", strings.NewReader(body))
		if auth {
			r.Header.Set("Authorization", "Token isolated-upload-inventory-fixture")
		}
		w := httptest.NewRecorder()
		middleware.CleanupIdentity(http.HandlerFunc(h.UploadInventory)).ServeHTTP(w, r)
		return w
	}
	if w := post(`{"event_ids":["owned"]}`, false); w.Code == 200 || measured != 0 {
		t.Fatal("unauthenticated inventory read")
	}
	for _, body := range []string{`{}`, `{"event_ids":[]}`, `{"event_ids":["../other"]}`, `{"event_ids":["owned","owned"]}`, `{"event_ids":["owned"],"path":"/tmp"}`} {
		if w := post(body, true); w.Code != 400 || measured != 0 {
			t.Fatal("invalid scope accepted", w.Code)
		}
	}
	w := post(`{"event_ids":["owned"]}`, true)
	if w.Code != 200 {
		t.Fatal("inventory failed", w.Code)
	}
	for _, private := range []string{"private-", "foreign-never-return", "999999", "888888"} {
		if strings.Contains(w.Body.String(), private) {
			t.Fatal("untrusted size or private metadata escaped")
		}
	}
	var reply struct {
		Bean struct {
			Packages []struct {
				EventID string `json:"event_id"`
				Bytes   *int64 `json:"bytes"`
			} `json:"packages"`
			Protocol int `json:"protocol"`
			Items    []struct {
				ID         string `json:"id"`
				Bytes      *int64 `json:"bytes"`
				SizeStatus string `json:"size_status"`
			} `json:"items"`
		} `json:"bean"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &reply); err != nil {
		t.Fatal(err)
	}
	if reply.Bean.Protocol != 1 || len(reply.Bean.Items) != 2 || measured != 2 {
		t.Fatal("incorrect scoped inventory")
	}
	if packages != 1 || len(reply.Bean.Packages) != 1 || reply.Bean.Packages[0].EventID != "owned" || reply.Bean.Packages[0].Bytes == nil || *reply.Bean.Packages[0].Bytes != 97 {
		t.Fatal("package was omitted or counted per session")
	}
	known, unknown := reply.Bean.Items[0], reply.Bean.Items[1]
	if known.ID != "known" || known.Bytes == nil || *known.Bytes != 17 || known.SizeStatus != "measured" {
		t.Fatal("actual stored size not returned")
	}
	if unknown.ID != "unknown" || unknown.Bytes != nil || unknown.SizeStatus != "unavailable" {
		t.Fatal("unknown size reported as zero")
	}
}

// capability_id: rainbond.cleanup.upload-inventory-bounds
func TestUploadInventoryRejectsOversizedResultBeforeReadingStorage(t *testing.T) {
	database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "bounded.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := database.AutoMigrate(&model.UploadSession{}).Error; err != nil {
		t.Fatal(err)
	}
	transaction := database.Begin()
	for i := 0; i < 501; i++ {
		if err := transaction.Create(&model.UploadSession{ID: fmt.Sprintf("owned-%d", i), EventID: "owned"}).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := transaction.Commit().Error; err != nil {
		t.Fatal(err)
	}
	calls := 0
	h := &CleanupCoordinationHandler{database: func() *gorm.DB { return database }, measureUploadChunks: func(context.Context, string) (storage.UploadChunkUsage, error) {
		calls++
		return storage.UploadChunkUsage{}, nil
	}}
	w := httptest.NewRecorder()
	h.UploadInventory(w, httptest.NewRequest(http.MethodPost, "/uploads/inventory", strings.NewReader(`{"event_ids":["owned"]}`)))
	if w.Code != 413 || calls != 0 {
		t.Fatal("truncated inventory accepted or unbounded storage reads", w.Code, calls)
	}
	w = httptest.NewRecorder()
	h.UploadInventory(w, httptest.NewRequest(http.MethodPost, "/uploads/inventory", strings.NewReader(`{"event_ids":["missing"]}`)))
	if w.Code != 200 || calls != 0 || !strings.Contains(w.Body.String(), `"items":[]`) {
		t.Fatal("empty explicit scope not represented")
	}
}
