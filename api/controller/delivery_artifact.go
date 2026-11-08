package controller

import (
	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/api/handler"
	"github.com/goodrain/rainbond/api/model"
	httputil "github.com/goodrain/rainbond/util/http"
	"net/http"
)

// ShareDeliveryArtifact accepts only backend-authenticated region API requests.
func (a *AppStruct) ShareDeliveryArtifact(w http.ResponseWriter, r *http.Request) {
	var request model.ArtifactShareRequest
	if !httputil.ValidatorRequestStructAndErrorResponse(r, w, &request, nil) {
		return
	}
	result, err := handler.GetShareHandle().ShareArtifact(request)
	if err != nil {
		httputil.ReturnError(r, w, 400, "Failed to synchronize delivery artifact")
		return
	}
	httputil.ReturnSuccess(r, w, result)
}

func (a *AppStruct) DeliveryArtifactStatus(w http.ResponseWriter, r *http.Request) {
	result, err := handler.GetShareHandle().ShareResult(chi.URLParam(r, "share_id"))
	if err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, result)
}
