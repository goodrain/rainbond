package helm

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/goodrain/rainbond/db"
	guard "github.com/goodrain/rainbond/pkg/cleanup"
	"github.com/jinzhu/gorm"
	"helm.sh/helm/v3/pkg/release"
)

// runCoordinatedMutation protects native Helm effects and its retained release
// records. Chart resolution and preview happen before this boundary. Uncertain
// writes stay protective; only verified terminal outcomes release admission.
func runCoordinatedMutation(database *gorm.DB, namespace, name, operation string, run func() (bool, error)) (resultErr error) {
	if database == nil || namespace == "" || name == "" || run == nil {
		return guard.ErrCoordinationChanged
	}
	if operation != "install" && operation != "upgrade" && operation != "rollback" && operation != "uninstall" {
		return guard.ErrCoordinationChanged
	}
	stores, err := guard.DiscoverStores(database)
	if err != nil {
		return err
	}
	identity, err := guard.NewActivationRevision()
	if err != nil {
		return err
	}
	raw, _ := json.Marshal([]string{namespace, name, operation})
	fingerprint := sha256.Sum256(raw)
	requests := []guard.CoordinationRequest{}
	finish := func(confirmed bool) error {
		var first error
		for _, r := range requests {
			if err := guard.FinishOperation(database, r, confirmed); err != nil && first == nil {
				first = err
			}
		}
		return first
	}
	for _, store := range stores {
		key := sha256.Sum256([]byte(identity + "\x00" + store.StorageID))
		r := guard.CoordinationRequest{StorageID: store.StorageID, Generation: store.Generation, OperationID: hex.EncodeToString(key[:]), Owner: "helm-" + operation, Kind: "producer", Scope: "*", Fingerprint: hex.EncodeToString(fingerprint[:])}
		created, err := guard.AcquireOperation(database, r)
		if err != nil || !created {
			_ = finish(true)
			if err != nil {
				return err
			}
			return guard.ErrCoordinationChanged
		}
		requests = append(requests, r)
	}
	confirmed := false
	defer func() {
		if err := finish(confirmed); err != nil && resultErr == nil {
			resultErr = err
		}
	}()
	confirmed, resultErr = run()
	return resultErr
}
func (h *Helm) mutate(name, operation string, run func() error) error {
	manager := db.GetManager()
	if manager == nil {
		return guard.ErrCoordinationUnavailable
	}
	if h.mutations == nil {
		return guard.ErrCoordinationUnavailable
	}
	return runCoordinatedMutation(manager.DB(), h.namespace, name, operation, func() (bool, error) {
		state := h.mutations.begin()
		defer h.mutations.end(state)
		err := run()
		return h.mutations.confirmed(state), err
	})
}
func (h *Helm) mutateRelease(name, operation string, run func() (*release.Release, error)) (*release.Release, error) {
	var result *release.Release
	err := h.mutate(name, operation, func() error { var err error; result, err = run(); return err })
	return result, err
}
