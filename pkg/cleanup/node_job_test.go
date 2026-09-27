package cleanup

import (
	"errors"
	"strings"
	"testing"

	"github.com/goodrain/rainbond/db/model"
)

// capability_id: rainbond.cleanup.node-job-intent
func TestNodeJobIntentIsImmutableAndDoesNotGrantRecreation(t *testing.T) {
	database, _ := coordinationDB(t)
	binding, err := ProvisionManagedCacheStorage(database, "cache-volume", "/cache/build")
	if err != nil {
		t.Fatal(err)
	}
	// Isolated coordinator fixture only; production registration stays collecting.
	database.Model(&model.CleanupStorage{}).Where("storage_id = ?", binding.StorageID).Update("mode", "ready")
	intent := NodeJobIntent{Namespace: "system", NodeName: "node", NodeUID: "node-uid", Entry: "selected", Fingerprint: strings.Repeat("a", 64), SpecHash: strings.Repeat("b", 64)}
	r := operation("node-job", "delete", nodeJobScope(intent))
	r.StorageID = binding.StorageID
	r.Generation = binding.Generation
	r.Target = nodeJobTarget(intent)
	if created, err := AcquireOperation(database, r); err != nil || !created {
		t.Fatal(created, err)
	}
	first, created, err := PrepareNodeJob(database, r, intent)
	if err != nil || !created || first.Name == "" {
		t.Fatal(first, created, err)
	}
	second, created, err := PrepareNodeJob(database, r, intent)
	if err != nil || created || first != second {
		t.Fatal("repeat granted recreation", created, err)
	}
	changed := intent
	changed.NodeUID = "other-node"
	if _, _, err := PrepareNodeJob(database, r, changed); err == nil {
		t.Fatal("changed target accepted")
	}
	if err := BindNodeJob(database, r, first, "job-original"); err != nil {
		t.Fatal(err)
	}
	if err := BindNodeJob(database, r, first, "job-replacement"); err == nil {
		t.Fatal("replacement job accepted")
	}
	observed, err := ReadNodeJobBinding(database, r)
	if err != nil || observed.JobUID != "job-original" {
		t.Fatal(observed, err)
	}
	if err := BeginDeletionAttempt(database, r); !errors.Is(err, ErrCoordinationChanged) {
		t.Fatal("generic attempt bypassed node executor admission", err)
	}
	changed = first
	changed.Name = "caller-chosen-job"
	if _, _, err := PrepareNodeJob(database, r, changed); err == nil {
		t.Fatal("caller-chosen job name accepted")
	}
	if err := FinishOperation(database, r, true); !errors.Is(err, ErrCoordinationChanged) {
		t.Fatal("generic finish bypassed node verification", err)
	}
}
