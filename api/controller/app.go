package controller

import (
	"net/http"
)

type AppStruct struct{}

func (a *AppStruct) ExportApp(w http.ResponseWriter, r *http.Request) {
	// var tr model.ExportAppStruct
	// ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &tr.Body, nil)
	// if !ok {
	// 	return
	// }

	// err := handler.GetAppHandler().ExportApp(&tr)
	// if err != nil {
	// 	httputil.ReturnError(r, w, 500, fmt.Sprintf("Failed to export app: %v", err))
	// 	return
	// }

	// httputil.ReturnSuccess(r, w, nil)
	// return
}

func (a *AppStruct) ImportApp(w http.ResponseWriter, r *http.Request) {
	//TODO
}

func (a *AppStruct) ExportRunnableApp(w http.ResponseWriter, r *http.Request) {
	//TODO
}

func (a *AppStruct) BackupApp(w http.ResponseWriter, r *http.Request) {
	//TODO
}

func (a *AppStruct) RecoverApp(w http.ResponseWriter, r *http.Request) {
	//TODO
}
