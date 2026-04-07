package controller

import (
	"io"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/api/handler"
	httputil "github.com/goodrain/rainbond/util/http"
)

// StorageController handles HTTP requests for StorageClass and PersistentVolume
type StorageController struct{}

// ListStorageClasses returns all StorageClasses with PV counts
func (c *StorageController) ListStorageClasses(w http.ResponseWriter, r *http.Request) {
	list, err := handler.GetStorageHandler().ListStorageClasses()
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	httputil.ReturnSuccess(r, w, map[string]interface{}{"list": list, "total": len(list)})
}

// CreateStorageClass creates a StorageClass from YAML body
func (c *StorageController) CreateStorageClass(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	sc, err := handler.GetStorageHandler().CreateStorageClass(body)
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	httputil.ReturnSuccess(r, w, sc)
}

// DeleteStorageClass deletes a StorageClass by name
func (c *StorageController) DeleteStorageClass(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if err := handler.GetStorageHandler().DeleteStorageClass(name); err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}

// ListPersistentVolumes returns all PersistentVolumes
func (c *StorageController) ListPersistentVolumes(w http.ResponseWriter, r *http.Request) {
	list, err := handler.GetStorageHandler().ListPersistentVolumes()
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	httputil.ReturnSuccess(r, w, map[string]interface{}{"list": list, "total": len(list)})
}

// CreatePersistentVolume creates a PersistentVolume from YAML body
func (c *StorageController) CreatePersistentVolume(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	pv, err := handler.GetStorageHandler().CreatePersistentVolume(body)
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	httputil.ReturnSuccess(r, w, pv)
}

// DeletePersistentVolume deletes a PersistentVolume by name
func (c *StorageController) DeletePersistentVolume(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if err := handler.GetStorageHandler().DeletePersistentVolume(name); err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}

// GetStorageController returns a new StorageController
func GetStorageController() *StorageController {
	return &StorageController{}
}
