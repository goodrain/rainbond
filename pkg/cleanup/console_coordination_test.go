package cleanup

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

// capability_id: rainbond.cleanup.console-signed-coordinator
func TestConsoleCoordinationUsesScopedSignatureWithoutAdminCredential(t *testing.T) {
	key := bytes.Repeat([]byte("x"), 32)
	expectedKey := append([]byte(nil), key...)
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		path := "/console/cleanup/internal/coordination/enterprise/rainbond"
		raw, _ := io.ReadAll(r.Body)
		digest := sha256.Sum256(raw)
		msg := strings.Join([]string{"cleanup-coordination-v1", "POST", path, "enterprise", "rainbond", r.Header.Get("X-Cleanup-Coordination-Time"), hex.EncodeToString(digest[:])}, "\n")
		mac := hmac.New(sha256.New, expectedKey)
		mac.Write([]byte(msg))
		if r.URL.Path != path || r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" || r.Header.Get("X-Cleanup-Coordination-Signature") != hex.EncodeToString(mac.Sum(nil)) {
			t.Error("invalid scoped control request")
		}
		var body struct{ Path, Body string }
		if json.Unmarshal(raw, &body) != nil || body.Path != "/v2/cleanup/stores/discover" || body.Body != "{}" {
			t.Error("changed original control request")
		}
		w.Write([]byte(`{"bean":{"protocol":1,"recorded":true}}`))
	}))
	defer server.Close()
	client, err := NewConsoleCoordinationClient(server.URL, ConsoleCoordinationScope{Enterprise: "enterprise", Region: "rainbond", Key: key}, true, http.DefaultTransport)
	if err != nil {
		t.Fatal(err)
	}
	key[0] = 'z'
	client.token = "must-not-forward"
	if _, err := client.callPath(context.Background(), "/v2/cleanup/stores/discover", struct{}{}); err != nil || calls != 1 {
		t.Fatal("signed request failed", err, calls)
	}
	for _, scope := range []ConsoleCoordinationScope{{Enterprise: "other/path", Region: "rainbond", Key: key}, {Enterprise: "enterprise", Region: "", Key: key}, {Enterprise: "enterprise", Region: "rainbond"}} {
		if _, err := NewConsoleCoordinationClient(server.URL, scope, true, http.DefaultTransport); err == nil {
			t.Fatal("invalid scope accepted")
		}
	}
}

func TestConsoleCoordinationRejectsUnsafeScopePathsAndDoesNotRetry(t *testing.T) {
	for _, status := range []int{307, 403, 503} {
		calls := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls++
			w.Header().Set("Location", "/unexpected")
			w.WriteHeader(status)
			w.Write([]byte(`{"msg":"COORDINATION_UNAVAILABLE"}`))
		}))
		scope := ConsoleCoordinationScope{Enterprise: "enterprise", Region: "rainbond", Key: bytes.Repeat([]byte("x"), 32)}
		client, err := NewConsoleCoordinationClient(server.URL, scope, true, http.DefaultTransport)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := client.callPath(context.Background(), "/v2/cleanup/stores/discover", struct{}{}); err == nil || calls != 1 {
			t.Fatal("retried/followed failed control request", status, calls)
		}
		for _, path := range []string{"/v2/tenants/example", "/v2/cleanup/../tenants", "/v2/cleanup/stores?extra=1"} {
			if _, err := client.callPath(context.Background(), path, struct{}{}); err == nil || calls != 1 {
				t.Fatal("forwarded forbidden path")
			}
		}
		if _, err := client.callPath(context.Background(), "/v2/cleanup/stores/discover", map[string]string{"padding": strings.Repeat("x", 16385)}); err == nil || calls != 1 {
			t.Fatal("forwarded oversized envelope")
		}
		if _, err := NewConsoleCoordinationClient(server.URL, scope, false, http.DefaultTransport); err == nil {
			t.Fatal("accepted implicit plaintext transport")
		}
		if _, err := NewCoordinationClient(server.URL, "", true, http.DefaultTransport); err == nil {
			t.Fatal("weakened legacy authentication")
		}
		server.Close()
	}
}

func TestSystemCoordinationBindsSeparateEndpoint(t *testing.T) {
	for _, system := range []bool{false, true} {
		t.Run(strconv.FormatBool(system), func(t *testing.T) {
			key := bytes.Repeat([]byte("x"), 64)
			route := "coordination"
			if system {
				route = "system-coordination"
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				path := "/console/cleanup/internal/" + route + "/enterprise/rainbond"
				raw, _ := io.ReadAll(r.Body)
				sum := sha256.Sum256(raw)
				mac := hmac.New(sha256.New, key)
				mac.Write([]byte(strings.Join([]string{"cleanup-coordination-v1", "POST", path, "enterprise", "rainbond", r.Header.Get("X-Cleanup-Coordination-Time"), hex.EncodeToString(sum[:])}, "\n")))
				if r.URL.Path != path || r.Header.Get("X-Cleanup-Coordination-Signature") != hex.EncodeToString(mac.Sum(nil)) {
					t.Error("wrong trust boundary")
				}
				w.Write([]byte(`{"bean":{"protocol":1,"recorded":true}}`))
			}))
			defer server.Close()
			client, err := NewConsoleCoordinationClient(server.URL, ConsoleCoordinationScope{Enterprise: "enterprise", Region: "rainbond", Key: key, System: system}, true, http.DefaultTransport)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = client.callPath(context.Background(), "/v2/cleanup/stores/discover", struct{}{}); err != nil {
				t.Fatal(err)
			}
		})
	}
	if !(ConsoleCoordinationScope{System: true}).Configured() {
		t.Fatal("system mode silently downgraded")
	}
}
