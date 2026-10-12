package cleanup

import (
	"errors"
	"testing"

	"github.com/goodrain/rainbond/db/model"
)

// capability_id: rainbond.cleanup.registered-storage-discovery
func TestDiscoverStoresRejectsIncompleteRegistration(t *testing.T) {
	database, _ := coordinationDB(t)
	if _, err := DiscoverStores(database); !errors.Is(err, ErrCoordinationChanged) {
		t.Fatal("invalid binding omitted from discovery", err)
	}
	if err := database.Where("storage_id = ?", "store").Delete(&model.CleanupStorage{}).Error; err != nil {
		t.Fatal(err)
	}
	stores, err := DiscoverStores(database)
	if err != nil || stores == nil || len(stores) != 0 {
		t.Fatal("empty enrollment not explicit", stores, err)
	}
	binding, err := ProvisionRegistryStorage(database, "owned-volume", "/registry")
	if err != nil {
		t.Fatal(err)
	}
	stores, err = DiscoverStores(database)
	if err != nil || len(stores) != 1 || stores[0].StorageID != binding.StorageID || stores[0].Generation != binding.Generation {
		t.Fatal(stores, err)
	}
	status, err := InspectStorage(database, binding.StorageID, binding.Generation)
	if err != nil || status.Mode != "collecting" {
		t.Fatal("discovery enabled cleanup", status, err)
	}
}
