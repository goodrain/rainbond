package exector

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/model"
	pb "github.com/goodrain/rainbond/mq/api/grpc/pb"
	guard "github.com/goodrain/rainbond/pkg/cleanup"
	"github.com/jinzhu/gorm"
)

// capability_id: rainbond.cleanup.tar-service-check-admission
func TestTarServiceCheckCannotPushDuringMaintenance(t *testing.T) {
	database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "check.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := database.AutoMigrate(&model.CleanupStorage{}, &model.CleanupOperation{}, &model.KeyValue{}).Error; err != nil {
		t.Fatal(err)
	}
	binding, err := guard.ProvisionRegistryStorage(database, "owned", "/registry")
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Model(&model.CleanupStorage{}).Where("storage_id = ?", binding.StorageID).Update("mode", "ready").Error; err != nil {
		t.Fatal(err)
	}
	request := guard.CoordinationRequest{StorageID: binding.StorageID, Generation: binding.Generation, OperationID: "gc", Owner: "test", Kind: "gc", Scope: "*", Fingerprint: "confirmed"}
	if _, err := guard.RequestMaintenance(database, request); err != nil {
		t.Fatal(err)
	}
	db.SetTestManager(tarImportManager{database: database})
	defer db.SetTestManager(nil)
	raw, _ := json.Marshal(ServiceCheckInput{CheckUUID: "owned-check", SourceType: "docker-run", SourceBody: "event owned-event"})
	// Rejection must happen before constructing a parser or initializing storage,
	// the image client, or event logger.
	new(exectorManager).serviceCheck(&pb.TaskMessage{TaskBody: raw})
	result, err := db.GetManager().KeyValueDao().Get("/servicecheck/owned-check")
	if err != nil || result == nil {
		t.Fatal("no terminal rejection", err)
	}
	var decoded ServiceCheckResult
	if json.Unmarshal([]byte(result.V), &decoded) != nil || decoded.CheckStatus != "Failure" {
		t.Fatal("invalid rejection")
	}
}
