//+build !license

package middleware

import (
	"net/http"

	"github.com/goodrain/rainbond/cmd/api/option"
)

// License -
type License struct {
	cfg *option.Config
}

// NewLicense -
func NewLicense(cfg *option.Config) *License{
	return &License{
		cfg: cfg,
	}
}

// Verify parses the license to make the content inside it take effect.
func (l *License) Verify(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	})
}
