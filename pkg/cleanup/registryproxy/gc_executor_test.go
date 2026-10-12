//go:build linux

package registryproxy

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	coordination "github.com/goodrain/rainbond/pkg/cleanup"
)

type gcRecorderTest struct {
	beginError          error
	completeError       error
	completed, observed int
	outcome             string
}

func (r *gcRecorderTest) BeginGC(context.Context, StorageMeasurement) error { return r.beginError }
func (r *gcRecorderTest) CompleteGC(_ context.Context, outcome string) error {
	r.completed++
	r.outcome = outcome
	return r.completeError
}
func (r *gcRecorderTest) ObserveGC(context.Context, StorageMeasurement) error {
	r.observed++
	return nil
}

// capability_id: rainbond.cleanup.registry-gc-executor
func TestGCExecutorDoesNotStartWithoutAdmission(t *testing.T) {
	root := t.TempDir()
	binding := coordination.StorageRegistration{StorageID: "owned", Generation: "one", VolumeUID: "volume", RootPath: "/registry"}
	if err := InitializeStorageIdentity(root, binding); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "owned-command")
	if err := os.WriteFile(binary, []byte("#!/bin/sh\n: > \"$0.ran\"\nexit 0\n"), 0700); err != nil {
		t.Fatal(err)
	}
	denied := errors.New("scope occupied")
	recorder := &gcRecorderTest{beginError: denied}
	if err := ExecuteGC(context.Background(), root, binding, gcTestRequest(binding), binary, recorder); !errors.Is(err, denied) {
		t.Fatal(err)
	}
	if _, err := os.Stat(binary + ".ran"); !os.IsNotExist(err) {
		t.Fatal("GC started without permission", err)
	}
	if recorder.completed != 0 || recorder.observed != 0 {
		t.Fatal("denied execution produced completion evidence")
	}
}

func TestGCExecutorRecordsProcessFailureAndDoesNotRetryLostReceipt(t *testing.T) {
	for _, receiptLost := range []bool{false, true} {
		name := "process-failed"
		if receiptLost {
			name = "receipt-lost"
		}
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			binding := coordination.StorageRegistration{StorageID: "owned", Generation: "one", VolumeUID: "volume", RootPath: "/registry"}
			if err := InitializeStorageIdentity(root, binding); err != nil {
				t.Fatal(err)
			}
			binary := filepath.Join(t.TempDir(), "owned-command")
			exit := "23"
			recorder := &gcRecorderTest{}
			if receiptLost {
				exit = "0"
				recorder.completeError = errors.New("receipt not acknowledged")
			}
			if err := os.WriteFile(binary, []byte("#!/bin/sh\nprintf 'run\\n' >> \"$0.ran\"\nexit "+exit+"\n"), 0700); err != nil {
				t.Fatal(err)
			}
			err := ExecuteGC(context.Background(), root, binding, gcTestRequest(binding), binary, recorder)
			if err == nil {
				t.Fatal("incomplete cleanup reported success")
			}
			content, readErr := os.ReadFile(binary + ".ran")
			if readErr != nil || string(content) != "run\n" || recorder.completed != 1 {
				t.Fatal("execution or completion was retried", readErr, recorder.completed)
			}
			if receiptLost {
				if !errors.Is(err, recorder.completeError) || recorder.outcome != "succeeded" || recorder.observed != 0 {
					t.Fatal("lost receipt treated as persisted success", err)
				}
				recorder.completeError = nil
				if err := RecoverGCReceipt(context.Background(), root, binding, gcTestRequest(binding), recorder); err != nil {
					t.Fatal("durable outcome could not be recovered", err)
				}
				recovered, err := os.ReadFile(binary + ".ran")
				if err != nil || string(recovered) != "run\n" || recorder.completed != 2 || recorder.observed != 1 {
					t.Fatal("recovery restarted GC or lost its outcome", err)
				}
			} else if !errors.Is(err, ErrGCExecution) || recorder.outcome != "failed" || recorder.observed != 1 {
				t.Fatal("known process failure misclassified", err)
			}
		})
	}
}

func gcTestRequest(binding coordination.StorageRegistration) coordination.CoordinationRequest {
	return coordination.CoordinationRequest{StorageID: binding.StorageID, Generation: binding.Generation, OperationID: "owned-gc", Owner: "executor-test", Kind: "gc", Scope: "*", Fingerprint: "confirmation"}
}
