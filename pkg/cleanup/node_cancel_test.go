package cleanup

import (
	"strings"
	"testing"

	"github.com/goodrain/rainbond/db/model"
)

// capability_id: rainbond.cleanup.node-cancel-before-grant
func TestNodeCancellationCannotOverrideNativeAdmission(t *testing.T) {
	for _, stage := range []string{"missing", "admitted", "prepared"} {
		database, _ := coordinationDB(t)
		storage, err := ProvisionManagedCacheStorage(database, "cache", "/cache/build")
		if err != nil {
			t.Fatal(err)
		}
		database.Model(&model.CleanupStorage{}).Where("storage_id = ?", storage.StorageID).Update("mode", "ready")
		intent := NodeJobIntent{Namespace: "system", NodeName: "node", NodeUID: "uid", Entry: "entry", Fingerprint: strings.Repeat("a", 64)}
		request, err := ManagedNodeRequest(storage, "owner", "cancel-node", "plan", intent)
		if err != nil {
			t.Fatal(err)
		}
		if stage != "missing" {
			if _, err := AcquireOperation(database, request); err != nil {
				t.Fatal(err)
			}
		}
		if stage == "prepared" {
			intent.SpecHash = strings.Repeat("b", 64)
			if _, _, err := PrepareNodeJob(database, request, intent); err != nil {
				t.Fatal(err)
			}
		}
		if err := CancelNodeBeforeGrant(database, request, intent); err != nil {
			t.Fatal(err)
		}
		progress, err := ReadNodeJobProgress(database, request)
		if err != nil || progress.State != "finished" || progress.Outcome != "canceled" || progress.Execution.CanceledBeforeGrantAt == nil || progress.Execution.Result != nil {
			t.Fatal("cancellation fabricated execution", progress, err)
		}
		if err := CancelNodeBeforeGrant(database, request, intent); err != nil {
			t.Fatal("lost reply not recoverable", err)
		}
		if granted, err := AcquireOperation(database, request); err != nil || granted {
			t.Fatal("late admission escaped cancellation", err)
		}
		if _, created, err := PrepareNodeJob(database, request, NodeJobIntent{Namespace: "system", NodeName: "node", NodeUID: "uid", Entry: "entry", Fingerprint: strings.Repeat("a", 64), SpecHash: strings.Repeat("b", 64)}); err == nil || created {
			t.Fatal("canceled task recreated")
		}
	}
	database, _ := coordinationDB(t)
	storage, err := ProvisionManagedCacheStorage(database, "another-cache", "/cache/build")
	if err != nil {
		t.Fatal(err)
	}
	database.Model(&model.CleanupStorage{}).Where("storage_id = ?", storage.StorageID).Update("mode", "ready")
	intent := NodeJobIntent{Namespace: "system", NodeName: "node", NodeUID: "uid", Entry: "entry", Fingerprint: strings.Repeat("a", 64), SpecHash: strings.Repeat("b", 64)}
	request, err := ManagedNodeRequest(storage, "owner", "native", "plan", intent)
	if err != nil {
		t.Fatal(err)
	}
	AcquireOperation(database, request)
	PrepareNodeJob(database, request, intent)
	database.Model(&model.CleanupOperation{}).Where("operation_id = ?", request.OperationID).Update("state", "executing")
	if err := CancelNodeBeforeGrant(database, request, intent); err == nil {
		t.Fatal("possible native grant canceled")
	}
}
