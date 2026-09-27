//go:build linux || darwin

package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	coordination "github.com/goodrain/rainbond/pkg/cleanup"
	"github.com/goodrain/rainbond/pkg/cleanup/registryproxy"
)

// capability_id: rainbond.cleanup.gc-executor-command
func TestGCCommandValidatesBeforeExecutionAndSeparatesRecovery(t *testing.T) {
	t.Setenv("CLEANUP_GC_OPERATION", "")
	directory := t.TempDir()
	configuration := filepath.Join(directory, "operation.json")
	credential := filepath.Join(directory, "credential")
	if err := os.WriteFile(configuration, []byte(`{"binding":{"storage_id":"owned","generation":"one","volume_uid":"volume","root_path":"/registry"},"request":{"generation":"one","operation_id":"gc","owner":"executor","kind":"gc","scope":"*","fingerprint":"confirmation"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(credential, []byte("isolated-fixture-credential-value"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("POD_NAME", "gc-pod")
	t.Setenv("POD_UID", "pod-uid")
	called := 0
	recoverMode := false
	invoke := func(ctx context.Context, root string, binding coordination.StorageRegistration, request coordination.CoordinationRequest, binary string, recorder registryproxy.GCExecutionRecorder, recover bool) error {
		called++
		recoverMode = recover
		if root != "/registry" || request.StorageID != "owned" || binary != "/bin/registry" || recorder == nil {
			t.Fatal("incorrect executable binding")
		}
		return nil
	}
	args := []string{"--configuration-file", configuration, "--credential-file", credential, "--coordination-api", "http://127.0.0.1:12345", "--allow-internal-http"}
	if err := runGC(context.Background(), args, invoke); err != nil || called != 1 || recoverMode {
		t.Fatal(called, err)
	}
	if err := runGC(context.Background(), append(append([]string{}, args...), "--recover"), invoke); err != nil || called != 2 || !recoverMode {
		t.Fatal(called, err)
	}
	descriptor, err := os.ReadFile(configuration)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("CLEANUP_GC_OPERATION", string(descriptor))
	if err := runGC(context.Background(), args[2:], invoke); err != nil || called != 3 {
		t.Fatal("Job environment descriptor failed", err)
	}
	if err := runGC(context.Background(), args, invoke); err == nil || called != 3 {
		t.Fatal("ambiguous descriptor accepted", err)
	}
	t.Setenv("CLEANUP_GC_OPERATION", "")
	t.Setenv("POD_UID", "")
	if err := runGC(context.Background(), args, invoke); err == nil || called != 3 {
		t.Fatal("missing downward API identity executed", err)
	}
	t.Setenv("POD_UID", "pod-uid")
	if err := os.WriteFile(configuration, []byte(`{"binding":{},"request":{},"binary":"/bin/sh"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := runGC(context.Background(), args, invoke); err == nil || called != 3 {
		t.Fatal("unknown execution override accepted", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := runGC(ctx, args, invoke); err == nil || called != 3 {
		t.Fatal("canceled execution invoked", err)
	}
}
