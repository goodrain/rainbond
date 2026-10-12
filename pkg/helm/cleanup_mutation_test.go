package helm

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/goodrain/rainbond/db/model"
	guard "github.com/goodrain/rainbond/pkg/cleanup"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
)

func TestHelmMutationRequiresDurableProducerAdmission(t *testing.T) {
	for _, scenario := range []string{"blocked", "success", "uncertain"} {
		t.Run(scenario, func(t *testing.T) {
			database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "helm.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer database.Close()
			database.LogMode(false)
			if err := database.AutoMigrate(&model.CleanupStorage{}, &model.CleanupOperation{}).Error; err != nil {
				t.Fatal(err)
			}
			if err := guard.RegisterStorage(database, guard.StorageRegistration{StorageID: "store", Generation: "one", VolumeUID: "volume", RootPath: "/registry"}); err != nil {
				t.Fatal(err)
			}
			if err := database.Model(&model.CleanupStorage{}).Where("storage_id = ?", "store").Update("mode", "ready").Error; err != nil {
				t.Fatal(err)
			}
			deletion := guard.CoordinationRequest{StorageID: "store", Generation: "one", OperationID: "delete", Owner: "test", Kind: "delete", Scope: "app", Target: "manifest", Fingerprint: "selected"}
			if scenario == "blocked" {
				if _, err := guard.AcquireOperation(database, deletion); err != nil {
					t.Fatal(err)
				}
			}
			calls := 0
			err = runCoordinatedMutation(database, "team", "release", "install", func() (bool, error) {
				calls++
				if _, err := guard.AcquireOperation(database, deletion); err != guard.ErrCoordinationBusy {
					t.Fatal("Helm effects outside admission", err)
				}
				if scenario == "uncertain" {
					return false, errors.New("fixture ambiguous result")
				}
				return true, nil
			})
			if scenario == "blocked" {
				if calls != 0 || err != guard.ErrCoordinationBusy {
					t.Fatal(calls, err)
				}
				return
			}
			if calls != 1 || (scenario == "success" && err != nil) || (scenario == "uncertain" && err == nil) {
				t.Fatal(calls, err)
			}
			var operations []model.CleanupOperation
			if err := database.Where("kind = ?", "producer").Find(&operations).Error; err != nil || len(operations) != 1 {
				t.Fatal(operations, err)
			}
			want := "finished"
			if scenario == "uncertain" {
				want = "uncertain"
			}
			if operations[0].State != want {
				t.Fatal(operations[0].State, want)
			}
		})
	}
}
