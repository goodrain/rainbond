package cleanup

import (
	"strings"
	"testing"

	"github.com/goodrain/rainbond/db/model"
)

func TestRegionReferenceAuditChecksRetainedVersionsAndPluginImages(t *testing.T) {
	database, _ := coordinationDB(t)
	if err := database.AutoMigrate(&model.VersionInfo{}, &model.TenantPluginBuildVersion{}, &model.K8sResource{}, &model.KeyValue{}).Error; err != nil {
		t.Fatal(err)
	}
	selected := operation("audit", "delete", "app")
	selected.Target = "sha256:" + strings.Repeat("a", 64)
	if _, err := AcquireOperation(database, selected); err != nil {
		t.Fatal(err)
	}
	check := func(wantReferenced, wantComplete bool) {
		t.Helper()
		result, err := AuditRegionManifestReferences(database, selected, []string{"v1"})
		if err != nil || result.Referenced != wantReferenced || result.Complete != wantComplete {
			t.Fatal(result, err)
		}
	}
	check(false, true)
	record := &model.VersionInfo{ServiceID: "owned-service", BuildVersion: "old", ImageName: "goodrain.me/app:v1"}
	if err := database.Create(record).Error; err != nil {
		t.Fatal(err)
	}
	check(true, true)
	if err := database.Delete(record).Error; err != nil {
		t.Fatal(err)
	}
	plugin := &model.TenantPluginBuildVersion{BuildLocalImage: "goodrain.me/another@" + selected.Target}
	if err := database.Create(plugin).Error; err != nil {
		t.Fatal(err)
	}
	check(true, true)
	if err := database.Delete(plugin).Error; err != nil {
		t.Fatal(err)
	}
	saved := &model.K8sResource{Content: "apiVersion: apps/v1\nkind: Deployment\nspec:\n  template:\n    spec:\n      containers:\n      - name: app\n        image: goodrain.me/app:v1\n"}
	if err := database.Create(saved).Error; err != nil {
		t.Fatal(err)
	}
	check(true, true)
	if err := database.Delete(saved).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Create(&model.VersionInfo{ImageName: "invalid image", FinalStatus: "success"}).Error; err != nil {
		t.Fatal(err)
	}
	check(false, false)
	changed := selected
	changed.Owner = "someone-else"
	if _, err := AuditRegionManifestReferences(database, changed, []string{"v1"}); err == nil {
		t.Fatal("unbound audit accepted")
	}
}

func TestReferenceInventoryBootstrapsWithoutGrantingDeletion(t *testing.T) {
	for _, mode := range []string{"collecting", "ready", "draining", "maintenance", "restoring", "recovery_required", "unknown"} {
		t.Run(mode, func(t *testing.T) {
			database, _ := coordinationDB(t)
			if err := database.AutoMigrate(&model.VersionInfo{}, &model.TenantPluginBuildVersion{}, &model.K8sResource{}, &model.KeyValue{}).Error; err != nil {
				t.Fatal(err)
			}
			if err := database.Model(&model.CleanupStorage{}).Where("storage_id = ?", "store").Update("mode", mode).Error; err != nil {
				t.Fatal(err)
			}
			if err := database.Create(&model.VersionInfo{ImageName: "goodrain.me/retained:v1", FinalStatus: "success"}).Error; err != nil {
				t.Fatal(err)
			}
			result, err := ReadRegionReferenceInventory(database, "store", "generation1")
			if mode != "collecting" && mode != "ready" {
				if err == nil || result.Complete {
					t.Fatal("unsafe storage mode accepted", mode)
				}
				return
			}
			if err != nil || !result.Complete || len(result.Images) != 1 || result.Images[0] != "goodrain.me/retained:v1" {
				t.Fatal("missing retained reference", result, err)
			}
			observed, err := InspectStorage(database, "store", "generation1")
			if err != nil || observed.Mode != mode {
				t.Fatal("inventory changed storage mode", observed, err)
			}
			if mode == "collecting" {
				if _, err := AcquireOperation(database, operation("not-authorized", "delete", "app")); err == nil {
					t.Fatal("read-only inventory granted deletion")
				}
				if _, err := ReadRegionReferenceInventory(database, "store", "foreign-generation"); err == nil {
					t.Fatal("foreign generation accepted")
				}
			}
		})
	}
}

func TestConfiguredHelperImagesRemainProtectedWithoutRunningPods(t *testing.T) {
	result := ConfiguredImageReferences([]string{"goodrain.me/helpers@sha256:" + strings.Repeat("a", 64), "", "goodrain.me/helpers@sha256:" + strings.Repeat("a", 64)})
	if !result.Complete || len(result.Images) != 1 {
		t.Fatal("configured helper reference missing or duplicated", result)
	}
	invalid := ConfiguredImageReferences([]string{"invalid image value"})
	if invalid.Complete || len(invalid.Images) != 0 {
		t.Fatal("invalid configuration became empty success")
	}
	if merged := MergeReferenceInventories(RegionReferenceInventory{Complete: true}, invalid); merged.Complete {
		t.Fatal("configuration failure lost during merge")
	}
}
