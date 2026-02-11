package license

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"time"
)

// LicenseToken is the license data structure for plugin authorization.
type LicenseToken struct {
	Code           string   `json:"code"`
	EnterpriseID   string   `json:"enterprise_id"`
	ClusterID      string   `json:"cluster_id"`
	Company        string   `json:"company"`
	Contact        string   `json:"contact"`
	Tier           string   `json:"tier"`
	AllowedPlugins []string `json:"allowed_plugins"`
	StartAt        int64    `json:"start_at"`
	ExpireAt       int64    `json:"expire_at"`
	SubscribeUntil int64    `json:"subscribe_until"`
	ClusterLimit   int      `json:"cluster_limit"`
	NodeLimit      int      `json:"node_limit"`
	MemoryLimit    int64    `json:"memory_limit"`
	CPULimit       int64    `json:"cpu_limit"`
	Signature      string   `json:"signature"`
}

// LicenseStatus represents the status response of a license validation.
type LicenseStatus struct {
	Valid          bool   `json:"valid"`
	Reason         string `json:"reason,omitempty"`
	Code           string `json:"code,omitempty"`
	EnterpriseID   string `json:"enterprise_id,omitempty"`
	ClusterID      string `json:"cluster_id,omitempty"`
	Company        string `json:"company,omitempty"`
	Contact        string `json:"contact,omitempty"`
	Tier           string `json:"tier,omitempty"`
	StartAt        int64  `json:"start_at,omitempty"`
	ExpireAt       int64  `json:"expire_at,omitempty"`
	SubscribeUntil int64  `json:"subscribe_until,omitempty"`
	ClusterLimit   int    `json:"cluster_limit,omitempty"`
	NodeLimit      int    `json:"node_limit,omitempty"`
	MemoryLimit    int64  `json:"memory_limit,omitempty"`
	CPULimit       int64  `json:"cpu_limit,omitempty"`
}

// ParsePublicKey parses a PEM-encoded RSA public key.
func ParsePublicKey(pemData []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, errors.New("failed to decode PEM block")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("not an RSA public key")
	}
	return rsaPub, nil
}

// DecodeLicense decodes a base64-encoded license string into a LicenseToken.
func DecodeLicense(encoded string) (*LicenseToken, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("base64 decode: %w", err)
	}
	var token LicenseToken
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, fmt.Errorf("json unmarshal: %w", err)
	}
	return &token, nil
}

// ParseLicenseJSON parses a JSON string directly into a LicenseToken (no base64).
func ParseLicenseJSON(jsonData string) (*LicenseToken, error) {
	var token LicenseToken
	if err := json.Unmarshal([]byte(jsonData), &token); err != nil {
		return nil, fmt.Errorf("json unmarshal: %w", err)
	}
	return &token, nil
}

// MarshalLicenseJSON marshals a LicenseToken to a JSON string.
func MarshalLicenseJSON(token *LicenseToken) (string, error) {
	data, err := json.Marshal(token)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// EncodeLicense encodes a LicenseToken into a base64 string.
func EncodeLicense(token LicenseToken) (string, error) {
	data, err := json.Marshal(token)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

// signingPayload creates the signing payload by zeroing the Signature field.
func signingPayload(token LicenseToken) ([]byte, error) {
	token.Signature = ""
	return json.Marshal(token)
}

// VerifySignature verifies the RSA-SHA256 PKCS1v15 signature of a LicenseToken.
func VerifySignature(token *LicenseToken, pubKey *rsa.PublicKey) error {
	sigBytes, err := base64.StdEncoding.DecodeString(token.Signature)
	if err != nil {
		return fmt.Errorf("decode signature: %w", err)
	}
	payload, err := signingPayload(*token)
	if err != nil {
		return fmt.Errorf("create signing payload: %w", err)
	}
	hash := sha256.Sum256(payload)
	return rsa.VerifyPKCS1v15(pubKey, crypto.SHA256, hash[:], sigBytes)
}

// ValidateToken validates signature, enterprise ID binding, and time window.
func ValidateToken(token *LicenseToken, pubKey *rsa.PublicKey, enterpriseID string, now time.Time) error {
	if err := VerifySignature(token, pubKey); err != nil {
		return fmt.Errorf("invalid signature: %w", err)
	}
	if token.EnterpriseID != enterpriseID {
		return fmt.Errorf("enterprise ID mismatch: license=%s, actual=%s", token.EnterpriseID, enterpriseID)
	}
	unix := now.Unix()
	if unix < token.StartAt {
		return errors.New("license not yet valid")
	}
	if unix > token.ExpireAt {
		return errors.New("license expired")
	}
	return nil
}

// IsPluginAllowed checks if a plugin is allowed by the license.
func IsPluginAllowed(token *LicenseToken, pluginID string) bool {
	for _, p := range token.AllowedPlugins {
		if p == "*" || p == pluginID {
			return true
		}
	}
	return false
}

// TokenToStatus converts a LicenseToken to a LicenseStatus.
func TokenToStatus(token *LicenseToken, valid bool, reason string) *LicenseStatus {
	s := &LicenseStatus{
		Valid:  valid,
		Reason: reason,
	}
	if token != nil {
		s.Code = token.Code
		s.EnterpriseID = token.EnterpriseID
		s.ClusterID = token.ClusterID
		s.Company = token.Company
		s.Contact = token.Contact
		s.Tier = token.Tier
		s.StartAt = token.StartAt
		s.ExpireAt = token.ExpireAt
		s.SubscribeUntil = token.SubscribeUntil
		s.ClusterLimit = token.ClusterLimit
		s.NodeLimit = token.NodeLimit
		s.MemoryLimit = token.MemoryLimit
		s.CPULimit = token.CPULimit
	}
	return s
}
