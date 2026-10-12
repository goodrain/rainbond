package cleanup

import (
	"testing"

	"github.com/goodrain/rainbond/db/model"
)

// capability_id: rainbond.cleanup.pending-import-references
func TestImportedImagesRemainReferencedUntilTheirRecordsAreReleased(t *testing.T) {
	database, _ := coordinationDB(t)
	if err := database.AutoMigrate(&model.VersionInfo{}, &model.TenantPluginBuildVersion{}, &model.K8sResource{}, &model.KeyValue{}).Error; err != nil {
		t.Fatal(err)
	}
	rows := []model.KeyValue{
		{K: "/rainbond/tarload/owned", V: `{"status":"success","target_images":{"nginx:one":"goodrain.me/team/nginx:one"}}`},
		{K: "/servicecheck/owned", V: `{"check_status":"Success","service_info":[{"tar_images":[{"name":"source.test/team/app:two","prefix":"goodrain.me/team"}]}]}`},
	}
	for i := range rows {
		if err := database.Create(&rows[i]).Error; err != nil {
			t.Fatal(err)
		}
	}
	result, err := collectRegionReferenceImages(database)
	if err != nil || !result.Complete || len(result.Images) != 2 {
		t.Fatal("import references omitted", result, err)
	}
	found := map[string]bool{}
	for _, image := range result.Images {
		found[image] = true
	}
	if !found["goodrain.me/team/nginx:one"] || !found["goodrain.me/team/app:two"] {
		t.Fatal(result)
	}
	if err := database.Delete(&rows[0]).Error; err != nil {
		t.Fatal(err)
	}
	result, err = collectRegionReferenceImages(database)
	if err != nil || len(result.Images) != 1 {
		t.Fatal("released reference retained", result, err)
	}
}
func TestIncompleteImportRecordsCannotProveAbsence(t *testing.T) {
	for _, value := range []string{`{"status":"success"}`, `{"status":"partial_success","target_images":{"one":"goodrain.me/a:v1"}}`, `broken`, `{"status":"success","target_images":{"one":"invalid image"}}`} {
		database, _ := coordinationDB(t)
		if err := database.AutoMigrate(&model.VersionInfo{}, &model.TenantPluginBuildVersion{}, &model.K8sResource{}, &model.KeyValue{}).Error; err != nil {
			t.Fatal(err)
		}
		if err := database.Create(&model.KeyValue{K: "/rainbond/tarload/owned", V: value}).Error; err != nil {
			t.Fatal(err)
		}
		result, err := collectRegionReferenceImages(database)
		if err == nil && result.Complete {
			t.Fatal("incomplete import treated as empty", value)
		}
	}
}

func TestRejectedAndNonImageChecksDoNotInventReferences(t *testing.T) {
	for _, record := range []struct{ key, value string }{
		{"/rainbond/tarload/rejected", `{"status":"failure"}`},
		{"/servicecheck/source", `{"check_status":"Success","service_info":[{"language":"Go"}]}`},
	} {
		images := []string{}
		if !inspectImportReceipt(record.key, record.value, func(image string) { images = append(images, image) }) || len(images) != 0 {
			t.Fatal("non-published images invented", record.key)
		}
	}
}

func TestMalformedServiceCheckCannotProveEmptyReferences(t *testing.T) {
	for _, value := range []string{`null`, `{}`, `{"check_status":"unknown"}`} {
		if inspectImportReceipt("/servicecheck/owned", value, func(string) {}) {
			t.Fatal("missing check status treated as complete")
		}
	}
}
