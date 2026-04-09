package controller

import (
	"net/http"

	"github.com/goodrain/rainbond/api/handler"
	"github.com/goodrain/rainbond/pkg/component/k8s"
	httputil "github.com/goodrain/rainbond/util/http"
)

type VMCapabilityController struct {
	buildCapabilities func() (*handler.VMCapability, error)
}

func (c *VMCapabilityController) GetCapabilities(w http.ResponseWriter, r *http.Request) {
	build := c.buildCapabilities
	if build == nil {
		build = func() (*handler.VMCapability, error) {
			return handler.BuildVMCapabilities(k8s.Default().DynamicClient)
		}
	}
	capabilities, err := build()
	if err != nil {
		httputil.ReturnError(r, w, http.StatusInternalServerError, "get vm capabilities failure: "+err.Error())
		return
	}
	httputil.ReturnSuccess(r, w, capabilities)
}

func GetVMCapabilityController() *VMCapabilityController {
	return &VMCapabilityController{}
}
