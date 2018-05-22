package controller

import (
	"fmt"
	"net/http"

	"io"
	"os"
	"strings"

	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/api/handler"
	"github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/db"
	httputil "github.com/goodrain/rainbond/util/http"
	"io/ioutil"
)

type AppStruct struct{}

func (a *AppStruct) ExportApp(w http.ResponseWriter, r *http.Request) {

	switch r.Method {
	case "POST":
		var tr model.ExportAppStruct
		ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &tr.Body, nil)
		if !ok {
			return
		}

		if err := handler.GetAppHandler().Complete(&tr); err != nil {
			return
		}

		// 要先更新数据库再通知builder组件
		app := model.NewAppStatusFromExport(&tr)
		db.GetManager().AppDao().DeleteModelByEventId(app.EventID)
		if err := db.GetManager().AppDao().AddModel(app); err != nil {
			httputil.ReturnError(r, w, 502, fmt.Sprintf("Failed to export app %s: %v", app.EventID, err))
			return
		}

		err := handler.GetAppHandler().ExportApp(&tr)
		if err != nil {
			httputil.ReturnError(r, w, 501, fmt.Sprintf("Failed to export app: %v", err))
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

		httputil.ReturnSuccess(r, w, res)
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

func (a *AppStruct) ImportID(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		eventId := strings.TrimSpace(chi.URLParam(r, "eventId"))
		if eventId == "" {
			httputil.ReturnError(r, w, 501, "Failed to parse eventId.")
			return
		}

		dirName := fmt.Sprintf("%s/import/%s", handler.GetAppHandler().GetStaticDir(), eventId)
		err := os.MkdirAll(dirName, 0755)
		if err != nil {
			httputil.ReturnError(r, w, 502, "Failed to create directory by event id: " + err.Error())
			return
		}

		httputil.ReturnSuccess(r, w, "successful")
	case "GET":
		dirs, err := ioutil.ReadDir(fmt.Sprintf("%s/import", handler.GetAppHandler().GetStaticDir()))
		if err != nil {
			httputil.ReturnError(r, w, 502, "Failed to list import id in directory.")
			return
		}

		dirArr := make([]string, 0, 10)
		for _, dir := range dirs {
			if !dir.IsDir() {
				continue
			}
			dirArr = append(dirArr, dir.Name())
 		}

		httputil.ReturnSuccess(r, w, dirArr)
	case "DELETE":
		eventId := strings.TrimSpace(chi.URLParam(r, "eventId"))
		if eventId == "" {
			httputil.ReturnError(r, w, 501, "Failed to parse eventId.")
			return
		}

		dirName := fmt.Sprintf("%s/import/%s", handler.GetAppHandler().GetStaticDir(), eventId)
		err := os.RemoveAll(dirName)
		if err != nil {
			httputil.ReturnError(r, w, 502, "Failed to delete directory by id: " + eventId)
			return
		}

		httputil.ReturnSuccess(r, w, "successful")
	}

}

func (a *AppStruct) Upload(w http.ResponseWriter, r *http.Request) {
	eventId := r.FormValue("eventId")
	if eventId == "" {
		httputil.ReturnError(r, w, 500, "Failed to parse eventId.")
		return
	}

	reader, header, err := r.FormFile("appTarFile")
	if err != nil {
		httputil.ReturnError(r, w, 501, "Failed to parse upload file.")
		return
	}
	defer reader.Close()

	dirName := fmt.Sprintf("%s/import/%s", handler.GetAppHandler().GetStaticDir(), eventId)
	os.MkdirAll(dirName, 0755)

	fileName := fmt.Sprintf("%s/%s", dirName, header.Filename)
	file, err := os.OpenFile(fileName, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		httputil.ReturnError(r, w, 502, "Failed to open file: "+err.Error())
	}
	defer file.Close()

	if _, err := io.Copy(file, reader); err != nil {
		httputil.ReturnError(r, w, 503, "Failed to write file: "+err.Error())
	}
	httputil.ReturnSuccess(r, w, "successful")
}

func (a *AppStruct) ImportApp(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		var importApp = model.ImportAppStruct{
			Format: "rainbond-app",
		}

		ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &importApp, nil)
		if !ok {
			return
		}

		// 获取tar包所在目录
		importApp.SourceDir = fmt.Sprintf("%s/import/%s", handler.GetAppHandler().GetStaticDir(), importApp.EventID)

		// 要先更新数据库再通知builder组件
		app := model.NewAppStatusFromImport(&importApp)
		db.GetManager().AppDao().DeleteModelByEventId(app.EventID)
		if err := db.GetManager().AppDao().AddModel(app); err != nil {
			httputil.ReturnError(r, w, 502, fmt.Sprintf("Failed to import app %s: %v", app.SourceDir, err))
			return
		}

		err := handler.GetAppHandler().ImportApp(&importApp)
		if err != nil {
			httputil.ReturnError(r, w, 501, fmt.Sprintf("Failed to import app: %v", err))
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

		httputil.ReturnSuccess(r, w, res)
	}

}

func (a *AppStruct) BackupApp(w http.ResponseWriter, r *http.Request) {
	//TODO
}

func (a *AppStruct) RecoverApp(w http.ResponseWriter, r *http.Request) {
	//TODO
}
