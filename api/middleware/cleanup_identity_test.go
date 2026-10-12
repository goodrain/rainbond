package middleware

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"net/http/httptest"
	"testing"
)

// capability_id: rainbond.cleanup.verified-client-identity
func TestCleanupIdentityRequiresVerifiedClientOrConfiguredToken(t *testing.T) {
	cert := &x509.Certificate{Raw: []byte("fixture-client")}
	other := &x509.Certificate{Raw: []byte("other-client")}
	verified := &tls.ConnectionState{PeerCertificates: []*x509.Certificate{cert}, VerifiedChains: [][]*x509.Certificate{{cert}}}
	for _, tc := range []struct {
		name, token, header string
		state               *tls.ConnectionState
		allowed             bool
	}{
		{name: "anonymous"},
		{name: "unverified certificate", state: &tls.ConnectionState{PeerCertificates: []*x509.Certificate{cert}}},
		{name: "verified client", state: verified, allowed: true},
		{name: "mismatched chain", state: &tls.ConnectionState{PeerCertificates: []*x509.Certificate{other}, VerifiedChains: verified.VerifiedChains}},
		{name: "configured token required even with certificate", token: "fixture", state: verified},
		{name: "configured token", token: "fixture", header: "Token fixture", allowed: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("TOKEN", tc.token)
			r := httptest.NewRequest("POST", "/v2/cleanup/stores/discover", nil)
			r.TLS = tc.state
			r.Header.Set("Authorization", tc.header)
			r.Header.Set("X-Forwarded-Client-Cert", "untrusted-claim")
			w := httptest.NewRecorder()
			CleanupIdentity(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })).ServeHTTP(w, r)
			if (w.Code == 204) != tc.allowed {
				t.Fatalf("unexpected status %d", w.Code)
			}
		})
	}
}
