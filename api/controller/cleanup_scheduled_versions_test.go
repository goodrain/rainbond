package controller

import (
	"context"
	"errors"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/goodrain/rainbond/db/model"
	guard "github.com/goodrain/rainbond/pkg/cleanup"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
)

type scheduledVersionRegistryFixture struct {
	database        *gorm.DB
	digests         map[string]string
	tags            []string
	deletes         int
	missing         bool
	unknown         bool
	delayed         bool
	keepPresent     bool
	failDigestReads int
	reject          bool
	existsReadError bool
}

func (f *scheduledVersionRegistryFixture) Resolve(_ context.Context, repository, ref string) (string, error) {
	if strings.HasPrefix(ref, "sha256:") {
		if f.failDigestReads > 0 {
			f.failDigestReads--
			return "", errors.New("fixture read unavailable")
		}
		for _, value := range f.digests {
			if value == ref && !f.missing {
				return ref, nil
			}
		}
	}
	if d := f.digests[ref]; d != "" {
		return d, nil
	}
	return "", errors.New("missing fixture manifest")
}
func (f *scheduledVersionRegistryFixture) Tags(context.Context, string) ([]string, error) {
	return append([]string(nil), f.tags...), nil
}
func (f *scheduledVersionRegistryFixture) Exists(context.Context, string, string) (bool, error) {
	if f.existsReadError {
		return false, errors.New("fixture HEAD unavailable")
	}
	return !f.missing, nil
}
func (f *scheduledVersionRegistryFixture) Delete(_ context.Context, repository, target, permit string) (int, error) {
	f.deletes++
	if permit == "" {
		return 0, errors.New("missing permit")
	}
	var op model.CleanupOperation
	if err := f.database.Where("kind = ? AND state = ? AND target = ?", "delete", "active", target).First(&op).Error; err != nil {
		return 0, err
	}
	r := guard.CoordinationRequest{StorageID: op.StorageID, Generation: op.Generation, Owner: op.Owner, OperationID: op.OperationID, Kind: op.Kind, Scope: op.Scope, Fingerprint: op.Fingerprint, Target: op.Target}
	if err := guard.BeginDeletionAttempt(f.database, r); err != nil {
		return 0, err
	}
	if f.unknown {
		return 0, errors.New("fixture lost response")
	}
	if f.reject {
		if err := guard.CompleteDeletionAttempt(f.database, r, "rejected"); err != nil {
			return 0, err
		}
		return http.StatusForbidden, nil
	}
	f.missing = !f.keepPresent
	if f.delayed {
		go func() { time.Sleep(30 * time.Millisecond); _ = guard.CompleteDeletionAttempt(f.database, r, "applied") }()
	} else if err := guard.CompleteDeletionAttempt(f.database, r, "applied"); err != nil {
		return 0, err
	}
	return http.StatusAccepted, nil
}
func scheduledVersionFixture(t *testing.T) (*CleanupCoordinationHandler, guard.StorageRegistration, *gorm.DB, *scheduledVersionRegistryFixture) {
	t.Helper()
	database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "versions.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	database.LogMode(false)
	if err := database.AutoMigrate(&model.CleanupStorage{}, &model.CleanupOperation{}, &model.TenantServices{}, &model.VersionInfo{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Exec("CREATE TABLE tenant_services_event (service_id TEXT, opt_type TEXT, final_status TEXT)").Error; err != nil {
		t.Fatal(err)
	}
	binding, err := guard.ProvisionRegistryStorage(database, "owned-volume", "/owned/registry")
	if err != nil {
		t.Fatal(err)
	}
	h := &CleanupCoordinationHandler{database: func() *gorm.DB { return database }, permitKey: func() []byte { return []byte(strings.Repeat("owned-fixture-", 4)) }}
	writers := installRegistryCoverageFixture(t, h, binding)
	h.inspectReferenceWriters = func(context.Context) ([]guard.ReferenceWriter, error) { return *writers, nil }
	h.clusterReferences = func(context.Context) (guard.RegionReferenceInventory, error) {
		return guard.RegionReferenceInventory{Complete: true}, nil
	}
	if err := database.Create(&model.TenantServices{TenantID: "tenant", ServiceID: "service", DeployVersion: "20260102"}).Error; err != nil {
		t.Fatal(err)
	}
	for _, v := range []string{"20260101", "20260102"} {
		if err := database.Create(&model.VersionInfo{ServiceID: "service", BuildVersion: v, ImageName: "goodrain.me/team/app:" + v, DeliveredPath: "goodrain.me/team/app:" + v, DeliveredType: "image", FinalStatus: "success", EventID: "build-" + v}).Error; err != nil {
			t.Fatal(err)
		}
	}
	old := "sha256:" + strings.Repeat("a", 64)
	current := "sha256:" + strings.Repeat("b", 64)
	registry := &scheduledVersionRegistryFixture{database: database, digests: map[string]string{"20260101": old, "20260102": current}, tags: []string{"20260101", "20260102"}}
	return h, binding, database, registry
}

// capability_id: rainbond.image-clean.scheduled-version-retirement
func TestScheduledVersionCleanupRetiresOnlyUnprotectedOldVersion(t *testing.T) {
	for _, scenario := range []string{"delete-old", "current-protected", "active-operation", "shared-digest-retained", "alias-referenced", "coverage-missing", "lost-delete-response", "delayed-proxy-receipt", "manifest-still-present", "external-origin", "system-image", "incomplete-references"} {
		t.Run(scenario, func(t *testing.T) {
			h, binding, database, registry := scheduledVersionFixture(t)
			switch scenario {
			case "current-protected":
				database.Model(&model.TenantServices{}).Where("service_id = ?", "service").Update("deploy_version", "20260101")
			case "active-operation":
				database.Exec("INSERT INTO tenant_services_event VALUES (?, ?, ?)", "service", "upgrade", "doing")
			case "shared-digest-retained":
				registry.digests["20260102"] = registry.digests["20260101"]
			case "alias-referenced":
				registry.tags = append(registry.tags, "000-alias")
				registry.digests["000-alias"] = registry.digests["20260101"]
				h.clusterReferences = func(context.Context) (guard.RegionReferenceInventory, error) {
					return guard.RegionReferenceInventory{Complete: true, Images: []string{"goodrain.me/team/app:000-alias"}}, nil
				}
			case "coverage-missing":
				h.inspectRegistryCoverage = nil
			case "lost-delete-response":
				registry.unknown = true
			case "delayed-proxy-receipt":
				registry.delayed = true
			case "manifest-still-present":
				registry.keepPresent = true
			case "external-origin":
				database.Model(&model.VersionInfo{}).Where("build_version = ?", "20260101").Updates(map[string]interface{}{"image_name": "external.invalid/team/app:20260101", "delivered_path": "external.invalid/team/app:20260101"})
			case "system-image":
				database.Model(&model.VersionInfo{}).Where("build_version = ?", "20260101").Updates(map[string]interface{}{"image_name": "goodrain.me/team/runner-helper:20260101", "delivered_path": "goodrain.me/team/runner-helper:20260101"})
			case "incomplete-references":
				h.clusterReferences = func(context.Context) (guard.RegionReferenceInventory, error) {
					return guard.RegionReferenceInventory{Complete: false}, nil
				}

			}
			err := h.runScheduledVersionCleanupWithRegistry(context.Background(), binding, "slot-one", 1, registry, "goodrain.me")
			shouldDelete := scenario == "delete-old" || scenario == "lost-delete-response" || scenario == "delayed-proxy-receipt" || scenario == "manifest-still-present"
			if shouldDelete && registry.deletes != 1 {
				t.Fatalf("delete calls=%d err=%v", registry.deletes, err)
			}
			if !shouldDelete && registry.deletes != 0 {
				t.Fatalf("protected image deleted: calls=%d err=%v", registry.deletes, err)
			}
			var current int
			if err := database.Model(&model.VersionInfo{}).Where("build_version = ?", "20260102").Count(&current).Error; err != nil || current != 1 {
				t.Fatal("newest version lost", err)
			}
			if scenario == "delete-old" || scenario == "delayed-proxy-receipt" {
				if err != nil {
					t.Fatal(err)
				}
				var count int
				database.Model(&model.VersionInfo{}).Where("build_version = ?", "20260101").Count(&count)
				if count != 0 {
					t.Fatal("old record not retired")
				}
				var op model.CleanupOperation
				database.Where("kind = ?", "delete").First(&op)
				if op.State != "finished" {
					t.Fatal(op.State)
				}
			}
			if scenario == "lost-delete-response" || scenario == "manifest-still-present" {
				var op model.CleanupOperation
				database.Where("kind = ?", "delete").First(&op)
				if err == nil || op.State != "uncertain" {
					t.Fatalf("lost response unlocked: state=%s err=%v", op.State, err)
				}
			}
		})
	}
}

// capability_id: rainbond.image-clean.scheduled-version-retirement
func TestScheduledVersionCleanupRetainsDeferredCandidatesAndNeverReplaysUnknownDelete(t *testing.T) {
	for _, scenario := range []string{"deferred-after-retirement", "referenced-alias-removed-later", "unknown-delete"} {
		t.Run(scenario, func(t *testing.T) {
			h, binding, database, remote := scheduledVersionFixture(t)
			switch scenario {
			case "deferred-after-retirement":
				remote.failDigestReads = 1
			case "referenced-alias-removed-later":
				remote.tags = append(remote.tags, "000-alias")
				remote.digests["000-alias"] = remote.digests["20260101"]
				h.clusterReferences = func(context.Context) (guard.RegionReferenceInventory, error) {
					return guard.RegionReferenceInventory{Complete: true, Images: []string{"goodrain.me/team/app:000-alias"}}, nil
				}
			case "unknown-delete":
				remote.unknown = true
			}
			_ = h.runScheduledVersionCleanupWithRegistry(context.Background(), binding, "first-slot", 1, remote, "goodrain.me")
			var oldCount int
			database.Model(&model.VersionInfo{}).Where("build_version = ?", "20260101").Count(&oldCount)
			switch scenario {
			case "deferred-after-retirement":
				var receipt model.CleanupOperation
				database.Where("outcome = ?", guard.ScheduledVersionRetirementPending).First(&receipt)
				if remote.deletes != 0 || oldCount != 0 || receipt.State != "finished" {
					t.Fatalf("retry candidate lost: deletes=%d old=%d receipt=%+v", remote.deletes, oldCount, receipt)
				}
			case "referenced-alias-removed-later":
				if remote.deletes != 0 || oldCount != 1 {
					t.Fatal("referenced version retired before it could be deleted")
				}
				h.clusterReferences = func(context.Context) (guard.RegionReferenceInventory, error) {
					return guard.RegionReferenceInventory{Complete: true}, nil
				}
			}
			err := h.runScheduledVersionCleanupWithRegistry(context.Background(), binding, "next-slot", 1, remote, "goodrain.me")
			if scenario == "unknown-delete" {
				if !errors.Is(err, guard.ErrCoordinationUncertain) || remote.deletes != 1 {
					t.Fatal("unknown deletion replayed", remote.deletes, err)
				}
				return
			}
			if err != nil || remote.deletes != 1 {
				t.Fatal("deferred candidate not reclaimed", remote.deletes, err)
			}
			if err := h.runScheduledVersionCleanupWithRegistry(context.Background(), binding, "third-slot", 1, remote, "goodrain.me"); err != nil || remote.deletes != 1 {
				t.Fatal("successful deletion replayed", remote.deletes, err)
			}
		})
	}
}

// capability_id: rainbond.image-clean.scheduled-version-retirement
func TestScheduledVersionCleanupRetiresSharedOldDigestAsOneGroup(t *testing.T) {
	h, binding, database, remote := scheduledVersionFixture(t)
	if err := database.Where("service_id = ?", "service").Delete(&model.VersionInfo{}).Error; err != nil {
		t.Fatal(err)
	}
	for _, version := range []string{"20260100", "20260101", "20260102"} {
		if err := database.Create(&model.VersionInfo{ServiceID: "service", BuildVersion: version, ImageName: "goodrain.me/team/app:" + version, DeliveredPath: "goodrain.me/team/app:" + version, DeliveredType: "image", FinalStatus: "success", EventID: "build-" + version}).Error; err != nil {
			t.Fatal(err)
		}
	}
	remote.tags = append(remote.tags, "20260100")
	remote.digests["20260100"] = remote.digests["20260101"]
	if err := h.runScheduledVersionCleanupWithRegistry(context.Background(), binding, "shared-slot", 1, remote, "goodrain.me"); err != nil || remote.deletes != 1 {
		t.Fatal("shared old digest not reclaimed once", remote.deletes, err)
	}
	var count int
	database.Model(&model.VersionInfo{}).Count(&count)
	if count != 1 {
		t.Fatal("newest version retention changed", count)
	}
}

// capability_id: rainbond.image-clean.scheduled-version-retirement
func TestScheduledVersionCleanupRetriesKnownRejectionWithoutDuplicatingCandidates(t *testing.T) {
	h, binding, database, remote := scheduledVersionFixture(t)
	remote.reject = true
	for _, slot := range []string{"first-rejected", "second-rejected"} {
		if err := h.runScheduledVersionCleanupWithRegistry(context.Background(), binding, slot, 1, remote, "goodrain.me"); !errors.Is(err, guard.ErrCoordinationDenied) {
			t.Fatal(err)
		}
	}
	var pending int
	database.Model(&model.CleanupOperation{}).Where("owner = ? AND state = ? AND outcome IN (?)", scheduledVersionCleanupOwner, "finished", []string{guard.ScheduledVersionRetirementPending, "rejected"}).Count(&pending)
	if pending != 1 || remote.deletes != 2 {
		t.Fatal("rejected retry multiplied candidate receipts", pending, remote.deletes)
	}
	remote.reject = false
	if err := h.runScheduledVersionCleanupWithRegistry(context.Background(), binding, "third-accepted", 1, remote, "goodrain.me"); err != nil || remote.deletes != 3 {
		t.Fatal("known rejection lost candidate", remote.deletes, err)
	}
	database.Model(&model.CleanupOperation{}).Where("owner = ? AND outcome IN (?)", scheduledVersionCleanupOwner, []string{guard.ScheduledVersionRetirementPending, "rejected"}).Count(&pending)
	if pending != 0 {
		t.Fatal("reclaimed candidate still pending", pending)
	}
}

// capability_id: rainbond.image-clean.scheduled-version-retirement
func TestScheduledVersionCleanupDistinguishesBusyProducerAndUncertainty(t *testing.T) {
	for _, uncertain := range []bool{false, true} {
		t.Run(map[bool]string{false: "active", true: "uncertain"}[uncertain], func(t *testing.T) {
			h, binding, database, remote := scheduledVersionFixture(t)
			writer := guard.CoordinationRequest{StorageID: binding.StorageID, Generation: binding.Generation, Owner: "fixture-writer", OperationID: "writer", Kind: "producer", Scope: "*", Fingerprint: "fixture-writer"}
			if _, err := guard.AcquireOperation(database, writer); err != nil {
				t.Fatal(err)
			}
			want := guard.ErrCoordinationBusy
			if uncertain {
				if err := guard.FinishOperation(database, writer, false); err != nil {
					t.Fatal(err)
				}
				want = guard.ErrCoordinationUncertain
			}
			if err := h.runScheduledVersionCleanupWithRegistry(context.Background(), binding, "busy-slot", 1, remote, "goodrain.me"); !errors.Is(err, want) || remote.deletes != 0 {
				t.Fatal("wrong producer state classification", err, remote.deletes)
			}
			var count int
			database.Model(&model.VersionInfo{}).Count(&count)
			if count != 2 {
				t.Fatal("version retired while busy", count)
			}
		})
	}
}

// capability_id: rainbond.image-clean.scheduled-version-retirement
func TestScheduledVersionCleanupRecoversCommittedCrashBeforeReadiness(t *testing.T) {
	for _, consumed := range []bool{false, true} {
		t.Run(map[bool]string{false: "crashed-before-dispatch", true: "old-permit-consumed"}[consumed], func(t *testing.T) {
			h, binding, database, remote := scheduledVersionFixture(t)
			target := remote.digests["20260101"]
			original := guard.CoordinationRequest{StorageID: binding.StorageID, Generation: binding.Generation, Owner: scheduledVersionCleanupOwner, OperationID: "original-crashed-operation", Kind: "delete", Scope: "team/app", Target: target, Fingerprint: "original-retirement"}
			if created, err := guard.AcquireOperation(database, original); err != nil || !created {
				t.Fatal(created, err)
			}
			expected := guard.ScheduledVersionRetirement{TenantID: "tenant", Expected: guard.VersionExpectation{ServiceID: "service", Version: "20260101", CurrentVersion: "20260102", Image: "goodrain.me/team/app:20260101", EventID: "build-20260101"}}
			if err := guard.RetireScheduledVersions(database, original, []guard.ScheduledVersionRetirement{expected}, 1, []string{"20260101"}, guard.RegionReferenceInventory{Complete: true}); err != nil {
				t.Fatal(err)
			}
			// Deliberately do not install a Finish defer. Only committed data survives
			// this simulated process exit; the next invocation has no caller state.
			var receipt model.CleanupOperation
			database.Where("operation_id = ?", original.OperationID).First(&receipt)
			if receipt.State != "active" || receipt.Outcome != guard.ScheduledVersionRetirementPending {
				t.Fatal("not the committed crash point", receipt.State, receipt.Outcome)
			}
			if consumed {
				if err := guard.BeginDeletionAttempt(database, original); err != nil {
					t.Fatal(err)
				}
			}
			err := h.runScheduledVersionCleanupWithRegistry(context.Background(), binding, "restarted-slot", 1, remote, "goodrain.me")
			database.Where("operation_id = ?", original.OperationID).First(&receipt)
			if consumed {
				if !errors.Is(err, guard.ErrCoordinationUncertain) || remote.deletes != 0 || receipt.State != "executing" {
					t.Fatal("consumed old permit was released or replayed", receipt.State, remote.deletes, err)
				}
				return
			}
			if err != nil || remote.deletes != 1 || receipt.State != "finished" || receipt.Outcome != "applied" {
				t.Fatal("committed candidate was not recovered", receipt.State, receipt.Outcome, remote.deletes, err)
			}
			if err := guard.BeginDeletionAttempt(database, original); err == nil {
				t.Fatal("old permit remained usable after recovery")
			}
			if err := h.runScheduledVersionCleanupWithRegistry(context.Background(), binding, "later-slot", 1, remote, "goodrain.me"); err != nil || remote.deletes != 1 {
				t.Fatal("recovered deletion was replayed", remote.deletes, err)
			}
		})
	}
}

// capability_id: rainbond.image-clean.scheduled-version-retirement
func TestScheduledVersionCleanupRecoversKnownProxyOutcomeWithoutReplayingDelete(t *testing.T) {
	for _, scenario := range []string{"primary-applied", "primary-rejected", "retry-applied", "retry-rejected", "retry-unstarted", "applied-still-present", "applied-read-failed"} {
		t.Run(scenario, func(t *testing.T) {
			h, binding, database, remote := scheduledVersionFixture(t)
			original := guard.CoordinationRequest{StorageID: binding.StorageID, Generation: binding.Generation, Owner: scheduledVersionCleanupOwner, OperationID: "primary-before-crash", Kind: "delete", Scope: "team/app", Target: remote.digests["20260101"], Fingerprint: "primary-original"}
			if _, err := guard.AcquireOperation(database, original); err != nil {
				t.Fatal(err)
			}
			selected := []guard.ScheduledVersionRetirement{{TenantID: "tenant", Expected: guard.VersionExpectation{ServiceID: "service", Version: "20260101", CurrentVersion: "20260102", Image: "goodrain.me/team/app:20260101", EventID: "build-20260101"}}}
			if err := guard.RetireScheduledVersions(database, original, selected, 1, []string{"20260101"}, guard.RegionReferenceInventory{Complete: true}); err != nil {
				t.Fatal(err)
			}
			crashed := original
			if strings.HasPrefix(scenario, "retry-") {
				if err := guard.FinishOperation(database, original, true); err != nil {
					t.Fatal(err)
				}
				crashed = guard.CoordinationRequest{StorageID: binding.StorageID, Generation: binding.Generation, Owner: "chaos-scheduled-version-retry", OperationID: "retry-before-crash", Kind: "delete", Scope: original.Scope, Target: original.Target, Fingerprint: "retry-original"}
				if _, err := guard.AcquireOperation(database, crashed); err != nil {
					t.Fatal(err)
				}
			}
			if scenario != "retry-unstarted" {
				if err := guard.BeginDeletionAttempt(database, crashed); err != nil {
					t.Fatal(err)
				}
				outcome := "applied"
				if strings.HasSuffix(scenario, "-rejected") {
					outcome = "rejected"
				}
				if err := guard.CompleteDeletionAttempt(database, crashed, outcome); err != nil {
					t.Fatal(err)
				}
				remote.missing = outcome == "applied"
			}
			if scenario == "applied-still-present" {
				remote.missing = false
			}
			if scenario == "applied-read-failed" {
				remote.existsReadError = true
			}
			// The proxy's terminal receipt is committed, but the original caller never
			// runs Finish. Recovery may observe it; it must never repeat that DELETE.
			err := h.runScheduledVersionCleanupWithRegistry(context.Background(), binding, "restart-after-proxy-receipt", 1, remote, "goodrain.me")
			var receipt model.CleanupOperation
			database.Where("operation_id = ?", crashed.OperationID).First(&receipt)
			if scenario == "applied-still-present" || scenario == "applied-read-failed" {
				if !errors.Is(err, guard.ErrCoordinationUncertain) || receipt.State != "applied" || remote.deletes != 0 {
					t.Fatal("unverified terminal receipt released", receipt.State, remote.deletes, err)
				}
				return
			}
			wantDeletes := 0
			if strings.HasSuffix(scenario, "-rejected") || scenario == "retry-unstarted" {
				wantDeletes = 1
			}
			if err != nil || receipt.State != "finished" || remote.deletes != wantDeletes {
				t.Fatal("proxy outcome recovery failed", receipt.State, remote.deletes, err)
			}
			if err := guard.BeginDeletionAttempt(database, crashed); err == nil {
				t.Fatal("recovered original permit still usable")
			}
			var pending int
			database.Model(&model.CleanupOperation{}).Where("owner = ? AND state = ? AND outcome IN (?)", scheduledVersionCleanupOwner, "finished", []string{guard.ScheduledVersionRetirementPending, "rejected"}).Count(&pending)
			if pending != 0 {
				t.Fatal("completed original candidate remains pending", pending)
			}
		})
	}
}
