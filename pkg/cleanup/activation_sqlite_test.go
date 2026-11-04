package cleanup

import (
	"path/filepath"
	"testing"

	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
)

// capability_id: rainbond.cleanup.sqlite-activation
func TestTrackServiceActivationSupportsSQLite(t *testing.T) {
	database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "activation.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	if err := database.AutoMigrate(&model.CleanupStorage{}, &model.TenantServices{}, &model.VersionInfo{}).Error; err != nil {
		t.Fatal(err)
	}
	service := &model.TenantServices{TenantID: "tenant", ServiceID: "service", DeployVersion: "old"}
	if err := database.Create(service).Error; err != nil {
		t.Fatal(err)
	}
	version := &model.VersionInfo{
		ServiceID:     "service",
		BuildVersion:  "new",
		ImageName:     "goodrain.me/tenant/service:new",
		DeliveredType: "image",
		FinalStatus:   "success",
	}
	if err := database.Create(version).Error; err != nil {
		t.Fatal(err)
	}

	err = TrackServiceActivation(database, "service", "new", func(tx *gorm.DB) error {
		return tx.Model(&model.TenantServices{}).
			Where("service_id = ?", "service").
			Update("deploy_version", "new").Error
	})
	if err != nil {
		t.Fatal(err)
	}

	var activated model.TenantServices
	if err := database.Where("service_id = ?", "service").First(&activated).Error; err != nil {
		t.Fatal(err)
	}
	if activated.DeployVersion != "new" {
		t.Fatalf("deploy version = %q, want new", activated.DeployVersion)
	}
	var checkpoint model.VersionInfo
	if err := database.Where("service_id = ? AND build_version = ?", "service", "new").First(&checkpoint).Error; err != nil {
		t.Fatal(err)
	}
	if checkpoint.ActivationRevision == "" {
		t.Fatal("activation revision was not persisted")
	}
}
