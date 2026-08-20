package dao

import (
	"path/filepath"
	"testing"

	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
)

// capability_id: rainbond.volume-dependency.precise-delete
func TestTenantServiceMountRelationDaoDeletesOnlyRequestedProviderVolume(t *testing.T) {
	database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "mount-relations.db"))
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	defer database.Close()
	database.LogMode(false)
	if err := database.AutoMigrate(&model.TenantServiceMountRelation{}).Error; err != nil {
		t.Fatalf("migrate mount relation table: %v", err)
	}

	dao := &TenantServiceMountRelationDaoImpl{DB: database}
	const (
		consumerID = "consumer-service"
		volumeName = "shared-config"
		providerA  = "provider-a"
		providerB  = "provider-b"
	)
	for _, providerID := range []string{providerA, providerB} {
		if err := dao.AddModel(&model.TenantServiceMountRelation{
			ServiceID:       consumerID,
			DependServiceID: providerID,
			VolumeName:      volumeName,
			VolumePath:      "/etc/config/" + providerID,
			VolumeType:      model.ConfigFileVolumeType.String(),
		}); err != nil {
			t.Fatalf("add mount relation for %s: %v", providerID, err)
		}
	}

	if err := dao.DeleteTenantServiceMountRelation(consumerID, providerA, volumeName); err != nil {
		t.Fatalf("delete provider A mount relation: %v", err)
	}
	relations, err := dao.GetTenantServiceMountRelationsByService(consumerID)
	if err != nil {
		t.Fatalf("list remaining mount relations: %v", err)
	}
	if len(relations) != 1 || relations[0].DependServiceID != providerB {
		t.Fatalf("expected only provider B relation to remain, got %#v", relations)
	}
}
