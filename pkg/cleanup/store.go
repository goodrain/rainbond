package cleanup

import (
	"github.com/jinzhu/gorm"
)

type serviceRow struct {
	ServiceID     string
	TenantID      string
	DeployVersion string
}
type versionRow struct {
	ActivationRevision string
	EventID            string
	ID                 uint `gorm:"column:ID"`
	ServiceID          string
	BuildVersion       string
	ImageName          string
	DeliveredPath      string
	DeliveredType      string
	FinalStatus        string
}

type RetirementResult struct {
	RecordRetired  bool    `json:"record_retired"`
	ImageDeleted   bool    `json:"image_deleted"`
	ReclaimedBytes *uint64 `json:"reclaimed_bytes"`
}

// RetireVersion removes only an immutable build record. Lock order must remain
// service then version, matching SelectRollbackVersion below.
func RetireVersion(begin func() *gorm.DB, tenantID string, expected VersionExpectation) (RetirementResult, error) {
	tx := begin()
	if tx.Error != nil {
		return RetirementResult{}, tx.Error
	}
	defer tx.Rollback()
	var service serviceRow
	if err := tx.Table("tenant_services").Set("gorm:query_option", "FOR UPDATE").Where("service_id = ? AND tenant_id = ?", expected.ServiceID, tenantID).First(&service).Error; err != nil {
		return RetirementResult{}, err
	}
	var version versionRow
	if err := tx.Table("tenant_service_version").Set("gorm:query_option", "FOR UPDATE").Where("service_id = ? AND build_version = ?", expected.ServiceID, expected.Version).First(&version).Error; err != nil {
		return RetirementResult{}, err
	}
	var active int
	if err := tx.Table("tenant_services_event").Where("service_id = ? AND COALESCE(opt_type, '') <> ? AND (final_status IS NULL OR final_status NOT IN (?))", expected.ServiceID, "cleanup-retire-buildversion", []string{"complete", "failure", "emptycomplete"}).Count(&active).Error; err != nil {
		return RetirementResult{}, err
	}
	image := version.ImageName
	if image == "" {
		image = version.DeliveredPath
	}
	state := VersionState{ServiceID: version.ServiceID, Version: version.BuildVersion, CurrentVersion: service.DeployVersion, Image: image, EventID: version.EventID, ActivationRevision: version.ActivationRevision, Status: version.FinalStatus, DeliveredType: version.DeliveredType, ActiveOperation: active > 0}
	if err := ValidateVersionRetirement(state, expected); err != nil {
		return RetirementResult{}, err
	}
	deleted := tx.Exec("DELETE FROM tenant_service_version WHERE ID = ? AND service_id = ? AND build_version = ?", version.ID, expected.ServiceID, expected.Version)
	if deleted.Error != nil {
		return RetirementResult{}, deleted.Error
	}
	if deleted.RowsAffected != 1 {
		return RetirementResult{}, ErrStateChanged
	}
	if err := tx.Commit().Error; err != nil {
		return RetirementResult{}, err
	}
	return RetirementResult{RecordRetired: true}, nil
}

// SelectRollbackVersion cannot reactivate a record that retirement already
// removed. Both paths serialize against the same service row.
func SelectRollbackVersion(begin func() *gorm.DB, tenantID, serviceID, version, operationID string, expectedCurrent ...string) (string, error) {
	if operationID == "" || len(operationID) > 64 {
		return "", ErrStateChanged
	}
	tx := begin()
	if tx.Error != nil {
		return "", tx.Error
	}
	defer tx.Rollback()
	var service serviceRow
	var record versionRow
	prepare := func(tx *gorm.DB) ([]string, error) {
		if err := tx.Table("tenant_services").Set("gorm:query_option", "FOR UPDATE").Where("service_id = ? AND tenant_id = ?", serviceID, tenantID).First(&service).Error; err != nil {
			return nil, err
		}
		if len(expectedCurrent) > 0 && service.DeployVersion != expectedCurrent[0] {
			return nil, ErrStateChanged
		}
		if version == "" && len(expectedCurrent) > 0 {
			return nil, nil
		}
		if err := tx.Table("tenant_service_version").Set("gorm:query_option", "FOR UPDATE").Where("service_id = ? AND build_version = ?", serviceID, version).First(&record).Error; err != nil {
			return nil, err
		}
		if record.FinalStatus != "success" {
			return nil, ErrVersionProtected
		}
		return versionRowReferenceScopes(record), nil
	}
	if err := withResolvedReferenceMutation(tx, prepare, func(tx *gorm.DB) error {
		if version != "" {
			if err := tx.Table("tenant_service_version").Where("service_id = ? AND build_version = ?", serviceID, version).Update("activation_revision", operationID).Error; err != nil {
				return err
			}
		}
		return tx.Table("tenant_services").Where("service_id = ? AND tenant_id = ?", serviceID, tenantID).Update("deploy_version", version).Error
	}); err != nil {
		return "", err
	}
	if err := tx.Commit().Error; err != nil {
		return "", err
	}
	return service.DeployVersion, nil
}

// Inspection exposes reference checkpoints, never build credentials or source configuration.
type Inspection struct {
	Protocol        int               `json:"protocol"`
	CurrentVersion  string            `json:"current_version"`
	ActiveOperation bool              `json:"active_operation"`
	Checkpoints     map[string]string `json:"checkpoints"`
}

func InspectVersions(begin func() *gorm.DB, serviceID string) (Inspection, error) {
	tx := begin()
	if tx.Error != nil {
		return Inspection{}, tx.Error
	}
	defer tx.Rollback()
	var service serviceRow
	if err := tx.Table("tenant_services").Set("gorm:query_option", "FOR UPDATE").Where("service_id = ?", serviceID).First(&service).Error; err != nil {
		return Inspection{}, err
	}
	var active int
	if err := tx.Table("tenant_services_event").Where("service_id = ? AND COALESCE(opt_type, '') <> ? AND (final_status IS NULL OR final_status NOT IN (?))", serviceID, "cleanup-retire-buildversion", []string{"complete", "failure", "emptycomplete"}).Count(&active).Error; err != nil {
		return Inspection{}, err
	}
	var versions []versionRow
	if err := tx.Table("tenant_service_version").Select("event_id, activation_revision").Where("service_id = ?", serviceID).Find(&versions).Error; err != nil {
		return Inspection{}, err
	}
	result := Inspection{Protocol: 2, CurrentVersion: service.DeployVersion, ActiveOperation: active > 0, Checkpoints: map[string]string{}}
	for _, v := range versions {
		if v.EventID != "" {
			result.Checkpoints[v.EventID] = v.ActivationRevision
		}
	}
	if err := tx.Commit().Error; err != nil {
		return Inspection{}, err
	}
	return result, nil
}
