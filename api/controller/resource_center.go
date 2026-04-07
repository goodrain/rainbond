package controller

import (
	"net/http"

	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/api/handler"
	httputil "github.com/goodrain/rainbond/util/http"
)

type ResourceCenterController struct{}

func (c *ResourceCenterController) GetWorkloadDetail(w http.ResponseWriter, r *http.Request) {
	tenantName := chi.URLParam(r, "tenant_name")
	resource := chi.URLParam(r, "resource")
	name := chi.URLParam(r, "name")
	group := r.URL.Query().Get("group")
	version := r.URL.Query().Get("version")
	bean, err := handler.GetResourceCenterHandler().GetWorkloadDetail(tenantName, group, version, resource, name)
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	httputil.ReturnSuccess(r, w, bean)
}

func (c *ResourceCenterController) GetPodDetail(w http.ResponseWriter, r *http.Request) {
	tenantName := chi.URLParam(r, "tenant_name")
	podName := chi.URLParam(r, "pod_name")
	bean, err := handler.GetResourceCenterHandler().GetPodDetail(tenantName, podName)
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	httputil.ReturnSuccess(r, w, bean)
}

func (c *ResourceCenterController) ListEvents(w http.ResponseWriter, r *http.Request) {
	tenantName := chi.URLParam(r, "tenant_name")
	namespace := r.URL.Query().Get("namespace")
	kind := r.URL.Query().Get("kind")
	name := r.URL.Query().Get("name")
	bean, err := handler.GetResourceCenterHandler().ListEvents(tenantName, namespace, kind, name)
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	httputil.ReturnSuccess(r, w, map[string]interface{}{
		"list":  bean,
		"total": len(bean),
	})
}

func GetResourceCenterController() *ResourceCenterController {
	return &ResourceCenterController{}
}
