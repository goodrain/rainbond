// Package nodecert exposes only expiration metadata for fixed public RKE2 certificates.
package nodecert

import (
	"bytes"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// MaxFileSize bounds reads of public certificate chains.
const MaxFileSize = 1024 * 1024

// RequiredFiles are the active Agent certificate paths relative to the RKE2 data directory.
var RequiredFiles = []string{"agent/client-kubelet.crt", "agent/serving-kubelet.crt", "agent/client-kube-proxy.crt", "agent/client-rke2-controller.crt"}

// Certificate contains no raw certificate material.
type Certificate struct {
	Name      string `json:"name"`
	ExpiresAt string `json:"expires_at"`
}

// Result describes one fresh read, including incomplete coverage.
type Result struct {
	CollectedAt       string        `json:"collected_at"`
	CertificateStatus string        `json:"certificate_status"`
	Certificates      []Certificate `json:"certificates"`
}

// Collect reads the current public files on every request so renewal is visible immediately.
func Collect(dir string) Result {
	result := Result{CollectedAt: time.Now().UTC().Format(time.RFC3339), CertificateStatus: "unavailable", Certificates: []Certificate{}}
	root, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return result
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return result
	}
	for _, name := range RequiredFiles {
		expiry, err := readExpiry(root, name)
		if err == nil {
			result.Certificates = append(result.Certificates, Certificate{Name: name, ExpiresAt: expiry.UTC().Format(time.RFC3339)})
		}
	}
	if len(result.Certificates) == len(RequiredFiles) {
		result.CertificateStatus = "ok"
	} else if len(result.Certificates) > 0 {
		result.CertificateStatus = "partial"
	}
	return result
}
func readExpiry(root, name string) (time.Time, error) {
	path, err := filepath.EvalSymlinks(filepath.Join(root, name))
	if err != nil {
		return time.Time{}, err
	}
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return time.Time{}, fmt.Errorf("certificate path escapes data directory")
	}
	f, err := os.Open(path)
	if err != nil {
		return time.Time{}, err
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		return time.Time{}, err
	}
	if !stat.Mode().IsRegular() || stat.Size() > MaxFileSize {
		return time.Time{}, fmt.Errorf("invalid certificate file")
	}
	data, err := io.ReadAll(io.LimitReader(f, MaxFileSize+1))
	if err != nil || len(data) > MaxFileSize {
		return time.Time{}, fmt.Errorf("certificate read failed")
	}
	var earliest time.Time
	for len(bytes.TrimSpace(data)) > 0 {
		block, rest := pem.Decode(data)
		if block == nil || block.Type != "CERTIFICATE" {
			return time.Time{}, fmt.Errorf("invalid public certificate chain")
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return time.Time{}, err
		}
		if earliest.IsZero() || cert.NotAfter.Before(earliest) {
			earliest = cert.NotAfter
		}
		data = rest
	}
	if earliest.IsZero() {
		return earliest, fmt.Errorf("empty public certificate chain")
	}
	return earliest, nil
}

// Handler serves metadata without using Kubernetes credentials.
func Handler(dir string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/certificates", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(Collect(dir))
	})
	return mux
}

// Run starts the standalone collector without initializing other API components.
func Run() error {
	dir := os.Getenv("RKE2_CERTIFICATE_DATA_DIR")
	if dir == "" {
		dir = "/host-var-lib/rancher/rke2"
	}
	server := &http.Server{Addr: ":9351", Handler: Handler(dir), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second, IdleTimeout: 30 * time.Second}
	return server.ListenAndServe()
}
