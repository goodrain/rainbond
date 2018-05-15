package controller

import (
	"net/http"
	"fmt"

	"github.com/goodrain/rainbond/api/handler"
	httputil "github.com/goodrain/rainbond/util/http"
	"github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"strings"
	"github.com/go-chi/chi"
	"os"
)

type AppStruct struct {}

func (a *AppStruct) ExportApp(w http.ResponseWriter, r *http.Request) {

	switch r.Method{
	case "POST":
		var tr model.ExportAppStruct
		ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &tr.Body, nil)
		if !ok {
			return
		}

		if err := handler.GetAppHandler().Complete(&tr); err != nil {
			return
		}

		err := handler.GetAppHandler().ExportApp(&tr)
		if err != nil {
			httputil.ReturnError(r, w, 501, fmt.Sprintf("Failed to export app: %v", err))
			return
		}

		app := model.NewAppStatusFrom(&tr)

		db.GetManager().AppDao().DeleteModelByEventId(app.EventID)
		if err := db.GetManager().AppDao().AddModel(app); err != nil {
			httputil.ReturnError(r, w, 502, fmt.Sprintf("Failed to export app %s: %v", app.GroupKey, err))
			return
		}

		httputil.ReturnSuccess(r, w, nil)
	case "GET":
		eventId := strings.TrimSpace(chi.URLParam(r, "eventId"))
		if eventId == "" {
			httputil.ReturnError(r, w, 501, fmt.Sprintf("Arguments eventId is must defined."))
			return
		}

		res, err := db.GetManager().AppDao().GetByEventId(eventId)
		if err != nil {
			httputil.ReturnError(r, w, 502, fmt.Sprintf("Failed to query status of export app by event id %s: %v", eventId, err))
			return
		}

		status := res.(*dbmodel.AppStatus)
		httputil.ReturnSuccess(r, w, status)
	}

}

func (a *AppStruct) Download(w http.ResponseWriter, r *http.Request) {
	format := strings.TrimSpace(chi.URLParam(r, "format"))
	fileName := strings.TrimSpace(chi.URLParam(r, "fileName"))
	tarFile := fmt.Sprintf("%s/%s/%s", handler.GetAppHandler().GetStaticDir(), format, fileName)

	// return status code 502 if the file not exists.
	if _, err := os.Stat(tarFile); os.IsNotExist(err) {
		httputil.ReturnError(r, w, 502, fmt.Sprintf("Not found export app tar file: %s", tarFile))
		return
	}

	http.ServeFile(w, r, tarFile)
}

func (a *AppStruct) Upload(w http.ResponseWriter, r *http.Request) {
	//TODO
}

func (a *AppStruct) ImportApp(w http.ResponseWriter, r *http.Request) {
	//TODO
}

func (a *AppStruct) BackupApp(w http.ResponseWriter, r *http.Request) {
	//TODO
}

func (a *AppStruct) RecoverApp(w http.ResponseWriter, r *http.Request) {
	//TODO
}
