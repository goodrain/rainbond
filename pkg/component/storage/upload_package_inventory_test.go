package storage

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

// capability_id: rainbond.storage.upload-package-size
func TestUploadPackageSizeIsScopedAndIncludesExtractedFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "extracted"), 0700); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"app.zip": "package", "extracted/source": "source"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	usage, err := measureLocalUploadEvent(context.Background(), root)
	if err != nil || usage.Bytes != 13 || usage.Objects != 2 {
		t.Fatalf("incomplete package size %+v: %v", usage, err)
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "linked")); err != nil {
		t.Fatal(err)
	}
	if _, err := measureLocalUploadEvent(context.Background(), root); err == nil {
		t.Fatal("followed unowned linked directory")
	}
	if _, err := (&LocalStorage{}).MeasureUploadEvent(context.Background(), "../foreign"); err == nil {
		t.Fatal("invalid event accepted")
	}
	if _, err := measureLocalUploadEvent(context.Background(), filepath.Join(outside, "missing")); err == nil {
		t.Fatal("missing mount reported as empty")
	}
}

func TestS3UploadPackageMeasurementUsesExactEventPrefix(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		prefix := "package_build/temp/events/owned/"
		if r.Method != http.MethodGet || r.URL.Query().Get("prefix") != prefix {
			t.Error("wrong event measurement scope")
		}
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprintf(w, "<ListBucketResult><Name>grdata</Name><Prefix>%s</Prefix><IsTruncated>false</IsTruncated><Contents><Key>%sapp.zip</Key><Size>31</Size></Contents><Contents><Key>%sextracted/source</Key><Size>17</Size></Contents></ListBucketResult>", prefix, prefix, prefix)
	}))
	defer server.Close()
	client := s3.New(session.Must(session.NewSession(&aws.Config{Endpoint: aws.String(server.URL), Region: aws.String("test"), S3ForcePathStyle: aws.Bool(true), Credentials: credentials.AnonymousCredentials, MaxRetries: aws.Int(0)})))
	store := &S3Storage{s3Client: client}
	usage, err := store.MeasureUploadEvent(context.Background(), "owned")
	if err != nil || usage.Bytes != 48 || usage.Objects != 2 {
		t.Fatal("incorrect event size", usage, err)
	}
	if _, err := store.MeasureUploadEvent(context.Background(), "../other"); err == nil || calls != 1 {
		t.Fatal("invalid event reached storage")
	}
}
