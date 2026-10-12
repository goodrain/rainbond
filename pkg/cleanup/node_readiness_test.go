package cleanup

import (
	"testing"

	"github.com/goodrain/rainbond/db/model"
)

// capability_id: rainbond.cleanup.node-candidate-readiness
func TestNodeReadinessDoesNotIgnoreActiveOperations(t *testing.T) {
	database, _ := coordinationDB(t)
	binding, err := ProvisionManagedCacheStorage(database, "cache-volume", "/cache/build")
	if err != nil {
		t.Fatal(err)
	}
	before, err := InspectStorage(database, binding.StorageID, binding.Generation)
	if err != nil {
		t.Fatal(err)
	}
	_, idle, err := InspectManagedCacheReadiness(database, binding)
	if err != nil || idle {
		t.Fatal("collecting reported deletable", err)
	}
	database.Model(&model.CleanupStorage{}).Where("storage_id = ?", binding.StorageID).Update("mode", "ready")
	_, idle, err = InspectManagedCacheReadiness(database, binding)
	if err != nil || !idle {
		t.Fatal("empty ready store not observed", err)
	}
	pending := model.CleanupOperation{OperationID: "active-build", StorageID: binding.StorageID, Generation: binding.Generation, Kind: "producer", Owner: "builder", Scope: "*", State: "active"}
	if err := database.Create(&pending).Error; err != nil {
		t.Fatal(err)
	}
	observed, idle, err := InspectManagedCacheReadiness(database, binding)
	if err != nil || idle || observed.Revision != before.Revision {
		t.Fatal("active producer ignored or revision mutated", err)
	}
	database.Model(&pending).Update("state", "finished")
	_, idle, err = InspectManagedCacheReadiness(database, binding)
	if err != nil || !idle {
		t.Fatal("finished producer still active", err)
	}
}
