package cleanup

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/jinzhu/gorm"
)

// NativeTarRequest is shared by enqueue and execution to bind exact serialized input.
func NativeTarRequest(store StoreIdentity, loadID string, body []byte) CoordinationRequest {
	return QueuedNativeRequest(store, "tar-image", loadID, body)
}

// QueuedNativeRequest binds a supported native task across API and builder.
func QueuedNativeRequest(store StoreIdentity, kind, loadID string, body []byte) CoordinationRequest {
	identity := sha256.Sum256([]byte(kind + "-build\x00" + loadID + "\x00" + store.StorageID))
	fingerprint := sha256.Sum256(body)
	return CoordinationRequest{StorageID: store.StorageID, Generation: store.Generation, OperationID: hex.EncodeToString(identity[:]), Owner: "native-" + kind + "-builder", Kind: "producer", Scope: "*", Fingerprint: hex.EncodeToString(fingerprint[:])}
}

// ReserveQueuedTarLoad protects all enrolled stores before the MQ request. The
// executor claims these same records; enqueue success alone does not release them.
func ReserveQueuedTarLoad(database *gorm.DB, loadID string, body []byte) error {
	return ReserveQueuedNativeTask(database, "tar-image", loadID, body)
}

// ReserveQueuedNativeTask reserves only supported package task types.
func ReserveQueuedNativeTask(database *gorm.DB, kind, loadID string, body []byte) error {
	if kind != "tar-image" && kind != "service-check" {
		return ErrCoordinationChanged
	}
	if database == nil || !coordinationIdentity.MatchString(loadID) || len(body) == 0 || len(body) > 1<<20 {
		return ErrCoordinationChanged
	}
	stores, err := DiscoverStores(database)
	if err != nil {
		return err
	}
	reserved := []CoordinationRequest{}
	for _, store := range stores {
		request := QueuedNativeRequest(store, kind, loadID, body)
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
