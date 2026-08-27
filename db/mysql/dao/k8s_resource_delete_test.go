// RAINBOND, Application Management Platform
// Copyright (C) 2026 Goodrain Co., Ltd.

package dao

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/goodrain/rainbond/api/model"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
)

func newK8sResourceDeleteTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "k8s-resource-delete.db"))
	if err != nil {
		t.Fatalf("open sqlite database: %v", err)
	}
	db.LogMode(false)
	if err := db.AutoMigrate(&dbmodel.K8sResource{}).Error; err != nil {
		db.Close()
		t.Fatalf("migrate k8s resources: %v", err)
	}
	return db
}

func createK8sResourceForDeleteTest(t *testing.T, dao *K8sResourceDaoImpl, resource *dbmodel.K8sResource) {
	t.Helper()
	if err := dao.AddModel(resource); err != nil {
		t.Fatalf("create k8s resource: %v", err)
	}
}

func claimK8sResourceDeleteForTest(t *testing.T, dao *K8sResourceDaoImpl, resource dbmodel.K8sResource) *dbmodel.K8sResource {
	t.Helper()
	now := time.Now().Add(time.Second)
	claimed, err := dao.ClaimK8sResourceDeleteLease(resource.ID, resource.DeleteGeneration, now, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("claim deletion lease: %v", err)
	}
	if claimed == nil || claimed.DeleteLeaseToken == "" {
		t.Fatalf("expected deletion lease token, got %#v", claimed)
	}
	return claimed
}

// capability_id: rainbond.k8s-resource.persist-delete-lifecycle
func TestK8sResourceDeleteLifecycleKeepsCreateUpdateState(t *testing.T) {
	db := newK8sResourceDeleteTestDB(t)
	defer db.Close()

	dao := &K8sResourceDaoImpl{DB: db}
	resource := &dbmodel.K8sResource{
		AppID:         "app-1",
		Name:          "app-config",
		Kind:          "ConfigMap",
		Content:       "apiVersion: v1",
		ErrorOverview: "existing create result",
		State:         model.UpdateSuccess,
	}
	createK8sResourceForDeleteTest(t, dao, resource)

	accepted, err := dao.MarkK8sResourcesDeletingByIDs(resource.AppID, []uint{resource.ID})
	if err != nil {
		t.Fatalf("mark deleting: %v", err)
	}
	if len(accepted) != 1 || accepted[0].ID != resource.ID || accepted[0].DeleteGeneration != 1 {
		t.Fatalf("expected accepted resource id and first generation, got %#v", accepted)
	}

	deleting, err := dao.GetK8sResourceByName(resource.AppID, resource.Name, resource.Kind)
	if err != nil {
		t.Fatalf("get deleting resource: %v", err)
	}
	if deleting.State != model.UpdateSuccess {
		t.Fatalf("delete lifecycle must not change create/update state: got %d", deleting.State)
	}
	if deleting.DeleteStatus != dbmodel.K8sResourceDeleteStatusDeleting {
		t.Fatalf("expected deleting status, got %d", deleting.DeleteStatus)
	}
	if deleting.DeleteGeneration != accepted[0].DeleteGeneration {
		t.Fatalf("expected persisted generation %d, got %d", accepted[0].DeleteGeneration, deleting.DeleteGeneration)
	}
	if deleting.DeleteStartedAt == nil {
		t.Fatal("expected delete started time to be recorded")
	}
	if deleting.DeleteError != "" || deleting.DeleteAttempts != 0 {
		t.Fatalf("expected a fresh delete lifecycle, got error %q and attempts %d", deleting.DeleteError, deleting.DeleteAttempts)
	}
	lease := claimK8sResourceDeleteForTest(t, dao, accepted[0])
	updated, err := dao.IncreaseK8sResourceDeleteAttempts(accepted[0].ID, accepted[0].DeleteGeneration, lease.DeleteLeaseToken)
	if err != nil {
		t.Fatalf("increase delete attempts: %v", err)
	}
	if !updated {
		t.Fatal("expected active generation attempt to be recorded")
	}
	attempted, err := dao.GetK8sResourceByName(resource.AppID, resource.Name, resource.Kind)
	if err != nil {
		t.Fatalf("get attempted resource: %v", err)
	}
	if attempted.DeleteAttempts != 1 || attempted.State != model.UpdateSuccess {
		t.Fatalf("expected one delete attempt without changing State, got %#v", attempted)
	}

	updated, err = dao.MarkK8sResourceDeleteFailed(accepted[0].ID, accepted[0].DeleteGeneration, lease.DeleteLeaseToken, "resource remains terminating")
	if err != nil {
		t.Fatalf("mark delete failed: %v", err)
	}
	if !updated {
		t.Fatal("expected active generation to be marked failed")
	}

	failed, err := dao.GetK8sResourceByName(resource.AppID, resource.Name, resource.Kind)
	if err != nil {
		t.Fatalf("get failed resource: %v", err)
	}
	if failed.State != model.UpdateSuccess {
		t.Fatalf("delete failure must not change create/update state: got %d", failed.State)
	}
	if failed.DeleteStatus != dbmodel.K8sResourceDeleteStatusFailed {
		t.Fatalf("expected failed status, got %d", failed.DeleteStatus)
	}
	if failed.DeleteError != "resource remains terminating" {
		t.Fatalf("unexpected delete error: %q", failed.DeleteError)
	}
	if failed.DeleteAttempts != 1 {
		t.Fatalf("expected failed lifecycle to retain attempt count, got %d", failed.DeleteAttempts)
	}
}

// capability_id: rainbond.k8s-resource.persist-delete-lifecycle
func TestK8sResourceDeleteMarkingDoesNotOverwriteInProgressResource(t *testing.T) {
	db := newK8sResourceDeleteTestDB(t)
	defer db.Close()

	dao := &K8sResourceDaoImpl{DB: db}
	inProgress := &dbmodel.K8sResource{
		AppID:            "app-1",
		Name:             "in-progress",
		Kind:             "Secret",
		State:            model.CreateSuccess,
		DeleteStatus:     dbmodel.K8sResourceDeleteStatusDeleting,
		DeleteError:      "already submitted",
		DeleteAttempts:   3,
		DeleteGeneration: 7,
	}
	failed := &dbmodel.K8sResource{
		AppID:          "app-1",
		Name:           "retryable",
		Kind:           "ConfigMap",
		State:          model.CreateSuccess,
		DeleteStatus:   dbmodel.K8sResourceDeleteStatusFailed,
		DeleteError:    "previous timeout",
		DeleteAttempts: 2,
	}
	failedWithSameNameAndKind := &dbmodel.K8sResource{
		AppID:          "app-1",
		Name:           "retryable",
		Kind:           "ConfigMap",
		State:          model.CreateSuccess,
		DeleteStatus:   dbmodel.K8sResourceDeleteStatusFailed,
		DeleteError:    "another previous timeout",
		DeleteAttempts: 4,
	}
	otherAppResource := &dbmodel.K8sResource{
		AppID: "app-2",
		Name:  "other-app-resource",
		Kind:  "ConfigMap",
		State: model.CreateSuccess,
	}
	uniqueLegacyResource := &dbmodel.K8sResource{
		AppID: "app-1",
		Name:  "unique-legacy-resource",
		Kind:  "Secret",
		State: model.CreateSuccess,
	}
	createK8sResourceForDeleteTest(t, dao, inProgress)
	createK8sResourceForDeleteTest(t, dao, failed)
	createK8sResourceForDeleteTest(t, dao, failedWithSameNameAndKind)
	createK8sResourceForDeleteTest(t, dao, otherAppResource)
	createK8sResourceForDeleteTest(t, dao, uniqueLegacyResource)

	_, err := dao.MarkK8sResourcesDeletingByNames("app-1", []dbmodel.K8sResourceDeleteTarget{
		{Name: uniqueLegacyResource.Name, Kind: uniqueLegacyResource.Kind},
		{Name: failed.Name, Kind: failed.Kind},
	})
	if err == nil {
		t.Fatal("expected name/kind acceptance to reject ambiguous Region record identity")
	}
	var ambiguity *dbmodel.K8sResourceDeleteIdentityAmbiguousError
	if !errors.As(err, &ambiguity) {
		t.Fatalf("expected ambiguous identity error, got %v", err)
	}
	if ambiguity.AppID != "app-1" || ambiguity.Name != failed.Name || ambiguity.Kind != failed.Kind || len(ambiguity.ResourceIDs) != 2 {
		t.Fatalf("ambiguous identity error must expose candidate record IDs, got %#v", ambiguity)
	}
	var persistedUniqueLegacy dbmodel.K8sResource
	if err := db.Where("ID = ?", uniqueLegacyResource.ID).First(&persistedUniqueLegacy).Error; err != nil {
		t.Fatalf("get unique legacy resource: %v", err)
	}
	if persistedUniqueLegacy.DeleteStatus != dbmodel.K8sResourceDeleteStatusActive {
		t.Fatalf("ambiguous compatibility batch must not partially accept other targets, got %#v", persistedUniqueLegacy)
	}
	legacyAccepted, err := dao.MarkK8sResourcesDeletingByNames("app-1", []dbmodel.K8sResourceDeleteTarget{{Name: uniqueLegacyResource.Name, Kind: uniqueLegacyResource.Kind}})
	if err != nil || len(legacyAccepted) != 1 || legacyAccepted[0].ID != uniqueLegacyResource.ID || legacyAccepted[0].DeleteGeneration != 1 {
		t.Fatalf("unique name/kind compatibility target must accept exactly one Region record: accepted=%#v err=%v", legacyAccepted, err)
	}
	accepted, err := dao.MarkK8sResourcesDeletingByIDs("app-1", []uint{inProgress.ID, failed.ID, failedWithSameNameAndKind.ID})
	if err != nil {
		t.Fatalf("mark resources deleting: %v", err)
	}
	if len(accepted) != 2 {
		t.Fatalf("expected every matching retryable record to be accepted, got %#v", accepted)
	}
	acceptedGenerations := make(map[uint]int64, len(accepted))
	for _, acceptedResource := range accepted {
		acceptedGenerations[acceptedResource.ID] = acceptedResource.DeleteGeneration
	}
	if acceptedGenerations[failed.ID] != 1 || acceptedGenerations[failedWithSameNameAndKind.ID] != 1 {
		t.Fatalf("accepted records must carry their own IDs and generations, got %#v", accepted)
	}

	persistedInProgress, err := dao.GetK8sResourceByName("app-1", inProgress.Name, inProgress.Kind)
	if err != nil {
		t.Fatalf("get in-progress resource: %v", err)
	}
	if persistedInProgress.DeleteStatus != dbmodel.K8sResourceDeleteStatusDeleting || persistedInProgress.DeleteAttempts != 3 || persistedInProgress.DeleteError != "already submitted" {
		t.Fatalf("must not overwrite an in-progress delete: %#v", persistedInProgress)
	}

	persistedRetry, err := dao.GetK8sResourceByName("app-1", failed.Name, failed.Kind)
	if err != nil {
		t.Fatalf("get retryable resource: %v", err)
	}
	if persistedRetry.DeleteStatus != dbmodel.K8sResourceDeleteStatusDeleting {
		t.Fatalf("expected retryable resource to be marked deleting, got %d", persistedRetry.DeleteStatus)
	}
	if persistedRetry.DeleteAttempts != 0 || persistedRetry.DeleteError != "" || persistedRetry.DeleteStartedAt == nil {
		t.Fatalf("expected retry lifecycle to reset fields, got %#v", persistedRetry)
	}
	var persistedDuplicate dbmodel.K8sResource
	err = db.Where("ID = ?", failedWithSameNameAndKind.ID).First(&persistedDuplicate).Error
	if err != nil {
		t.Fatalf("get duplicate identity resource: %v", err)
	}
	if persistedDuplicate.DeleteStatus != dbmodel.K8sResourceDeleteStatusDeleting || persistedDuplicate.DeleteAttempts != 0 || persistedDuplicate.DeleteError != "" {
		t.Fatalf("expected duplicate identity record to be independently accepted, got %#v", persistedDuplicate)
	}
	var persistedOtherApp dbmodel.K8sResource
	if err := db.Where("ID = ?", otherAppResource.ID).First(&persistedOtherApp).Error; err != nil {
		t.Fatalf("get out-of-app resource: %v", err)
	}
	if persistedOtherApp.DeleteStatus != dbmodel.K8sResourceDeleteStatusActive {
		t.Fatalf("record IDs outside the requested app must not be accepted, got %#v", persistedOtherApp)
	}
	if _, err := dao.MarkK8sResourcesDeletingByIDs("app-1", []uint{otherAppResource.ID}); err == nil {
		t.Fatal("out-of-app Region record ID must reject acceptance")
	}
}

// capability_id: rainbond.k8s-resource.persist-delete-lifecycle
func TestK8sResourceDeleteStatusLookupUsesExactNameAndKind(t *testing.T) {
	db := newK8sResourceDeleteTestDB(t)
	defer db.Close()

	dao := &K8sResourceDaoImpl{DB: db}
	createK8sResourceForDeleteTest(t, dao, &dbmodel.K8sResource{
		AppID:            "app-1",
		Name:             "shared-name",
		Kind:             "ConfigMap",
		Content:          "private resource yaml",
		State:            model.CreateSuccess,
		DeleteStatus:     dbmodel.K8sResourceDeleteStatusDeleting,
		DeleteGeneration: 4,
	})
	createK8sResourceForDeleteTest(t, dao, &dbmodel.K8sResource{
		AppID:        "app-1",
		Name:         "shared-name",
		Kind:         "Secret",
		State:        model.CreateSuccess,
		DeleteStatus: dbmodel.K8sResourceDeleteStatusFailed,
	})
	createK8sResourceForDeleteTest(t, dao, &dbmodel.K8sResource{
		AppID:        "another-app",
		Name:         "shared-name",
		Kind:         "ConfigMap",
		State:        model.CreateSuccess,
		DeleteStatus: dbmodel.K8sResourceDeleteStatusFailed,
	})

	statuses, err := dao.ListK8sResourcesDeleteStatus("app-1", []dbmodel.K8sResourceDeleteTarget{{Name: "shared-name", Kind: "ConfigMap"}})
	if err != nil {
		t.Fatalf("list delete statuses: %v", err)
	}
	if len(statuses) != 1 {
		t.Fatalf("expected one exact status, got %#v", statuses)
	}
	if statuses[0].ID == 0 || statuses[0].Kind != "ConfigMap" || statuses[0].DeleteStatus != dbmodel.K8sResourceDeleteStatusDeleting || statuses[0].DeleteGeneration != 4 {
		t.Fatalf("expected ConfigMap deleting status, got %#v", statuses[0])
	}
	if statuses[0].Content != "" {
		t.Fatalf("delete status lookup must not return resource YAML, got %q", statuses[0].Content)
	}

	statuses, err = dao.ListK8sResourcesDeleteStatus("app-1", []dbmodel.K8sResourceDeleteTarget{{ResourceID: statuses[0].ID}})
	if err != nil || len(statuses) != 1 || statuses[0].ID == 0 || statuses[0].Content != "" {
		t.Fatalf("status lookup must support the preferred Region record ID without returning YAML: statuses=%#v err=%v", statuses, err)
	}
}

// capability_id: rainbond.k8s-resource.persist-delete-lifecycle
func TestK8sResourceDeleteGenerationRejectsStaleWorkerWrites(t *testing.T) {
	db := newK8sResourceDeleteTestDB(t)
	defer db.Close()

	dao := &K8sResourceDaoImpl{DB: db}
	resource := &dbmodel.K8sResource{
		AppID: "app-1",
		Name:  "recreated-name",
		Kind:  "ConfigMap",
		State: model.CreateSuccess,
	}
	createK8sResourceForDeleteTest(t, dao, resource)

	firstAccepted, err := dao.MarkK8sResourcesDeletingByIDs(resource.AppID, []uint{resource.ID})
	if err != nil || len(firstAccepted) != 1 {
		t.Fatalf("accept first delete: resources=%#v err=%v", firstAccepted, err)
	}
	first := firstAccepted[0]
	firstLease := claimK8sResourceDeleteForTest(t, dao, first)
	updated, err := dao.MarkK8sResourceDeleteFailed(first.ID, first.DeleteGeneration, firstLease.DeleteLeaseToken, "first task timed out")
	if err != nil || !updated {
		t.Fatalf("fail first delete: updated=%v err=%v", updated, err)
	}

	secondAccepted, err := dao.MarkK8sResourcesDeletingByIDs(resource.AppID, []uint{resource.ID})
	if err != nil || len(secondAccepted) != 1 {
		t.Fatalf("accept retry delete: resources=%#v err=%v", secondAccepted, err)
	}
	second := secondAccepted[0]
	if second.ID != first.ID || second.DeleteGeneration != first.DeleteGeneration+1 {
		t.Fatalf("expected same record with a newer generation, got first=%#v second=%#v", first, second)
	}

	secondLease := claimK8sResourceDeleteForTest(t, dao, second)
	updated, err = dao.IncreaseK8sResourceDeleteAttempts(first.ID, first.DeleteGeneration, firstLease.DeleteLeaseToken)
	if err != nil || updated {
		t.Fatalf("stale worker must not record an attempt: updated=%v err=%v", updated, err)
	}
	updated, err = dao.MarkK8sResourceDeleteFailed(first.ID, first.DeleteGeneration, firstLease.DeleteLeaseToken, "stale task failure")
	if err != nil || updated {
		t.Fatalf("stale worker must not mark the newer generation failed: updated=%v err=%v", updated, err)
	}
	deleted, err := dao.DeleteK8sResourceByIDAndGeneration(first.ID, first.DeleteGeneration, firstLease.DeleteLeaseToken)
	if err != nil || deleted {
		t.Fatalf("stale worker must not delete the newer generation: deleted=%v err=%v", deleted, err)
	}

	current, err := dao.GetK8sResourceByName(resource.AppID, resource.Name, resource.Kind)
	if err != nil {
		t.Fatalf("get current generation: %v", err)
	}
	if current.DeleteStatus != dbmodel.K8sResourceDeleteStatusDeleting || current.DeleteGeneration != second.DeleteGeneration || current.DeleteAttempts != 0 || current.DeleteError != "" {
		t.Fatalf("stale worker write leaked into current generation: %#v", current)
	}

	deleted, err = dao.DeleteK8sResourceByIDAndGeneration(second.ID, second.DeleteGeneration, secondLease.DeleteLeaseToken)
	if err != nil || !deleted {
		t.Fatalf("current worker must delete current generation: deleted=%v err=%v", deleted, err)
	}
}

// capability_id: rainbond.k8s-resource.persist-delete-lifecycle
func TestK8sResourceDeleteLeaseAndRetryMakeAcceptedWorkRecoverable(t *testing.T) {
	db := newK8sResourceDeleteTestDB(t)
	defer db.Close()

	dao := &K8sResourceDaoImpl{DB: db}
	resource := &dbmodel.K8sResource{
		AppID:   "app-1",
		Name:    "recoverable-delete",
		Kind:    "ConfigMap",
		Content: "private resource yaml",
		State:   model.CreateSuccess,
	}
	createK8sResourceForDeleteTest(t, dao, resource)
	accepted, err := dao.MarkK8sResourcesDeletingByIDs(resource.AppID, []uint{resource.ID})
	if err != nil || len(accepted) != 1 {
		t.Fatalf("accept deletion: resources=%#v err=%v", accepted, err)
	}

	now := time.Now().Add(time.Second)
	due, err := dao.ListDueK8sResourceDeletions(now, 10)
	if err != nil {
		t.Fatalf("list due deletions: %v", err)
	}
	if len(due) != 1 || due[0].ID != accepted[0].ID || due[0].DeleteGeneration != accepted[0].DeleteGeneration {
		t.Fatalf("accepted work must be recoverable by scanner, got %#v", due)
	}
	if due[0].Content != "" {
		t.Fatalf("scanner must not load saved YAML, got %q", due[0].Content)
	}

	leaseUntil := now.Add(time.Minute)
	claimed, err := dao.ClaimK8sResourceDeleteLease(accepted[0].ID, accepted[0].DeleteGeneration, now, leaseUntil)
	if err != nil || claimed == nil || claimed.DeleteLeaseToken == "" {
		t.Fatalf("claim due deletion lease: claimed=%#v err=%v", claimed, err)
	}
	firstLease := claimed
	owned, err := dao.GetK8sResourceForDeletion(accepted[0].ID, accepted[0].DeleteGeneration, firstLease.DeleteLeaseToken)
	if err != nil || owned.Content != "private resource yaml" {
		t.Fatalf("lease owner must load saved resource: resource=%#v err=%v", owned, err)
	}
	if _, err := dao.GetK8sResourceForDeletion(accepted[0].ID, accepted[0].DeleteGeneration, "wrong-token"); err == nil {
		t.Fatal("worker without the lease token must not load the deletion resource")
	}
	claimed, err = dao.ClaimK8sResourceDeleteLease(accepted[0].ID, accepted[0].DeleteGeneration, now, leaseUntil)
	if err != nil || claimed != nil {
		t.Fatalf("active lease must not be claimed twice: claimed=%#v err=%v", claimed, err)
	}

	due, err = dao.ListDueK8sResourceDeletions(now, 10)
	if err != nil || len(due) != 0 {
		t.Fatalf("leased work must be excluded from scanner: due=%#v err=%v", due, err)
	}
	replacementLeaseUntil := leaseUntil.Add(time.Minute)
	replacementLease, err := dao.ClaimK8sResourceDeleteLease(accepted[0].ID, accepted[0].DeleteGeneration, leaseUntil.Add(time.Second), replacementLeaseUntil)
	if err != nil || replacementLease == nil || replacementLease.DeleteLeaseToken == firstLease.DeleteLeaseToken {
		t.Fatalf("expired lease must be reclaimed with a new token: lease=%#v err=%v", replacementLease, err)
	}
	updated, err := dao.IncreaseK8sResourceDeleteAttempts(accepted[0].ID, accepted[0].DeleteGeneration, firstLease.DeleteLeaseToken)
	if err != nil || updated {
		t.Fatalf("worker with expired lease token must not write: updated=%v err=%v", updated, err)
	}
	scheduled, err := dao.ScheduleK8sResourceDeleteRetry(accepted[0].ID, accepted[0].DeleteGeneration, firstLease.DeleteLeaseToken, replacementLeaseUntil.Add(time.Minute), "stale retry")
	if err != nil || scheduled {
		t.Fatalf("worker with expired lease token must not schedule retry: scheduled=%v err=%v", scheduled, err)
	}
	failed, err := dao.MarkK8sResourceDeleteFailed(accepted[0].ID, accepted[0].DeleteGeneration, firstLease.DeleteLeaseToken, "stale failure")
	if err != nil || failed {
		t.Fatalf("worker with expired lease token must not mark failure: failed=%v err=%v", failed, err)
	}
	deleted, err := dao.DeleteK8sResourceByIDAndGeneration(accepted[0].ID, accepted[0].DeleteGeneration, firstLease.DeleteLeaseToken)
	if err != nil || deleted {
		t.Fatalf("worker with expired lease token must not delete: deleted=%v err=%v", deleted, err)
	}
	nextRetryAt := replacementLeaseUntil.Add(time.Minute)
	scheduled, err = dao.ScheduleK8sResourceDeleteRetry(accepted[0].ID, accepted[0].DeleteGeneration, replacementLease.DeleteLeaseToken, nextRetryAt, "temporary kubernetes API failure")
	if err != nil || !scheduled {
		t.Fatalf("schedule retry: scheduled=%v err=%v", scheduled, err)
	}

	var retrying dbmodel.K8sResource
	if err := db.Where("ID = ?", accepted[0].ID).First(&retrying).Error; err != nil {
		t.Fatalf("get retrying deletion: %v", err)
	}
	if retrying.DeleteLeaseUntil != nil || retrying.DeleteLeaseToken != "" || retrying.DeleteNextRetryAt == nil || retrying.DeleteError != "temporary kubernetes API failure" {
		t.Fatalf("retry must release lease and persist due time, got %#v", retrying)
	}
	due, err = dao.ListDueK8sResourceDeletions(now, 10)
	if err != nil || len(due) != 0 {
		t.Fatalf("future retry must not be scanned early: due=%#v err=%v", due, err)
	}
	due, err = dao.ListDueK8sResourceDeletions(nextRetryAt.Add(time.Second), 10)
	if err != nil || len(due) != 1 || due[0].ID != accepted[0].ID {
		t.Fatalf("expired retry must be scanned: due=%#v err=%v", due, err)
	}

	terminalLease, err := dao.ClaimK8sResourceDeleteLease(accepted[0].ID, accepted[0].DeleteGeneration, nextRetryAt.Add(time.Second), nextRetryAt.Add(time.Minute))
	if err != nil || terminalLease == nil {
		t.Fatalf("claim terminal deletion lease: lease=%#v err=%v", terminalLease, err)
	}
	failed, err = dao.MarkK8sResourceDeleteFailed(accepted[0].ID, accepted[0].DeleteGeneration, terminalLease.DeleteLeaseToken, "permanent RBAC denial")
	if err != nil || !failed {
		t.Fatalf("mark terminal failure: failed=%v err=%v", failed, err)
	}
	var terminal dbmodel.K8sResource
	if err := db.Where("ID = ?", accepted[0].ID).First(&terminal).Error; err != nil {
		t.Fatalf("get terminal failure: %v", err)
	}
	if terminal.DeleteStatus != dbmodel.K8sResourceDeleteStatusFailed || terminal.DeleteLeaseUntil != nil || terminal.DeleteLeaseToken != "" || terminal.DeleteNextRetryAt != nil {
		t.Fatalf("terminal failure must release retry lease state, got %#v", terminal)
	}
}

// capability_id: rainbond.k8s-resource.persist-delete-lifecycle
func TestK8sResourceDaoUpdateModelPreservesDeleteLifecycleAndRejectsDeletingRows(t *testing.T) {
	db := newK8sResourceDeleteTestDB(t)
	defer db.Close()

	dao := &K8sResourceDaoImpl{DB: db}
	active := &dbmodel.K8sResource{
		AppID:                 "app-1",
		Name:                  "active-resource",
		Kind:                  "ConfigMap",
		Content:               "old-content",
		ResourceUID:           "old-uid",
		ResourceOrigin:        dbmodel.K8sResourceOriginUser,
		ForceFinalizerAllowed: true,
		ErrorOverview:         "old-error",
		State:                 model.CreateSuccess,
		DeleteError:           "historical deletion error",
		DeleteAttempts:        3,
		DeleteGeneration:      5,
		DeleteLeaseToken:      "historical-token",
	}
	deleting := &dbmodel.K8sResource{
		AppID:            "app-1",
		Name:             "deleting-resource",
		Kind:             "Secret",
		Content:          "original-content",
		ErrorOverview:    "original-error",
		State:            model.CreateSuccess,
		DeleteStatus:     dbmodel.K8sResourceDeleteStatusDeleting,
		DeleteGeneration: 1,
	}
	createK8sResourceForDeleteTest(t, dao, active)
	createK8sResourceForDeleteTest(t, dao, deleting)

	if err := dao.UpdateModel(&dbmodel.K8sResource{
		Model:                 dbmodel.Model{ID: active.ID},
		Content:               "new-content",
		ResourceUID:           "new-uid",
		ResourceOrigin:        dbmodel.K8sResourceOriginUser,
		ForceFinalizerAllowed: true,
		ErrorOverview:         "new-error",
		State:                 model.UpdateSuccess,
		DeleteStatus:          dbmodel.K8sResourceDeleteStatusFailed,
		DeleteError:           "must not overwrite",
		DeleteAttempts:        99,
		DeleteGeneration:      99,
	}); err != nil {
		t.Fatalf("update active resource: %v", err)
	}
	var updated dbmodel.K8sResource
	if err := db.Where("ID = ?", active.ID).First(&updated).Error; err != nil {
		t.Fatalf("get updated active resource: %v", err)
	}
	if updated.Content != "new-content" || updated.ResourceUID != "new-uid" || updated.ResourceOrigin != dbmodel.K8sResourceOriginUser || !updated.ForceFinalizerAllowed || updated.ErrorOverview != "new-error" || updated.State != model.UpdateSuccess {
		t.Fatalf("expected create/update fields to be saved, got %#v", updated)
	}
	if updated.DeleteStatus != dbmodel.K8sResourceDeleteStatusActive || updated.DeleteError != "historical deletion error" || updated.DeleteAttempts != 3 || updated.DeleteGeneration != 5 || updated.DeleteLeaseToken != "historical-token" {
		t.Fatalf("UpdateModel must not overwrite deletion lifecycle, got %#v", updated)
	}

	err := dao.UpdateModel(&dbmodel.K8sResource{
		Model:         dbmodel.Model{ID: deleting.ID},
		Content:       "must-not-persist",
		ErrorOverview: "must-not-persist",
		State:         model.UpdateSuccess,
	})
	if err == nil {
		t.Fatal("expected deleting resource update to be rejected")
	}
	var persistedDeleting dbmodel.K8sResource
	if err := db.Where("ID = ?", deleting.ID).First(&persistedDeleting).Error; err != nil {
		t.Fatalf("get deleting resource: %v", err)
	}
	if persistedDeleting.Content != "original-content" || persistedDeleting.ErrorOverview != "original-error" || persistedDeleting.State != model.CreateSuccess || persistedDeleting.DeleteStatus != dbmodel.K8sResourceDeleteStatusDeleting {
		t.Fatalf("deleting resource must remain unchanged, got %#v", persistedDeleting)
	}
}

// capability_id: rainbond.k8s-resource.create-delete-reservation
func TestK8sResourceCreateReservationBlocksDeletingRecords(t *testing.T) {
	db := newK8sResourceDeleteTestDB(t)
	defer db.Close()

	dao := &K8sResourceDaoImpl{DB: db}
	active := &dbmodel.K8sResource{AppID: "app-1", Name: "active", Kind: "ConfigMap", State: model.CreateSuccess}
	deleting := &dbmodel.K8sResource{AppID: "app-1", Name: "deleting", Kind: "Secret", State: model.CreateSuccess, DeleteStatus: dbmodel.K8sResourceDeleteStatusDeleting}
	failed := &dbmodel.K8sResource{AppID: "app-1", Name: "failed", Kind: "Service", State: model.CreateSuccess, DeleteStatus: dbmodel.K8sResourceDeleteStatusFailed}
	createK8sResourceForDeleteTest(t, dao, active)
	createK8sResourceForDeleteTest(t, dao, deleting)
	createK8sResourceForDeleteTest(t, dao, failed)

	if err := dao.EnsureK8sResourcesCreatable("app-1", []dbmodel.K8sResourceDeleteTarget{{Name: active.Name, Kind: active.Kind}, {Name: "new", Kind: "ConfigMap"}}); err != nil {
		t.Fatalf("active or new resources must be creatable: %v", err)
	}
	for _, target := range []dbmodel.K8sResourceDeleteTarget{{Name: deleting.Name, Kind: deleting.Kind}, {Name: failed.Name, Kind: failed.Kind}} {
		err := dao.EnsureK8sResourcesCreatable("app-1", []dbmodel.K8sResourceDeleteTarget{target})
		if err == nil {
			t.Fatalf("expected cleanup reservation to block %s/%s", target.Kind, target.Name)
		}
		var blocked *dbmodel.K8sResourceCreateBlockedError
		if !errors.As(err, &blocked) || blocked.AppID != "app-1" || blocked.Name != target.Name || blocked.Kind != target.Kind {
			t.Fatalf("expected precise cleanup reservation error, got %v", err)
		}
	}
}

// capability_id: rainbond.k8s-resource.persist-delete-lifecycle
func TestK8sResourceDeleteMixedTargetsAreAcceptedAtomically(t *testing.T) {
	db := newK8sResourceDeleteTestDB(t)
	defer db.Close()

	dao := &K8sResourceDaoImpl{DB: db}
	exact := &dbmodel.K8sResource{AppID: "app-1", Name: "exact", Kind: "ConfigMap", State: model.CreateSuccess}
	legacyUnique := &dbmodel.K8sResource{AppID: "app-1", Name: "legacy-unique", Kind: "Secret", State: model.CreateSuccess}
	legacyAmbiguousOne := &dbmodel.K8sResource{AppID: "app-1", Name: "legacy-ambiguous", Kind: "Service", State: model.CreateSuccess}
	legacyAmbiguousTwo := &dbmodel.K8sResource{AppID: "app-1", Name: "legacy-ambiguous", Kind: "Service", State: model.CreateSuccess}
	createK8sResourceForDeleteTest(t, dao, exact)
	createK8sResourceForDeleteTest(t, dao, legacyUnique)
	createK8sResourceForDeleteTest(t, dao, legacyAmbiguousOne)
	createK8sResourceForDeleteTest(t, dao, legacyAmbiguousTwo)

	_, err := dao.MarkK8sResourcesDeleting("app-1", []dbmodel.K8sResourceDeleteTarget{
		{ResourceID: exact.ID},
		{Name: legacyUnique.Name, Kind: legacyUnique.Kind},
		{Name: legacyAmbiguousOne.Name, Kind: legacyAmbiguousOne.Kind},
	})
	var ambiguity *dbmodel.K8sResourceDeleteIdentityAmbiguousError
	if !errors.As(err, &ambiguity) {
		t.Fatalf("mixed batch must surface legacy identity ambiguity, got %v", err)
	}

	for _, resourceID := range []uint{exact.ID, legacyUnique.ID} {
		var persisted dbmodel.K8sResource
		if err := db.Where("ID = ?", resourceID).First(&persisted).Error; err != nil {
			t.Fatalf("get mixed batch resource %d: %v", resourceID, err)
		}
		if persisted.DeleteStatus != dbmodel.K8sResourceDeleteStatusActive || persisted.DeleteGeneration != 0 {
			t.Fatalf("ambiguous mixed batch must roll back all acceptance, got %#v", persisted)
		}
	}

	accepted, err := dao.MarkK8sResourcesDeleting("app-1", []dbmodel.K8sResourceDeleteTarget{
		{ResourceID: exact.ID},
		{Name: legacyUnique.Name, Kind: legacyUnique.Kind},
	})
	if err != nil || len(accepted) != 2 {
		t.Fatalf("mixed exact and unique legacy targets must accept atomically: accepted=%#v err=%v", accepted, err)
	}
	acceptedIDs := map[uint]bool{}
	for _, resource := range accepted {
		acceptedIDs[resource.ID] = resource.DeleteGeneration == 1
	}
	if !acceptedIDs[exact.ID] || !acceptedIDs[legacyUnique.ID] {
		t.Fatalf("mixed target acceptance must return actual Region IDs and generations, got %#v", accepted)
	}
}

// capability_id: rainbond.k8s-resource.persist-delete-lifecycle
func TestK8sResourceDeleteAcceptReturnsIdempotentReceiptAndRejectsInvalidIDs(t *testing.T) {
	db := newK8sResourceDeleteTestDB(t)
	defer db.Close()

	dao := &K8sResourceDaoImpl{DB: db}
	active := &dbmodel.K8sResource{AppID: "app-1", Name: "active", Kind: "ConfigMap", State: model.CreateSuccess}
	alreadyDeleting := &dbmodel.K8sResource{
		AppID:            "app-1",
		Name:             "already-deleting",
		Kind:             "Secret",
		State:            model.CreateSuccess,
		DeleteStatus:     dbmodel.K8sResourceDeleteStatusDeleting,
		DeleteGeneration: 4,
	}
	otherApp := &dbmodel.K8sResource{AppID: "app-2", Name: "other-app", Kind: "ConfigMap", State: model.CreateSuccess}
	createK8sResourceForDeleteTest(t, dao, active)
	createK8sResourceForDeleteTest(t, dao, alreadyDeleting)
	createK8sResourceForDeleteTest(t, dao, otherApp)

	resolved, newlyAccepted, err := dao.AcceptK8sResourceDeletions("app-1", []dbmodel.K8sResourceDeleteTarget{
		{ResourceID: active.ID},
		{ResourceID: otherApp.ID},
	})
	var missing *dbmodel.K8sResourceDeleteTargetNotFoundError
	if !errors.As(err, &missing) || len(resolved) != 0 || len(newlyAccepted) != 0 {
		t.Fatalf("out-of-app Region ID must reject the whole batch: resolved=%#v newly=%#v err=%v", resolved, newlyAccepted, err)
	}
	if missing.AppID != "app-1" || missing.Target.ResourceID != otherApp.ID {
		t.Fatalf("missing target error must expose the requested Region ID without resolving another app, got %#v", missing)
	}
	var rejectedActive dbmodel.K8sResource
	if err := db.Where("ID = ?", active.ID).First(&rejectedActive).Error; err != nil {
		t.Fatalf("get active resource after rejected batch: %v", err)
	}
	if rejectedActive.DeleteGeneration != 0 || rejectedActive.DeleteStatus != dbmodel.K8sResourceDeleteStatusActive {
		t.Fatalf("rejected batch must not accept valid rows, got %#v", rejectedActive)
	}

	resolved, newlyAccepted, err = dao.AcceptK8sResourceDeletions("app-1", []dbmodel.K8sResourceDeleteTarget{
		{ResourceID: active.ID},
		{ResourceID: alreadyDeleting.ID},
	})
	if err != nil || len(resolved) != 2 || len(newlyAccepted) != 1 {
		t.Fatalf("accept mixed active/deleting targets: resolved=%#v newly=%#v err=%v", resolved, newlyAccepted, err)
	}
	if newlyAccepted[0].ID != active.ID || newlyAccepted[0].DeleteGeneration != 1 {
		t.Fatalf("active target must be newly accepted once, got %#v", newlyAccepted)
	}
	resolvedByID := make(map[uint]dbmodel.K8sResource, len(resolved))
	for _, resource := range resolved {
		resolvedByID[resource.ID] = resource
	}
	if resolvedByID[active.ID].DeleteStatus != dbmodel.K8sResourceDeleteStatusDeleting || resolvedByID[active.ID].DeleteGeneration != 1 {
		t.Fatalf("resolved active target must reflect accepted lifecycle, got %#v", resolvedByID[active.ID])
	}
	if resolvedByID[alreadyDeleting.ID].DeleteStatus != dbmodel.K8sResourceDeleteStatusDeleting || resolvedByID[alreadyDeleting.ID].DeleteGeneration != 4 {
		t.Fatalf("existing deleting target must be returned without a new generation, got %#v", resolvedByID[alreadyDeleting.ID])
	}

}

// capability_id: rainbond.k8s-resource.cross-app-physical-reservation
func TestK8sResourceCreateReservationBlocksCrossAppPhysicalIdentity(t *testing.T) {
	db := newK8sResourceDeleteTestDB(t)
	defer db.Close()

	dao := &K8sResourceDaoImpl{DB: db}
	physicalIdentity := dbmodel.K8sResourcePhysicalIdentity("apps", "deployments", "tenant-a", "shared")
	deleting := &dbmodel.K8sResource{
		AppID:            "old-app",
		Name:             "shared",
		Kind:             "Deployment",
		ResourceIdentity: physicalIdentity,
		State:            model.CreateSuccess,
		DeleteStatus:     dbmodel.K8sResourceDeleteStatusDeleting,
	}
	createK8sResourceForDeleteTest(t, dao, deleting)

	err := dao.EnsureK8sResourcesCreatable("new-app", []dbmodel.K8sResourceDeleteTarget{{
		Name:             "shared",
		Kind:             "Deployment",
		ResourceIdentity: physicalIdentity,
	}})
	if err == nil {
		t.Fatal("a cleanup reservation from another app must block reinstall of the exact Kubernetes object")
	}
	var blocked *dbmodel.K8sResourceCreateBlockedError
	if !errors.As(err, &blocked) || blocked.AppID != "old-app" {
		t.Fatalf("expected the original app cleanup reservation, got %v", err)
	}

	if err := dao.EnsureK8sResourcesCreatable("new-app", []dbmodel.K8sResourceDeleteTarget{{
		Name:             "shared",
		Kind:             "Deployment",
		ResourceIdentity: dbmodel.K8sResourcePhysicalIdentity("apps", "deployments", "tenant-b", "shared"),
	}}); err != nil {
		t.Fatalf("a different physical Kubernetes identity must remain creatable: %v", err)
	}
}

// capability_id: rainbond.k8s-resource.cross-app-physical-reservation
func TestK8sResourcePhysicalIdentityIsFixedWidthAndNamespaceScoped(t *testing.T) {
	name := strings.Repeat("a", 253)
	identity := dbmodel.K8sResourcePhysicalIdentity(strings.Repeat("g", 253), strings.Repeat("r", 253), "tenant-a", name)
	if len(identity) != 64 {
		t.Fatalf("physical identity must be a fixed-width MySQL-safe SHA-256 key, got %d characters", len(identity))
	}
	if identity == dbmodel.K8sResourcePhysicalIdentity(strings.Repeat("g", 253), strings.Repeat("r", 253), "tenant-b", name) {
		t.Fatal("different namespaces must not share a physical resource identity")
	}
}

// capability_id: rainbond.k8s-resource.uid-adoption-lease-guard
func TestK8sResourceDeleteAdoptsObservedUIDOnlyForOwnedLease(t *testing.T) {
	db := newK8sResourceDeleteTestDB(t)
	defer db.Close()

	dao := &K8sResourceDaoImpl{DB: db}
	resource := &dbmodel.K8sResource{
		AppID:            "app-1",
		Name:             "legacy-config",
		Kind:             "ConfigMap",
		State:            model.CreateSuccess,
		DeleteStatus:     dbmodel.K8sResourceDeleteStatusDeleting,
		DeleteGeneration: 2,
	}
	createK8sResourceForDeleteTest(t, dao, resource)
	lease := claimK8sResourceDeleteForTest(t, dao, *resource)
	identity := dbmodel.K8sResourcePhysicalIdentity("", "configmaps", "tenant-a", "legacy-config")

	updated, err := dao.AdoptK8sResourceDeleteIdentity(resource.ID, resource.DeleteGeneration, lease.DeleteLeaseToken, "observed-uid", identity)
	if err != nil || !updated {
		t.Fatalf("the active deletion lease must adopt the observed object identity: updated=%v err=%v", updated, err)
	}
	updated, err = dao.AdoptK8sResourceDeleteIdentity(resource.ID, resource.DeleteGeneration, "other-lease", "other-uid", identity)
	if err != nil || updated {
		t.Fatalf("a stale lease must not overwrite the adopted UID: updated=%v err=%v", updated, err)
	}
	var persisted dbmodel.K8sResource
	if err := db.Where("ID = ?", resource.ID).First(&persisted).Error; err != nil {
		t.Fatalf("load adopted resource: %v", err)
	}
	if persisted.ResourceUID != "observed-uid" || persisted.ResourceIdentity != identity {
		t.Fatalf("expected the observed Kubernetes identity to persist, got %#v", persisted)
	}
}

// capability_id: rainbond.k8s-resource.batched-delete-acceptance
func TestK8sResourceDeleteAcceptsLargeMixedBatchAtomically(t *testing.T) {
	db := newK8sResourceDeleteTestDB(t)
	defer db.Close()

	dao := &K8sResourceDaoImpl{DB: db}
	targets := make([]dbmodel.K8sResourceDeleteTarget, 0, 48)
	for index := 0; index < 48; index++ {
		resource := &dbmodel.K8sResource{AppID: "app-1", Name: fmt.Sprintf("resource-%d", index), Kind: "ConfigMap", State: model.CreateSuccess}
		createK8sResourceForDeleteTest(t, dao, resource)
		if index%2 == 0 {
			targets = append(targets, dbmodel.K8sResourceDeleteTarget{ResourceID: resource.ID})
		} else {
			targets = append(targets, dbmodel.K8sResourceDeleteTarget{Name: resource.Name, Kind: resource.Kind})
		}
	}
	resolved, accepted, err := dao.AcceptK8sResourceDeletions("app-1", targets)
	if err != nil || len(resolved) != len(targets) || len(accepted) != len(targets) {
		t.Fatalf("the complete mixed batch must accept in one lifecycle transition: resolved=%d accepted=%d err=%v", len(resolved), len(accepted), err)
	}
	for _, resource := range accepted {
		if resource.DeleteGeneration != 1 || resource.DeleteStatus != dbmodel.K8sResourceDeleteStatusDeleting {
			t.Fatalf("every accepted resource must receive its generation in the batch transition: %#v", resource)
		}
	}
}
