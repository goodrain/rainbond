package cleanup

import (
	"sort"
	"strings"

	"github.com/docker/distribution/reference"
	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
)

// ScheduledVersionRetirementPending marks a retired manifest awaiting deletion.
const ScheduledVersionRetirementPending = "retired-pending"

// ScheduledVersionRetirement is an immutable saved build record selected for
// the same manifest. Every record is re-read under the original deletion grant.
type ScheduledVersionRetirement struct {
	TenantID string
	Expected VersionExpectation
}

// RetireScheduledVersions atomically retires an entire selected manifest's old
// records only if the remaining references are complete and unreferenced. The
// existing operation stores a durable candidate before any DELETE is sent.
func RetireScheduledVersions(database *gorm.DB, request CoordinationRequest, selected []ScheduledVersionRetirement, keepCount uint, tags []string, extra ...RegionReferenceInventory) error {
	if database == nil || !request.valid() || request.Kind != "delete" || !registryDeletionDigest.MatchString(request.Target) || len(selected) == 0 || len(selected) > 1000 || keepCount < 1 || keepCount > 10000 || len(tags) > 256 || len(extra) == 0 {
		return ErrCoordinationChanged
	}
	aliases := map[string]bool{}
	for _, tag := range tags {
		if !referenceAuditTag.MatchString(tag) {
			return ErrCoordinationChanged
		}
		aliases[tag] = true
	}
	ordered := append([]ScheduledVersionRetirement(nil), selected...)
	sort.Slice(ordered, func(i, j int) bool {
		a, b := ordered[i].Expected, ordered[j].Expected
		return a.ServiceID+"\x00"+a.Version < b.ServiceID+"\x00"+b.Version
	})
	tx := database.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer tx.Rollback()
	store, err := lockCleanupStorage(tx, request)
	if err != nil {
		return err
	}
	if store.Mode != "ready" && store.Mode != "draining" {
		return ErrCoordinationBusy
	}
	var operation model.CleanupOperation
	if err := tx.Where("operation_id = ?", request.OperationID).First(&operation).Error; err != nil {
		return err
	}
	if !request.matches(operation) || operation.State != "active" || operation.Outcome != "" || operation.NodeExecutionJSON != "" {
		return ErrCoordinationChanged
	}
	seen := map[string]bool{}
	for _, candidate := range ordered {
		expected := candidate.Expected
		key := expected.ServiceID + "\x00" + expected.Version
		if seen[key] || candidate.TenantID == "" {
			return ErrCoordinationChanged
		}
		seen[key] = true
		named, err := reference.ParseNormalizedNamed(expected.Image)
		if err != nil || reference.Path(named) != request.Scope {
			return ErrCoordinationChanged
		}
		tagged, ok := named.(reference.Tagged)
		if !ok || !aliases[tagged.Tag()] {
			return ErrCoordinationChanged
		}
		var service serviceRow
		if err := versionStoreQuery(tx).Table("tenant_services").Where("service_id = ? AND tenant_id = ?", expected.ServiceID, candidate.TenantID).First(&service).Error; err != nil {
			return err
		}
		var version versionRow
		if err := versionStoreQuery(tx).Table("tenant_service_version").Where("service_id = ? AND build_version = ?", expected.ServiceID, expected.Version).First(&version).Error; err != nil {
			return err
		}
		if version.FinalStatus != "success" {
			return ErrVersionProtected
		}
		var newer uint
		if err := tx.Model(&model.VersionInfo{}).Where("service_id = ? AND final_status = ? AND ID > ?", expected.ServiceID, "success", version.ID).Count(&newer).Error; err != nil {
			return err
		}
		if newer < keepCount {
			return ErrVersionProtected
		}
		result, err := retireVersionInTransaction(tx, candidate.TenantID, expected)
		if err != nil {
			return err
		}
		if !result.RecordRetired {
			return ErrStateChanged
		}
	}
	inventory, err := collectRegionReferenceImages(tx)
	if err != nil {
		return err
	}
	inventory = MergeReferenceInventories(append([]RegionReferenceInventory{inventory}, extra...)...)
	referenced, err := scheduledManifestReferenced(inventory, request, aliases)
	if err != nil {
		return err
	}
	if referenced {
		return ErrVersionProtected
	}
	updated := tx.Model(&model.CleanupOperation{}).Where("operation_id = ? AND state = ? AND outcome = ?", request.OperationID, "active", "").UpdateColumn("outcome", ScheduledVersionRetirementPending)
	if updated.Error != nil {
		return updated.Error
	}
	if updated.RowsAffected != 1 {
		return ErrCoordinationChanged
	}
	if err := advanceCleanupRevision(tx, store); err != nil {
		return err
	}
	return tx.Commit().Error
}

func scheduledManifestReferenced(inventory RegionReferenceInventory, request CoordinationRequest, aliases map[string]bool) (bool, error) {
	if !inventory.Complete {
		return false, ErrCoordinationUnavailable
	}
	for _, image := range inventory.Images {
		named, err := reference.ParseNormalizedNamed(image)
		if err != nil {
			return false, ErrCoordinationChanged
		}
		if digest, ok := named.(reference.Digested); ok {
			if digest.Digest().String() == request.Target {
				return true, nil
			}
			continue
		}
		tagged, ok := reference.TagNameOnly(named).(reference.Tagged)
		if !ok {
			return false, ErrCoordinationChanged
		}
		repository := reference.Path(named)
		if (repository == request.Scope || strings.TrimPrefix(repository, "library/") == request.Scope) && aliases[tagged.Tag()] {
			return true, nil
		}
	}
	return false, nil
}

// RecoverUnstartedScheduledVersionDeletion revokes only an unconsumed deletion
// grant whose version retirement already committed. BeginDeletionAttempt takes
// this same storage lock: either it consumes the grant, or this recovery makes
// every old permit permanently unable to forward a DELETE.
func RecoverUnstartedScheduledVersionDeletion(database *gorm.DB, request CoordinationRequest) error {
	if database == nil || !request.valid() || request.Kind != "delete" || (request.Owner != "chaos-scheduled-version-cleanup" && request.Owner != "chaos-scheduled-version-retry") || !registryDeletionDigest.MatchString(request.Target) {
		return ErrCoordinationChanged
	}
	tx := database.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer tx.Rollback()
	store, err := lockCleanupStorage(tx, request)
	if err != nil {
		return err
	}
	var operation model.CleanupOperation
	if err := tx.Where("operation_id = ?", request.OperationID).First(&operation).Error; err != nil {
		return err
	}
	if !request.matches(operation) || operation.NodeExecutionJSON != "" || operation.ExternalKey != nil || operation.ParentOperationID != "" {
		return ErrCoordinationChanged
	}
	if operation.State != "active" && operation.State != "finished" {
		return ErrCoordinationUncertain
	}
	expectedOutcome := ScheduledVersionRetirementPending
	if request.Owner == "chaos-scheduled-version-retry" {
		expectedOutcome = ""
		if operation.State == "active" {
			var candidates int
			if err := tx.Model(&model.CleanupOperation{}).Where("storage_id = ? AND generation = ? AND owner = ? AND kind = ? AND scope = ? AND target = ? AND state = ? AND outcome IN (?)", request.StorageID, request.Generation, "chaos-scheduled-version-cleanup", "delete", request.Scope, request.Target, "finished", []string{ScheduledVersionRetirementPending, "rejected"}).Count(&candidates).Error; err != nil {
				return err
			}
			if candidates == 0 {
				return ErrCoordinationChanged
			}
		}
	}
	if operation.Outcome != expectedOutcome {
		return ErrCoordinationChanged
	}
	if operation.State == "finished" {
		return tx.Commit().Error
	}
	updated := tx.Model(&model.CleanupOperation{}).Where("operation_id = ? AND state = ? AND outcome = ?", request.OperationID, "active", expectedOutcome).UpdateColumn("state", "finished")
	if updated.Error != nil {
		return updated.Error
	}
	if updated.RowsAffected != 1 {
		return ErrCoordinationChanged
	}
	if err := advanceCleanupRevision(tx, store); err != nil {
		return err
	}
	return tx.Commit().Error
}
