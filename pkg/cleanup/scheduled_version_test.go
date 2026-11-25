package cleanup

import (
	"errors"
	"strings"
	"testing"

	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
)

// capability_id: rainbond.image-clean.atomic-scheduled-retirement
func TestScheduledRetirementCommitsCandidatesOnlyAfterReferenceAudit(t *testing.T) {
	for _, scenario := range []string{"retire-group", "referenced-alias", "current-version", "retention-count", "incomplete-inventory"} {
		t.Run(scenario, func(t *testing.T) {
			database, _ := coordinationDB(t)
			if err := database.AutoMigrate(&model.TenantServices{}, &model.VersionInfo{}, &model.TenantPluginBuildVersion{}, &model.K8sResource{}, &model.KeyValue{}).Error; err != nil {
				t.Fatal(err)
			}
			if err := database.Exec("CREATE TABLE tenant_services_event (service_id TEXT, opt_type TEXT, final_status TEXT)").Error; err != nil {
				t.Fatal(err)
			}
			database.Create(&model.TenantServices{TenantID: "tenant", ServiceID: "service", DeployVersion: "new"})
			candidates := []ScheduledVersionRetirement{}
			for _, version := range []string{"old-one", "old-two", "new"} {
				row := model.VersionInfo{ServiceID: "service", BuildVersion: version, ImageName: "goodrain.me/app:" + version, DeliveredType: "image", FinalStatus: "success", EventID: "build-" + version}
				if err := database.Create(&row).Error; err != nil {
					t.Fatal(err)
				}
				if version != "new" {
					candidates = append(candidates, ScheduledVersionRetirement{TenantID: "tenant", Expected: VersionExpectation{ServiceID: "service", Version: version, CurrentVersion: "new", Image: row.ImageName, EventID: row.EventID}})
				}
			}
			selected := operation("selected", "delete", "app")
			selected.Target = "sha256:" + strings.Repeat("a", 64)
			if _, err := AcquireOperation(database, selected); err != nil {
				t.Fatal(err)
			}
			references := RegionReferenceInventory{Complete: true}
			keep := uint(1)
			switch scenario {
			case "referenced-alias":
				references.Images = []string{"goodrain.me/app:alias"}
			case "current-version":
				database.Model(&model.TenantServices{}).Where("service_id = ?", "service").Update("deploy_version", "old-one")
			case "retention-count":
				keep = 2
			case "incomplete-inventory":
				references.Complete = false
			}
			err := RetireScheduledVersions(database, selected, candidates, keep, []string{"old-one", "old-two", "alias"}, references)
			var count int
			database.Model(&model.VersionInfo{}).Where("build_version <> ?", "new").Count(&count)
			var op model.CleanupOperation
			database.Where("operation_id = ?", selected.OperationID).First(&op)
			if scenario == "retire-group" {
				if err != nil || count != 0 || op.Outcome != "retired-pending" {
					t.Fatalf("group not atomically retired: count=%d outcome=%s err=%v", count, op.Outcome, err)
				}
			} else if err == nil || count != 2 || op.Outcome != "" {
				t.Fatalf("protected group changed: count=%d outcome=%s err=%v", count, op.Outcome, err)
			}
			if scenario == "referenced-alias" && !errors.Is(err, ErrVersionProtected) {
				t.Fatal(err)
			}
		})
	}
}

func scheduledCrashFixture(t *testing.T) (*gorm.DB, CoordinationRequest) {
	t.Helper()
	database, _ := coordinationDB(t)
	if err := database.AutoMigrate(&model.TenantServices{}, &model.VersionInfo{}, &model.TenantPluginBuildVersion{}, &model.K8sResource{}, &model.KeyValue{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Exec("CREATE TABLE tenant_services_event (service_id TEXT, opt_type TEXT, final_status TEXT)").Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Create(&model.TenantServices{TenantID: "tenant", ServiceID: "service", DeployVersion: "new"}).Error; err != nil {
		t.Fatal(err)
	}
	for _, version := range []string{"old", "new"} {
		if err := database.Create(&model.VersionInfo{ServiceID: "service", BuildVersion: version, ImageName: "goodrain.me/app:" + version, DeliveredType: "image", FinalStatus: "success", EventID: "build-" + version}).Error; err != nil {
			t.Fatal(err)
		}
	}
	request := operation("crashed-before-delete", "delete", "app")
	request.Owner = "chaos-scheduled-version-cleanup"
	request.Target = "sha256:" + strings.Repeat("a", 64)
	if created, err := AcquireOperation(database, request); err != nil || !created {
		t.Fatal(created, err)
	}
	if err := RetireScheduledVersions(database, request, []ScheduledVersionRetirement{{TenantID: "tenant", Expected: VersionExpectation{ServiceID: "service", Version: "old", CurrentVersion: "new", Image: "goodrain.me/app:old", EventID: "build-old"}}}, 1, []string{"old"}, RegionReferenceInventory{Complete: true}); err != nil {
		t.Fatal(err)
	}
	// No deferred Finish is installed: this is the exact persisted crash point.
	var receipt model.CleanupOperation
	database.Where("operation_id = ?", request.OperationID).First(&receipt)
	if receipt.State != "active" || receipt.Outcome != ScheduledVersionRetirementPending {
		t.Fatal("incorrect committed crash fixture", receipt.State, receipt.Outcome)
	}
	return database, request
}

// capability_id: rainbond.image-clean.atomic-scheduled-retirement
func TestRecoverScheduledRetirementOnlyBeforeDeletionAdmission(t *testing.T) {
	for _, state := range []string{"active", "executing", "applied", "rejected", "uncertain"} {
		t.Run(state, func(t *testing.T) {
			database, request := scheduledCrashFixture(t)
			if state == "executing" || state == "applied" || state == "rejected" {
				if err := BeginDeletionAttempt(database, request); err != nil {
					t.Fatal(err)
				}
			}
			if state == "applied" || state == "rejected" {
				if err := CompleteDeletionAttempt(database, request, state); err != nil {
					t.Fatal(err)
				}
			}
			if state == "uncertain" {
				if err := FinishOperation(database, request, false); err != nil {
					t.Fatal(err)
				}
			}
			err := RecoverUnstartedScheduledVersionDeletion(database, request)
			var receipt model.CleanupOperation
			database.Where("operation_id = ?", request.OperationID).First(&receipt)
			if state == "active" {
				if err != nil || receipt.State != "finished" || receipt.Outcome != ScheduledVersionRetirementPending {
					t.Fatal("unstarted candidate not safely recovered", receipt.State, receipt.Outcome, err)
				}
				if err := BeginDeletionAttempt(database, request); err == nil {
					t.Fatal("old permit admitted after candidate recovery")
				}
				if err := RecoverUnstartedScheduledVersionDeletion(database, request); err != nil {
					t.Fatal("recovery receipt was not idempotent", err)
				}
			} else if !errors.Is(err, ErrCoordinationUncertain) || receipt.State != state {
				t.Fatal("consumed or unknown deletion was released", receipt.State, err)
			}
		})
	}
}

// capability_id: rainbond.image-clean.atomic-scheduled-retirement
func TestRecoveryAndDeletionAdmissionHaveOneWinner(t *testing.T) {
	for iteration := 0; iteration < 8; iteration++ {
		database, request := scheduledCrashFixture(t)
		start := make(chan struct{})
		recovered := make(chan error, 1)
		admitted := make(chan error, 1)
		go func() { <-start; recovered <- RecoverUnstartedScheduledVersionDeletion(database, request) }()
		go func() { <-start; admitted <- BeginDeletionAttempt(database, request) }()
		close(start)
		recoverErr, admitErr := <-recovered, <-admitted
		var receipt model.CleanupOperation
		database.Where("operation_id = ?", request.OperationID).First(&receipt)
		if recoverErr == nil {
			if admitErr == nil || receipt.State != "finished" {
				t.Fatal("old permit survived recovery", admitErr, receipt.State)
			}
		} else if admitErr == nil {
			if !errors.Is(recoverErr, ErrCoordinationUncertain) || receipt.State != "executing" {
				t.Fatal("admitted deletion was released", recoverErr, receipt.State)
			}
		} else {
			t.Fatal("neither contender acquired the existing storage lock", recoverErr, admitErr)
		}
	}
}
