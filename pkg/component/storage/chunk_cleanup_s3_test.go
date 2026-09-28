package storage

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

// capability_id: rainbond.storage.chunk-cleanup-confirmation
func TestChunkCleanupRequiresCompleteScopedAcknowledgements(t *testing.T) {
	for _, scenario := range []string{"multi-page", "partial-delete", "foreign-prefix", "delete-lost", "remaining"} {
		t.Run(scenario, func(t *testing.T) {
			prefix := "package_build/temp/chunks/owned/"
			deletes, gets := 0, 0
			deleted := false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/xml")
				if r.Method == http.MethodGet {
					gets++
					key := prefix + "chunk_0"
					truncated := "false"
					token := ""
					if deleted {
						if scenario == "remaining" {
							key = prefix + "chunk_1"
						} else {
							key = ""
						}
					}
					if scenario == "foreign-prefix" {
						key = "package_build/temp/chunks/other/chunk_0"
					}
					if scenario == "multi-page" && !deleted {
						if r.URL.Query().Get("continuation-token") == "" {
							truncated = "true"
							token = "<NextContinuationToken>next</NextContinuationToken>"
						} else {
							key = prefix + "chunk_1"
						}
					}
					fmt.Fprintf(w, "<ListBucketResult><Name>grdata</Name><Prefix>%s</Prefix><IsTruncated>%s</IsTruncated>%s", r.URL.Query().Get("prefix"), truncated, token)
					if key != "" {
						fmt.Fprintf(w, "<Contents><Key>%s</Key><Size>1</Size></Contents>", key)
					}
					fmt.Fprint(w, "</ListBucketResult>")
					return
				}
				if r.Method != http.MethodPost {
					t.Error("unexpected mutation", r.Method)
					w.WriteHeader(400)
					return
				}
				deletes++
				var body struct {
					Objects []struct {
						Key string `xml:"Key"`
					} `xml:"Object"`
				}
				if err := xml.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				if scenario == "delete-lost" {
					w.WriteHeader(500)
					fmt.Fprint(w, "<Error><Code>InternalError</Code></Error>")
					return
				}
				if scenario == "partial-delete" {
					fmt.Fprintf(w, "<DeleteResult><Error><Key>%s</Key><Code>AccessDenied</Code></Error></DeleteResult>", prefix+"chunk_0")
					return
				}
				if scenario == "multi-page" && len(body.Objects) != 2 {
					t.Error("truncated inventory deleted", len(body.Objects))
				}
				deleted = true
				fmt.Fprint(w, "<DeleteResult>")
				for _, object := range body.Objects {
					fmt.Fprintf(w, "<Deleted><Key>%s</Key></Deleted>", object.Key)
				}
				fmt.Fprint(w, "</DeleteResult>")
			}))
			defer server.Close()
			sess, err := session.NewSession(&aws.Config{Endpoint: aws.String(server.URL), Region: aws.String("rainbond"), Credentials: credentials.NewStaticCredentials("fixture-access", "fixture-secret", ""), S3ForcePathStyle: aws.Bool(true), MaxRetries: aws.Int(2)})
			if err != nil {
				t.Fatal(err)
			}
			storage := &S3Storage{s3Client: s3.New(sess)}
			err = storage.CleanupChunks("owned")
			if (err == nil) != (scenario == "multi-page") {
				t.Fatal("incorrect cleanup result", err)
			}
			expected := 1
			if scenario == "foreign-prefix" {
				expected = 0
			}
			if deletes != expected {
				t.Fatal("unsafe or repeated deletion", deletes)
			}
			if scenario == "multi-page" && gets != 3 {
				t.Fatal("missing complete listing or final check", gets)
			}
		})
	}
}
