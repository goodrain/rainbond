package cleanup

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"strings"
	"time"
)

const storageObservationPermitDomain = "rainbond/registry-observation/v1\x00"

type storageObservationPermit struct {
	Protocol           int                 `json:"protocol"`
	Binding            StorageRegistration `json:"binding"`
	BindingFingerprint string              `json:"binding_fingerprint"`
	IssuedAt           int64               `json:"issued_at"`
	ExpiresAt          int64               `json:"expires_at"`
}

func storageObservationMAC(key, payload []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(storageObservationPermitDomain))
	mac.Write(payload)
	return mac.Sum(nil)
}

// IssueStorageObservationPermit grants one short-lived, read-only measurement
// of an exact registered filesystem identity. It is not a deletion permit.
func IssueStorageObservationPermit(key []byte, binding StorageRegistration, now time.Time) (string, error) {
	fingerprint, err := binding.Fingerprint()
	if len(key) < 32 || err != nil || now.Unix() <= 0 {
		return "", ErrCoordinationDenied
	}
	payload, err := json.Marshal(storageObservationPermit{Protocol: 1, Binding: binding, BindingFingerprint: fingerprint, IssuedAt: now.Unix(), ExpiresAt: now.Add(time.Minute).Unix()})
	if err != nil {
		return "", ErrCoordinationDenied
	}
	return base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(storageObservationMAC(key, payload)), nil
}

// VerifyStorageObservationPermit authenticates the exact storage generation;
// the sidecar still rechecks its mounted marker and current Core binding.
func VerifyStorageObservationPermit(key []byte, encoded string, now time.Time) (StorageRegistration, error) {
	var denied StorageRegistration
	if len(key) < 32 || len(encoded) > 8192 {
		return denied, ErrCoordinationDenied
	}
	body, signature, ok := strings.Cut(encoded, ".")
	if !ok || strings.Contains(signature, ".") {
		return denied, ErrCoordinationDenied
	}
	raw, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil || len(raw) > 4096 {
		return denied, ErrCoordinationDenied
	}
	mac, err := base64.RawURLEncoding.DecodeString(signature)
	if err != nil || !hmac.Equal(mac, storageObservationMAC(key, raw)) {
		return denied, ErrCoordinationDenied
	}
	var payload storageObservationPermit
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&payload) != nil || decoder.Decode(&struct{}{}) != io.EOF || payload.Protocol != 1 || payload.IssuedAt <= 0 || payload.ExpiresAt-payload.IssuedAt != 60 || now.Unix() < payload.IssuedAt-5 || now.Unix() >= payload.ExpiresAt {
		return denied, ErrCoordinationDenied
	}
	fingerprint, err := payload.Binding.Fingerprint()
	if err != nil || fingerprint != payload.BindingFingerprint {
		return denied, ErrCoordinationDenied
	}
	return payload.Binding, nil
}
