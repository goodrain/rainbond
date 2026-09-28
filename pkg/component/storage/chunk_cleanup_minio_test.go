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
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
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
	if _, err := client.CreateBucketWithContext(ctx, &s3.CreateBucketInput{Bucket: aws.String("grdata")}); err != nil {
		t.Fatal("owned bucket creation failed")
	}
	selected := "package_build/temp/chunks/owned/"
	retained := "package_build/temp/chunks/owned-retained/chunk_0"
	for i := 0; i < 1001; i++ {
		key := fmt.Sprintf("%schunk_%d", selected, i)
		if _, err := client.PutObjectWithContext(ctx, &s3.PutObjectInput{Bucket: aws.String("grdata"), Key: aws.String(key), Body: strings.NewReader("owned")}); err != nil {
			t.Fatal("owned chunk creation failed")
		}
	}
	if _, err := client.PutObjectWithContext(ctx, &s3.PutObjectInput{Bucket: aws.String("grdata"), Key: aws.String(retained), Body: strings.NewReader("retained")}); err != nil {
		t.Fatal("retained chunk creation failed")
	}
	store := &S3Storage{s3Client: client}
	if err := store.CleanupChunks("owned"); err != nil {
		t.Fatal("real chunk cleanup failed", err)
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
