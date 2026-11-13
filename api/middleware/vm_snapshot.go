package middleware

import "net/http"

// SnapshotCommand preserves the existing license check while snapshot events are
// owned by the persistent operation and completed only after its final ACK.
func SnapshotCommand(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := LicenseVerification(r, true); err != nil {
			err.Handle(r, w)
			return
		}
		next(w, r)
	}
}
