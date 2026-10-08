package handler

import (
	"errors"
	"path/filepath"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/goodrain/rainbond/db/model"
	guard "github.com/goodrain/rainbond/pkg/cleanup"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
)

func TestWorkloadAdmissionProtectsExternalApplyThroughCommit(t *testing.T) {
	database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "admission.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	database.LogMode(false)
	if err := database.AutoMigrate(&model.CleanupStorage{}, &model.CleanupOperation{}, &model.K8sResource{}).Error; err != nil {
		t.Fatal(err)
	}
	binding := guard.StorageRegistration{StorageID: "store", Generation: "one", VolumeUID: "volume", RootPath: "/registry"}
	if err := guard.RegisterStorage(database, binding); err != nil {
		t.Fatal(err)
	}
	if err := database.Model(&model.CleanupStorage{}).Where("storage_id = ?", "store").Update("mode", "ready").Error; err != nil {
		t.Fatal(err)
	}
	admission, err := admitWorkload(database, []byte("fixture-workload"))
	if err != nil {
		t.Fatal(err)
	}
	deletion := guard.CoordinationRequest{StorageID: "store", Generation: "one", OperationID: "delete", Owner: "test", Kind: "delete", Scope: "app", Fingerprint: "selected", Target: "selected-manifest"}
	if _, err := guard.AcquireOperation(database, deletion); err != guard.ErrCoordinationBusy {
		t.Fatal("delete raced admitted external apply", err)
	}
	if err := admission.write(func(tx *gorm.DB) error { return tx.Create(&model.K8sResource{Content: "saved reference"}).Error }); err != nil {
		t.Fatal(err)
	}
	if err := admission.finish(true); err != nil {
		t.Fatal(err)
	}
	if _, err := guard.AcquireOperation(database, deletion); err != nil {
		t.Fatal("producer did not finish", err)
	}
	if _, err := admitWorkload(database, []byte("second")); err == nil {
		t.Fatal("external apply admitted during deletion")
	}
}

func TestWorkloadAdmissionRetainsOnlyUnresolvedEffects(t *testing.T) {
	for _, scenario := range []string{"not-sent", "rejected", "lost-response", "created-db-failed", "created-saved", "lost-then-rejected"} {
		t.Run(scenario, func(t *testing.T) {
			database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "effects.db"))
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
			admission, err := admitWorkload(database, []byte("workload"))
			if err != nil {
				t.Fatal(err)
			}
			if scenario != "not-sent" {
				admission.observe(true, nil)
				switch scenario {
				case "rejected":
					admission.observe(false, apierrors.NewBadRequest("fixture rejection"))
				case "lost-response", "lost-then-rejected":
					admission.observe(false, errors.New("transport uncertainty"))
				default:
					admission.observe(false, nil)
				}
			}
			if scenario == "lost-then-rejected" {
				admission.observe(true, nil)
				admission.observe(false, apierrors.NewBadRequest("later rejection"))
			}
			if err := admission.finish(scenario == "created-saved"); err != nil {
				t.Fatal(err)
			}
			var operation model.CleanupOperation
			if err := database.Where("operation_id = ?", admission.requests[0].OperationID).First(&operation).Error; err != nil {
				t.Fatal(err)
			}
			want := "uncertain"
			if scenario == "not-sent" || scenario == "rejected" || scenario == "created-saved" {
				want = "finished"
			}
			if operation.State != want {
				t.Fatal("incorrect durable outcome", scenario, operation.State, want)
			}
		})
	}
}
