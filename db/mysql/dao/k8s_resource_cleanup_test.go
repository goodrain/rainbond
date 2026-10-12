package dao

import (
	"path/filepath"
	"testing"

	"github.com/goodrain/rainbond/db/model"
	guard "github.com/goodrain/rainbond/pkg/cleanup"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
)

func TestSavedWorkloadWritesRespectCleanupAdmission(t *testing.T) {
	for _, method := range []string{"add", "update", "batch"} {
		t.Run(method, func(t *testing.T) {
			db, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "workloads.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			db.LogMode(false)
			if err := db.AutoMigrate(&model.K8sResource{}, &model.CleanupStorage{}, &model.CleanupOperation{}).Error; err != nil {
				t.Fatal(err)
			}
			original := model.K8sResource{Content: "original"}
			if err := db.Create(&original).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Create(&model.CleanupStorage{StorageID: "hub", Generation: "one", Mode: "ready"}).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Create(&model.CleanupOperation{OperationID: "deleting", StorageID: "hub", Generation: "one", Kind: "delete", Scope: "app", State: "executing"}).Error; err != nil {
				t.Fatal(err)
			}
			dao := &K8sResourceDaoImpl{DB: db}
			candidate := &model.K8sResource{Content: "new reference"}
			write := func() error { return dao.AddModel(candidate) }
			if method == "update" {
				candidate.ID = original.ID
				write = func() error { return dao.UpdateModel(candidate) }
			}
			if method == "batch" {
				write = func() error { return dao.CreateK8sResource([]*model.K8sResource{candidate}) }
			}
			if err := write(); err != guard.ErrCoordinationBusy {
				t.Fatal("unprotected reference write", err)
			}
			var stored []model.K8sResource
			if err := db.Find(&stored).Error; err != nil || len(stored) != 1 || stored[0].Content != "original" {
				t.Fatal("blocked operation changed data", stored, err)
			}
			if err := db.Model(&model.CleanupOperation{}).Where("operation_id = ?", "deleting").Update("state", "finished").Error; err != nil {
				t.Fatal(err)
			}
			if err := write(); err != nil {
				t.Fatal("finished cleanup blocks ordinary reference write", err)
			}
		})
	}
}
