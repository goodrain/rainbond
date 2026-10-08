package cleanup

import (
	"path/filepath"
	"testing"

	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
)

// capability_id: rainbond.cleanup.upload-retained-references
func TestUploadReferencesProtectRetainedVersionsAndSourceChecks(t *testing.T) {
	database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "references.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := database.AutoMigrate(&model.VersionInfo{}, &model.CodeCheckResult{}).Error; err != nil {
		t.Fatal(err)
	}
	for _, row := range []model.VersionInfo{
		{ServiceID: "one", RepoURL: "/grdata/package_build/components/one/events/retained", FinalStatus: "success"},
		{ServiceID: "two", RepoURL: "https://private.invalid/repo?credential=never-return", FinalStatus: "success"},
	} {
		if err := database.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := database.Create(&model.CodeCheckResult{ServiceID: "three", GitURL: "/grdata/package_build/temp/events/pending", CodeFrom: "pkg"}).Error; err != nil {
		t.Fatal(err)
	}
	refs, err := ReadUploadPackageReferences(database, []string{"retained", "pending", "unmatched"})
	if err != nil || !refs["retained"] || !refs["pending"] || refs["unmatched"] || len(refs) != 3 {
		t.Fatal("incorrect bounded references", refs, err)
	}
	if _, err := ReadUploadPackageReferences(database, []string{"../foreign"}); err == nil {
		t.Fatal("invalid event scope accepted")
	}
}

func TestUploadReferencePathsAreCanonical(t *testing.T) {
	for _, value := range []string{"/grdata/package_build/temp/events/event", "/grdata/package_build/components/service/events/event"} {
		id, err := uploadReferenceEvent(value)
		if err != nil || id != "event" {
			t.Fatal("canonical package reference rejected")
		}
	}
	for _, value := range []string{"/grdata/package_build/temp/events/../event", "/grdata/package_build/components/service/events/event/child", "/grdata/package_build/temp/events/%65vent", "/grdata/package_build/unknown/event"} {
		if _, err := uploadReferenceEvent(value); err == nil {
			t.Fatal("ambiguous package reference accepted")
		}
	}
}
