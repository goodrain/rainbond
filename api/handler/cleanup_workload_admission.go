package handler

import (
	"crypto/sha256"
	"encoding/hex"

	guard "github.com/goodrain/rainbond/pkg/cleanup"
	"github.com/jinzhu/gorm"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// workloadAdmission outlives individual database transactions so Kubernetes
// requests never run while holding a SQL transaction. Ambiguous effects retain
// the original durable producer records instead of expiring protection.
type workloadAdmission struct {
	database *gorm.DB
	requests []guard.CoordinationRequest
	pending  int
	changed  bool
}

func admitWorkload(database *gorm.DB, body []byte) (*workloadAdmission, error) {
	stores, err := guard.DiscoverStores(database)
	if err != nil {
		return nil, err
	}
	id, err := guard.NewActivationRevision()
	if err != nil {
		return nil, err
	}
	fingerprint := sha256.Sum256(body)
	admission := &workloadAdmission{database: database}
	for _, store := range stores {
		identity := sha256.Sum256([]byte(id + "\x00" + store.StorageID))
		request := guard.CoordinationRequest{StorageID: store.StorageID, Generation: store.Generation, OperationID: hex.EncodeToString(identity[:]), Owner: "api-workload", Kind: "producer", Scope: "*", Fingerprint: hex.EncodeToString(fingerprint[:])}
		created, err := guard.AcquireOperation(database, request)
		if err != nil || !created {
			// No Kubernetes call occurred. Release only admissions definitely created
			// by this invocation, never a rejected or uncertain acquisition.
			_ = admission.finish(true)
			if err != nil {
				return nil, err
			}
			return nil, guard.ErrCoordinationChanged
		}
		admission.requests = append(admission.requests, request)
	}
	return admission, nil
}
func (a *workloadAdmission) finish(confirmed bool) error {
	confirmed = a.pending == 0 && (confirmed || !a.changed)
	var first error
	for _, request := range a.requests {
		if err := guard.FinishOperation(a.database, request, confirmed); err != nil && first == nil {
			first = err
		}
	}
	return first
}
func (a *workloadAdmission) write(write func(*gorm.DB) error) error {
	if len(a.requests) == 0 {
		return write(a.database)
	}
	return guard.WithProducerReferenceMutation(a.database, a.requests, []string{"*"}, write)
}

// observe brackets each Kubernetes mutation. Only explicit API rejection proves
// no effect; transport failures and panics leave the original admission held.
func (a *workloadAdmission) observe(start bool, err error) {
	if start {
		a.pending++
		return
	}
	if err == nil {
		a.changed = true
		a.pending--
		return
	}
	if apierrors.IsAlreadyExists(err) || apierrors.IsConflict(err) || apierrors.IsInvalid(err) || apierrors.IsBadRequest(err) || apierrors.IsForbidden(err) || apierrors.IsUnauthorized(err) || apierrors.IsNotFound(err) || apierrors.IsMethodNotSupported(err) {
		a.pending--
	}
}
