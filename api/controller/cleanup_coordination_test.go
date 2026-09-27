package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/api/middleware"
	"github.com/goodrain/rainbond/db/model"
	guard "github.com/goodrain/rainbond/pkg/cleanup"
	"github.com/goodrain/rainbond/pkg/cleanup/kubeidentity"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
)

func TestStorageDiscoveryReturnsOnlyRegisteredIdentities(t *testing.T) {
	database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "discovery.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := database.AutoMigrate(&model.CleanupStorage{}).Error; err != nil {
		t.Fatal(err)
	}
	binding, err := guard.ProvisionRegistryStorage(database, "owned-volume", "/owned/registry")
	if err != nil {
		t.Fatal(err)
	}
	h := &CleanupCoordinationHandler{database: func() *gorm.DB { return database }}
	response := httptest.NewRecorder()
	h.DiscoverStores(response, httptest.NewRequest("POST", "/stores/discover", strings.NewReader("{}")))
	var result struct {
		Bean struct {
			Protocol int                   `json:"protocol"`
			Stores   []guard.StoreIdentity `json:"stores"`
		} `json:"bean"`
	}
	if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &result) != nil || result.Bean.Protocol != 1 || len(result.Bean.Stores) != 1 || result.Bean.Stores[0].StorageID != binding.StorageID {
		t.Fatal("invalid storage discovery response", response.Code)
	}
	if strings.Contains(response.Body.String(), "/owned/registry") {
		t.Fatal("unnecessary physical storage details exposed")
	}
}

func TestCleanupCoordinationAuthenticationAndRequestBinding(t *testing.T) {
	database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "coordination.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	database.LogMode(false)
	if err := database.AutoMigrate(&model.CleanupStorage{}, &model.CleanupOperation{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Create(&model.CleanupStorage{StorageID: "store", Generation: "generation", Mode: "ready"}).Error; err != nil {
		t.Fatal(err)
	}
	calls := 0
	h := &CleanupCoordinationHandler{database: func() *gorm.DB { calls++; return database }}
	t.Setenv("TOKEN", "isolated-region-fixture")
	payload := []byte(`{"generation":"generation","owner":"builder/instance","operation_id":"op","kind":"producer","scope":"app/image","fingerprint":"bound-request"}`)
	run := func(body []byte, authorized bool, action http.HandlerFunc) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "/v2/cleanup/stores/store/operations", bytes.NewReader(body))
		route := chi.NewRouteContext()
		route.URLParams.Add("storage_id", "store")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, route))
		if authorized {
			req.Header.Set("Authorization", "Token isolated-region-fixture")
		}
		response := httptest.NewRecorder()
		middleware.FullToken(action).ServeHTTP(response, req)
		return response
	}
	if response := run(payload, false, h.Acquire); response.Code != 401 || calls != 0 {
		t.Fatal("unauthorized request reached storage", response.Code, calls)
	}
	if response := run([]byte(`{"storage_id":"other"}`), true, h.Acquire); response.Code != 400 {
		t.Fatal("accepted body-controlled store", response.Code)
	}
	if response := run(append(append([]byte{}, payload...), []byte(` {}`)...), true, h.Acquire); response.Code != 400 {
		t.Fatal("accepted trailing JSON", response.Code)
	}
	for _, first := range []bool{true, false} {
		response := run(payload, true, h.Acquire)
		if response.Code != 200 {
			t.Fatal(response.Code, response.Body.String())
		}
		var envelope map[string]interface{}
		if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
			t.Fatal(err)
		}
		data, ok := envelope["bean"].(map[string]interface{})
		if !ok {
			data, _ = envelope["data"].(map[string]interface{})
		}
		if data == nil || data["newly_admitted"] != first {
			t.Fatal("wrong replay admission", envelope)
		}
	}
	t.Setenv("TOKEN", "")
	before := calls
	if response := run(payload, true, h.Acquire); response.Code != 401 || calls != before {
		t.Fatal("missing configured token bypassed authorization")
	}
}

func TestCoordinationClientThroughAuthenticatedMaintenanceAPI(t *testing.T) {
	for _, restored := range []bool{true, false} {
		t.Run(fmt.Sprint(restored), func(t *testing.T) {
			database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "coordination.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer database.Close()
			database.LogMode(false)
			database.DB().SetMaxOpenConns(1)
			if err := database.AutoMigrate(&model.CleanupStorage{}, &model.CleanupOperation{}).Error; err != nil {
				t.Fatal(err)
			}
			if err := database.Create(&model.CleanupStorage{StorageID: "store", Generation: "generation", Mode: "ready"}).Error; err != nil {
				t.Fatal(err)
			}
			t.Setenv("TOKEN", "isolated-region-fixture")
			h := &CleanupCoordinationHandler{database: func() *gorm.DB { return database }}
			router := chi.NewRouter()
			router.Use(middleware.FullToken)
			base := "/v2/cleanup/stores/{storage_id}/operations"
			router.Post(base, h.Acquire)
			router.Post(base+"/{operation_id}/finish", h.Finish)
			router.Post(base+"/{operation_id}/inspect", h.Inspect)
			router.Post(base+"/{operation_id}/maintenance/request", h.RequestMaintenance)
			router.Post(base+"/{operation_id}/maintenance/enter", h.EnterMaintenance)
			router.Post(base+"/{operation_id}/maintenance/complete", h.CompleteMaintenanceWork)
			router.Post(base+"/{operation_id}/maintenance/restore", h.BeginRestore)
			router.Post(base+"/{operation_id}/maintenance/restored", h.FinishRestore)
			server := httptest.NewServer(router)
			defer server.Close()
			client, err := guard.NewCoordinationClient(server.URL, "isolated-region-fixture", true)
			if err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			producer := guard.CoordinationRequest{StorageID: "store", Generation: "generation", Owner: "builder", OperationID: "build", Kind: "producer", Scope: "app/image", Fingerprint: "original-build"}
			if admitted, err := client.Acquire(ctx, producer); err != nil || !admitted {
				t.Fatal(admitted, err)
			}
			gc := guard.CoordinationRequest{StorageID: "store", Generation: "generation", Owner: "gc-executor", OperationID: "gc", Kind: "gc", Scope: "*", Fingerprint: "manual-gc"}
			if admitted, err := client.RequestMaintenance(ctx, gc); err != nil || !admitted {
				t.Fatal(admitted, err)
			}
			if err := client.EnterMaintenance(ctx, gc); !errors.Is(err, guard.ErrCoordinationBusy) {
				t.Fatal("GC began before producer stopped", err)
			}
			if err := client.Finish(ctx, producer, true); err != nil {
				t.Fatal(err)
			}
			if err := client.EnterMaintenance(ctx, gc); err != nil {
				t.Fatal(err)
			}
			if err := client.EnterMaintenance(ctx, gc); !errors.Is(err, guard.ErrCoordinationChanged) {
				t.Fatal("duplicate GC execution grant", err)
			}
			if err := client.CompleteMaintenanceWork(ctx, gc, "succeeded"); err != nil {
				t.Fatal(err)
			}
			if err := client.BeginRestore(ctx, gc); err != nil {
				t.Fatal(err)
			}
			if state, err := client.Inspect(ctx, gc); err != nil || state != "restoring" {
				t.Fatal(state, err)
			}
			if err := client.FinishRestore(ctx, gc, restored); err != nil {
				t.Fatal(err)
			}
			producer.OperationID = "next-build"
			admitted, err := client.Acquire(ctx, producer)
			if restored && (err != nil || !admitted) {
				t.Fatal("confirmed restore did not open admission", admitted, err)
			}
			if !restored {
				if !errors.Is(err, guard.ErrCoordinationBusy) || admitted {
					t.Fatal("unconfirmed restore admitted writes", admitted, err)
				}
				if err := client.FinishRestore(ctx, gc, true); !errors.Is(err, guard.ErrCoordinationUncertain) {
					t.Fatal("blind retry released uncertain restore", err)
				}
			}
		})
	}
}

// capability_id: rainbond.cleanup.registry-preparation
func TestRegistryPreparationDerivesIdentityAndNeverPromotesReady(t *testing.T) {
	database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "registry.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	database.LogMode(false)
	if err := database.AutoMigrate(&model.CleanupStorage{}, &model.CleanupOperation{}).Error; err != nil {
		t.Fatal(err)
	}
	calls := 0
	h := &CleanupCoordinationHandler{database: func() *gorm.DB { return database }, inspectRegistry: func(_ context.Context, pod, uid string) (kubeidentity.RegistryPreparation, error) {
		calls++
		if pod != "owned-pod" || uid != "owned-uid" {
			return kubeidentity.RegistryPreparation{}, kubeidentity.ErrBinding
		}
		return kubeidentity.RegistryPreparation{Mount: kubeidentity.RegistryMountObservation{VolumeUID: "observed-volume"}, Container: "registry", Root: "/var/lib/registry"}, nil
	}}
	t.Setenv("TOKEN", "isolated-fixture")
	invoke := func(body string) *httptest.ResponseRecorder {
		request := httptest.NewRequest("POST", "/registry/prepare", strings.NewReader(body))
		request.Header.Set("Authorization", "Token isolated-fixture")
		response := httptest.NewRecorder()
		middleware.FullToken(http.HandlerFunc(h.PrepareRegistry)).ServeHTTP(response, request)
		return response
	}
	if response := invoke(`{"pod":"owned-pod","pod_uid":"owned-uid","storage_id":"forged"}`); response.Code != 400 || calls != 0 {
		t.Fatal("caller supplied identity accepted")
	}
	var first guard.StorageRegistration
	for i := 0; i < 2; i++ {
		response := invoke(`{"pod":"owned-pod","pod_uid":"owned-uid"}`)
		if response.Code != 200 {
			t.Fatal("preparation failed", response.Code)
		}
		var decoded struct {
			Bean struct {
				Registration guard.StorageRegistration `json:"registration"`
				Storage      guard.StorageObservation  `json:"storage"`
			} `json:"bean"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &decoded); err != nil {
			t.Fatal(err)
		}
		if decoded.Bean.Storage.Mode != "collecting" || decoded.Bean.Registration.VolumeUID != "observed-volume" {
			t.Fatal("unverified caller state promoted")
		}
		if i == 0 {
			first = decoded.Bean.Registration
		} else if first != decoded.Bean.Registration {
			t.Fatal("generation changed on retry")
		}
	}
	if response := invoke(`{"pod":"other-pod","pod_uid":"owned-uid"}`); response.Code != 503 {
		t.Fatal("failed inspection did not block preparation")
	}
}

func TestRegistryParticipantEndpointRejectsClaimedRuntimeFacts(t *testing.T) {
	database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "participants.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	database.LogMode(false)
	if err := database.AutoMigrate(&model.CleanupStorage{}, &model.CleanupOperation{}, &model.CleanupParticipant{}).Error; err != nil {
		t.Fatal(err)
	}
	binding, err := guard.ProvisionRegistryStorage(database, "observed-volume", "/var/lib/registry")
	if err != nil {
		t.Fatal(err)
	}
	fingerprint, _ := binding.Fingerprint()
	calls := 0
	h := &CleanupCoordinationHandler{database: func() *gorm.DB { return database }, inspectParticipant: func(_ context.Context, pod, uid, owner string, _ guard.StorageRegistration) (guard.ParticipantRegistration, error) {
		calls++
		return guard.ParticipantRegistration{StorageID: binding.StorageID, Generation: binding.Generation, Owner: owner, Role: "registry-ingress", PodUID: uid, ContainerID: "observed-container", ImageID: "observed-image", BindingFingerprint: fingerprint}, nil
	}}
	t.Setenv("TOKEN", "isolated-fixture")
	router := chi.NewRouter()
	router.Use(middleware.FullToken)
	router.Post("/stores/{storage_id}/participants/registry", h.RegisterRegistryParticipant)
	send := func(extra bool) int {
		body := map[string]interface{}{"generation": binding.Generation, "pod": "pod", "pod_uid": "pod-uid", "owner": "instance"}
		if extra {
			body["container_id"] = "forged"
		}
		raw, _ := json.Marshal(body)
		request := httptest.NewRequest("POST", "/stores/"+binding.StorageID+"/participants/registry", bytes.NewReader(raw))
		request.Header.Set("Authorization", "Token isolated-fixture")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		return response.Code
	}
	if status := send(true); status != 400 || calls != 0 {
		t.Fatal("claimed container reached inspector", status)
	}
	if status := send(false); status != 200 {
		t.Fatal("verified participant not registered", status)
	}
	var participant model.CleanupParticipant
	if err := database.First(&participant).Error; err != nil || participant.ContainerID != "observed-container" {
		t.Fatal(participant, err)
	}
	observed, err := guard.InspectStorage(database, binding.StorageID, binding.Generation)
	if err != nil || observed.Mode != "collecting" {
		t.Fatal("registration enabled cleanup", err)
	}
}

func TestRegistryReferenceAuditUsesBoundAuthenticatedOperation(t *testing.T) {
	database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "references.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	database.LogMode(false)
	if err := database.AutoMigrate(&model.CleanupStorage{}, &model.CleanupOperation{}, &model.VersionInfo{}, &model.TenantPluginBuildVersion{}, &model.K8sResource{}, &model.KeyValue{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Create(&model.CleanupStorage{StorageID: "owned", Generation: "one", Mode: "ready"}).Error; err != nil {
		t.Fatal(err)
	}
	selected := guard.CoordinationRequest{StorageID: "owned", Generation: "one", OperationID: "delete", Owner: "owner", Kind: "delete", Scope: "app", Target: "sha256:" + strings.Repeat("a", 64), Fingerprint: "confirmation"}
	if _, err := guard.AcquireOperation(database, selected); err != nil {
		t.Fatal(err)
	}
	h := &CleanupCoordinationHandler{database: func() *gorm.DB { return database }}
	router := chi.NewRouter()
	router.Use(middleware.FullToken)
	router.Post("/stores/{storage_id}/operations/{operation_id}/registry-references", h.RegistryReferences)
	router.Post("/stores/{storage_id}/reference-inventory", h.RegistryReferenceInventory)
	t.Setenv("TOKEN", "isolated-reference-fixture")
	invoke := func(auth string) *httptest.ResponseRecorder {
		raw, _ := json.Marshal(struct {
			guard.CoordinationRequest
			Tags []string `json:"tags"`
		}{selected, []string{"v1"}})
		request := httptest.NewRequest("POST", "/stores/owned/operations/delete/registry-references", bytes.NewReader(raw))
		request.Header.Set("Authorization", auth)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		return response
	}
	if response := invoke(""); response.Code == 200 {
		t.Fatal("unauthenticated reference audit")
	}
	for _, referenced := range []bool{false, true} {
		if referenced {
			if err := database.Create(&model.VersionInfo{ImageName: "goodrain.me/app:v1"}).Error; err != nil {
				t.Fatal(err)
			}
		}
		response := invoke("Token isolated-reference-fixture")
		var body struct {
			Bean struct {
				Protocol   int  `json:"protocol"`
				Complete   bool `json:"region_records_complete"`
				Referenced bool `json:"referenced"`
			} `json:"bean"`
		}
		if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &body) != nil || body.Bean.Protocol != 1 || !body.Bean.Complete || body.Bean.Referenced != referenced {
			t.Fatal("incorrect reference audit", response.Code)
		}
	}
	request := httptest.NewRequest("POST", "/stores/owned/reference-inventory", strings.NewReader(`{"generation":"one"}`))
	request.Header.Set("Authorization", "Token isolated-reference-fixture")
	snapshot := httptest.NewRecorder()
	router.ServeHTTP(snapshot, request)
	var inventory struct {
		Bean struct {
			StorageID  string `json:"storage_id"`
			Generation string `json:"generation"`
			guard.RegionReferenceInventory
		} `json:"bean"`
	}
	if snapshot.Code != 200 || json.Unmarshal(snapshot.Body.Bytes(), &inventory) != nil || inventory.Bean.StorageID != "owned" || inventory.Bean.Generation != "one" || !inventory.Bean.Complete || len(inventory.Bean.Images) != 1 {
		t.Fatal("invalid advisory reference inventory", snapshot.Code)
	}
	selected.Owner = "other"
	if response := invoke("Token isolated-reference-fixture"); response.Code == 200 {
		t.Fatal("foreign admission accepted")
	}
}

// capability_id: rainbond.cleanup.managed-cache-binding
func TestManagedCachePreparationDerivesIdentityAndNeverPromotesReady(t *testing.T) {
	database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "registry.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	database.LogMode(false)
	if err := database.AutoMigrate(&model.CleanupStorage{}, &model.CleanupOperation{}).Error; err != nil {
		t.Fatal(err)
	}
	calls := 0
	h := &CleanupCoordinationHandler{database: func() *gorm.DB { return database }, inspectManagedCache: func(_ context.Context, pod, uid string) (kubeidentity.ManagedCachePreparation, error) {
		calls++
		if pod != "owned-pod" || uid != "owned-uid" {
			return kubeidentity.ManagedCachePreparation{}, kubeidentity.ErrBinding
		}
		return kubeidentity.ManagedCachePreparation{Mount: kubeidentity.RegistryMountObservation{VolumeUID: "observed-volume"}, NodeUID: "observed-node", Root: "/cache/build"}, nil
	}}
	t.Setenv("TOKEN", "isolated-fixture")
	invoke := func(body string) *httptest.ResponseRecorder {
		request := httptest.NewRequest("POST", "/managed-cache/prepare", strings.NewReader(body))
		request.Header.Set("Authorization", "Token isolated-fixture")
		response := httptest.NewRecorder()
		middleware.FullToken(http.HandlerFunc(h.PrepareManagedCache)).ServeHTTP(response, request)
		return response
	}
	if response := invoke(`{"pod":"owned-pod","pod_uid":"owned-uid","storage_id":"forged"}`); response.Code != 400 || calls != 0 {
		t.Fatal("caller supplied identity accepted")
	}
	var first guard.StorageRegistration
	for i := 0; i < 2; i++ {
		response := invoke(`{"pod":"owned-pod","pod_uid":"owned-uid"}`)
		if response.Code != 200 {
			t.Fatal("preparation failed", response.Code)
		}
		var decoded struct {
			Bean struct {
				Registration guard.StorageRegistration `json:"registration"`
				Storage      guard.StorageObservation  `json:"storage"`
			} `json:"bean"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &decoded); err != nil {
			t.Fatal(err)
		}
		if decoded.Bean.Storage.Mode != "collecting" || decoded.Bean.Registration.VolumeUID != "observed-volume" {
			t.Fatal("unverified caller state promoted")
		}
		if i == 0 {
			first = decoded.Bean.Registration
		} else if first != decoded.Bean.Registration {
			t.Fatal("generation changed on retry")
		}
	}
	if response := invoke(`{"pod":"other-pod","pod_uid":"owned-uid"}`); response.Code != 503 {
		t.Fatal("failed inspection did not block preparation")
	}
}
