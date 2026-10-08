package exector

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/event"
	pb "github.com/goodrain/rainbond/mq/api/grpc/pb"
	guard "github.com/goodrain/rainbond/pkg/cleanup"
	"github.com/jinzhu/gorm"
)

type importAdmissionWorker struct {
	admission *nativeBuildAdmission
	stubTaskWorker
	run func() error
}

func (w *importAdmissionWorker) Run(time.Duration) error {
	if w.admission == nil {
		return errors.New("missing worker admission")
	}
	return w.run()
}
func (w *importAdmissionWorker) setCleanupAdmission(a *nativeBuildAdmission) { w.admission = a }

// capability_id: rainbond.cleanup.app-import-admission
func TestImportWorkerHoldsAdmissionUntilPublication(t *testing.T) {
	for _, kind := range []string{"import_app", "backup_apps_restore"} {
		t.Run(kind, func(t *testing.T) { checkImportWorkerAdmission(t, kind) })
	}
}
func checkImportWorkerAdmission(t *testing.T, kind string) {
	database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "import.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	database.DB().SetMaxOpenConns(1)
	if err := database.AutoMigrate(&model.CleanupStorage{}, &model.CleanupOperation{}).Error; err != nil {
		t.Fatal(err)
	}
	binding, err := guard.ProvisionRegistryStorage(database, "owned", "/registry")
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Model(&model.CleanupStorage{}).Where("storage_id = ?", binding.StorageID).Update("mode", "ready").Error; err != nil {
		t.Fatal(err)
	}
	db.SetTestManager(tarImportManager{database: database})
	defer db.SetTestManager(nil)
	event.NewTestManager(&stubEventManager{})
	defer event.NewTestManager(nil)
	old := workerCreaterList[kind]
	defer func() { workerCreaterList[kind] = old }()
	gc := guard.CoordinationRequest{StorageID: binding.StorageID, Generation: binding.Generation, OperationID: "gc", Owner: "test", Kind: "gc", Scope: "*", Fingerprint: "confirmed"}
	calls := 0
	RegisterWorker(kind, func([]byte, *exectorManager) (TaskWorker, error) {
		return &importAdmissionWorker{stubTaskWorker: stubTaskWorker{logger: event.NewLogger("owned-event", make(chan []byte, 20))}, run: func() error {
			calls++
			if _, err := guard.RequestMaintenance(database, gc); err != nil {
				t.Fatal(err)
			}
			if err := guard.EnterMaintenance(database, gc); !errors.Is(err, guard.ErrCoordinationBusy) {
				t.Fatal("GC entered during import", err)
			}
			return nil
		}}, nil
	})
	task := &pb.TaskMessage{TaskType: kind, TaskId: "owned-import", TaskBody: []byte("original")}
	if err := new(exectorManager).exec(task); err != nil {
		t.Fatal(err)
	}
	if err := guard.EnterMaintenance(database, gc); err != nil {
		t.Fatal(err)
	}
	if err := new(exectorManager).exec(task); err == nil || calls != 1 {
		t.Fatal("import replayed", err)
	}
}

func TestImportSuccessWaitsForMetadataUpload(t *testing.T) {
	failure := errors.New("upload unavailable")
	published := false
	if err := completeImportedMetadata(func() error { return failure }, func() error { published = true; return nil }); !errors.Is(err, failure) || published {
		t.Fatal("published before upload", err)
	}
	uploaded := false
	if err := completeImportedMetadata(func() error { uploaded = true; return nil }, func() error {
		if !uploaded {
			t.Fatal("wrong order")
		}
		return failure
	}); !errors.Is(err, failure) {
		t.Fatal("lost publication treated as success", err)
	}
}
