package controller

import (
	"github.com/goodrain/rainbond/api/handler"
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
