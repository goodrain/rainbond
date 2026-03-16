package controller

import (
	"io"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/api/handler"
	httputil "github.com/goodrain/rainbond/util/http"
)

// ClusterResourceController handles HTTP requests for cluster-scoped resources
type ClusterResourceController struct{}

func (c *ClusterResourceController) ListResourceTypes(w http.ResponseWriter, r *http.Request) {
	types, err := handler.GetClusterResourceHandler().ListResourceTypes()
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	httputil.ReturnSuccess(r, w, map[string]interface{}{
		"list":  types,
		"total": len(types),
	})
}

func (c *ClusterResourceController) ListResources(w http.ResponseWriter, r *http.Request) {
	group := r.URL.Query().Get("group")
	version := r.URL.Query().Get("version")
	resource := r.URL.Query().Get("resource")
	list, err := handler.GetClusterResourceHandler().ListResources(group, version, resource)
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	httputil.ReturnSuccess(r, w, map[string]interface{}{
		"list":  list,
		"total": len(list),
	})
}

func (c *ClusterResourceController) GetResource(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	group := r.URL.Query().Get("group")
	version := r.URL.Query().Get("version")
	resource := r.URL.Query().Get("resource")
	obj, err := handler.GetClusterResourceHandler().GetResource(group, version, resource, name)
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	httputil.ReturnSuccess(r, w, obj)
}

func (c *ClusterResourceController) CreateResource(w http.ResponseWriter, r *http.Request) {
	group := r.URL.Query().Get("group")
	version := r.URL.Query().Get("version")
	resource := r.URL.Query().Get("resource")
	body, err := io.ReadAll(r.Body)
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	obj, err := handler.GetClusterResourceHandler().CreateResource(group, version, resource, body)
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	httputil.ReturnSuccess(r, w, obj)
}

func (c *ClusterResourceController) DeleteResource(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	group := r.URL.Query().Get("group")
	version := r.URL.Query().Get("version")
	resource := r.URL.Query().Get("resource")
	if err := handler.GetClusterResourceHandler().DeleteResource(group, version, resource, name); err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}

// GetClusterResourceController returns a new ClusterResourceController
func GetClusterResourceController() *ClusterResourceController {
	return &ClusterResourceController{}
}
