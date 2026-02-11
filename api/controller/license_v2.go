package controller

import (
	"encoding/json"
	"net/http"

	"github.com/goodrain/rainbond/api/handler"
	httputil "github.com/goodrain/rainbond/util/http"
	"github.com/sirupsen/logrus"
)

// LicenseV2Controller handles license V2 HTTP endpoints.
type LicenseV2Controller struct{}

var licenseV2Controller *LicenseV2Controller

// GetLicenseV2Controller returns the singleton LicenseV2Controller.
func GetLicenseV2Controller() *LicenseV2Controller {
	if licenseV2Controller != nil {
		return licenseV2Controller
	}
	licenseV2Controller = &LicenseV2Controller{}
	return licenseV2Controller
}

// GetClusterID returns the cluster ID.
func (l *LicenseV2Controller) GetClusterID(w http.ResponseWriter, r *http.Request) {
	clusterID, err := handler.GetLicenseV2Handler().GetClusterID(r.Context())
	if err != nil {
		logrus.Errorf("get cluster ID: %v", err)
		httputil.ReturnError(r, w, 500, err.Error())
		return
	}
	httputil.ReturnSuccess(r, w, map[string]string{"cluster_id": clusterID})
}

// ActivateLicense activates a license.
func (l *LicenseV2Controller) ActivateLicense(w http.ResponseWriter, r *http.Request) {
	var req struct {
		LicenseCode  string `json:"license_code"`
		EnterpriseID string `json:"enterprise_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.ReturnError(r, w, 400, "invalid request body")
		return
	}
	if req.LicenseCode == "" {
		httputil.ReturnError(r, w, 400, "license_code field is required")
		return
	}
	if req.EnterpriseID == "" {
		httputil.ReturnError(r, w, 400, "enterprise_id field is required")
		return
	}

	status, err := handler.GetLicenseV2Handler().ActivateLicense(r.Context(), req.LicenseCode, req.EnterpriseID)
	if err != nil {
		logrus.Errorf("activate license: %v", err)
		httputil.ReturnError(r, w, 500, err.Error())
		return
	}
	httputil.ReturnSuccess(r, w, status)
}

// GetLicenseStatus returns the current license status.
func (l *LicenseV2Controller) GetLicenseStatus(w http.ResponseWriter, r *http.Request) {
	status, err := handler.GetLicenseV2Handler().GetLicenseStatus(r.Context())
	if err != nil {
		logrus.Errorf("get license status: %v", err)
		httputil.ReturnError(r, w, 500, err.Error())
		return
	}
	httputil.ReturnSuccess(r, w, status)
}
