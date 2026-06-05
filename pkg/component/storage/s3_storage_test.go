package storage

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/sirupsen/logrus"
)

// capability_id: rainbond.storage.s3-lifecycle-skip-logs
func TestEnsureBucketExistsDoesNotLogInfoWhenLifecycleAlreadyConfigured(t *testing.T) {
	t.Helper()

	var lifecycleRequests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodHead && r.URL.Path == "/grdata":
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodGet && r.URL.Path == "/grdata" && strings.Contains(r.URL.RawQuery, "lifecycle"):
			lifecycleRequests++
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<LifecycleConfiguration xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Rule>
    <ID>delete-chunks-1d</ID>
    <Status>Enabled</Status>
  </Rule>
</LifecycleConfiguration>`))
		default:
			t.Fatalf("unexpected request: %s %s?%s", r.Method, r.URL.Path, r.URL.RawQuery)
		}
	}))
	defer server.Close()

	sess, err := session.NewSession(&aws.Config{
		Endpoint:         aws.String(server.URL),
		Region:           aws.String("rainbond"),
		Credentials:      credentials.NewStaticCredentials("access", "secret", ""),
		S3ForcePathStyle: aws.Bool(true),
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	storage := &S3Storage{s3Client: s3.New(sess)}

	logger := logrus.StandardLogger()
	originalOut := logger.Out
	originalLevel := logger.GetLevel()
	var logBuffer bytes.Buffer
	logger.SetOutput(&logBuffer)
	logger.SetLevel(logrus.InfoLevel)
	defer logger.SetOutput(originalOut)
	defer logger.SetLevel(originalLevel)

	if err := storage.ensureBucketExists("grdata"); err != nil {
		t.Fatalf("first ensureBucketExists: %v", err)
	}
	if err := storage.ensureBucketExists("grdata"); err != nil {
		t.Fatalf("second ensureBucketExists: %v", err)
	}

	if lifecycleRequests != 2 {
		t.Fatalf("expected lifecycle lookup on each ensureBucketExists call, got %d", lifecycleRequests)
	}

	if logBuffer.Len() != 0 {
		t.Fatalf("expected no info logs for already-configured lifecycle, got %q", logBuffer.String())
	}
}
