package cleanup

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"

	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
	"k8s.io/apimachinery/pkg/util/validation"
)

// ReferenceWriterProtocol identifies code implementing coordinated references.
const ReferenceWriterProtocol = "registry-reference-v1"

// ReferenceWriter must be constructed from current control-plane runtime facts,
// never accepted as caller-supplied image/container claims or a readiness flag.
type ReferenceWriter struct {
	Namespace, PodName, PodUID, ContainerName, ContainerID, ImageID, Role, Protocol string
}

func (w ReferenceWriter) identity() (string, string, error) {
	containers := map[string]string{"api": "rbd-api", "worker": "rbd-worker", "builder": "rbd-chaos", "console": "rbd-app-ui"}
	if containers[w.Role] == "" || w.ContainerName != containers[w.Role] || w.Protocol != ReferenceWriterProtocol || len(validation.IsDNS1123Label(w.Namespace)) != 0 || len(validation.IsDNS1123Subdomain(w.PodName)) != 0 || !coordinationIdentity.MatchString(w.PodUID) {
		return "", "", ErrCoordinationChanged
	}
	for _, field := range []struct {
		value string
		max   int
	}{{w.ContainerID, 256}, {w.ImageID, 512}} {
		if field.value == "" || len(field.value) > field.max || strings.ContainsAny(field.value, "\x00\r\n") {
			return "", "", ErrCoordinationChanged
		}
	}
	key := sha256.Sum256([]byte(w.Namespace + "\x00" + w.Role + "\x00" + w.PodUID + "\x00" + w.ContainerID))
	raw, _ := json.Marshal(w)
	fingerprint := sha256.Sum256(raw)
	return hex.EncodeToString(key[:]), hex.EncodeToString(fingerprint[:]), nil
}

// RegisterReferenceWriter saves immutable evidence. It never creates a storage,
// changes its mode, expires an operation, or adopts a previous container's proof.
func RegisterReferenceWriter(database *gorm.DB, w ReferenceWriter) error {
	if database == nil {
		return ErrCoordinationUnavailable
	}
	id, fingerprint, err := w.identity()
	if err != nil {
		return err
	}
	var existing model.CleanupReferenceWriter
	err = database.Where("id = ?", id).First(&existing).Error
	if err == nil {
		if existing.Fingerprint != fingerprint {
			return ErrCoordinationChanged
		}
		return nil
	}
	if !gorm.IsRecordNotFoundError(err) {
		return err
	}
	row := model.CleanupReferenceWriter{ID: id, Namespace: w.Namespace, PodName: w.PodName, PodUID: w.PodUID, ContainerName: w.ContainerName, ContainerID: w.ContainerID, ImageID: w.ImageID, Role: w.Role, Protocol: w.Protocol, Fingerprint: fingerprint}
	if err := database.Create(&row).Error; err != nil {
		// A concurrent identical startup may win creation. Never overwrite its row.
		var winner model.CleanupReferenceWriter
		if readErr := database.Where("id = ?", id).First(&winner).Error; readErr == nil && winner.Fingerprint == fingerprint {
			return nil
		}
		return err
	}
	return nil
}

// ReferenceWriterRegistered matches the exact observed runtime and protocol.
func ReferenceWriterRegistered(database *gorm.DB, w ReferenceWriter) (bool, error) {
	if database == nil {
		return false, ErrCoordinationUnavailable
	}
	id, fingerprint, err := w.identity()
	if err != nil {
		return false, err
	}
	var row model.CleanupReferenceWriter
	err = database.Where("id = ?", id).First(&row).Error
	if gorm.IsRecordNotFoundError(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return row.Fingerprint == fingerprint, nil
}
