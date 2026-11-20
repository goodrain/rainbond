package vmrestore

import (
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
)

// PublicAttributeCopies strips internal state and all database identity on export
// or import. Import callers must pass their server-derived target scope.
func PublicAttributeCopies(rows []*dbmodel.ComponentK8sAttributes, target *Scope) []*dbmodel.ComponentK8sAttributes {
	result := make([]*dbmodel.ComponentK8sAttributes, 0, len(rows))
	for _, row := range rows {
		if row == nil || IsInternalAttribute(row.Name) {
			continue
		}
		copy := *row
		copy.Model = dbmodel.Model{}
		if target != nil {
			copy.TenantID = target.TenantID
			copy.ComponentID = target.ServiceID
		}
		result = append(result, &copy)
	}
	return result
}

// ReplacePublicAttributesTx replaces only public rows of this exact component.
// It is NOT a replacement for permanent DeleteByComponentIDs during actual GC.
// Its caller must have obtained component mutation admission before bulk sync.
func ReplacePublicAttributesTx(tx *gorm.DB, scope Scope, rows []*dbmodel.ComponentK8sAttributes) error {
	if tx == nil || !validUID(scope.TenantID) || !validUID(scope.ServiceID) {
		return ErrCorrupt
	}
	for _, row := range rows {
		if row == nil {
			return ErrCorrupt
		}
		if err := CheckPublicAttribute(row.Name); err != nil {
			return err
		}
	}
	var existing []dbmodel.ComponentK8sAttributes
	if err := lockedQuery(tx).Select("ID, tenant_id, component_id, name").Where("component_id = ?", scope.ServiceID).Order("ID").Find(&existing).Error; err != nil {
		return err
	}
	ids := make([]uint, 0, len(existing))
	for _, row := range existing {
		if row.TenantID != scope.TenantID {
			return ErrCorrupt
		}
		if !IsInternalAttribute(row.Name) {
			ids = append(ids, row.ID)
		}
	}
	if len(ids) > 0 {
		if err := tx.Where("component_id = ? AND tenant_id = ? AND ID IN (?)", scope.ServiceID, scope.TenantID, ids).Delete(&dbmodel.ComponentK8sAttributes{}).Error; err != nil {
			return err
		}
	}
	for _, row := range PublicAttributeCopies(rows, &scope) {
		if err := tx.Create(row).Error; err != nil {
			return err
		}
	}
	return nil
}

// DeletePublicAttribute deletes by server-observed primary key, so a MySQL
// case/accent-insensitive name comparison cannot delete a different internal row.
func DeletePublicAttribute(database *gorm.DB, componentID string, rowID uint) error {
	tx := database.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer tx.Rollback()
	var row dbmodel.ComponentK8sAttributes
	if err := lockedQuery(tx).Where("ID = ? AND component_id = ?", rowID, componentID).Take(&row).Error; err != nil {
		return err
	}
	if err := CheckPublicAttribute(row.Name); err != nil {
		return err
	}
	if err := tx.Where("ID = ? AND component_id = ? AND tenant_id = ?", row.ID, row.ComponentID, row.TenantID).Delete(&dbmodel.ComponentK8sAttributes{}).Error; err != nil {
		return err
	}
	return tx.Commit().Error
}
