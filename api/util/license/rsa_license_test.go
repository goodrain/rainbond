package license

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"testing"
	"time"
)

func generateTestKeyPair(t *testing.T) (*rsa.PrivateKey, *rsa.PublicKey) {
	t.Helper()
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}
	return privKey, &privKey.PublicKey
}

func publicKeyToPEM(t *testing.T, pub *rsa.PublicKey) []byte {
	t.Helper()
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		t.Fatalf("marshal public key: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})
}

func signTestToken(t *testing.T, token *LicenseToken, privKey *rsa.PrivateKey) {
	t.Helper()
	token.Signature = ""
	payload, err := json.Marshal(token)
	if err != nil {
		t.Fatalf("marshal token: %v", err)
	}
	hash := sha256.Sum256(payload)
	sig, err := rsa.SignPKCS1v15(rand.Reader, privKey, crypto.SHA256, hash[:])
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	token.Signature = base64.StdEncoding.EncodeToString(sig)
}

func newValidToken(enterpriseID string) LicenseToken {
	now := time.Now().Unix()
	return LicenseToken{
		Code:           "TEST-001",
		EnterpriseID:   enterpriseID,
		ClusterID:      "cluster-audit-only",
		Company:        "Test Corp",
		Contact:        "test@example.com",
		Tier:           "advanced",
		AllowedPlugins: []string{"*"},
		StartAt:        now - 3600,
		ExpireAt:       now + 86400,
		SubscribeUntil: now + 86400,
		ClusterLimit:   -1,
		NodeLimit:      -1,
		MemoryLimit:    -1,
		CPULimit:       -1,
	}
}

func TestDecodeLicense(t *testing.T) {
	token := newValidToken("ent-1")
	data, err := json.Marshal(token)
	if err != nil {
		t.Fatal(err)
	}
	encoded := base64.StdEncoding.EncodeToString(data)

	decoded, err := DecodeLicense(encoded)
	if err != nil {
		t.Fatalf("DecodeLicense: %v", err)
	}
	if decoded.Code != "TEST-001" {
		t.Errorf("expected code TEST-001, got %s", decoded.Code)
	}
	if decoded.Company != "Test Corp" {
		t.Errorf("expected company Test Corp, got %s", decoded.Company)
	}
}

func TestDecodeLicense_InvalidBase64(t *testing.T) {
	_, err := DecodeLicense("not-valid-base64!!!")
	if err == nil {
		t.Fatal("expected error for invalid base64")
	}
}

func TestDecodeLicense_InvalidJSON(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte("not json"))
	_, err := DecodeLicense(encoded)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestVerifySignature_Valid(t *testing.T) {
	privKey, pubKey := generateTestKeyPair(t)
	token := newValidToken("ent-1")
	signTestToken(t, &token, privKey)

	if err := VerifySignature(&token, pubKey); err != nil {
		t.Fatalf("VerifySignature failed for valid token: %v", err)
	}
}

func TestVerifySignature_Tampered(t *testing.T) {
	privKey, pubKey := generateTestKeyPair(t)
	token := newValidToken("ent-1")
	signTestToken(t, &token, privKey)

	token.Company = "Tampered Corp"
	if err := VerifySignature(&token, pubKey); err == nil {
		t.Fatal("expected error for tampered token")
	}
}

func TestVerifySignature_WrongKey(t *testing.T) {
	privKey, _ := generateTestKeyPair(t)
	_, wrongPubKey := generateTestKeyPair(t)
	token := newValidToken("ent-1")
	signTestToken(t, &token, privKey)

	if err := VerifySignature(&token, wrongPubKey); err == nil {
		t.Fatal("expected error for wrong public key")
	}
}

func TestValidateToken_Valid(t *testing.T) {
	privKey, pubKey := generateTestKeyPair(t)
	token := newValidToken("ent-1")
	signTestToken(t, &token, privKey)

	if err := ValidateToken(&token, pubKey, "ent-1", time.Now()); err != nil {
		t.Fatalf("ValidateToken failed for valid token: %v", err)
	}
}

func TestValidateToken_EnterpriseMismatch(t *testing.T) {
	privKey, pubKey := generateTestKeyPair(t)
	token := newValidToken("ent-1")
	signTestToken(t, &token, privKey)

	if err := ValidateToken(&token, pubKey, "ent-2", time.Now()); err == nil {
		t.Fatal("expected error for enterprise ID mismatch")
	}
}

func TestValidateToken_Expired(t *testing.T) {
	privKey, pubKey := generateTestKeyPair(t)
	token := newValidToken("ent-1")
	token.ExpireAt = time.Now().Unix() - 3600
	signTestToken(t, &token, privKey)

	if err := ValidateToken(&token, pubKey, "ent-1", time.Now()); err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestValidateToken_NotYetValid(t *testing.T) {
	privKey, pubKey := generateTestKeyPair(t)
	token := newValidToken("ent-1")
	token.StartAt = time.Now().Unix() + 86400
	signTestToken(t, &token, privKey)

	if err := ValidateToken(&token, pubKey, "ent-1", time.Now()); err == nil {
		t.Fatal("expected error for not-yet-valid token")
	}
}

func TestIsPluginAllowed_Wildcard(t *testing.T) {
	token := &LicenseToken{AllowedPlugins: []string{"*"}}
	if !IsPluginAllowed(token, "any-plugin") {
		t.Fatal("expected wildcard to allow any plugin")
	}
}

func TestIsPluginAllowed_Specific(t *testing.T) {
	token := &LicenseToken{AllowedPlugins: []string{"plugin-a", "plugin-b"}}
	if !IsPluginAllowed(token, "plugin-a") {
		t.Fatal("expected plugin-a to be allowed")
	}
	if !IsPluginAllowed(token, "plugin-b") {
		t.Fatal("expected plugin-b to be allowed")
	}
}

func TestIsPluginAllowed_Denied(t *testing.T) {
	token := &LicenseToken{AllowedPlugins: []string{"plugin-a"}}
	if IsPluginAllowed(token, "plugin-c") {
		t.Fatal("expected plugin-c to be denied")
	}
}

func TestParsePublicKey(t *testing.T) {
	_, pubKey := generateTestKeyPair(t)
	pemData := publicKeyToPEM(t, pubKey)
	parsed, err := ParsePublicKey(pemData)
	if err != nil {
		t.Fatalf("ParsePublicKey: %v", err)
	}
	if parsed.N.Cmp(pubKey.N) != 0 {
		t.Fatal("parsed key does not match original")
	}
}

func TestParsePublicKey_InvalidPEM(t *testing.T) {
	_, err := ParsePublicKey([]byte("not a pem"))
	if err == nil {
		t.Fatal("expected error for invalid PEM")
	}
}

func TestRoundTrip(t *testing.T) {
	privKey, pubKey := generateTestKeyPair(t)
	token := newValidToken("ent-round-trip")
	signTestToken(t, &token, privKey)

	// Encode
	encoded, err := EncodeLicense(token)
	if err != nil {
		t.Fatalf("EncodeLicense: %v", err)
	}

	// Decode
	decoded, err := DecodeLicense(encoded)
	if err != nil {
		t.Fatalf("DecodeLicense: %v", err)
	}

	// Verify
	if err := VerifySignature(decoded, pubKey); err != nil {
		t.Fatalf("VerifySignature after round trip: %v", err)
	}

	// Validate
	if err := ValidateToken(decoded, pubKey, "ent-round-trip", time.Now()); err != nil {
		t.Fatalf("ValidateToken after round trip: %v", err)
	}

	// Check fields
	if decoded.Code != token.Code {
		t.Errorf("Code mismatch: %s != %s", decoded.Code, token.Code)
	}
	if decoded.Company != token.Company {
		t.Errorf("Company mismatch: %s != %s", decoded.Company, token.Company)
	}
	if decoded.EnterpriseID != token.EnterpriseID {
		t.Errorf("EnterpriseID mismatch: %s != %s", decoded.EnterpriseID, token.EnterpriseID)
	}
}

func TestTokenToStatus(t *testing.T) {
	token := newValidToken("ent-1")
	status := TokenToStatus(&token, true, "")
	if !status.Valid {
		t.Fatal("expected valid status")
	}
	if status.Company != "Test Corp" {
		t.Errorf("expected company Test Corp, got %s", status.Company)
	}
	if status.EnterpriseID != "ent-1" {
		t.Errorf("expected enterprise_id ent-1, got %s", status.EnterpriseID)
	}

	status = TokenToStatus(nil, false, "no license")
	if status.Valid {
		t.Fatal("expected invalid status")
	}
	if status.Reason != "no license" {
		t.Errorf("expected reason 'no license', got %s", status.Reason)
	}
}
