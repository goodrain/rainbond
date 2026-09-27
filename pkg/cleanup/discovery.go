package cleanup

import (
	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
)

// StoreIdentity is an advisory locator, never a cleanup or producer admission.
type StoreIdentity struct {
	StorageID  string `json:"storage_id"`
	Generation string `json:"generation"`
}

// DiscoverStores exposes existing immutable bindings for trusted native writers.
// Missing or corrupt enrollment is an error, not an empty successful inventory.
func DiscoverStores(database *gorm.DB) ([]StoreIdentity, error) {
	var rows []model.CleanupStorage
	if err := database.Order("storage_id").Limit(65).Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) > 64 {
		return nil, ErrCoordinationUnavailable
	}
	stores := make([]StoreIdentity, 0, len(rows))
	for _, row := range rows {
		binding, err := StorageBinding(database, row.StorageID, row.Generation)
		if err != nil {
			return nil, err
		}
		stores = append(stores, StoreIdentity{StorageID: binding.StorageID, Generation: binding.Generation})
	}
	return stores, nil
}
