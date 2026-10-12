package controller

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/api/middleware"
	"github.com/goodrain/rainbond/db/model"
	guard "github.com/goodrain/rainbond/pkg/cleanup"
	"github.com/jinzhu/gorm"
)

func TestConsoleWriterAnnouncementRequiresAuthenticationAndObservedIdentity(t *testing.T) {
	database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "console.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	database.LogMode(false)
	if err := database.AutoMigrate(&model.CleanupReferenceWriter{}).Error; err != nil {
		t.Fatal(err)
	}
	calls := 0
	h := &CleanupCoordinationHandler{database: func() *gorm.DB { return database }, inspectConsoleWriter: func(ctx context.Context, pod, uid string) (guard.ReferenceWriter, error) {
		calls++
		return guard.ReferenceWriter{Namespace: "system", PodName: pod, PodUID: uid, ContainerName: "rbd-app-ui", ContainerID: "containerd://observed", ImageID: "observed-image", Role: "console", Protocol: guard.ReferenceWriterProtocol}, nil
	}}
	router := chi.NewRouter()
	router.Use(middleware.CleanupIdentity)
	router.Post("/reference-writers/console", h.RegisterConsoleReferenceWriter)
	t.Setenv("TOKEN", "isolated-test-credential")
	invoke := func(auth, body string) int {
		request := httptest.NewRequest("POST", "/reference-writers/console", strings.NewReader(body))
		request.Header.Set("Authorization", auth)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		return response.Code
	}
	valid := `{"pod":"console-pod","pod_uid":"pod-uid","protocol":"registry-reference-v1"}`
	if invoke("", valid) == 200 || calls != 0 {
		t.Fatal("unauthenticated announcement")
	}
	for _, body := range []string{`{"pod":"console-pod","pod_uid":"pod-uid","protocol":"old"}`, `{"pod":"console-pod","pod_uid":"pod-uid","protocol":"registry-reference-v1","image_id":"caller-claim"}`} {
		if invoke("Token isolated-test-credential", body) == 200 || calls != 0 {
			t.Fatal("unsupported or claimed identity accepted")
		}
	}
	if status := invoke("Token isolated-test-credential", valid); status != 200 || calls != 1 {
		t.Fatal(status, calls)
	}
	var records []model.CleanupReferenceWriter
	if err := database.Find(&records).Error; err != nil || len(records) != 1 || records[0].ImageID != "observed-image" || records[0].Role != "console" {
		t.Fatal("wrong observed identity", err)
	}
	if database.HasTable(&model.CleanupStorage{}) {
		t.Fatal("announcement created a storage")
	}
}
