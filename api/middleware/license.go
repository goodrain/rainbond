package middleware

import (
	"github.com/goodrain/rainbond/config/configs"
	"net/http"

	httputil "github.com/goodrain/rainbond/util/http"
	licutil "github.com/goodrain/rainbond/util/license"
)

// License -
type License struct {
	LicensePath string `json:"license_path"`
	LicSoPath   string `json:"lic_so_path"`
}

// NewLicense -
func NewLicense() *License {
	apiConfig := configs.Default().APIConfig
	return &License{
		LicensePath: apiConfig.LicensePath,
		LicSoPath:   apiConfig.LicSoPath,
	}
}

// Verify parses the license to make the content inside it take effect.
func (l *License) Verify(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !licutil.VerifyTime(l.LicensePath, l.LicSoPath) {
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
