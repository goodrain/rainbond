//go:build linux || darwin

package registryproxy

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	coordination "github.com/goodrain/rainbond/pkg/cleanup"
)

// GCExecutionRecorder must durably admit this attempt once, after writers drain,
// and persist its before observation. No production no-op implementation exists.
// Recording a known process outcome must not restore write admission.
type GCExecutionRecorder interface {
	BeginGC(context.Context, StorageMeasurement) error
	CompleteGC(context.Context, string) error
	ObserveGC(context.Context, StorageMeasurement) error
}

// ErrGCExecution reports failure without exposing native process output.
var ErrGCExecution = errors.New("registry GC execution failed")

// ErrGCPlatform rejects hosts without the required Linux descriptor semantics.
var ErrGCPlatform = errors.New("descriptor-pinned registry GC requires Linux")

// ExecuteGC runs the verified native Registry executable once against a pinned
// directory descriptor. The trusted launcher must verify the executable/container
// identity and ingress isolation; neither a browser path nor a tag is sufficient.
// Process interruption never authorizes an automatic retry or write restoration.
func ExecuteGC(ctx context.Context, root string, binding coordination.StorageRegistration, request coordination.CoordinationRequest, binary string, recorder GCExecutionRecorder) error {
	if runtime.GOOS != "linux" {
		return ErrGCPlatform
	}
	if _, _, err := gcReceiptKey(binding, request); err != nil {
		return err
	}
	if recorder == nil || !filepath.IsAbs(binary) {
		return ErrGCExecution
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	info, err := os.Stat(binary)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 {
		return ErrGCExecution
	}
	fd, err := openIdentityRoot(root)
	if err != nil {
		return err
	}
	rootFile := os.NewFile(uintptr(fd), "registry-gc-root")
	defer rootFile.Close()
	before, err := measureStorageFD(fd, binding)
	if err != nil {
		return err
	}
	configuration, err := os.CreateTemp("", "registry-gc-config-*.json")
	if err != nil {
		return ErrGCExecution
	}
	defer os.Remove(configuration.Name())
	defer configuration.Close()
	config := map[string]interface{}{"version": "0.1", "log": map[string]string{"level": "error"},
		"storage": map[string]interface{}{"filesystem": map[string]string{"rootdirectory": gcDescriptorPath(3)}}}
	if json.NewEncoder(configuration).Encode(config) != nil {
		return ErrGCExecution
	}
	if _, err := configuration.Seek(0, io.SeekStart); err != nil {
		return ErrGCExecution
	}
	if err := recorder.BeginGC(ctx, before); err != nil {
		return err
	}
	journal, err := reserveGCReceipt(fd, binding, request, before)
	if err != nil {
		return err
	}
	defer journal.Close()
	// Do not use --delete-untagged: retained untagged manifests were not selected.
	command := exec.CommandContext(ctx, binary, "garbage-collect", gcDescriptorPath(4))
	command.ExtraFiles = []*os.File{rootFile, configuration}
	// Inherited REGISTRY_* variables must not redirect storage away from fd 3.
	command.Env = []string{"PATH=/usr/bin:/bin", "OTEL_TRACES_EXPORTER=none", "OTEL_METRICS_EXPORTER=none"}
	command.Stdout, command.Stderr = io.Discard, io.Discard
	command.WaitDelay = 5 * time.Second
	configureGCProcess(command)
	runErr := func() error {
		// Linux's parent-death signal follows the creating OS thread, not just
		// the process; keep that thread alive until the child has been reaped.
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		return command.Run()
	}()
	outcome := "succeeded"
	if runErr != nil {
		outcome = "failed"
	}
	if errors.Is(runErr, exec.ErrWaitDelay) || (command.Process != nil && command.ProcessState == nil) {
		outcome = "unknown"
	}
	var after *StorageMeasurement
	var measurementErr error
	if outcome != "unknown" {
		observed, err := measureStorageFD(fd, binding)
		measurementErr = err
		if err == nil {
			after = &observed
		}
	}
	if err := journal.Complete(outcome, after); err != nil {
		return err
	}
	ack, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if err := recorder.CompleteGC(ack, outcome); err != nil {
		return err
	}
	if outcome == "unknown" {
		return coordination.ErrCoordinationUncertain
	}
	if measurementErr != nil {
		return measurementErr
	}
	if err := recorder.ObserveGC(ack, *after); err != nil {
		return err
	}
	if runErr != nil {
		return ErrGCExecution
	}
	return nil
}

// RecoverGCReceipt forwards the original durable outcome without running GC or
// issuing another admission. Write restoration remains a separate verified step.
func RecoverGCReceipt(ctx context.Context, root string, binding coordination.StorageRegistration, request coordination.CoordinationRequest, recorder GCExecutionRecorder) error {
	if recorder == nil {
		return ErrGCExecution
	}
	receipt, err := ReadGCReceipt(root, binding, request)
	if err != nil {
		return err
	}
	if receipt.Outcome == "unknown" {
		return coordination.ErrCoordinationUncertain
	}
	if err := recorder.CompleteGC(ctx, receipt.Outcome); err != nil {
		return err
	}
	if receipt.After == nil {
		return ErrStorageIdentity
	}
	if err := recorder.ObserveGC(ctx, *receipt.After); err != nil {
		return err
	}
	if receipt.Outcome == "failed" {
		return ErrGCExecution
	}
	return nil
}
