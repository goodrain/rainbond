package controller

import (
	"net/http"

	"github.com/goodrain/rainbond/api/handler"
	httputil "github.com/goodrain/rainbond/util/http"
	"github.com/sirupsen/logrus"
)

// LicenseManager license manager
type LicenseManager struct{}

var licenseManager *LicenseManager

// GetLicenseManager get license Manager
func GetLicenseManager() *LicenseManager {
	if licenseManager != nil {
		return licenseManager
	}
	licenseManager = &LicenseManager{}
	return licenseManager
}

// GetlicenseFeature returns license features (now returns status from RSA license).
func (l *LicenseManager) GetlicenseFeature(w http.ResponseWriter, r *http.Request) {
	status, err := handler.GetLicenseV2Handler().GetLicenseStatus(r.Context())
	if err != nil {
		logrus.Errorf("get license status: %v", err)
		httputil.ReturnError(r, w, 500, err.Error())
		return
	}
	httputil.ReturnSuccess(r, w, status)
}

// Getlicense returns the current license status.
func (l *LicenseManager) Getlicense(w http.ResponseWriter, r *http.Request) {
	status, err := handler.GetLicenseV2Handler().GetLicenseStatus(r.Context())
	if err != nil {
		logrus.Errorf("get license: %v", err)
		httputil.ReturnError(r, w, 500, err.Error())
		return
	}
	httputil.ReturnSuccess(r, w, status)
}
