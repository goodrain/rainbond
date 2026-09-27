package exector

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/model"
	guard "github.com/goodrain/rainbond/pkg/cleanup"
	"github.com/jinzhu/gorm"
)

type nativeBuildAdmission struct {
	database *gorm.DB
	requests []guard.CoordinationRequest
}

// Admission precedes any native image work. Retries of an existing task do not
// receive another execution grant. Unknown outcomes stay protective.
func admitBuild(database *gorm.DB, kind, taskID string, body []byte) (*nativeBuildAdmission, error) {
	if (kind != "image" && kind != "source" && kind != "vm" && kind != "image-share" && kind != "share-plugin" && kind != "plugin-image" && kind != "plugin-dockerfile" && kind != "tar-image" && kind != "service-check" && kind != "import_app" && kind != "backup_apps_restore") || database == nil || taskID == "" || len(taskID) > 128 || strings.ContainsAny(taskID, "\x00\r\n") {
		return nil, guard.ErrCoordinationChanged
	}
	stores, err := guard.DiscoverStores(database)
	if err != nil {
		return nil, err
	}
	admission := &nativeBuildAdmission{database: database}
	fingerprint := sha256.Sum256(body)
	for _, store := range stores {
		identity := sha256.Sum256([]byte(kind + "-build\x00" + taskID + "\x00" + store.StorageID))
		// Image builders may resolve source tags and choose a destination internally;
		// unresolved repository scope must not be guessed from the requested tag.
		request := guard.CoordinationRequest{StorageID: store.StorageID, Generation: store.Generation, OperationID: hex.EncodeToString(identity[:]), Owner: "native-" + kind + "-builder", Kind: "producer", Scope: "*", Fingerprint: hex.EncodeToString(fingerprint[:])}
		created, err := guard.AcquireOperation(database, request)
		if err != nil || !created {
			// Nothing has executed yet. Release only grants this call definitely created,
			// never the rejected/ambiguous admission or an earlier process's operation.
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
func (a *nativeBuildAdmission) finish(confirmed bool) error {
	var first error
	for _, r := range a.requests {
		if err := guard.FinishOperation(a.database, r, confirmed); err != nil && first == nil {
			first = err
		}
	}
	return first
}
func (a *nativeBuildAdmission) write(write func(*gorm.DB) error) error {
	if len(a.requests) == 0 {
		return write(a.database)
	}
	return guard.WithProducerReferenceMutation(a.database, a.requests, []string{"*"}, write)
}
func (a *nativeBuildAdmission) saveVersion(version *model.VersionInfo) error {
	if a == nil {
		return db.GetManager().VersionInfoDao().UpdateModel(version)
	}
	return a.write(func(tx *gorm.DB) error { return db.GetManager().VersionInfoDaoTransactions(tx).UpdateModel(version) })
}
func (a *nativeBuildAdmission) activateVersion(serviceID, version string) error {
	if a == nil {
		return db.GetManager().TenantServiceDao().UpdateDeployVersion(serviceID, version)
	}
	return a.write(func(tx *gorm.DB) error {
		return db.GetManager().TenantServiceDaoTransactions(tx).UpdateDeployVersion(serviceID, version)
	})
}

func (a *nativeBuildAdmission) savePluginVersion(version *model.TenantPluginBuildVersion) error {
	if a == nil {
		return db.GetManager().TenantPluginBuildVersionDao().UpdateModel(version)
	}
	return a.write(func(tx *gorm.DB) error {
		return db.GetManager().TenantPluginBuildVersionDaoTransactions(tx).UpdateModel(version)
	})
}

func runTarImageWithAdmission(database *gorm.DB, loadID string, body []byte, run func(*nativeBuildAdmission) bool) error {
	return runNativeBuildWithAdmission(database, "tar-image", loadID, body, run)
}

func runNativeBuildWithAdmission(database *gorm.DB, kind, id string, body []byte, run func(*nativeBuildAdmission) bool) (err error) {
	if run == nil {
		return guard.ErrCoordinationChanged
	}
	admission, err := admitBuild(database, kind, id, body)
	if err != nil {
		return err
	}
	confirmed := false
	defer func() {
		if finishErr := admission.finish(confirmed); err == nil {
			err = finishErr
		}
	}()
	confirmed = run(admission)
	return nil
}

func (a *nativeBuildAdmission) saveTarImageResult(loadID, result string) error {
	return a.write(func(tx *gorm.DB) error {
		return tx.Create(&model.KeyValue{K: "/rainbond/tarload/" + loadID, V: result}).Error
	})
}

func (a *nativeBuildAdmission) saveServiceCheckResult(id, result string) error {
	if a == nil {
		return db.GetManager().KeyValueDao().Put("/servicecheck/"+id, result)
	}
	return a.write(func(tx *gorm.DB) error { return tx.Create(&model.KeyValue{K: "/servicecheck/" + id, V: result}).Error })
}
