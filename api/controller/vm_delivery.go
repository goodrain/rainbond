package controller

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/api/handler"
	ctxutil "github.com/goodrain/rainbond/api/util/ctx"
	httputil "github.com/goodrain/rainbond/util/http"
)

// VMDeliveryCapture has bounded control calls; disk/image I/O runs in builder.
func VMDeliveryCapture(w http.ResponseWriter, r *http.Request) {
	action := "status"
	switch r.Method {
	case http.MethodPut:
		action = "prepare"
	case http.MethodPost:
		action = "image"
	case http.MethodDelete:
		action = "release"
	}
	var request handler.VMDeliveryRequest
	if r.Method == http.MethodPost {
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10)).Decode(&request); err != nil {
			httputil.ReturnError(r, w, 400, "invalid VM delivery request")
			return
		}
	}
	result, err := handler.VMDeliveryCaptureCommand(r.Context(), r.Context().Value(ctxutil.ContextKey("service_id")).(string), chi.URLParam(r, "name"), action, request)
	if err != nil {
		message := "vm_delivery_unavailable"
		for _, safe := range []error{handler.ErrVMDeliveryShutdown, handler.ErrVMDeliveryDiskCount, handler.ErrVMDeliveryDiskInUse} {
			if errors.Is(err, safe) {
				message = safe.Error()
			}
		}
		httputil.ReturnError(r, w, 409, message)
		return
	}
	httputil.ReturnSuccess(r, w, result)
}
