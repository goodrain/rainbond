// RAINBOND, Application Management Platform
// Copyright (C) 2022-2022 Goodrain Co., Ltd.

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version. For any non-GPL usage of Rainbond,
// one or multiple Commercial Licenses authorized by Goodrain Co., Ltd.
// must be obtained first.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package dao

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	gormbulkups "github.com/atcdot/gorm-bulk-upsert"
	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
	pkgerr "github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// K8sResourceDaoImpl k8s resource dao
type K8sResourceDaoImpl struct {
	DB *gorm.DB
}

// AddModel add model
func (t *K8sResourceDaoImpl) AddModel(mo model.Interface) error {
	resource, ok := mo.(*model.K8sResource)
	if !ok {
		return fmt.Errorf("mo.(*model.K8sResource) err")
	}
	return t.DB.Transaction(func(tx *gorm.DB) error {
		if resource.ResourceIdentity != "" {
			if err := ensureK8sResourceCreationAllowed(tx, resource.AppID, []model.K8sResourceDeleteTarget{{
				Name:             resource.Name,
				Kind:             resource.Kind,
				ResourceIdentity: resource.ResourceIdentity,
			}}, 0); err != nil {
				return err
			}
		}
		return tx.Create(resource).Error
	})
}

// UpdateModel update model
func (t *K8sResourceDaoImpl) UpdateModel(mo model.Interface) error {
	resource, ok := mo.(*model.K8sResource)
	if !ok {
		return fmt.Errorf("mo.(*model.K8sResource) err")
	}
	return t.DB.Transaction(func(tx *gorm.DB) error {
		if resource.ResourceIdentity != "" {
			if err := ensureK8sResourceCreationAllowed(tx, resource.AppID, []model.K8sResourceDeleteTarget{{
				Name:             resource.Name,
				Kind:             resource.Kind,
				ResourceIdentity: resource.ResourceIdentity,
			}}, resource.ID); err != nil {
				return err
			}
		}
		result := tx.Model(&model.K8sResource{}).
			Where("ID = ? AND delete_status = ?", resource.ID, model.K8sResourceDeleteStatusActive).
			Updates(map[string]interface{}{
				"content":                 resource.Content,
				"resource_uid":            resource.ResourceUID,
				"resource_identity":       resource.ResourceIdentity,
				"resource_origin":         resource.ResourceOrigin,
				"force_finalizer_allowed": resource.ForceFinalizerAllowed,
				"status":                  resource.ErrorOverview,
				"success":                 resource.State,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected > 0 {
			return nil
		}

		var existing model.K8sResource
		if err := tx.Where("ID = ?", resource.ID).First(&existing).Error; err != nil {
			return err
		}
		if existing.DeleteStatus != model.K8sResourceDeleteStatusActive {
			return fmt.Errorf("k8s resource %d cannot be updated while deletion is in progress", resource.ID)
		}
		return nil
	})
}

// ListByAppID list by app id
func (t *K8sResourceDaoImpl) ListByAppID(appID string) ([]model.K8sResource, error) {
	var resources []model.K8sResource
	if err := t.DB.Where("app_id = ?", appID).Find(&resources).Error; err != nil {
		return nil, err
	}
	return resources, nil
}

// GetK8sResourcesByNames is a controller compatibility helper that selects saved resources by app, name and kind.
// Name and Kind are not a deletion task identity; workers must use the Region record ID, DeleteGeneration and lease token instead.
func (t *K8sResourceDaoImpl) GetK8sResourcesByNames(appID string, targets []model.K8sResourceDeleteTarget) ([]model.K8sResource, error) {
	var result []model.K8sResource
	if err := controllerK8sResourceQuery(t.DB, appID, targets).Find(&result).Error; err != nil {
		return nil, err
	}
	return result, nil
}

// ListK8sResourcesDeleteStatus returns only deletion lifecycle fields for controller status reconciliation.
// A physically deleted resource is intentionally absent; callers own the missing-resource semantics.
func (t *K8sResourceDaoImpl) ListK8sResourcesDeleteStatus(appID string, targets []model.K8sResourceDeleteTarget) ([]model.K8sResource, error) {
	var result []model.K8sResource
	if err := k8sResourceDeleteStatusQuery(t.DB, appID, targets).
		Select("ID, name, kind, delete_status, delete_error, delete_attempts, delete_started_at, delete_generation").
		Find(&result).Error; err != nil {
		return nil, err
	}
	return result, nil
}

// EnsureK8sResourcesCreatable conditionally checks create/sync identities in a
// locked transaction. A DELETING or DELETE_FAILED record is a durable cleanup
// reservation and must never be bypassed by a reinstall, including a reinstall
// into another app. If a delete begins immediately after this check, the
// persisted Kubernetes UID still protects the new object from a stale worker.
func (t *K8sResourceDaoImpl) EnsureK8sResourcesCreatable(appID string, targets []model.K8sResourceDeleteTarget) error {
	if len(targets) == 0 {
		return nil
	}
	return t.DB.Transaction(func(tx *gorm.DB) error {
		return ensureK8sResourceCreationAllowed(tx, appID, targets, 0)
	})
}

// ensureK8sResourceCreationAllowed locks the exact physical identities before
// accepting a create/update write. Only non-active rows block: unrelated
// ACTIVE records retain the historical behavior.
func ensureK8sResourceCreationAllowed(tx *gorm.DB, appID string, targets []model.K8sResourceDeleteTarget, ignoreResourceID uint) error {
	identities := uniqueK8sResourcePhysicalIdentities(targets)
	legacyTargets := make([]model.K8sResourceDeleteTarget, 0, len(targets))
	seenLegacy := make(map[string]struct{}, len(targets))
	for _, target := range targets {
		if target.ResourceIdentity != "" || target.Name == "" || target.Kind == "" {
			continue
		}
		key := k8sResourceLegacyTargetKey(target.Name, target.Kind)
		if _, exists := seenLegacy[key]; exists {
			continue
		}
		seenLegacy[key] = struct{}{}
		legacyTargets = append(legacyTargets, target)
	}
	if len(identities) == 0 && len(legacyTargets) == 0 {
		return nil
	}
	clauses := make([]string, 0, len(legacyTargets)+1)
	args := make([]interface{}, 0, len(identities)+len(legacyTargets)*3)
	if len(identities) > 0 {
		clauses = append(clauses, "resource_identity IN (?)")
		args = append(args, identities)
	}
	for _, target := range legacyTargets {
		clauses = append(clauses, "(app_id = ? AND name = ? AND kind = ?)")
		args = append(args, appID, target.Name, target.Kind)
	}
	query := lockK8sResourceRows(tx).
		Select("ID, app_id, name, kind, delete_status").
		Where("("+strings.Join(clauses, " OR ")+") AND delete_status IN (?)", append(args, []int{model.K8sResourceDeleteStatusDeleting, model.K8sResourceDeleteStatusFailed})...)
	if ignoreResourceID > 0 {
		query = query.Where("ID <> ?", ignoreResourceID)
	}
	var resources []model.K8sResource
	if err := query.Find(&resources).Error; err != nil {
		return err
	}
	if len(resources) == 0 {
		return nil
	}
	resource := resources[0]
	return &model.K8sResourceCreateBlockedError{
		AppID:        resource.AppID,
		Name:         resource.Name,
		Kind:         resource.Kind,
		DeleteStatus: resource.DeleteStatus,
	}
}

func uniqueK8sResourcePhysicalIdentities(targets []model.K8sResourceDeleteTarget) []string {
	identities := make([]string, 0, len(targets))
	seen := make(map[string]struct{}, len(targets))
	for _, target := range targets {
		if target.ResourceIdentity == "" {
			continue
		}
		if _, exists := seen[target.ResourceIdentity]; exists {
			continue
		}
		seen[target.ResourceIdentity] = struct{}{}
		identities = append(identities, target.ResourceIdentity)
	}
	return identities
}

// MarkK8sResourcesDeletingByIDs accepts only Region record IDs that belong to appID.
// The returned records are the only resources accepted by this call; existing DELETING rows are not returned.
func (t *K8sResourceDaoImpl) MarkK8sResourcesDeletingByIDs(appID string, resourceIDs []uint) ([]model.K8sResource, error) {
	targets := make([]model.K8sResourceDeleteTarget, 0, len(resourceIDs))
	for _, resourceID := range resourceIDs {
		targets = append(targets, model.K8sResourceDeleteTarget{ResourceID: resourceID})
	}
	return t.MarkK8sResourcesDeleting(appID, targets)
}

// MarkK8sResourcesDeletingByNames is the legacy name/kind compatibility path.
// Every target must resolve to at most one saved Region record before any row is marked deleting.
func (t *K8sResourceDaoImpl) MarkK8sResourcesDeletingByNames(appID string, targets []model.K8sResourceDeleteTarget) ([]model.K8sResource, error) {
	legacyTargets := make([]model.K8sResourceDeleteTarget, 0, len(targets))
	for _, target := range targets {
		legacyTargets = append(legacyTargets, model.K8sResourceDeleteTarget{Name: target.Name, Kind: target.Kind})
	}
	return t.MarkK8sResourcesDeleting(appID, legacyTargets)
}

// MarkK8sResourcesDeleting accepts a mixed batch in one transaction. Region record IDs are preferred;
// targets without ResourceID use the legacy name/kind path and must resolve uniquely before any row is marked.
func (t *K8sResourceDaoImpl) MarkK8sResourcesDeleting(appID string, targets []model.K8sResourceDeleteTarget) ([]model.K8sResource, error) {
	_, newlyAccepted, err := t.AcceptK8sResourceDeletions(appID, targets)
	if err != nil {
		return nil, err
	}
	return newlyAccepted, nil
}

// AcceptK8sResourceDeletions resolves every target and accepts ACTIVE or DELETE_FAILED records in one transaction.
// Existing DELETING records are returned as idempotent receipts without starting another generation.
func (t *K8sResourceDaoImpl) AcceptK8sResourceDeletions(appID string, targets []model.K8sResourceDeleteTarget) (resolved []model.K8sResource, newlyAccepted []model.K8sResource, err error) {
	if len(targets) == 0 {
		return []model.K8sResource{}, []model.K8sResource{}, nil
	}

	resolved = make([]model.K8sResource, 0, len(targets))
	newlyAccepted = make([]model.K8sResource, 0, len(targets))
	err = t.DB.Transaction(func(tx *gorm.DB) error {
		resourceIDs, legacyTargets := splitK8sResourceDeleteTargets(targets)
		whereClause, whereArgs := k8sResourceDeleteTargetWhereClause(resourceIDs, legacyTargets)
		var locked []model.K8sResource
		query := lockK8sResourceRows(tx).Where("app_id = ?", appID).Where(whereClause, whereArgs...).Order("ID ASC")
		if err := query.Find(&locked).Error; err != nil {
			return err
		}
		resolvedByID := make(map[uint]model.K8sResource, len(locked))
		matchesByLegacyKey := make(map[string][]model.K8sResource, len(legacyTargets))
		for _, match := range locked {
			resolvedByID[match.ID] = match
			matchesByLegacyKey[k8sResourceLegacyTargetKey(match.Name, match.Kind)] = append(matchesByLegacyKey[k8sResourceLegacyTargetKey(match.Name, match.Kind)], match)
		}
		resolvedIDs := make([]uint, 0, len(targets))
		seenResolvedIDs := make(map[uint]struct{}, len(targets))
		for _, resourceID := range resourceIDs {
			if _, found := resolvedByID[resourceID]; !found {
				return &model.K8sResourceDeleteTargetNotFoundError{AppID: appID, Target: model.K8sResourceDeleteTarget{ResourceID: resourceID}}
			}
			if _, exists := seenResolvedIDs[resourceID]; !exists {
				seenResolvedIDs[resourceID] = struct{}{}
				resolvedIDs = append(resolvedIDs, resourceID)
			}
		}
		for _, target := range legacyTargets {
			matches := matchesByLegacyKey[k8sResourceLegacyTargetKey(target.Name, target.Kind)]
			if len(matches) == 0 {
				return &model.K8sResourceDeleteTargetNotFoundError{AppID: appID, Target: target}
			}
			if len(matches) > 1 {
				matchIDs := make([]uint, 0, len(matches))
				for _, match := range matches {
					matchIDs = append(matchIDs, match.ID)
				}
				return &model.K8sResourceDeleteIdentityAmbiguousError{AppID: appID, Name: target.Name, Kind: target.Kind, ResourceIDs: matchIDs}
			}
			resourceID := matches[0].ID
			if _, exists := seenResolvedIDs[resourceID]; !exists {
				seenResolvedIDs[resourceID] = struct{}{}
				resolvedIDs = append(resolvedIDs, resourceID)
			}
		}

		candidates := make([]model.K8sResource, 0, len(resolvedIDs))
		for _, resourceID := range resolvedIDs {
			candidates = append(candidates, resolvedByID[resourceID])
		}
		newlyAccepted, err = markK8sResourcesDeleting(tx, appID, candidates)
		if err != nil {
			return err
		}
		for _, accepted := range newlyAccepted {
			resolvedByID[accepted.ID] = accepted
		}
		resolved = resolved[:0]
		for _, resourceID := range resolvedIDs {
			resolved = append(resolved, resolvedByID[resourceID])
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return resolved, newlyAccepted, nil
}

func splitK8sResourceDeleteTargets(targets []model.K8sResourceDeleteTarget) ([]uint, []model.K8sResourceDeleteTarget) {
	resourceIDs := make([]uint, 0, len(targets))
	legacyTargets := make([]model.K8sResourceDeleteTarget, 0, len(targets))
	seenIDs := make(map[uint]struct{}, len(targets))
	seenLegacy := make(map[string]struct{}, len(targets))
	for _, target := range targets {
		if target.ResourceID > 0 {
			if _, exists := seenIDs[target.ResourceID]; !exists {
				seenIDs[target.ResourceID] = struct{}{}
				resourceIDs = append(resourceIDs, target.ResourceID)
			}
			continue
		}
		key := k8sResourceLegacyTargetKey(target.Name, target.Kind)
		if _, exists := seenLegacy[key]; !exists {
			seenLegacy[key] = struct{}{}
			legacyTargets = append(legacyTargets, target)
		}
	}
	return resourceIDs, legacyTargets
}

func k8sResourceLegacyTargetKey(name, kind string) string {
	return name + "\x00" + kind
}

func k8sResourceDeleteTargetWhereClause(resourceIDs []uint, legacyTargets []model.K8sResourceDeleteTarget) (string, []interface{}) {
	clauses := make([]string, 0, len(legacyTargets)+1)
	args := make([]interface{}, 0, len(resourceIDs)+len(legacyTargets)*2)
	if len(resourceIDs) > 0 {
		clauses = append(clauses, "ID IN (?)")
		args = append(args, resourceIDs)
	}
	for _, target := range legacyTargets {
		clauses = append(clauses, "(name = ? AND kind = ?)")
		args = append(args, target.Name, target.Kind)
	}
	return "(" + strings.Join(clauses, " OR ") + ")", args
}

func markK8sResourcesDeleting(tx *gorm.DB, appID string, candidates []model.K8sResource) ([]model.K8sResource, error) {
	accepted := make([]model.K8sResource, 0, len(candidates))
	startedAt := time.Now()
	acceptedIDs := make([]uint, 0, len(candidates))
	caseStatement := strings.Builder{}
	caseStatement.WriteString("CASE ID")
	caseArgs := make([]interface{}, 0, len(candidates)*2)
	for _, candidate := range candidates {
		if candidate.DeleteStatus != model.K8sResourceDeleteStatusActive && candidate.DeleteStatus != model.K8sResourceDeleteStatusFailed {
			continue
		}
		deleteGeneration := candidate.DeleteGeneration + 1
		acceptedIDs = append(acceptedIDs, candidate.ID)
		caseStatement.WriteString(" WHEN ? THEN ?")
		caseArgs = append(caseArgs, candidate.ID, deleteGeneration)
		candidate.DeleteStatus = model.K8sResourceDeleteStatusDeleting
		candidate.DeleteError = ""
		candidate.DeleteAttempts = 0
		candidate.DeleteStartedAt = &startedAt
		candidate.DeleteGeneration = deleteGeneration
		candidate.DeleteNextRetryAt = &startedAt
		candidate.DeleteLeaseUntil = nil
		candidate.DeleteLeaseToken = ""
		accepted = append(accepted, candidate)
	}
	if len(accepted) == 0 {
		return accepted, nil
	}
	caseStatement.WriteString(" END")
	result := tx.Model(&model.K8sResource{}).
		Where("ID IN (?) AND app_id = ? AND delete_status IN (?)", acceptedIDs, appID, []int{model.K8sResourceDeleteStatusActive, model.K8sResourceDeleteStatusFailed}).
		Updates(map[string]interface{}{
			"delete_status":        model.K8sResourceDeleteStatusDeleting,
			"delete_error":         "",
			"delete_attempts":      0,
			"delete_started_at":    startedAt,
			"delete_generation":    gorm.Expr(caseStatement.String(), caseArgs...),
			"delete_next_retry_at": startedAt,
			"delete_lease_until":   nil,
			"delete_lease_token":   "",
		})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected != int64(len(accepted)) {
		return nil, fmt.Errorf("k8s resource deletion acceptance changed while rows were locked: expected %d updates, got %d", len(accepted), result.RowsAffected)
	}
	return accepted, nil
}

func lockK8sResourceRows(db *gorm.DB) *gorm.DB {
	if db.Dialect().GetName() == "sqlite3" {
		return db
	}
	return db.Set("gorm:query_option", "FOR UPDATE")
}

// ListDueK8sResourceDeletions returns unleased DELETING rows whose retry time has elapsed.
// The scanner receives only Region record IDs and generations, then loads the saved YAML after claiming a lease.
func (t *K8sResourceDaoImpl) ListDueK8sResourceDeletions(now time.Time, limit int) ([]model.K8sResource, error) {
	if limit <= 0 {
		return []model.K8sResource{}, nil
	}

	var resources []model.K8sResource
	if err := t.DB.Model(&model.K8sResource{}).
		Select("ID, delete_generation").
		Where("delete_status = ? AND (delete_next_retry_at IS NULL OR delete_next_retry_at <= ?) AND (delete_lease_until IS NULL OR delete_lease_until <= ?)", model.K8sResourceDeleteStatusDeleting, now, now).
		Order("delete_next_retry_at ASC, ID ASC").
		Limit(limit).
		Find(&resources).Error; err != nil {
		return nil, err
	}
	return resources, nil
}

// ClaimK8sResourceDeleteLease atomically claims due deletion work for a bounded worker lease.
func (t *K8sResourceDaoImpl) ClaimK8sResourceDeleteLease(resourceID uint, deleteGeneration int64, now, leaseUntil time.Time) (*model.K8sResource, error) {
	if !leaseUntil.After(now) {
		return nil, fmt.Errorf("delete lease must expire after it is claimed")
	}
	deleteLeaseToken, err := newK8sResourceDeleteLeaseToken()
	if err != nil {
		return nil, err
	}

	result := deletingGenerationQuery(t.DB, resourceID, deleteGeneration).
		Model(&model.K8sResource{}).
		Where("(delete_next_retry_at IS NULL OR delete_next_retry_at <= ?) AND (delete_lease_until IS NULL OR delete_lease_until <= ?)", now, now).
		Updates(map[string]interface{}{
			"delete_lease_until": leaseUntil,
			"delete_lease_token": deleteLeaseToken,
		})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, nil
	}
	return &model.K8sResource{
		Model:            model.Model{ID: resourceID},
		DeleteGeneration: deleteGeneration,
		DeleteLeaseUntil: &leaseUntil,
		DeleteLeaseToken: deleteLeaseToken,
	}, nil
}

// GetK8sResourceForDeletion loads the saved Region resource only when the worker owns its active generation.
func (t *K8sResourceDaoImpl) GetK8sResourceForDeletion(resourceID uint, deleteGeneration int64, deleteLeaseToken string) (model.K8sResource, error) {
	var resource model.K8sResource
	if err := deletingGenerationLeaseQuery(t.DB, resourceID, deleteGeneration, deleteLeaseToken).First(&resource).Error; err != nil {
		return model.K8sResource{}, err
	}
	return resource, nil
}

// ScheduleK8sResourceDeleteRetry releases the current lease and leaves the deletion recoverable by a future scan.
func (t *K8sResourceDaoImpl) ScheduleK8sResourceDeleteRetry(resourceID uint, deleteGeneration int64, deleteLeaseToken string, nextRetryAt time.Time, deleteError string) (bool, error) {
	result := deletingGenerationLeaseQuery(t.DB, resourceID, deleteGeneration, deleteLeaseToken).
		Model(&model.K8sResource{}).
		Updates(map[string]interface{}{
			"delete_error":         deleteError,
			"delete_next_retry_at": nextRetryAt,
			"delete_lease_until":   nil,
			"delete_lease_token":   "",
		})
	return result.RowsAffected > 0, result.Error
}

// MarkK8sResourceDeleteFailed records a physical deletion failure only for the active worker generation.
func (t *K8sResourceDaoImpl) MarkK8sResourceDeleteFailed(resourceID uint, deleteGeneration int64, deleteLeaseToken, deleteError string) (bool, error) {
	result := deletingGenerationLeaseQuery(t.DB, resourceID, deleteGeneration, deleteLeaseToken).
		Model(&model.K8sResource{}).
		Updates(map[string]interface{}{
			"delete_status":        model.K8sResourceDeleteStatusFailed,
			"delete_error":         deleteError,
			"delete_next_retry_at": nil,
			"delete_lease_until":   nil,
			"delete_lease_token":   "",
		})
	return result.RowsAffected > 0, result.Error
}

// IncreaseK8sResourceDeleteAttempts records a physical deletion attempt only for the active worker generation.
func (t *K8sResourceDaoImpl) IncreaseK8sResourceDeleteAttempts(resourceID uint, deleteGeneration int64, deleteLeaseToken string) (bool, error) {
	result := deletingGenerationLeaseQuery(t.DB, resourceID, deleteGeneration, deleteLeaseToken).
		Model(&model.K8sResource{}).
		UpdateColumn("delete_attempts", gorm.Expr("delete_attempts + ?", 1))
	return result.RowsAffected > 0, result.Error
}

// AdoptK8sResourceDeleteIdentity stores the UID observed by a delete worker
// only while it owns the exact deletion generation. Legacy rows without a UID
// therefore become safe to delete without allowing an old task to overwrite a
// newer object's identity.
func (t *K8sResourceDaoImpl) AdoptK8sResourceDeleteIdentity(resourceID uint, deleteGeneration int64, deleteLeaseToken, resourceUID, resourceIdentity string) (bool, error) {
	if resourceUID == "" || resourceIdentity == "" {
		return false, fmt.Errorf("observed Kubernetes UID and physical identity are required for adoption")
	}
	result := deletingGenerationLeaseQuery(t.DB, resourceID, deleteGeneration, deleteLeaseToken).
		Where("(resource_uid = '' OR resource_uid = ?) AND (resource_identity = '' OR resource_identity = ?)", resourceUID, resourceIdentity).
		Model(&model.K8sResource{}).
		Updates(map[string]interface{}{
			"resource_uid":      resourceUID,
			"resource_identity": resourceIdentity,
		})
	return result.RowsAffected > 0, result.Error
}

// DeleteK8sResourceByIDAndGeneration physically removes a resource only after its active generation is verified deleted.
// New generic delete flows must use this guarded method rather than the legacy unrestricted delete helpers below.
func (t *K8sResourceDaoImpl) DeleteK8sResourceByIDAndGeneration(resourceID uint, deleteGeneration int64, deleteLeaseToken string) (bool, error) {
	result := deletingGenerationLeaseQuery(t.DB, resourceID, deleteGeneration, deleteLeaseToken).Delete(&model.K8sResource{})
	return result.RowsAffected > 0, result.Error
}

func controllerK8sResourceQuery(db *gorm.DB, appID string, targets []model.K8sResourceDeleteTarget) *gorm.DB {
	if len(targets) == 0 {
		return db.Where("1 = 0")
	}

	identities := make([]string, 0, len(targets))
	args := make([]interface{}, 0, len(targets)*2)
	for _, target := range targets {
		identities = append(identities, "(name = ? AND kind = ?)")
		args = append(args, target.Name, target.Kind)
	}
	return db.Where("app_id = ?", appID).Where("("+strings.Join(identities, " OR ")+")", args...)
}

func k8sResourceDeleteStatusQuery(db *gorm.DB, appID string, targets []model.K8sResourceDeleteTarget) *gorm.DB {
	if len(targets) == 0 {
		return db.Where("1 = 0")
	}

	identities := make([]string, 0, len(targets))
	args := make([]interface{}, 0, len(targets)*2)
	for _, target := range targets {
		if target.ResourceID > 0 {
			identities = append(identities, "ID = ?")
			args = append(args, target.ResourceID)
			continue
		}
		identities = append(identities, "(name = ? AND kind = ?)")
		args = append(args, target.Name, target.Kind)
	}
	return db.Where("app_id = ?", appID).Where("("+strings.Join(identities, " OR ")+")", args...)
}

func deletingGenerationQuery(db *gorm.DB, resourceID uint, deleteGeneration int64) *gorm.DB {
	return db.Where("ID = ? AND delete_generation = ? AND delete_status = ?", resourceID, deleteGeneration, model.K8sResourceDeleteStatusDeleting)
}

func deletingGenerationLeaseQuery(db *gorm.DB, resourceID uint, deleteGeneration int64, deleteLeaseToken string) *gorm.DB {
	return deletingGenerationQuery(db, resourceID, deleteGeneration).Where("delete_lease_token = ?", deleteLeaseToken)
}

func newK8sResourceDeleteLeaseToken() (string, error) {
	bytes := make([]byte, 24)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// DeleteK8sResourceByIDs is a legacy unrestricted delete helper. It remains for existing callers only.
// New generic K8s resource deletion must use DeleteK8sResourceByIDAndGeneration.
func (t *K8sResourceDaoImpl) DeleteK8sResourceByIDs(ids []uint) error {
	if err := t.DB.Where("ID in (?)", ids).Delete(&model.K8sResource{}).Error; err != nil {
		return err
	}
	return nil
}

// CreateK8sResource -
func (t *K8sResourceDaoImpl) CreateK8sResource(k8sResources []*model.K8sResource) error {
	if len(k8sResources) == 0 {
		return nil
	}
	return t.DB.Transaction(func(tx *gorm.DB) error {
		targets := make([]model.K8sResourceDeleteTarget, 0, len(k8sResources))
		for _, resource := range k8sResources {
			if resource == nil {
				continue
			}
			targets = append(targets, model.K8sResourceDeleteTarget{Name: resource.Name, Kind: resource.Kind, ResourceIdentity: resource.ResourceIdentity})
		}
		if err := ensureK8sResourceCreationAllowed(tx, "", targets, 0); err != nil {
			return err
		}
		dbType := tx.Dialect().GetName()
		if dbType == "sqlite3" {
			for _, resource := range k8sResources {
				if resource == nil {
					continue
				}
				if err := tx.Create(resource).Error; err != nil {
					logrus.Error("batch Update or update k8sResources error:", err)
					return err
				}
			}
			return nil
		}
		objects := make([]interface{}, 0, len(k8sResources))
		for _, resource := range k8sResources {
			if resource != nil {
				objects = append(objects, *resource)
			}
		}
		if err := gormbulkups.BulkUpsert(tx, objects, 2000); err != nil {
			return pkgerr.Wrap(err, "create K8sResource groups in batch")
		}
		return nil
	})
}

// DeleteK8sResource -
func (t *K8sResourceDaoImpl) DeleteK8sResource(appID, name string, kind string) error {
	return t.DB.Where("app_id=? and name=? and kind=?", appID, name, kind).Delete(&model.K8sResource{}).Error
}

// GetK8sResourceByName -
func (t *K8sResourceDaoImpl) GetK8sResourceByName(appID, name, kind string) (model.K8sResource, error) {
	var resources model.K8sResource
	if err := t.DB.Where("app_id=? and name=? and kind=?", appID, name, kind).First(&resources).Error; err != nil {
		return model.K8sResource{}, err
	}
	return resources, nil
}
