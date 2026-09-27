package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	coordination "github.com/goodrain/rainbond/pkg/cleanup"
	"github.com/goodrain/rainbond/pkg/cleanup/registryproxy"
)

// capability_id: rainbond.cleanup.verified-storage-measurement
func TestMeasurementModeDoesNotInitializeOrNeedCredentials(t *testing.T) {
	root := t.TempDir()
	args := []string{"--measure-storage", "--storage-root", root, "--storage-id", "store", "--storage-generation", "one", "--volume-uid", "volume", "--registry-path", "/var/lib/registry"}
	var output bytes.Buffer
	if err := runWithOutput(context.Background(), args, &output); err == nil {
		t.Fatal("measurement initialized missing identity")
	}
	if _, err := os.Stat(filepath.Join(root, ".rainbond-cleanup")); !os.IsNotExist(err) {
		t.Fatal("read-only measurement created metadata", err)
	}
	binding := coordination.StorageRegistration{StorageID: "store", Generation: "one", VolumeUID: "volume", RootPath: "/var/lib/registry"}
	if err := registryproxy.InitializeStorageIdentity(root, binding); err != nil {
		t.Fatal(err)
	}
	if err := runWithOutput(context.Background(), args, &output); err != nil {
		t.Fatal(err)
	}
	var measured registryproxy.StorageMeasurement
	if json.Unmarshal(output.Bytes(), &measured) != nil || measured.StorageID != "store" || measured.TotalBytes == 0 {
		t.Fatal("missing measurement response")
	}
	if err := runWithOutput(context.Background(), append(args, "--initialize-storage-identity"), &output); err == nil {
		t.Fatal("mixed measurement and initialization accepted")
	}
}

func TestInstallerModeInitializesOnlyIdentityWithoutCredentials(t *testing.T) {
	root := t.TempDir()
	args := []string{"--initialize-storage-identity", "--storage-root", root, "--storage-id", "store", "--storage-generation", "one", "--volume-uid", "volume", "--registry-path", "/var/lib/registry"}
	if err := run(context.Background(), args); err != nil {
		t.Fatal(err)
	}
	if err := registryproxy.VerifyStorageIdentity(root, coordination.StorageRegistration{StorageID: "store", Generation: "one", VolumeUID: "volume", RootPath: "/var/lib/registry"}); err != nil {
		t.Fatal(err)
	}
}
func TestStartupCannotCreateIdentityOrAcceptInlineCredentials(t *testing.T) {
	root := t.TempDir()
	args := []string{"--storage-root", root, "--storage-id", "store", "--storage-generation", "one", "--volume-uid", "volume", "--registry-path", "/var/lib/registry", "--owner", "instance", "--credential-file", filepath.Join(root, "missing")}
	if err := run(context.Background(), args); err == nil {
		t.Fatal("startup accepted missing credential")
	}
	if err := run(context.Background(), append(args, "--token=fixture-only")); err == nil {
		t.Fatal("inline credential accepted")
	}
	if _, err := (tlsFiles{key: "missing"}).transport(); err == nil {
		t.Fatal("incomplete client TLS accepted")
	}
}

// capability_id: rainbond.cleanup.coordinator-runtime
func TestCoordinatorRunsReadinessAndStopsWithContext(t *testing.T) {
	checkCoordinatorRuntime(t, false)
}

// capability_id: rainbond.cleanup.console-signed-coordinator-runtime
func TestCoordinatorUsesSignedConsoleAndIndependentPermitKey(t *testing.T) {
	checkCoordinatorRuntime(t, true)
}

func checkCoordinatorRuntime(t *testing.T, signed bool) {
	root := t.TempDir()
	binding := coordination.StorageRegistration{StorageID: "store", Generation: "one", VolumeUID: "volume", RootPath: "/var/lib/registry"}
	if err := registryproxy.InitializeStorageIdentity(root, binding); err != nil {
		t.Fatal(err)
	}
	fingerprint, _ := binding.Fingerprint()
	key := strings.Repeat("fixture-only-", 4)
	keyPath := filepath.Join(t.TempDir(), "credential")
	if err := os.WriteFile(keyPath, []byte(key), 0600); err != nil {
		t.Fatal(err)
	}
	core := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if signed {
			if r.Header.Get("Authorization") != "" || r.Header.Get("X-Cleanup-Coordination-Signature") == "" {
				t.Error("missing installation signature")
			}
			var envelope struct{ Path, Body string }
			if json.NewDecoder(r.Body).Decode(&envelope) != nil {
				t.Error("invalid signed envelope")
			}
			path = envelope.Path
		} else if r.Header.Get("Authorization") != "Token "+key {
			t.Error("wrong control-plane request")
		}
		if path == "/v2/cleanup/stores/store/participants/registry" {
			json.NewEncoder(w).Encode(map[string]interface{}{"bean": map[string]interface{}{"protocol": 1, "recorded": true}})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"bean": map[string]interface{}{"protocol": 1, "storage": coordination.StorageObservation{StorageID: "store", Generation: "one", RegistrationFingerprint: fingerprint, Mode: "collecting"}}})
	}))
	defer core.Close()
	registry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Docker-Distribution-API-Version", "registry/2.0")
		w.WriteHeader(401)
	}))
	defer registry.Close()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	listener.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	exited := make(chan error, 1)
	args := []string{"--listen", address, "--storage-root", root, "--storage-id", "store", "--storage-generation", "one", "--volume-uid", "volume", "--registry-path", "/var/lib/registry", "--owner", "fixture-instance", "--pod-name", "pod", "--pod-uid", "uid", "--credential-file", keyPath, "--coordination-api", core.URL, "--allow-internal-http", "--upstream", registry.URL}
	if signed {
		permitPath := filepath.Join(t.TempDir(), "permit")
		if err := os.WriteFile(permitPath, []byte(strings.Repeat("permit-only-", 4)), 0600); err != nil {
			t.Fatal(err)
		}
		args = append(args, "--console-enterprise=enterprise", "--console-region=rainbond", "--permit-key-file="+permitPath)
	}
	go func() { exited <- run(ctx, args) }()
	client := &http.Client{Timeout: time.Second}
	ready := false
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); {
		response, err := client.Get("http://" + address + "/readyz")
		if err == nil {
			response.Body.Close()
			if response.StatusCode == 200 {
				ready = true
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	select {
	case err := <-exited:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("coordinator did not stop")
	}
	if !ready {
		t.Fatal("coordinator readiness never succeeded")
	}
}

func TestSignedCoordinatorRejectsSharedOrMissingPermitKey(t *testing.T) {
	control := filepath.Join(t.TempDir(), "control")
	permit := filepath.Join(t.TempDir(), "permit")
	if err := os.WriteFile(control, []byte(strings.Repeat("control-only-", 4)), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := registryPermitKeyReader(control, "", true); err == nil {
		t.Fatal("accepted missing independent key")
	}
	raw, _ := os.ReadFile(control)
	os.WriteFile(permit, raw, 0600)
	if _, err := registryPermitKeyReader(control, permit, true); err == nil {
		t.Fatal("reused control authentication key for permits")
	}
	os.WriteFile(permit, []byte(strings.Repeat("permit-only-", 4)), 0600)
	read, err := registryPermitKeyReader(control, permit, true)
	if err != nil || string(read()) == string(raw) {
		t.Fatal("independent key not loaded", err)
	}
	os.WriteFile(permit, raw, 0600)
	if len(read()) != 0 {
		t.Fatal("accepted unsafe key rotation")
	}
}
