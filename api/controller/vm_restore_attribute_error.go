package controller

import (
	"errors"
	"net/http"

	"github.com/goodrain/rainbond/pkg/vmrestore"
	httputil "github.com/goodrain/rainbond/util/http"
)

func returnK8sAttributeError(r *http.Request, w http.ResponseWriter, err error) {
	if errors.Is(err, vmrestore.ErrInternalAttribute) {
		httputil.ReturnError(r, w, http.StatusForbidden, "VM restore internal attributes are not public")
		return
	}
	httputil.ReturnBcodeError(r, w, err)
}
