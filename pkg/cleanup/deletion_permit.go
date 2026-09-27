package cleanup

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"regexp"
	"strings"
	"time"
)

var registryDeletionDigest = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)

const registryPermitDomain = "rainbond/registry-delete/v1\x00"

type registryDeletionPermit struct {
	Protocol    int    `json:"protocol"`
	StorageID   string `json:"storage_id"`
	Generation  string `json:"generation"`
	Owner       string `json:"owner"`
	OperationID string `json:"operation_id"`
	Scope       string `json:"scope"`
	Fingerprint string `json:"fingerprint"`
	Target      string `json:"target"`
	IssuedAt    int64  `json:"issued_at"`
	ExpiresAt   int64  `json:"expires_at"`
}

func permitMAC(key, payload []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(registryPermitDomain))
	mac.Write(payload)
	return mac.Sum(nil)
}

// IssueRegistryDeletionPermit delegates only an exact immutable manifest target.
// It must be issued for a verified active coordination operation. Expiry limits
// this credential's use; it never expires or releases the persistent operation.
func IssueRegistryDeletionPermit(key []byte, r CoordinationRequest, now time.Time) (string, error) {
	if len(key) < 32 || !r.valid() || r.Kind != "delete" || !registryDeletionDigest.MatchString(r.Target) || now.Unix() <= 0 {
		return "", ErrCoordinationDenied
	}
	payload, err := json.Marshal(registryDeletionPermit{Protocol: 1, StorageID: r.StorageID, Generation: r.Generation, Owner: r.Owner, OperationID: r.OperationID, Scope: r.Scope, Fingerprint: r.Fingerprint, Target: r.Target, IssuedAt: now.Unix(), ExpiresAt: now.Add(5 * time.Minute).Unix()})
	if err != nil {
		return "", ErrCoordinationDenied
	}
	return base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(permitMAC(key, payload)), nil
}

// VerifyRegistryDeletionPermit authenticates the binding; the sidecar must then
// consume its durable one-time attempt before sending DELETE to the Registry.
func VerifyRegistryDeletionPermit(key []byte, encoded string, now time.Time) (CoordinationRequest, error) {
	var denied CoordinationRequest
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
	if err != nil || !hmac.Equal(mac, permitMAC(key, raw)) {
		return denied, ErrCoordinationDenied
	}
	var payload registryDeletionPermit
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&payload) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return denied, ErrCoordinationDenied
	}
	if payload.Protocol != 1 || payload.IssuedAt <= 0 || payload.ExpiresAt-payload.IssuedAt != 300 || now.Unix() < payload.IssuedAt-30 || now.Unix() >= payload.ExpiresAt {
		return denied, ErrCoordinationDenied
	}
	result := CoordinationRequest{StorageID: payload.StorageID, Generation: payload.Generation, Owner: payload.Owner, OperationID: payload.OperationID, Kind: "delete", Scope: payload.Scope, Fingerprint: payload.Fingerprint, Target: payload.Target}
	if !result.valid() || !registryDeletionDigest.MatchString(result.Target) {
		return denied, ErrCoordinationDenied
	}
	return result, nil
}
