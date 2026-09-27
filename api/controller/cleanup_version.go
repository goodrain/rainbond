package controller

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi"
	ctxutil "github.com/goodrain/rainbond/api/util/ctx"
	"github.com/goodrain/rainbond/db"
	guard "github.com/goodrain/rainbond/pkg/cleanup"
	httputil "github.com/goodrain/rainbond/util/http"
	"github.com/jinzhu/gorm"
)

// RetireBuildVersion removes a build record only; registry content is retained.
func (t *TenantStruct) RetireBuildVersion(w http.ResponseWriter, r *http.Request) {
	var expected guard.VersionExpectation
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&expected) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		httputil.ReturnError(r, w, 400, "invalid retirement request")
		return
	}
	serviceID, _ := r.Context().Value(ctxutil.ContextKey("service_id")).(string)
	tenantID, _ := r.Context().Value(ctxutil.ContextKey("tenant_id")).(string)
	if serviceID == "" || tenantID == "" || expected.ServiceID != serviceID || expected.Version != chi.URLParam(r, "build_version") {
		httputil.ReturnError(r, w, 400, "invalid retirement scope")
		return
	}
	result, err := guard.RetireVersion(db.GetManager().Begin, tenantID, expected)
	if err != nil {
		status := 500
		if errors.Is(err, guard.ErrStateChanged) || errors.Is(err, guard.ErrVersionProtected) {
			status = 409
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status = 404
		}
		httputil.ReturnError(r, w, status, "version retirement rejected; refresh inventory")
		return
	}
	httputil.ReturnSuccess(r, w, result)
}
