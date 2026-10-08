package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/api/middleware"
	"github.com/goodrain/rainbond/db/model"
	guard "github.com/goodrain/rainbond/pkg/cleanup"
	"github.com/jinzhu/gorm"
)

func TestStorageObservationPermitAPIIsAuthenticatedAndServerBound(t *testing.T) {
	database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "observation.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := database.AutoMigrate(&model.CleanupStorage{}).Error; err != nil {
		t.Fatal(err)
	}
	binding, err := guard.ProvisionRegistryStorage(database, "volume", "/var/lib/registry")
	if err != nil {
		t.Fatal(err)
	}
	key := []byte(strings.Repeat("observation-key-", 3))
	h := &CleanupCoordinationHandler{database: func() *gorm.DB { return database }, permitKey: func() []byte { return key }}
	t.Setenv("TOKEN", "isolated-observation-token")
	router := chi.NewRouter()
	router.Use(middleware.CleanupIdentity)
	router.Post("/stores/{storage_id}/observation-permit", h.StorageObservationPermit)
	invoke := func(body, token string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodPost, "/stores/"+binding.StorageID+"/observation-permit", strings.NewReader(body))
		if token != "" {
			request.Header.Set("Authorization", "Token "+token)
		}
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		return response
	}
	body := `{"generation":"` + binding.Generation + `"}`
	if response := invoke(body, ""); response.Code != 401 && response.Code != 403 {
		t.Fatal("unauthenticated observation permit", response.Code)
	}
	if response := invoke(`{"generation":"`+binding.Generation+`","path":"/foreign"}`, "isolated-observation-token"); response.Code != 400 {
		t.Fatal("caller path accepted", response.Code)
	}
	response := invoke(body, "isolated-observation-token")
	var decoded struct {
		Bean struct {
			Permit      string `json:"permit"`
			Fingerprint string `json:"binding_fingerprint"`
		} `json:"bean"`
	}
	if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &decoded) != nil || decoded.Bean.Permit == "" {
		t.Fatal("observation permit unavailable", response.Code, response.Body.String())
	}
	observed, err := guard.VerifyStorageObservationPermit(key, decoded.Bean.Permit, time.Now())
	fingerprint, _ := binding.Fingerprint()
	if err != nil || observed != binding || decoded.Bean.Fingerprint != fingerprint {
		t.Fatal("permit not bound to registered storage", observed, err)
	}
}
