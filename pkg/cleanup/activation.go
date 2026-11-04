package cleanup

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/jinzhu/gorm"
)

// NewActivationRevision returns a random checkpoint for a successful activation.
func NewActivationRevision() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}

// TrackServiceActivation keeps the service change and its usage checkpoint in
// one transaction. Existing caller-owned transactions are preserved.
func TrackServiceActivation(database *gorm.DB, serviceID, version string, save func(*gorm.DB) error) error {
	if version == "" {
		return save(database)
	}
	revision, err := NewActivationRevision()
	if err != nil {
		return err
	}
	prepare := func(tx *gorm.DB) ([]string, error) {
		query := tx
		// SQLite has no row-lock syntax; mutateReferences already holds its
		// write transaction before this callback runs.
		if tx.Dialect().GetName() != "sqlite3" {
			query = tx.Set("gorm:query_option", "FOR UPDATE")
		}
		var current serviceRow
		if err := query.Table("tenant_services").Where("service_id = ?", serviceID).First(&current).Error; err != nil {
			return nil, err
		}
		var target versionRow
		if err := query.Table("tenant_service_version").Where("service_id = ? AND build_version = ?", serviceID, version).First(&target).Error; err != nil {
			if gorm.IsRecordNotFoundError(err) && current.DeployVersion == version {
				// A configuration-only save may retain a legacy current version.
				// Its unresolved dependency still conflicts with any deletion.
				return nil, nil
			}
			return nil, err
		}
		return versionRowReferenceScopes(target), nil
	}
	return withResolvedReferenceMutation(database, prepare, func(tx *gorm.DB) error {
		if err := save(tx); err != nil {
			return err
		}
		return tx.Table("tenant_service_version").Where("service_id = ? AND build_version = ?", serviceID, version).Update("activation_revision", revision).Error
	})
}
