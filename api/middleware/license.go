package middleware

import (
	"net/http"

	"github.com/goodrain/rainbond/cmd/api/option"
	httputil "github.com/goodrain/rainbond/util/http"
	licutil "github.com/goodrain/rainbond/util/license"
)

// License -
type License struct {
	cfg *option.Config
}

// NewLicense -
func NewLicense(cfg *option.Config) *License {
	return &License{
		cfg: cfg,
	}
}

// Verify parses the license to make the content inside it take effect.
func (l *License) Verify(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !licutil.VerifyTime(l.cfg.LicensePath, l.cfg.LicSoPath) {
			httputil.Return(r, w, 401, httputil.ResponseBody{
				Bean: map[string]interface{}{
					"msg":  "invalid license",
					"code": 10400,
				},
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}
