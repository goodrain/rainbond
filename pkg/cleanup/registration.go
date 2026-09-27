package cleanup

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path"
	"strings"

	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
)

// StorageRegistration contains identities observed by the trusted installer.
// Registration alone is not proof that participants or references are complete.
type StorageRegistration struct {
	StorageID  string `json:"storage_id"`
	Generation string `json:"generation"`
	VolumeUID  string `json:"volume_uid"`
	RootPath   string `json:"root_path"`
}

// Fingerprint validates and hashes the immutable registration descriptor.
func (r StorageRegistration) Fingerprint() (string, error) {
	if !coordinationIdentity.MatchString(r.StorageID) || !coordinationIdentity.MatchString(r.Generation) || !coordinationIdentity.MatchString(r.VolumeUID) || !path.IsAbs(r.RootPath) || r.RootPath == "/" || path.Clean(r.RootPath) != r.RootPath || len(r.RootPath) > 1024 || strings.ContainsAny(r.RootPath, "\x00\\") {
		return "", ErrCoordinationChanged
	}
	raw, err := json.Marshal(r)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

// RegisterStorage never promotes readiness or changes an existing generation.
// Collecting allows participants to enroll without interrupting normal writes;
// destructive admission stays closed until independent readiness verification.
func RegisterStorage(database *gorm.DB, r StorageRegistration) error {
	fingerprint, err := r.Fingerprint()
	if err != nil {
		return err
	}
	match := func(existing model.CleanupStorage) error {
		if existing.Generation != r.Generation || existing.RegistrationFingerprint != fingerprint {
			return ErrCoordinationChanged
		}
		return nil
	}
	var existing model.CleanupStorage
	err = database.Where("storage_id = ?", r.StorageID).First(&existing).Error
	if err == nil {
		return match(existing)
	}
	if !gorm.IsRecordNotFoundError(err) {
		return err
	}
	encoded, _ := json.Marshal(r)
	created := model.CleanupStorage{StorageID: r.StorageID, Generation: r.Generation, RegistrationFingerprint: fingerprint, RegistrationJSON: string(encoded), Mode: "collecting"}
	if err := database.Create(&created).Error; err != nil {
		// A concurrent identical registration is harmless. Never use an upsert that
		// could overwrite a live generation or clear an existing maintenance state.
		if readErr := database.Where("storage_id = ?", r.StorageID).First(&existing).Error; readErr == nil {
			return match(existing)
		}
		return err
	}
	return nil
}

// StorageObservation is an advisory identity/status response, not a cleanup grant.
type StorageObservation struct {
	StorageID               string `json:"storage_id"`
	Generation              string `json:"generation"`
	RegistrationFingerprint string `json:"registration_fingerprint"`
	Mode                    string `json:"mode"`
	Revision                uint64 `json:"revision"`
}

// InspectStorage reads one committed row without changing readiness or leases.
func InspectStorage(database *gorm.DB, storage, generation string) (StorageObservation, error) {
	if !coordinationIdentity.MatchString(storage) || !coordinationIdentity.MatchString(generation) {
		return StorageObservation{}, ErrCoordinationChanged
	}
	var row model.CleanupStorage
	if err := database.Where("storage_id = ?", storage).First(&row).Error; err != nil {
		return StorageObservation{}, err
	}
	if row.Generation != generation {
		return StorageObservation{}, ErrCoordinationChanged
	}
	return StorageObservation{StorageID: row.StorageID, Generation: row.Generation, RegistrationFingerprint: row.RegistrationFingerprint, Mode: row.Mode, Revision: row.Revision}, nil
}

// ProvisionRegistryStorage assigns identity from an observed physical volume.
// Retried provisioning returns the original generation, never a replacement.
func ProvisionRegistryStorage(database *gorm.DB, volumeUID, root string) (StorageRegistration, error) {
	return provisionFilesystemStorage(database, "registry-filesystem", volumeUID, root)
}

// ProvisionManagedCacheStorage enrolls observed cache storage in collecting mode.
// It does not certify writer coverage or enable deletion.
func ProvisionManagedCacheStorage(database *gorm.DB, volumeUID, root string) (StorageRegistration, error) {
	if root != "/cache/build" {
		return StorageRegistration{}, ErrCoordinationChanged
	}
	return provisionFilesystemStorage(database, "managed-build-cache", volumeUID, root)
}

func provisionFilesystemStorage(database *gorm.DB, domain, volumeUID, root string) (StorageRegistration, error) {
	key := sha256.Sum256([]byte(domain + "\x00" + volumeUID))
	storageID := hex.EncodeToString(key[:])
	read := func() (StorageRegistration, error) {
		var stored model.CleanupStorage
		if err := database.Where("storage_id = ?", storageID).First(&stored).Error; err != nil {
			return StorageRegistration{}, err
		}
		var binding StorageRegistration
		if json.Unmarshal([]byte(stored.RegistrationJSON), &binding) != nil {
			return binding, ErrCoordinationChanged
		}
		fingerprint, err := binding.Fingerprint()
		if err != nil || binding.StorageID != storageID || binding.Generation != stored.Generation || binding.VolumeUID != volumeUID || binding.RootPath != root || fingerprint != stored.RegistrationFingerprint {
			return StorageRegistration{}, ErrCoordinationChanged
		}
		return binding, nil
	}
	current, err := read()
	if err == nil {
		return current, nil
	}
	if !gorm.IsRecordNotFoundError(err) {
		return StorageRegistration{}, err
	}
	generation, err := NewActivationRevision()
	if err != nil {
		return StorageRegistration{}, err
	}
	created := StorageRegistration{StorageID: storageID, Generation: generation, VolumeUID: volumeUID, RootPath: root}
	if err := RegisterStorage(database, created); err != nil {
		// Another installer may have won the same physical identity concurrently.
		if existing, readErr := read(); readErr == nil {
			return existing, nil
		}
		return StorageRegistration{}, err
	}
	return created, nil
}

// StorageBinding retrieves only a valid immutable registered descriptor.
func StorageBinding(database *gorm.DB, storage, generation string) (StorageRegistration, error) {
	var row model.CleanupStorage
	if !coordinationIdentity.MatchString(storage) || !coordinationIdentity.MatchString(generation) {
		return StorageRegistration{}, ErrCoordinationChanged
	}
	if err := database.Where("storage_id = ?", storage).First(&row).Error; err != nil {
		return StorageRegistration{}, err
	}
	var binding StorageRegistration
	if json.Unmarshal([]byte(row.RegistrationJSON), &binding) != nil {
		return StorageRegistration{}, ErrCoordinationChanged
	}
	fingerprint, err := binding.Fingerprint()
	if err != nil || binding.StorageID != storage || binding.Generation != generation || row.Generation != generation || fingerprint != row.RegistrationFingerprint {
		return StorageRegistration{}, ErrCoordinationChanged
	}
	return binding, nil
}
