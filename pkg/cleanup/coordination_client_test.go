package cleanup

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCoordinationClientBindsPathsAndNeverRetriesWrites(t *testing.T) {
	for _, status := range []int{200, 409, 503, 307} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.URL.Path != "/v2/cleanup/stores/store/operations" || r.Method != "POST" || r.Header.Get("Authorization") != "Token fixture-only" {
					t.Error("wrong request binding")
				}
				w.Header().Set("Location", "/should-not-follow")
				w.WriteHeader(status)
				fmt.Fprint(w, `{"bean":{"protocol":1,"newly_admitted":true}}`)
			}))
			defer server.Close()
			client, err := NewCoordinationClient(server.URL, "fixture-only", true)
			if err != nil {
				t.Fatal(err)
			}
			admitted, err := client.Acquire(context.Background(), operation("op", "producer", "app/a"))
			if calls != 1 {
				t.Fatal("write retried or redirected", calls)
			}
			if status == 200 && (err != nil || !admitted) {
				t.Fatal(admitted, err)
			}
			if status != 200 && (err == nil || admitted) {
				t.Fatal("invalid status admitted operation", admitted, err)
			}
		})
	}
}
func TestCoordinationClientRequiresExplicitProtocolAndAdmission(t *testing.T) {
	for _, body := range []string{`{}`, `{"bean":{"newly_admitted":true}}`, `{"bean":{"protocol":1}}`, `{"bean":{"protocol":2,"newly_admitted":true}}`, `{"bean":{"protocol":1,"newly_admitted":true}} {}`} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) }))
		client, err := NewCoordinationClient(server.URL, "fixture-only", true)
		if err != nil {
			t.Fatal(err)
		}
		admitted, err := client.Acquire(context.Background(), operation("op", "producer", "app/a"))
		server.Close()
		if err == nil || admitted {
			t.Fatal("invalid acknowledgement admitted operation")
		}
	}
}

func TestPrepareRegistryClientRejectsMismatchedBinding(t *testing.T) {
	for _, matches := range []bool{true, false} {
		binding := StorageRegistration{StorageID: "derived", Generation: "one", VolumeUID: "observed", RootPath: "/var/lib/registry"}
		fingerprint, _ := binding.Fingerprint()
		if !matches {
			fingerprint = "wrong"
		}
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v2/cleanup/registry/prepare" {
				t.Error("wrong preparation path")
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"bean": map[string]interface{}{"protocol": 1, "registration": binding, "storage": StorageObservation{StorageID: binding.StorageID, Generation: binding.Generation, RegistrationFingerprint: fingerprint, Mode: "collecting"}, "registry_container": "registry"}})
		}))
		client, err := NewCoordinationClient(server.URL, "fixture-only", true)
		if err != nil {
			t.Fatal(err)
		}
		observed, err := client.PrepareRegistry(context.Background(), "pod", "uid")
		server.Close()
		if matches && (err != nil || observed.Registration != binding) {
			t.Fatal("valid preparation rejected", err)
		}
		if !matches && err == nil {
			t.Fatal("mismatched preparation accepted")
		}
	}
}
