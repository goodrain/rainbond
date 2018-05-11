package controller

import (
	"net/http"
	"fmt"

	"github.com/goodrain/rainbond/api/handler"
	httputil "github.com/goodrain/rainbond/util/http"
	"github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/db"
)

type AppStruct struct {}

func (a *AppStruct) ExportApp(w http.ResponseWriter, r *http.Request) {
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
		httputil.ReturnError(r, w, 500, fmt.Sprintf("Failed to export app: %v", err))
		return
	}

	app := model.NewAppStatusFrom(&tr)

	db.GetManager().AppDao().DeleteModel(app.GroupKey, app.Version)
	if err := db.GetManager().AppDao().AddModel(app); err != nil {
		httputil.ReturnError(r, w, 500, fmt.Sprintf("Failed to export app %s: %v", app.GroupKey, err))
		return
	}

	httputil.ReturnSuccess(r, w, nil)
	return
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
