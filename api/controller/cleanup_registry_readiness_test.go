package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/db/model"
	guard "github.com/goodrain/rainbond/pkg/cleanup"
	"github.com/jinzhu/gorm"
)

// The observed fixtures exercise SQL/HTTP contracts, not production deployment proof.
func installRegistryCoverageFixture(t *testing.T, h *CleanupCoordinationHandler, binding guard.StorageRegistration) *[]guard.ReferenceWriter {
	t.Helper()
	database := h.database()
	if err := database.AutoMigrate(&model.CleanupParticipant{}, &model.CleanupReferenceWriter{}, &model.VersionInfo{}, &model.TenantPluginBuildVersion{}, &model.K8sResource{}, &model.KeyValue{}).Error; err != nil {
		t.Fatal(err)
	}
	fingerprint, _ := binding.Fingerprint()
	ingress := guard.ParticipantRegistration{StorageID: binding.StorageID, Generation: binding.Generation, Owner: "proxy:owned", Role: "registry-ingress", PodUID: "proxy-pod", ContainerID: "containerd://proxy", ImageID: "proxy-image", BindingFingerprint: fingerprint}
	if err := guard.RegisterParticipant(database, ingress); err != nil {
		t.Fatal(err)
	}
	writers := []guard.ReferenceWriter{}
	for role, name := range map[string]string{"api": "rbd-api", "worker": "rbd-worker", "builder": "rbd-chaos", "console": "rbd-app-ui"} {
		writer := guard.ReferenceWriter{Namespace: "system", PodName: name, PodUID: "pod-" + role, ContainerName: name, ContainerID: "containerd://" + role, ImageID: "image-" + role, Role: role, Protocol: guard.ReferenceWriterProtocol}
		if err := guard.RegisterReferenceWriter(database, writer); err != nil {
			t.Fatal(err)
		}
		writers = append(writers, writer)
	}
	h.inspectRegistryCoverage = func(context.Context, guard.StorageRegistration, []guard.ParticipantRegistration) (guard.RegistryCoverage, error) {
		return guard.RegistryCoverage{Ingress: []guard.ParticipantRegistration{ingress}, Writers: writers, References: guard.RegionReferenceInventory{Complete: true}}, nil
	}
	if _, ready, err := h.certifyRegistry(context.Background(), binding, nil); err != nil || !ready {
		t.Fatal("fixture not certified", ready, err)
	}
	return &writers
}

func TestRegistryCertificationReturnsCollectedReferences(t *testing.T) {
	database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "inventory.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	database.LogMode(false)
	if err := database.AutoMigrate(&model.CleanupStorage{}, &model.CleanupOperation{}).Error; err != nil {
		t.Fatal(err)
	}
	binding, err := guard.ProvisionRegistryStorage(database, "owned-volume", "/owned/registry")
	if err != nil {
		t.Fatal(err)
	}
	h := &CleanupCoordinationHandler{database: func() *gorm.DB { return database }}
	installRegistryCoverageFixture(t, h, binding)
	inspect := h.inspectRegistryCoverage
	calls := 0
	h.inspectRegistryCoverage = func(ctx context.Context, binding guard.StorageRegistration, records []guard.ParticipantRegistration) (guard.RegistryCoverage, error) {
		calls++
		coverage, err := inspect(ctx, binding, records)
		coverage.References.Images = []string{"goodrain.me/retained:v1"}
		return coverage, err
	}
	_, ready, references, collected, err := h.certifyRegistryInventory(context.Background(), binding, nil)
	if err != nil || !ready || !collected || calls != 1 || len(references.Images) != 1 || references.Images[0] != "goodrain.me/retained:v1" {
		t.Fatal("certification did not return its exact reference observation", ready, collected, calls, references, err)
	}
}

// capability_id: rainbond.cleanup.registry-permit-fresh-coverage
func TestRegistryPermitRevalidatesCurrentCoverage(t *testing.T) {
	database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "permit.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	database.LogMode(false)
	if err := database.AutoMigrate(&model.CleanupStorage{}, &model.CleanupOperation{}).Error; err != nil {
		t.Fatal(err)
	}
	binding, err := guard.ProvisionRegistryStorage(database, "owned-volume", "/owned/registry")
	if err != nil {
		t.Fatal(err)
	}
	h := &CleanupCoordinationHandler{database: func() *gorm.DB { return database }, permitKey: func() []byte { return []byte(strings.Repeat("owned-fixture-", 4)) }}
	writers := installRegistryCoverageFixture(t, h, binding)
	request := guard.CoordinationRequest{StorageID: binding.StorageID, Generation: binding.Generation, OperationID: "selected", Owner: "plugin", Kind: "delete", Scope: "app", Target: "sha256:" + strings.Repeat("a", 64), Fingerprint: "selection"}
	if _, err := guard.AcquireOperation(database, request); err != nil {
		t.Fatal(err)
	}
	router := chi.NewRouter()
	router.Post("/stores/{storage_id}/operations/{operation_id}/registry-permit", h.RegistryPermit)
	invoke := func() *httptest.ResponseRecorder {
		raw, _ := json.Marshal(request)
		result := httptest.NewRecorder()
		router.ServeHTTP(result, httptest.NewRequest("POST", "/stores/"+binding.StorageID+"/operations/selected/registry-permit", bytes.NewReader(raw)))
		return result
	}
	if result := invoke(); result.Code != 200 {
		t.Fatal("valid original selection denied", result.Code)
	}
	(*writers)[0].ContainerID = "containerd://replacement"
	if result := invoke(); result.Code == 200 || strings.Contains(result.Body.String(), "\"permit\"") {
		t.Fatal("stale scan received permit", result.Code)
	}
}
