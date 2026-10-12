package cleanup

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/goodrain/rainbond/db/model"
)

// capability_id: rainbond.cleanup.node-result-finalization
func TestNodeResultIsImmutableAndRequiresExitedOriginalExecutor(t *testing.T) {
	database, _ := coordinationDB(t)
	storage, err := ProvisionManagedCacheStorage(database, "volume", "/cache/build")
	if err != nil {
		t.Fatal(err)
	}
	database.Model(&model.CleanupStorage{}).Where("storage_id = ?", storage.StorageID).Update("mode", "ready")
	intent := NodeJobIntent{Namespace: "system", NodeName: "node", NodeUID: "uid", Entry: "entry", Fingerprint: strings.Repeat("a", 64), SpecHash: strings.Repeat("b", 64)}
	r, err := ManagedNodeRequest(storage, "owner", "node-result", "request", intent)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AcquireOperation(database, r); err != nil {
		t.Fatal(err)
	}
	intent, _, err = PrepareNodeJob(database, r, intent)
	if err != nil {
		t.Fatal(err)
	}
	if err := BindNodeJob(database, r, intent, "job"); err != nil {
		t.Fatal(err)
	}
	observed := NodeExecutorIdentity{Namespace: "system", JobName: intent.Name, JobUID: "job", PodName: "pod", PodUID: "pod-uid", NodeName: "node", NodeUID: "uid", VolumeUID: "volume", SpecHash: intent.SpecHash, ContainerID: "containerd://one", ImageID: "image"}
	if err := EnterNodeExecution(database, r, observed); err != nil {
		t.Fatal(err)
	}
	before := NodeStorageMeasurement{RootDevice: 1, RootInode: 2, TotalBytes: 1000, FreeBytes: 100, AvailableBytes: 80, TotalInodes: 100, FreeInodes: 10, ObservedAt: time.Now().UTC()}
	after := before
	after.FreeBytes = 120
	after.AvailableBytes = 100
	after.ObservedAt = after.ObservedAt.Add(time.Second)
	result := NodeExecutionResult{State: "deleted", Before: &before, After: &after}
	if err := RecordNodeResult(database, r, observed, result); err != nil {
		t.Fatal(err)
	}
	if err := RecordNodeResult(database, r, observed, result); err != nil {
		t.Fatal("lost receipt acknowledgment not idempotent", err)
	}
	changed := result
	changed.State = "failed"
	if err := RecordNodeResult(database, r, observed, changed); err == nil {
		t.Fatal("original outcome overwritten")
	}
	if err := FinishNodeExecution(database, r, observed); err == nil {
		t.Fatal("running executor released")
	}
	observed.FinishedAt = after.ObservedAt.Add(time.Second)
	wrong := observed
	wrong.ContainerID = "containerd://replacement"
	if err := FinishNodeExecution(database, r, wrong); err == nil {
		t.Fatal("replacement executor released scope")
	}
	if err := FinishNodeExecution(database, r, observed); err != nil {
		t.Fatal(err)
	}
	if err := FinishNodeExecution(database, r, observed); err != nil {
		t.Fatal("lost finish reply not idempotent", err)
	}
	if state, err := InspectOperation(database, r); err != nil || state != "finished" {
		t.Fatal(state, err)
	}
	writer := CoordinationRequest{StorageID: r.StorageID, Generation: r.Generation, Owner: "writer", OperationID: "writer-after-node", Kind: "producer", Scope: "*", Fingerprint: "writer"}
	if created, err := AcquireOperation(database, writer); err != nil || !created {
		t.Fatal("finished deletion still blocks writers", err)
	}
	if err := FinishOperation(database, writer, true); err != nil {
		t.Fatal(err)
	}
	intent.Name = ""
	unknownRequest, err := ManagedNodeRequest(storage, "owner", "node-unknown", "request", intent)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AcquireOperation(database, unknownRequest); err != nil {
		t.Fatal(err)
	}
	unknownIntent, _, err := PrepareNodeJob(database, unknownRequest, intent)
	if err != nil {
		t.Fatal(err)
	}
	if err := BindNodeJob(database, unknownRequest, unknownIntent, "unknown-job"); err != nil {
		t.Fatal(err)
	}
	observed.JobName = unknownIntent.Name
	observed.JobUID = "unknown-job"
	observed.FinishedAt = time.Time{}
	if err := EnterNodeExecution(database, unknownRequest, observed); err != nil {
		t.Fatal(err)
	}
	if err := RecordNodeResult(database, unknownRequest, observed, NodeExecutionResult{State: "unknown"}); err != nil {
		t.Fatal(err)
	}
	observed.FinishedAt = time.Now().UTC()
	if err := FinishNodeExecution(database, unknownRequest, observed); !errors.Is(err, ErrCoordinationUncertain) {
		t.Fatal("unknown outcome released", err)
	}
	writer.OperationID = "blocked-writer"
	if _, err := AcquireOperation(database, writer); !errors.Is(err, ErrCoordinationBusy) {
		t.Fatal("unknown deletion stopped protecting storage", err)
	}

}

func TestNodeResultRejectsInvalidFilesystemEvidence(t *testing.T) {
	before := NodeStorageMeasurement{RootDevice: 1, RootInode: 2, TotalBytes: 100, FreeBytes: 20, AvailableBytes: 10, ObservedAt: time.Now().UTC()}
	for _, kind := range []string{"missing", "root", "bytes", "inodes", "time"} {
		after := before
		result := NodeExecutionResult{State: "deleted", Before: &before, After: &after}
		switch kind {
		case "missing":
			result.After = nil
		case "root":
			after.RootInode++
		case "bytes":
			after.AvailableBytes = 101
		case "inodes":
			after.FreeInodes = 1
		case "time":
			after.ObservedAt = before.ObservedAt.Add(-time.Second)
		}
		if validNodeResult(result) {
			t.Fatal("invalid filesystem receipt accepted", kind)
		}
	}
}
