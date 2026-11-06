package cleanup

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"

	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
)

// ParticipantRegistration is populated after querying actual platform objects,
// not copied from a participant's claims about its image or container identity.
type ParticipantRegistration struct {
	StorageID, Generation, Owner, Role, PodUID, ContainerID, ImageID, BindingFingerprint string
}

func participantIdentity(p ParticipantRegistration) (string, string, error) {
	if !p.valid() {
		return "", "", ErrCoordinationChanged
	}
	raw, _ := json.Marshal(p)
	digest := sha256.Sum256(raw)
	key := sha256.Sum256([]byte(p.StorageID + "\x00" + p.Generation + "\x00" + p.Role + "\x00" + p.Owner))
	return hex.EncodeToString(key[:]), hex.EncodeToString(digest[:]), nil
}

func (p ParticipantRegistration) valid() bool {
	if !coordinationIdentity.MatchString(p.StorageID) || !coordinationIdentity.MatchString(p.Generation) || !coordinationIdentity.MatchString(p.PodUID) || (p.Role != "registry-ingress" && p.Role != "cache-builder") {
		return false
	}
	for _, field := range []struct {
		value string
		max   int
	}{{p.Owner, 128}, {p.ContainerID, 256}, {p.ImageID, 512}, {p.BindingFingerprint, 64}} {
		if field.value == "" || len(field.value) > field.max || strings.ContainsAny(field.value, "\x00\r\n") {
			return false
		}
	}
	if p.Role == "cache-builder" {
		identity := sha256.Sum256([]byte(p.PodUID + "\x00" + p.ContainerID))
		if p.Owner != "cache-builder:"+hex.EncodeToString(identity[:]) {
			return false
		}
	}
	return true
}

// RegisterParticipant does not grant cleanup readiness or expire old operations.
func RegisterParticipant(database *gorm.DB, p ParticipantRegistration) error {
	id, fingerprint, err := participantIdentity(p)
	if err != nil {
		return err
	}
	tx := database.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer tx.Rollback()
	store, err := lockCleanupStorage(tx, CoordinationRequest{StorageID: p.StorageID, Generation: p.Generation})
	if err != nil {
		return err
	}
	if store.RegistrationFingerprint == "" || store.RegistrationFingerprint != p.BindingFingerprint {
		return ErrCoordinationChanged
	}
	var existing model.CleanupParticipant
	err = tx.Where("id = ?", id).First(&existing).Error
	if err == nil {
		if existing.RegistrationHash != fingerprint {
			return ErrCoordinationChanged
		}
		return tx.Commit().Error
	}
	if !gorm.IsRecordNotFoundError(err) {
		return err
	}
	record := model.CleanupParticipant{ID: id, StorageID: p.StorageID, Generation: p.Generation, Owner: p.Owner, Role: p.Role, PodUID: p.PodUID, ContainerID: p.ContainerID, ImageID: p.ImageID, BindingFingerprint: p.BindingFingerprint, RegistrationHash: fingerprint}
	if err := tx.Create(&record).Error; err != nil {
		return err
	}
	if store.Mode == "ready" {
		// Membership changes require a new aggregate assessment, not implicit trust.
		if err := tx.Model(&model.CleanupStorage{}).Where("storage_id = ?", p.StorageID).Update("mode", "collecting").Error; err != nil {
			return err
		}
	}
	if err := advanceCleanupRevision(tx, store); err != nil {
		return err
	}
	return tx.Commit().Error
}
