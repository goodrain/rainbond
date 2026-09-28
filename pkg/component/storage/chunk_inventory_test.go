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

// capability_id: rainbond.storage.chunk-size-inventory
func TestUploadChunkUsageRequiresCompletePhysicalInventory(t *testing.T) {
	for _, scenario := range []string{"pages", "empty", "missing-size", "negative", "overflow", "foreign", "duplicate", "bad-cursor", "unavailable"} {
		t.Run(scenario, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != http.MethodGet {
					t.Error("measurement mutated storage")
					w.WriteHeader(400)
					return
				}
				prefix := "package_build/temp/chunks/owned/"
				if r.URL.Query().Get("prefix") != prefix {
					t.Error("unscoped inventory")
				}
				if scenario == "unavailable" {
					w.WriteHeader(403)
					return
				}
				key, size, truncated, token := prefix+"chunk_0", "<Size>17</Size>", "false", ""
				if scenario == "pages" || scenario == "duplicate" || scenario == "bad-cursor" {
					if calls == 1 || scenario == "bad-cursor" {
						truncated = "true"
						token = "<NextContinuationToken>next</NextContinuationToken>"
					} else if scenario == "pages" {
						key = prefix + "chunk_1"
						size = "<Size>23</Size>"
					}
				}
				switch scenario {
				case "missing-size":
					size = ""
				case "negative":
					size = "<Size>-1</Size>"
				case "overflow":
					size = "<Size>9223372036854775807</Size>"
				case "foreign":
					key = "package_build/temp/chunks/owned-other/chunk_0"
				}
				w.Header().Set("Content-Type", "application/xml")
				fmt.Fprintf(w, "<ListBucketResult><Name>grdata</Name><Prefix>%s</Prefix><IsTruncated>%s</IsTruncated>%s", prefix, truncated, token)
				if scenario != "empty" {
					fmt.Fprintf(w, "<Contents><Key>%s</Key>%s</Contents>", key, size)
				}
				if scenario == "overflow" {
					fmt.Fprintf(w, "<Contents><Key>%schunk_1</Key><Size>1</Size></Contents>", prefix)
				}
				fmt.Fprint(w, "</ListBucketResult>")
			}))
			defer server.Close()
			client := s3.New(session.Must(session.NewSession(&aws.Config{Endpoint: aws.String(server.URL), Region: aws.String("test"), S3ForcePathStyle: aws.Bool(true), Credentials: credentials.AnonymousCredentials, MaxRetries: aws.Int(0)})))
			store := &S3Storage{s3Client: client}
			usage, err := store.MeasureUploadChunks(context.Background(), "owned")
			good := scenario == "pages" || scenario == "empty"
			if (err == nil) != good {
				t.Fatalf("unexpected result: %+v %v", usage, err)
			}
			if scenario == "pages" && (usage.Bytes != 40 || usage.Objects != 2 || calls != 2) {
				t.Fatalf("truncated or inaccurate measurement: %+v calls=%d", usage, calls)
			}
			if !good && (usage.Bytes != 0 || usage.Objects != 0) {
				t.Fatal("partial inventory escaped as a measurement")
			}
			before := calls
			if _, err := store.MeasureUploadChunks(context.Background(), "../other"); err == nil || calls != before {
				t.Fatal("invalid scope reached storage")
			}
		})
	}
}

// capability_id: rainbond.storage.local-chunk-size-inventory
func TestLocalUploadChunkUsageReadsFilesWithoutFollowingLinks(t *testing.T) {
	for _, scenario := range []string{"files", "empty", "missing", "symlink", "directory"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			if scenario == "files" {
				for name, body := range map[string]string{"chunk_0": "hello", "chunk_1": "world!"} {
					if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0600); err != nil {
						t.Fatal(err)
					}
				}
			}
			if scenario == "missing" {
				root = filepath.Join(root, "absent")
			}
			if scenario == "symlink" {
				if err := os.Symlink(t.TempDir(), filepath.Join(root, "chunk_0")); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "directory" {
				if err := os.Mkdir(filepath.Join(root, "unexpected"), 0700); err != nil {
					t.Fatal(err)
				}
			}
			usage, err := measureLocalChunkDirectory(context.Background(), root)
			good := scenario == "files" || scenario == "empty"
			if (err == nil) != good {
				t.Fatalf("unexpected usage %+v: %v", usage, err)
			}
			if scenario == "files" && (usage.Bytes != 11 || usage.Objects != 2) {
				t.Fatalf("incorrect physical file sizes %+v", usage)
			}
			if !good && usage != (UploadChunkUsage{}) {
				t.Fatal("partial local size escaped")
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := measureLocalChunkDirectory(ctx, t.TempDir()); err == nil {
		t.Fatal("ignored cancellation")
	}
	if _, err := (&LocalStorage{}).MeasureUploadChunks(context.Background(), "../foreign"); err == nil {
		t.Fatal("accepted invalid scope")
	}
}
