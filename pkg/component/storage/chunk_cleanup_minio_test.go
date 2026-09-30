package storage

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/goodrain/rainbond/db/model"
	guard "github.com/goodrain/rainbond/pkg/cleanup"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
)

// TestMinIOChunkCleanupOwnsExactSession starts its own local server and bucket.
// It never accepts an existing endpoint or any production credentials.
// capability_id: rainbond.storage.minio-chunk-cleanup
func TestMinIOChunkCleanupOwnsExactSession(t *testing.T) {
	binary := os.Getenv("CLEANUP_TEST_MINIO_BINARY")
	if binary == "" {
		t.Skip("set trusted local MinIO binary for isolated acceptance")
	}
	root := t.TempDir()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	listener.Close()
	random := func() string {
		data := make([]byte, 24)
		if _, err := rand.Read(data); err != nil {
			t.Fatal("test randomness unavailable")
		}
		return hex.EncodeToString(data)
	}
	user, password := random(), random()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "server", filepath.Join(root, "data"), "--address", address, "--console-address", "127.0.0.1:0", "--quiet")
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + root, "TMPDIR=" + root, "MINIO_ROOT_USER=" + user, "MINIO_ROOT_PASSWORD=" + password, "MINIO_BROWSER=off", "MINIO_UPDATE=off"}
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		t.Fatal("isolated MinIO start failed")
	}
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-exited:
		case <-time.After(5 * time.Second):
			t.Error("isolated MinIO did not stop")
		}
	})
	endpoint := "http://" + address
	ready := false
	httpClient := &http.Client{Timeout: time.Second}
	for deadline := time.Now().Add(20 * time.Second); time.Now().Before(deadline); {
		response, err := httpClient.Get(endpoint + "/minio/health/ready")
		if err == nil {
			response.Body.Close()
			if response.StatusCode == 200 {
				ready = true
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !ready {
		t.Fatal("isolated MinIO unavailable")
	}
	sess, err := session.NewSession(&aws.Config{Endpoint: aws.String(endpoint), Region: aws.String("rainbond"), Credentials: credentials.NewStaticCredentials(user, password, ""), S3ForcePathStyle: aws.Bool(true), MaxRetries: aws.Int(0)})
	if err != nil {
		t.Fatal("test client unavailable")
	}
	client := s3.New(sess)
	// HTTP health can be ready before this MinIO version initializes the S3 API.
	// Poll only a read operation; do not replay a bucket mutation on uncertainty.
	apiReady := false
	for deadline := time.Now().Add(20 * time.Second); time.Now().Before(deadline); {
		if _, err := client.ListBucketsWithContext(ctx, &s3.ListBucketsInput{}); err == nil {
			apiReady = true
			break
		}
		if ctx.Err() != nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !apiReady {
		t.Fatal("isolated MinIO S3 API unavailable")
	}

	if _, err := client.CreateBucketWithContext(ctx, &s3.CreateBucketInput{Bucket: aws.String("grdata")}); err != nil {
		if failure, ok := err.(awserr.Error); ok {
			t.Fatal("owned bucket creation failed", failure.Code())
		}
		t.Fatal("owned bucket creation failed")
	}
	selected := "package_build/temp/chunks/owned/"
	retained := "package_build/temp/chunks/owned-retained/chunk_0"
	for i := 0; i < 1001; i++ {
		key := fmt.Sprintf("%schunk_%d", selected, i)
		if _, err := client.PutObjectWithContext(ctx, &s3.PutObjectInput{Bucket: aws.String("grdata"), Key: aws.String(key), Body: strings.NewReader("owned")}); err != nil {
			if failure, ok := err.(awserr.Error); ok {
				t.Fatal("owned chunk creation failed", failure.Code())
			}
			t.Fatal("owned chunk creation failed")
		}
	}
	if _, err := client.PutObjectWithContext(ctx, &s3.PutObjectInput{Bucket: aws.String("grdata"), Key: aws.String(retained), Body: strings.NewReader("retained")}); err != nil {
		t.Fatal("retained chunk creation failed")
	}
	for key, body := range map[string]string{"package_build/temp/events/owned/app.zip": "package", "package_build/temp/events/owned/extracted/source": "source"} {
		if _, err := client.PutObjectWithContext(ctx, &s3.PutObjectInput{Bucket: aws.String("grdata"), Key: aws.String(key), Body: strings.NewReader(body)}); err != nil {
			t.Fatal("owned package creation failed")
		}
	}
	store := &S3Storage{s3Client: client}
	packageUsage, err := store.MeasureUploadEvent(ctx, "owned")
	if err != nil || packageUsage.Bytes != 13 || packageUsage.Objects != 2 {
		t.Fatal("real package inventory incorrect", packageUsage, err)
	}
	usage, err := store.MeasureUploadChunks(ctx, "owned")
	if err != nil || usage.Bytes != 5005 || usage.Objects != 1001 {
		t.Fatalf("real chunk size incorrect: %+v %v", usage, err)
	}
	database, err := gorm.Open("sqlite3", filepath.Join(root, "retirement.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := database.AutoMigrate(&model.UploadSession{}, &model.PackageUploadUse{}).Error; err != nil {
		t.Fatal(err)
	}
	upload := model.UploadSession{ID: "owned", EventID: "event", Status: "uploading", ExpiresAt: time.Now().Add(-time.Hour)}
	if err := database.Create(&upload).Error; err != nil {
		t.Fatal(err)
	}
	fingerprint := guard.UploadSessionFingerprint(upload)
	deletes := 0
	remove := func(id string) error { deletes++; return store.CleanupChunks(id) }
	for attempt := 0; attempt < 2; attempt++ {
		if err := guard.DeleteExpiredUploadChunks(database, "owned-retirement", "event", "owned", fingerprint, remove); err != nil {
			t.Fatal("real chunk retirement failed", err)
		}
	}
	if deletes != 1 {
		t.Fatal("real object deletion replayed")
	}

	usage, err = store.MeasureUploadChunks(ctx, "owned")
	if err != nil || usage.Bytes != 0 || usage.Objects != 0 {
		t.Fatalf("deleted scope measurement incorrect: %+v %v", usage, err)
	}
	retainedUsage, err := store.MeasureUploadChunks(ctx, "owned-retained")
	if err != nil || retainedUsage.Bytes != 8 || retainedUsage.Objects != 1 {
		t.Fatalf("retained scope measurement incorrect: %+v %v", retainedUsage, err)
	}
	packageUsage, err = store.MeasureUploadEvent(ctx, "owned")
	if err != nil || packageUsage.Bytes != 13 || packageUsage.Objects != 2 {
		t.Fatal("chunk cleanup changed completed package", packageUsage, err)
	}
	remaining, err := client.ListObjectsV2WithContext(ctx, &s3.ListObjectsV2Input{Bucket: aws.String("grdata"), Prefix: aws.String(selected)})
	if err != nil || len(remaining.Contents) != 0 {
		t.Fatal("selected chunks remain")
	}
	result, err := client.GetObjectWithContext(ctx, &s3.GetObjectInput{Bucket: aws.String("grdata"), Key: aws.String(retained)})
	if err != nil {
		t.Fatal("unselected session removed")
	}
	defer result.Body.Close()
	data, err := io.ReadAll(result.Body)
	if err != nil || string(data) != "retained" {
		t.Fatal("unselected chunk changed")
	}
	t.Log("deleted 1001 selected chunks across pages; sibling session remained readable")
}
