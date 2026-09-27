package controller

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/api/middleware"
	"github.com/goodrain/rainbond/db/model"
	guard "github.com/goodrain/rainbond/pkg/cleanup"
	"github.com/goodrain/rainbond/pkg/cleanup/registryproxy"
	"github.com/jinzhu/gorm"
)

// This owns every process, database, image and file. The fixture's ready state
// proves isolated protocol behavior, not enrollment of production participants.
// capability_id: rainbond.cleanup.coordinated-registry-delete-gc
func TestCoordinatedRegistryRealDeletionAndGC(t *testing.T) {
	runCoordinatedRegistryGC(t, false)
}

// capability_id: rainbond.cleanup.registry-gc-executor
func TestCoordinatedRegistryExecutorRealDeletionAndGC(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux descriptor-pinned GC executor requires /proc/self/fd")
	}
	runCoordinatedRegistryGC(t, true)
}

func runCoordinatedRegistryGC(t *testing.T, useExecutor bool) {
	binary := os.Getenv("CLEANUP_TEST_REGISTRY_BINARY")
	if binary == "" {
		t.Skip("set CLEANUP_TEST_REGISTRY_BINARY to an official local Distribution binary")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	listener.Close()
	root := t.TempDir()
	storage := filepath.Join(root, "storage")
	if err := os.MkdirAll(storage, 0755); err != nil {
		t.Fatal(err)
	}
	measurementBinding := guard.StorageRegistration{StorageID: "isolated", Generation: "one", VolumeUID: "owned-test-volume", RootPath: storage}
	if err := registryproxy.InitializeStorageIdentity(storage, measurementBinding); err != nil {
		t.Fatal(err)
	}
	bindingFingerprint, _ := measurementBinding.Fingerprint()
	bindingJSON, _ := json.Marshal(measurementBinding)
	configPath := filepath.Join(root, "registry.yml")
	testID := filepath.Base(root)
	config := fmt.Sprintf("version: 0.1\nlog:\n  level: error\nstorage:\n  filesystem:\n    rootdirectory: %q\n  delete:\n    enabled: true\nhttp:\n  addr: %q\n  headers:\n    X-Cleanup-Test-ID: [%q]\n", storage, address, testID)
	if err := os.WriteFile(configPath, []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	environment := []string{"PATH=" + os.Getenv("PATH"), "HOME=" + root, "TMPDIR=" + root, "OTEL_TRACES_EXPORTER=none", "OTEL_METRICS_EXPORTER=none"}
	httpClient := &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	origin := "http://" + address
	startRegistry := func() func() {
		command := exec.CommandContext(ctx, binary, "serve", configPath)
		command.Env = environment
		command.Stdout = io.Discard
		command.Stderr = io.Discard
		if err := command.Start(); err != nil {
			t.Fatal("isolated Registry failed to start")
		}
		exited := make(chan error, 1)
		go func() { exited <- command.Wait() }()
		var once sync.Once
		stop := func() {
			once.Do(func() {
				command.Process.Signal(os.Interrupt)
				select {
				case <-exited:
				case <-time.After(3 * time.Second):
					command.Process.Kill()
					<-exited
				}
			})
		}
		for deadline := time.Now().Add(15 * time.Second); time.Now().Before(deadline); {
			response, err := httpClient.Get(origin + "/v2/")
			if err == nil {
				response.Body.Close()
				if response.StatusCode == 200 && response.Header.Get("X-Cleanup-Test-ID") == testID {
					return stop
				}
			}
			time.Sleep(25 * time.Millisecond)
		}
		stop()
		t.Fatal("owned Registry readiness failed; no other endpoint used")
		return nil
	}
	stop := startRegistry()
	defer func() { stop() }()
	database, err := gorm.Open("sqlite3", filepath.Join(root, "coordination.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	database.LogMode(false)
	database.DB().SetMaxOpenConns(1)
	if err := database.AutoMigrate(&model.CleanupStorage{}, &model.CleanupOperation{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Create(&model.CleanupStorage{StorageID: "isolated", Generation: "one", Mode: "ready", RegistrationFingerprint: bindingFingerprint, RegistrationJSON: string(bindingJSON)}).Error; err != nil {
		t.Fatal(err)
	}
	fixtureKey := strings.Repeat("isolated-fixture-", 3)
	t.Setenv("TOKEN", fixtureKey)
	h := &CleanupCoordinationHandler{database: func() *gorm.DB { return database }, permitKey: func() []byte { return []byte(fixtureKey) }}
	router := chi.NewRouter()
	router.Use(middleware.FullToken)
	base := "/v2/cleanup/stores/{storage_id}/operations"
	router.Post(base, h.Acquire)
	for suffix, handler := range map[string]http.HandlerFunc{"finish": h.Finish, "inspect": h.Inspect, "registry-permit": h.RegistryPermit, "attempt": h.BeginAttempt, "attempt/complete": h.CompleteAttempt, "upload": h.BindUpload, "upload/requests": h.AcquireUpload, "upload/requests/finish": h.FinishUpload, "maintenance/measurement": h.RecordMaintenanceMeasurement, "maintenance/request": h.RequestMaintenance, "maintenance/enter": h.EnterMaintenance, "maintenance/complete": h.CompleteMaintenanceWork, "maintenance/restore": h.BeginRestore, "maintenance/restored": h.FinishRestore} {
		router.Post(base+"/{operation_id}/"+suffix, handler)
	}
	router.Post("/v2/cleanup/stores/{storage_id}/uploads/lookup", h.LookupUpload)
	apiServer := httptest.NewServer(router)
	defer apiServer.Close()
	client, err := guard.NewCoordinationClient(apiServer.URL, fixtureKey, true)
	if err != nil {
		t.Fatal(err)
	}
	newProxy := func(owner string) *registryproxy.Proxy {
		coordinator, err := registryproxy.NewCoordinator(registryproxy.CoordinatorConfig{StorageID: "isolated", Generation: "one", Owner: owner, Backend: client, PermitKey: func() []byte { return []byte(fixtureKey) }})
		if err != nil {
			t.Fatal(err)
		}
		proxy, err := registryproxy.NewProxy(origin, coordinator, nil)
		if err != nil {
			t.Fatal(err)
		}
		return proxy
	}
	var active atomic.Value
	active.Store(newProxy("sidecar-first"))
	sidecar := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { active.Load().(*registryproxy.Proxy).ServeHTTP(w, r) }))
	defer sidecar.Close()
	request := func(method, target, media string, data []byte, permit string, want int) http.Header {
		t.Helper()
		req, err := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(data))
		if err != nil {
			t.Fatal("invalid isolated request")
		}
		if media != "" {
			req.Header.Set("Content-Type", media)
		}
		req.Header.Set("Accept", "application/vnd.oci.image.manifest.v1+json")
		if permit != "" {
			req.Header.Set("X-Rainbond-Cleanup-Permit", permit)
		}
		response, err := httpClient.Do(req)
		if err != nil {
			t.Fatal("isolated Registry request failed")
		}
		defer response.Body.Close()
		io.Copy(io.Discard, io.LimitReader(response.Body, 1<<20))
		if response.StatusCode != want {
			t.Fatalf("isolated %s response=%d want=%d", method, response.StatusCode, want)
		}
		return response.Header
	}
	digest := func(raw []byte) string { sum := sha256.Sum256(raw); return "sha256:" + hex.EncodeToString(sum[:]) }
	upload := func(repository string, data []byte) string {
		headers := request("POST", sidecar.URL+"/v2/"+repository+"/blobs/uploads/", "", nil, "", 202)
		// Replace the sidecar instance between requests; no in-memory upload map survives.
		active.Store(newProxy("sidecar-replaced"))
		location, err := url.Parse(headers.Get("Location"))
		if err != nil {
			t.Fatal("invalid upload location")
		}
		originURL, _ := url.Parse(sidecar.URL)
		location = originURL.ResolveReference(location)
		if location.Host != originURL.Host || location.User != nil {
			t.Fatal("upload location bypassed sidecar")
		}
		value := digest(data)
		query := location.Query()
		query.Set("digest", value)
		location.RawQuery = query.Encode()
		request("PUT", location.String(), "application/octet-stream", data, "", 201)
		return value
	}
	shared := []byte("owned shared layer")
	manifests := map[string]string{}
	configs := map[string]string{}
	for _, repository := range []string{"test/selected", "test/retained", "test/untagged"} {
		configBody := []byte(fmt.Sprintf(`{"architecture":"amd64","os":"linux","config":{"Labels":{"fixture":%q}},"rootfs":{"type":"layers","diff_ids":[]}}`, repository))
		configDigest := upload(repository, configBody)
		layerDigest := upload(repository, shared)
		configs[repository] = configDigest
		manifest, _ := json.Marshal(map[string]interface{}{"schemaVersion": 2, "mediaType": "application/vnd.oci.image.manifest.v1+json", "config": map[string]interface{}{"mediaType": "application/vnd.oci.image.config.v1+json", "digest": configDigest, "size": len(configBody)}, "layers": []interface{}{map[string]interface{}{"mediaType": "application/vnd.oci.image.layer.v1.tar", "digest": layerDigest, "size": len(shared)}}})
		reference := "v1"
		if repository == "test/untagged" {
			reference = digest(manifest)
		}
		request("PUT", sidecar.URL+"/v2/"+repository+"/manifests/"+reference, "application/vnd.oci.image.manifest.v1+json", manifest, "", 201)
		manifests[repository] = digest(manifest)
	}
	for deadline := time.Now().Add(5 * time.Second); ; {
		var outstanding int
		if err := database.Model(&model.CleanupOperation{}).Where("state <> ?", "finished").Count(&outstanding).Error; err != nil {
			t.Fatal(err)
		}
		if outstanding == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("upload leases failed to close")
		}
		time.Sleep(10 * time.Millisecond)
	}
	selected := guard.CoordinationRequest{StorageID: "isolated", Generation: "one", Owner: "cleanup-test", OperationID: "selected-delete", Kind: "delete", Scope: "test/selected", Fingerprint: "explicit-selection", Target: manifests["test/selected"]}
	if admitted, err := client.Acquire(ctx, selected); err != nil || !admitted {
		t.Fatal("selected deletion not admitted", err)
	}
	permit, err := client.RegistryPermit(ctx, selected)
	if err != nil {
		t.Fatal("permit not issued")
	}
	request("DELETE", sidecar.URL+"/v2/test/retained/manifests/"+manifests["test/retained"], "", nil, permit, 403)
	request("DELETE", sidecar.URL+"/v2/test/selected/manifests/"+selected.Target, "", nil, permit, 202)
	request("DELETE", sidecar.URL+"/v2/test/selected/manifests/"+selected.Target, "", nil, permit, 409)
	request("GET", sidecar.URL+"/v2/test/selected/manifests/"+selected.Target, "", nil, "", 404)
	request("GET", sidecar.URL+"/v2/test/retained/manifests/"+manifests["test/retained"], "", nil, "", 200)
	if err := client.Finish(ctx, selected, true); err != nil {
		t.Fatal(err)
	}
	gc := guard.CoordinationRequest{StorageID: "isolated", Generation: "one", Owner: "cleanup-test", OperationID: "manual-gc", Kind: "gc", Scope: "*", Fingerprint: "separate-confirmation"}
	if admitted, err := client.RequestMaintenance(ctx, gc); err != nil || !admitted {
		t.Fatal(err)
	}
	request("POST", sidecar.URL+"/v2/test/new/blobs/uploads/", "", nil, "", 409)
	stop()
	t.Setenv("REGISTRY_STORAGE_FILESYSTEM_ROOTDIRECTORY", t.TempDir())
	recorder, err := registryproxy.NewGCRecorder(client, measurementBinding, gc)
	if err != nil {
		t.Fatal(err)
	}
	if useExecutor {
		if err := registryproxy.ExecuteGC(ctx, storage, measurementBinding, gc, binary, recorder); err != nil {
			t.Fatal("owned executor GC failed", err)
		}
		if err := registryproxy.RecoverGCReceipt(ctx, storage, measurementBinding, gc, recorder); err != nil {
			t.Fatal("original executor receipt could not be reconciled", err)
		}
	} else {
		// This exercises the protocol and native GC, not the Linux executor.
		before, err := registryproxy.MeasureStorage(storage, measurementBinding)
		if err != nil {
			t.Fatal(err)
		}
		if err := recorder.BeginGC(ctx, before); err != nil {
			t.Fatal(err)
		}
		command := exec.CommandContext(ctx, binary, "garbage-collect", configPath)
		command.Env, command.Stdout, command.Stderr = environment, io.Discard, io.Discard
		if err := command.Run(); err != nil {
			t.Fatal("owned manual GC failed", err)
		}
		if err := recorder.CompleteGC(ctx, "succeeded"); err != nil {
			t.Fatal(err)
		}
		after, err := registryproxy.MeasureStorage(storage, measurementBinding)
		if err != nil {
			t.Fatal(err)
		}
		if err := recorder.ObserveGC(ctx, after); err != nil {
			t.Fatal(err)
		}
	}
	beforeGC, afterGC, err := guard.MaintenanceMeasurements(database, gc)
	if err != nil || beforeGC == nil || afterGC == nil || beforeGC.FilesystemID != afterGC.FilesystemID {
		t.Fatal("bound GC observations missing", err)
	}
	if err := client.RecordMaintenanceMeasurement(ctx, gc, "after", *afterGC); err != nil {
		t.Fatal("identical measurement retry failed", err)
	}
	changedMeasurement := *afterGC
	changedMeasurement.FilesystemID = "different-filesystem"
	if err := client.RecordMaintenanceMeasurement(ctx, gc, "after", changedMeasurement); !errors.Is(err, guard.ErrCoordinationChanged) {
		t.Fatal("HTTP observation replaced immutable evidence", err)
	}
	wrongIdentity := gc
	wrongIdentity.Owner = "another-executor"
	if err := client.RecordMaintenanceMeasurement(ctx, wrongIdentity, "after", *afterGC); !errors.Is(err, guard.ErrCoordinationChanged) {
		t.Fatal("observation accepted for another operation owner", err)
	}
	// Other processes also use the host filesystem. Its free-space delta is an
	// observation, not proof that all changed bytes were reclaimed by this GC.
	t.Logf("verified backing filesystem: available bytes before=%d after=%d; inode counts available=%t", beforeGC.AvailableBytes, afterGC.AvailableBytes, beforeGC.FreeInodes != nil && afterGC.FreeInodes != nil)
	blobPath := func(d string) string {
		hex := strings.TrimPrefix(d, "sha256:")
		return filepath.Join(storage, "docker/registry/v2/blobs/sha256", hex[:2], hex, "data")
	}
	if _, err := os.Stat(blobPath(configs["test/selected"])); !os.IsNotExist(err) {
		t.Fatal("selected-only config blob was not reclaimed")
	}
	if _, err := os.Stat(blobPath(digest(shared))); err != nil {
		t.Fatal("GC removed retained shared layer")
	}
	if _, err := os.Stat(blobPath(configs["test/untagged"])); err != nil {
		t.Fatal("unselected untagged manifest content was deleted", err)
	}
	if err := client.BeginRestore(ctx, gc); err != nil {
		t.Fatal(err)
	}
	stop = startRegistry()
	request("GET", sidecar.URL+"/v2/test/untagged/manifests/"+manifests["test/untagged"], "", nil, "", 200)
	request("GET", sidecar.URL+"/v2/test/retained/manifests/"+manifests["test/retained"], "", nil, "", 200)
	request("POST", sidecar.URL+"/v2/test/new/blobs/uploads/", "", nil, "", 409)
	if err := client.FinishRestore(ctx, gc, true); err != nil {
		t.Fatal(err)
	}
	upload("test/after-gc", []byte("writes restored"))
	t.Log("real Distribution: uploads survive coordinator replacement; only selected manifest deleted; replay blocked; manual GC preserved shared content; writes restored")
}
