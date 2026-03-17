package controller

import (
	"io"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/api/handler"
	httputil "github.com/goodrain/rainbond/util/http"
)

// NsResourceController handles namespace-scoped K8s resource HTTP requests
type NsResourceController struct{}

// ListNsResourceTypes lists all namespace-scoped resource types
func (c *NsResourceController) ListNsResourceTypes(w http.ResponseWriter, r *http.Request) {
	types, err := handler.GetNsResourceHandler().ListNsResourceTypes()
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	httputil.ReturnSuccess(r, w, map[string]interface{}{"list": types, "total": len(types)})
}

// ListNsResources lists all resources of the given GVR in the tenant namespace
func (c *NsResourceController) ListNsResources(w http.ResponseWriter, r *http.Request) {
	tenantName := chi.URLParam(r, "tenant_name")
	group := r.URL.Query().Get("group")
	version := r.URL.Query().Get("version")
	resource := r.URL.Query().Get("resource")
	list, err := handler.GetNsResourceHandler().ListNsResources(tenantName, group, version, resource)
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	httputil.ReturnSuccess(r, w, map[string]interface{}{"list": list, "total": len(list)})
}

// GetNsResource retrieves a single resource by name from the tenant namespace
func (c *NsResourceController) GetNsResource(w http.ResponseWriter, r *http.Request) {
	tenantName := chi.URLParam(r, "tenant_name")
	name := chi.URLParam(r, "name")
	group := r.URL.Query().Get("group")
	version := r.URL.Query().Get("version")
	resource := r.URL.Query().Get("resource")
	obj, err := handler.GetNsResourceHandler().GetNsResource(tenantName, group, version, resource, name)
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	httputil.ReturnSuccess(r, w, obj)
}

// CreateNsResource creates a resource in the tenant namespace from the request body
func (c *NsResourceController) CreateNsResource(w http.ResponseWriter, r *http.Request) {
	tenantName := chi.URLParam(r, "tenant_name")
	group := r.URL.Query().Get("group")
	version := r.URL.Query().Get("version")
	resource := r.URL.Query().Get("resource")
	source := r.URL.Query().Get("source")
	if source == "" {
		source = "manual"
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	obj, err := handler.GetNsResourceHandler().CreateNsResource(tenantName, group, version, resource, source, body)
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	httputil.ReturnSuccess(r, w, obj)
}

// DeleteNsResource deletes a resource by name from the tenant namespace
func (c *NsResourceController) DeleteNsResource(w http.ResponseWriter, r *http.Request) {
	tenantName := chi.URLParam(r, "tenant_name")
	name := chi.URLParam(r, "name")
	group := r.URL.Query().Get("group")
	version := r.URL.Query().Get("version")
	resource := r.URL.Query().Get("resource")
	if err := handler.GetNsResourceHandler().DeleteNsResource(tenantName, group, version, resource, name); err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}

// GetNsResourceController returns a new NsResourceController
func GetNsResourceController() *NsResourceController {
	return &NsResourceController{}
}
