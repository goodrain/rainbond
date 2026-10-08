package cleanup

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
)

// capability_id: rainbond.cleanup.import_reference_handoff
func TestImportHandoffWaitsForEverySuccessfulVersionAndRollsBack(t *testing.T) {
	database, _ := coordinationDB(t)
	if err := database.AutoMigrate(&model.VersionInfo{}, &model.KeyValue{}).Error; err != nil {
		t.Fatal(err)
	}
	original := `{"status":"success","target_images":{"a":"goodrain.me/a:v1","b":"goodrain.me/b:v1"},"message":"keep receipt"}`
	row := model.KeyValue{K: "/rainbond/tarload/owned", V: original}
	if err := database.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	a := model.VersionInfo{ServiceID: "a", BuildVersion: "one", ImageName: "goodrain.me/a:v1", FinalStatus: "success"}
	if err := database.Create(&a).Error; err != nil {
		t.Fatal(err)
	}
	handoff := func(tx *gorm.DB) error { return TransferImportedReferencesToVersions(tx) }
	if err := WithReferenceMutation(database, nil, handoff); err != nil {
		t.Fatal(err)
	}
	var stored model.KeyValue
	database.Where("k = ?", row.K).First(&stored)
	if stored.V != original {
		t.Fatal("partial ownership released receipt")
	}
	tx := database.Begin()
	b := model.VersionInfo{ServiceID: "b", BuildVersion: "one", ImageName: "goodrain.me/b:v1", FinalStatus: "success"}
	if err := tx.Create(&b).Error; err != nil {
		t.Fatal(err)
	}
	if err := WithReferenceMutation(tx, nil, handoff); err != nil {
		t.Fatal(err)
	}
	tx.Rollback()
	database.Where("k = ?", row.K).First(&stored)
	if stored.V != original {
		t.Fatal("rolled-back version released receipt")
	}
	if err := database.Create(&b).Error; err != nil {
		t.Fatal(err)
	}
	if err := WithReferenceMutation(database, nil, handoff); err != nil {
		t.Fatal(err)
	}
	database.Where("k = ?", row.K).First(&stored)
	if !strings.Contains(stored.V, "keep receipt") {
		t.Fatal("original receipt lost")
	}
	var changed map[string]interface{}
	if err := json.Unmarshal([]byte(stored.V), &changed); err != nil {
		t.Fatal(err)
	}
	changed["target_images"] = map[string]string{"new": "goodrain.me/other:v2"}
	modified, _ := json.Marshal(changed)
	protected := 0
	inspectImportReceipt(row.K, string(modified), func(string) { protected++ })
	if protected != 1 {
		t.Fatal("handoff marker accepted for different images")
	}
	count := 0
	if !inspectImportReceipt(row.K, stored.V, func(string) { count++ }) || count != 0 {
		t.Fatal("transferred receipt retains temporary references")
	}
	if err := WithReferenceMutation(database, nil, handoff); err != nil {
		t.Fatal(err)
	}
}
