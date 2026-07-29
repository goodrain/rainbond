package controller

import (
	"github.com/goodrain/rainbond/api/handler"
	api_model "github.com/goodrain/rainbond/api/model"
	httputil "github.com/goodrain/rainbond/util/http"
	"net/http"
)

// RegistryImageRepositories -
func RegistryImageRepositories(w http.ResponseWriter, r *http.Request) {
	namespace := r.FormValue("namespace")
	repositories, err := handler.GetServiceManager().RegistryImageRepositories(namespace)
	if err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, repositories)
}

// RegistryImageTags -
func RegistryImageTags(w http.ResponseWriter, r *http.Request) {
	repository := r.FormValue("repository")
	tags, err := handler.GetServiceManager().RegistryImageTags(repository)
	if err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, tags)
}

// DeleteRegistryImageManifest -
func DeleteRegistryImageManifest(w http.ResponseWriter, r *http.Request) {
	var req api_model.DeleteRegistryImageManifestRequest
	if !httputil.ValidatorRequestStructAndErrorResponse(r, w, &req, nil) {
		return
	}
	result, err := handler.GetServiceManager().DeleteRegistryImageManifest(req.Image)
	if err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, result)
}
