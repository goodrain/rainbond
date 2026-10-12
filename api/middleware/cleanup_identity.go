package middleware

import (
	"bytes"
	"net/http"
	"os"
)

// CleanupIdentity accepts the platform's verified mutual TLS identity when
// token authentication is not configured. Configured tokens remain mandatory.
// Client identity comes only from the TLS handshake, never proxy headers.
func CleanupIdentity(next http.Handler) http.Handler {
	tokenHandler := FullToken(next)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if os.Getenv("TOKEN") == "" && r.TLS != nil && len(r.TLS.PeerCertificates) > 0 {
			peer := r.TLS.PeerCertificates[0]
			if peer != nil && len(peer.Raw) > 0 {
				for _, chain := range r.TLS.VerifiedChains {
					if len(chain) > 0 && chain[0] != nil && bytes.Equal(chain[0].Raw, peer.Raw) {
						next.ServeHTTP(w, r)
						return
					}
				}
			}
		}
		tokenHandler.ServeHTTP(w, r)
	})
}
