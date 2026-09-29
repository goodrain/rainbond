package controller

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/goodrain/rainbond/api/middleware"
	"github.com/goodrain/rainbond/db/model"
	guard "github.com/goodrain/rainbond/pkg/cleanup"
	"github.com/jinzhu/gorm"
)

// capability_id: rainbond.cleanup.upload-delete-api
func TestUploadDeleteAPIRequiresWriterStorageRuleAndSelectionEvidence(t *testing.T) {
	for _, scenario := range []string{"success", "unauthenticated", "old-writer", "different-storage", "too-recent", "invalid-rule", "caller-path", "storage-changed-after-retire"} {
		t.Run(scenario, func(t *testing.T) {
			database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "api.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer database.Close()
			if err := database.AutoMigrate(&model.UploadSession{}, &model.PackageUploadUse{}, &model.CleanupReferenceWriter{}).Error; err != nil {
				t.Fatal(err)
			}
			session := model.UploadSession{ID: "session", EventID: "event", Status: "uploading", ExpiresAt: time.Now().Add(-48 * time.Hour)}
			if scenario == "too-recent" {
				session.ExpiresAt = time.Now().Add(-time.Hour)
			}
			database.Create(&session)
			writer := guard.ReferenceWriter{Namespace: "system", PodName: "api", PodUID: "api", ContainerName: "rbd-api", ContainerID: "containerd://owned", ImageID: "registry@sha256:owned", Role: "api", Protocol: guard.UploadWriterProtocol}
			if scenario != "old-writer" {
				if err := guard.RegisterReferenceWriter(database, writer); err != nil {
					t.Fatal(err)
				}
			}
			deleted := 0
			bindingReads := 0
			h := &CleanupCoordinationHandler{database: func() *gorm.DB { return database }, inspectReferenceWriters: func(context.Context) ([]guard.ReferenceWriter, error) { return []guard.ReferenceWriter{writer}, nil }, uploadStorageBinding: func() (string, error) {
				bindingReads++
				if scenario == "storage-changed-after-retire" && bindingReads > 1 {
					return strings.Repeat("b", 64), nil
				}
				return strings.Repeat("a", 64), nil
			}, deleteUploadChunks: func(id string) error {
				deleted++
				if id != "session" {
					t.Fatal("wrong native target")
				}
				return nil
			}}
			body := map[string]interface{}{"operation_id": "operation", "session_id": "session", "event_id": "event", "state_fingerprint": guard.UploadSessionFingerprint(session), "storage_fingerprint": strings.Repeat("a", 64), "idle_days": 1}
			if scenario == "different-storage" {
				body["storage_fingerprint"] = strings.Repeat("b", 64)
			}
			if scenario == "invalid-rule" {
				body["idle_days"] = 0
			}
			if scenario == "caller-path" {
				body["path"] = "/foreign"
			}
			raw, _ := json.Marshal(body)
			t.Setenv("TOKEN", "isolated-upload-delete-fixture")
			request := httptest.NewRequest(http.MethodPost, "/uploads/chunks/delete", strings.NewReader(string(raw)))
			if scenario != "unauthenticated" {
				request.Header.Set("Authorization", "Token isolated-upload-delete-fixture")
			}
			response := httptest.NewRecorder()
			middleware.CleanupIdentity(http.HandlerFunc(h.DeleteUploadChunks)).ServeHTTP(response, request)
			if scenario == "success" {
				if response.Code != 200 || deleted != 1 {
					t.Fatal("valid selected deletion rejected", response.Code)
				}
			} else if response.Code == 200 || deleted != 0 {
				t.Fatal("failed guard reached physical deletion", response.Code, deleted)
			}
		})
	}
}
