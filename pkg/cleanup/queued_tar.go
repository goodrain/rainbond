package cleanup

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/jinzhu/gorm"
)

// NativeTarRequest is shared by enqueue and execution to bind exact serialized input.
func NativeTarRequest(store StoreIdentity, loadID string, body []byte) CoordinationRequest {
	identity := sha256.Sum256([]byte("tar-image-build\x00" + loadID + "\x00" + store.StorageID))
	fingerprint := sha256.Sum256(body)
	return CoordinationRequest{StorageID: store.StorageID, Generation: store.Generation, OperationID: hex.EncodeToString(identity[:]), Owner: "native-tar-image-builder", Kind: "producer", Scope: "*", Fingerprint: hex.EncodeToString(fingerprint[:])}
}

// ReserveQueuedTarLoad protects all enrolled stores before the MQ request. The
// executor claims these same records; enqueue success alone does not release them.
func ReserveQueuedTarLoad(database *gorm.DB, loadID string, body []byte) error {
	if database == nil || !coordinationIdentity.MatchString(loadID) || len(body) == 0 || len(body) > 1<<20 {
		return ErrCoordinationChanged
	}
	stores, err := DiscoverStores(database)
	if err != nil {
		return err
	}
	reserved := []CoordinationRequest{}
	for _, store := range stores {
		request := NativeTarRequest(store, loadID, body)
		created, err := QueueOperation(database, request)
		if err != nil || !created {
			for _, r := range reserved {
				_ = CancelUnsubmittedOperation(database, r)
			}
			if err != nil {
				return err
			}
			return ErrCoordinationChanged
		}
		reserved = append(reserved, request)
	}
	return nil
}
