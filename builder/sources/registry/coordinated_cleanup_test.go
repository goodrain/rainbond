package registry

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// capability_id: rainbond.image-clean.permitted-manifest-deletion
func TestCoordinatedCleanupClientDoesNotReplayDeletion(t *testing.T) {
	digest := "sha256:" + strings.Repeat("a", 64)
	calls, redirected := 0, 0
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { redirected++; w.WriteHeader(202) }))
	defer destination.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			calls++
			if r.Header.Get("X-Rainbond-Cleanup-Permit") != "fixture-permit" {
				t.Error("missing permit")
			}
			w.Header().Set("Location", destination.URL)
			w.WriteHeader(307)
			return
		}
		if r.Method == "HEAD" {
			w.Header().Set("Docker-Content-Digest", digest)
			return
		}
		fmt.Fprint(w, `{"name":"app","tags":["one","two"]}`)
	}))
	defer server.Close()
	client, err := NewCoordinatedCleanupClient(server.URL, "", "")
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := client.Resolve(context.Background(), "app", "one")
	if err != nil || resolved != digest {
		t.Fatal(resolved, err)
	}
	tags, err := client.Tags(context.Background(), "app")
	if err != nil || len(tags) != 2 {
		t.Fatal(tags, err)
	}
	status, err := client.Delete(context.Background(), "app", digest, "fixture-permit")
	if err != nil || status != 307 || calls != 1 || redirected != 0 {
		t.Fatalf("replayed deletion: status=%d calls=%d redirected=%d err=%v", status, calls, redirected, err)
	}
}

// capability_id: rainbond.image-clean.permitted-manifest-deletion
func TestCoordinatedCleanupClientRequiresCompleteInventoryAndExactAbsence(t *testing.T) {
	status := http.StatusForbidden
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "HEAD" {
			w.WriteHeader(status)
			return
		}
		w.Header().Set("Link", "</v2/app/tags/list?last=one>; rel=next")
		fmt.Fprint(w, `{"name":"app","tags":["one"]}`)
	}))
	defer server.Close()
	client, err := NewCoordinatedCleanupClient(server.URL, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Tags(context.Background(), "app"); err == nil {
		t.Fatal("partial tag inventory accepted")
	}
	if _, err := client.Exists(context.Background(), "app", "sha256:"+strings.Repeat("a", 64)); err == nil {
		t.Fatal("forbidden interpreted as absence")
	}
	status = 404
	if exists, err := client.Exists(context.Background(), "app", "sha256:"+strings.Repeat("a", 64)); err != nil || exists {
		t.Fatal(exists, err)
	}
	if _, err := client.Delete(context.Background(), "../other", "sha256:"+strings.Repeat("a", 64), "fixture-permit"); err == nil {
		t.Fatal("invalid repository accepted")
	}
	if _, err := client.Delete(context.Background(), "app", "latest", "fixture-permit"); err == nil {
		t.Fatal("mutable deletion accepted")
	}
	if _, err := client.Delete(context.Background(), "app", "sha256:"+strings.Repeat("a", 64), ""); err == nil {
		t.Fatal("unpermitted deletion accepted")
	}
}
