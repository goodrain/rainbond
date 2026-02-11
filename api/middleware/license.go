package middleware

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/goodrain/rainbond/api/handler"
	rsalicense "github.com/goodrain/rainbond/api/util/license"
	"github.com/goodrain/rainbond/pkg/component/k8s"
	httputil "github.com/goodrain/rainbond/util/http"
	"github.com/sirupsen/logrus"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// License holds license middleware state.
type License struct{}

// NewLicense creates a new License middleware.
func NewLicense() *License {
	// Wire up cache invalidation callback to handler
	handler.InvalidateLicenseCacheFunc = InvalidateRSALicenseCache
	return &License{}
}

// rsaLicenseCache caches RSA-based license verification results.
var rsaLicenseCache struct {
	sync.RWMutex
	token    *rsalicense.LicenseToken
	valid    bool
	reason   string
	expireAt time.Time
}

// InvalidateRSALicenseCache clears the RSA license cache.
func InvalidateRSALicenseCache() {
	rsaLicenseCache.Lock()
	rsaLicenseCache.token = nil
	rsaLicenseCache.valid = false
	rsaLicenseCache.reason = ""
	rsaLicenseCache.expireAt = time.Time{}
	rsaLicenseCache.Unlock()
}

// Verify parses the license to make the content inside it take effect.
func (l *License) Verify(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if enterprise plugin exists
		_, err := k8s.Default().RainbondClient.RainbondV1alpha1().RBDPlugins(metav1.NamespaceNone).Get(
			context.TODO(), "rainbond-enterprise-base", metav1.GetOptions{})
		if err != nil {
			// Open source version, no license needed
			next.ServeHTTP(w, r)
			return
		}

		// RSA-based ConfigMap verification
		if verifyRSALicense() {
			next.ServeHTTP(w, r)
			return
		}

		httputil.Return(r, w, 401, httputil.ResponseBody{
			Bean: map[string]interface{}{
				"msg":  "invalid license",
				"code": 10400,
			},
		})
	})
}

// verifyRSALicense checks the ConfigMap-based RSA license.
func verifyRSALicense() bool {
	now := time.Now()

	// Check cache
	rsaLicenseCache.RLock()
	if rsaLicenseCache.token != nil && rsaLicenseCache.expireAt.After(now) {
		valid := rsaLicenseCache.valid
		rsaLicenseCache.RUnlock()
		return valid
	}
	rsaLicenseCache.RUnlock()

	// Read from ConfigMap
	ctx := context.TODO()
	cm, err := k8s.Default().Clientset.CoreV1().ConfigMaps("rbd-system").Get(ctx, "rbd-license-info", metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			logrus.Debug("No rbd-license-info ConfigMap found")
		} else {
			logrus.Errorf("Failed to read license ConfigMap: %v", err)
		}
		return false
	}

	licenseData := cm.Data["license_token"]
	if licenseData == "" {
		return false
	}

	// Parse JSON directly (ConfigMap stores plain JSON, not base64)
	token, err := rsalicense.ParseLicenseJSON(licenseData)
	if err != nil {
		logrus.Errorf("Failed to decode RSA license: %v", err)
		cacheRSAResult(nil, false, err.Error(), now)
		return false
	}

	// Get public key
	pubKey, err := rsalicense.GetEmbeddedPublicKey()
	if err != nil {
		logrus.Errorf("Failed to get embedded public key: %v", err)
		return false
	}

	// Verify signature
	if err := rsalicense.VerifySignature(token, pubKey); err != nil {
		logrus.Errorf("RSA license signature verification failed: %v", err)
		cacheRSAResult(token, false, "invalid signature", now)
		return false
	}

	// Verify time window
	unix := now.Unix()
	if unix < token.StartAt {
		cacheRSAResult(token, false, "license not yet valid", now)
		return false
	}
	if unix > token.ExpireAt {
		cacheRSAResult(token, false, "license expired", now)
		return false
	}

	// Valid
	cacheRSAResult(token, true, "", now)
	return true
}

func cacheRSAResult(token *rsalicense.LicenseToken, valid bool, reason string, now time.Time) {
	rsaLicenseCache.Lock()
	rsaLicenseCache.token = token
	rsaLicenseCache.valid = valid
	rsaLicenseCache.reason = reason
	rsaLicenseCache.expireAt = now.Add(10 * time.Minute)
	rsaLicenseCache.Unlock()
}
