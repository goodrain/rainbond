package nodecert

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func certificate(t *testing.T, expiry time.Time) []byte {
	t.Helper()
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	template := &x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: time.Now().Add(-time.Hour), NotAfter: expiry}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}
func writeCertificate(t *testing.T, dir, name string, data []byte) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}

// capability_id: rainbond.nodes.certificate-expiration
func TestCollectCertificateExpiration(t *testing.T) {
	dir := t.TempDir()
	early := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Second)
	later := early.Add(365 * 24 * time.Hour)
	if got := Collect(dir); got.CertificateStatus != "unavailable" || len(got.Certificates) != 0 {
		t.Fatal(got)
	}
	for _, name := range RequiredFiles {
		writeCertificate(t, dir, name, append(certificate(t, later), certificate(t, early)...))
	}
	got := Collect(dir)
	if got.CertificateStatus != "ok" || len(got.Certificates) != 4 {
		t.Fatal(got)
	}
	for _, cert := range got.Certificates {
		if cert.ExpiresAt != early.Format(time.RFC3339) {
			t.Fatal(cert)
		}
	}
	writeCertificate(t, dir, RequiredFiles[0], certificate(t, later))
	if Collect(dir).Certificates[0].ExpiresAt != later.Format(time.RFC3339) {
		t.Fatal("renewal not observed")
	}
	writeCertificate(t, dir, RequiredFiles[1], []byte("invalid"))
	if Collect(dir).CertificateStatus != "partial" {
		t.Fatal("invalid certificate treated as healthy")
	}
	outside := filepath.Join(t.TempDir(), "outside.crt")
	os.WriteFile(outside, certificate(t, later), 0600)
	os.Remove(filepath.Join(dir, RequiredFiles[0]))
	os.Symlink(outside, filepath.Join(dir, RequiredFiles[0]))
	if len(Collect(dir).Certificates) != 2 {
		t.Fatal("followed escaping symlink")
	}
	writeCertificate(t, dir, RequiredFiles[2], make([]byte, MaxFileSize+1))
	if len(Collect(dir).Certificates) != 1 {
		t.Fatal("accepted oversized file")
	}
	recorder := httptest.NewRecorder()
	Handler(dir).ServeHTTP(recorder, httptest.NewRequest("GET", "/certificates", nil))
	if recorder.Code != 200 {
		t.Fatal(recorder.Code)
	}
	recorder = httptest.NewRecorder()
	Handler(dir).ServeHTTP(recorder, httptest.NewRequest("POST", "/certificates", nil))
	if recorder.Code != 405 {
		t.Fatal(recorder.Code)
	}
}
