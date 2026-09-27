package cleanup

import (
	"errors"
	"strings"
	"testing"

	"github.com/goodrain/rainbond/db/model"
)

// capability_id: rainbond.cleanup.node-executor-admission
func TestNodeExecutorAdmissionIsBoundAndSingleUse(t *testing.T) {
	database, _ := coordinationDB(t)
	storage, err := ProvisionManagedCacheStorage(database, "volume", "/cache/build")
	if err != nil {
		t.Fatal(err)
	}
	database.Model(&model.CleanupStorage{}).Where("storage_id = ?", storage.StorageID).Update("mode", "ready")
	intent := NodeJobIntent{Namespace: "system", NodeName: "node", NodeUID: "node-uid", Entry: "entry", Fingerprint: strings.Repeat("a", 64), SpecHash: strings.Repeat("b", 64)}
	r := operation("node-native", "delete", nodeJobScope(intent))
	r.StorageID = storage.StorageID
	r.Generation = storage.Generation
	r.Target = nodeJobTarget(intent)
	if _, err := AcquireOperation(database, r); err != nil {
		t.Fatal(err)
	}
	intent, _, err = PrepareNodeJob(database, r, intent)
	if err != nil {
		t.Fatal(err)
	}
	if err := BindNodeJob(database, r, intent, "job-uid"); err != nil {
		t.Fatal(err)
	}
	observed := NodeExecutorIdentity{Namespace: intent.Namespace, JobName: intent.Name, JobUID: "job-uid", PodName: "executor", PodUID: "pod-uid", NodeName: "node", NodeUID: "node-uid", VolumeUID: "volume", SpecHash: intent.SpecHash, ContainerID: "containerd://original", ImageID: "example.test/helper@sha256:" + strings.Repeat("c", 64)}
	changed := observed
	changed.NodeUID = "other"
	if err := EnterNodeExecution(database, r, changed); err == nil {
		t.Fatal("wrong node admitted")
	}
	if err := EnterNodeExecution(database, r, observed); err != nil {
		t.Fatal(err)
	}
	if err := EnterNodeExecution(database, r, observed); !errors.Is(err, ErrCoordinationUncertain) {
		t.Fatal("native grant replayed", err)
	}
	bound, err := ReadNodeJobBinding(database, r)
	if err != nil || bound.PodUID != observed.PodUID || bound.ContainerID != observed.ContainerID {
		t.Fatal(bound, err)
	}
	if state, err := InspectOperation(database, r); err != nil || state != "executing" {
		t.Fatal(state, err)
	}
}
