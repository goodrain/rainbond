package controller

import (
	"fmt"
	"net/http"

	"io"
	"os"
	"strings"

	"io/ioutil"
	"path/filepath"

	"github.com/Sirupsen/logrus"
	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/api/controller/upload"
	"github.com/goodrain/rainbond/api/handler"
	"github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/db"
	httputil "github.com/goodrain/rainbond/util/http"
	"github.com/jinzhu/gorm"
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
			httputil.ReturnError(r, w, 400, fmt.Sprintf("Arguments eventId is must defined."))
			return
		}

		res, err := db.GetManager().AppDao().GetByEventId(eventId)
		if err != nil {
			httputil.ReturnError(r, w, 404, fmt.Sprintf("Failed to query status of export app by event id %s: %v", eventId, err))
			return
		}

		httputil.ReturnSuccess(r, w, res)
	}

}

func (a *AppStruct) Download(w http.ResponseWriter, r *http.Request) {
	format := strings.TrimSpace(chi.URLParam(r, "format"))
	fileName := strings.TrimSpace(chi.URLParam(r, "fileName"))
	tarFile := fmt.Sprintf("%s/%s/%s", handler.GetAppHandler().GetStaticDir(), format, fileName)

	// return status code 404 if the file not exists.
	if _, err := os.Stat(tarFile); os.IsNotExist(err) {
		httputil.ReturnError(r, w, 404, fmt.Sprintf("Not found export app tar file: %s", tarFile))
		return
	}

	http.ServeFile(w, r, tarFile)
}

func (a *AppStruct) ImportID(w http.ResponseWriter, r *http.Request) {
	eventId := strings.TrimSpace(chi.URLParam(r, "eventId"))
	if eventId == "" {
		httputil.ReturnError(r, w, 400, "Failed to parse eventId.")
		return
	}
	dirName := fmt.Sprintf("%s/import/%s", handler.GetAppHandler().GetStaticDir(), eventId)

	switch r.Method {
	case "POST":
		err := os.MkdirAll(dirName, 0755)
		if err != nil {
			httputil.ReturnError(r, w, 502, "Failed to create directory by event id: "+err.Error())
			return
		}

		httputil.ReturnSuccess(r, w, map[string]string{"path": dirName})
	case "GET":
		_, err := os.Stat(dirName)
		if err != nil {
			if !os.IsExist(err) {
				err := os.MkdirAll(dirName, 0755)
				if err != nil {
					httputil.ReturnError(r, w, 502, "Failed to create directory by event id: "+err.Error())
					return
				}
			}
		}
		apps, err := ioutil.ReadDir(dirName)
		if err != nil {
			httputil.ReturnSuccess(r, w, map[string][]string{"apps": []string{}})
			return
		}

		appArr := make([]string, 0, 10)
		for _, dir := range apps {
			if dir.IsDir() {
				continue
			}
			ex := filepath.Ext(dir.Name())
			if ex != ".zip" {
				continue
			}
			appArr = append(appArr, dir.Name())
		}

		httputil.ReturnSuccess(r, w, map[string][]string{"apps": appArr})
	case "DELETE":
		err := os.RemoveAll(dirName)
		if err != nil {
			httputil.ReturnError(r, w, 501, "Failed to delete directory by id: "+eventId)
			return
		}
		res, err := db.GetManager().AppDao().GetByEventId(eventId)
		if err != nil {
			httputil.ReturnError(r, w, 404, fmt.Sprintf("Failed to query status of export app by event id %s: %v", eventId, err))
			return
		}
		res.Status = "cleaned"
		db.GetManager().AppDao().UpdateModel(res)

		httputil.ReturnSuccess(r, w, "successful")
	}

}

func (a *AppStruct) NewUpload(w http.ResponseWriter, r *http.Request) {
	eventId := strings.TrimSpace(chi.URLParam(r, "event_id"))
	switch r.Method {
	case "OPTIONS":
		origin := r.Header.Get("Origin")
		w.Header().Add("Access-Control-Allow-Origin", origin)
		w.Header().Add("Access-Control-Allow-Methods", "POST,OPTIONS")
		w.Header().Add("Access-Control-Allow-Credentials", "true")
		w.Header().Add("Access-Control-Allow-Headers", "x-requested-with,Content-Type,X-Custom-Header")
		httputil.ReturnSuccess(r, w, nil)

	case "POST":
		if eventId == "" {
			httputil.ReturnError(r, w, 500, "Failed to parse eventId.")
			return
		}
		dirName := fmt.Sprintf("%s/import/%s", handler.GetAppHandler().GetStaticDir(), eventId)

		st := upload.NewStorage(dirName)
		st.UploadHandler(w, r)
	}

}

func (a *AppStruct) Upload(w http.ResponseWriter, r *http.Request) {
	eventId := strings.TrimSpace(chi.URLParam(r, "event_id"))
	switch r.Method {
	case "POST":
		if eventId == "" {
			httputil.ReturnError(r, w, 400, "Failed to parse eventId.")
			return
		}

		logrus.Debug("Start receive upload file: ", eventId)
		reader, header, err := r.FormFile("appTarFile")
		if err != nil {
			logrus.Errorf("Failed to parse upload file: %s", err.Error())
			httputil.ReturnError(r, w, 501, "Failed to parse upload file.")
			return
		}
		defer reader.Close()

		dirName := fmt.Sprintf("%s/import/%s", handler.GetAppHandler().GetStaticDir(), eventId)
		os.MkdirAll(dirName, 0755)

		fileName := fmt.Sprintf("%s/%s", dirName, header.Filename)
		file, err := os.OpenFile(fileName, os.O_WRONLY|os.O_CREATE, 0644)
		if err != nil {
			logrus.Errorf("Failed to open file: %s", err.Error())
			httputil.ReturnError(r, w, 502, "Failed to open file: "+err.Error())
		}
		defer file.Close()

		logrus.Debug("Start write file to: ", fileName)
		if _, err := io.Copy(file, reader); err != nil {
			logrus.Errorf("Failed to write file：%s", err.Error())
			httputil.ReturnError(r, w, 503, "Failed to write file: "+err.Error())
		}

		logrus.Debug("successful write file to: ", fileName)
		origin := r.Header.Get("Origin")
		w.Header().Add("Access-Control-Allow-Origin", origin)
		w.Header().Add("Access-Control-Allow-Methods", "POST,OPTIONS")
		w.Header().Add("Access-Control-Allow-Credentials", "true")
		w.Header().Add("Access-Control-Allow-Headers", "x-requested-with,Content-Type,X-Custom-Header")
		httputil.ReturnSuccess(r, w, nil)

	case "OPTIONS":
		origin := r.Header.Get("Origin")
		w.Header().Add("Access-Control-Allow-Origin", origin)
		w.Header().Add("Access-Control-Allow-Methods", "POST,OPTIONS")
		w.Header().Add("Access-Control-Allow-Credentials", "true")
		w.Header().Add("Access-Control-Allow-Headers", "x-requested-with,Content-Type,X-Custom-Header")
		httputil.ReturnSuccess(r, w, nil)
	}

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
			httputil.ReturnError(r, w, 400, fmt.Sprintf("Arguments eventId is must defined."))
			return
		}

		res, err := db.GetManager().AppDao().GetByEventId(eventId)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				res.Status = "uploading"
				httputil.ReturnSuccess(r, w, res)
				return
			}
			httputil.ReturnError(r, w, 500, fmt.Sprintf("Failed to query status of export app by event id %s: %v", eventId, err))
			return
		}
		if res.Status == "cleaned" {
			res.Metadata = ""
			httputil.ReturnSuccess(r, w, res)
			return
		}

		if res.Status == "success" {
			metadatasFile := fmt.Sprintf("%s/metadatas.json", res.SourceDir)
			data, err := ioutil.ReadFile(metadatasFile)
			if err != nil {
				httputil.ReturnError(r, w, 503, fmt.Sprintf("Can not read apps metadata from metadatas.json file: %v", err))
				return
			}

			res.Metadata = string(data)
		}

		httputil.ReturnSuccess(r, w, res)

	case "DELETE":
		eventId := strings.TrimSpace(chi.URLParam(r, "eventId"))
		if eventId == "" {
			httputil.ReturnError(r, w, 400, fmt.Sprintf("Arguments eventId is must defined."))
			return
		}
		res, err := db.GetManager().AppDao().GetByEventId(eventId)
		if err != nil {
			httputil.ReturnError(r, w, 404, fmt.Sprintf("Failed to query status of export app by event id %s: %v", eventId, err))
			return
		}
		if err := db.GetManager().AppDao().DeleteModelByEventId(res.EventID); err != nil {
			httputil.ReturnError(r, w, 503, fmt.Sprintf("Deleting database records by event ID failed %s: %v", eventId, err))
			return
		}
		if _, err := os.Stat(res.SourceDir); err == nil {
			if err := os.RemoveAll(res.SourceDir); err != nil {
				httputil.ReturnError(r, w, 504, fmt.Sprintf("Deleting uploading application directory failed %s : %v", res.SourceDir, err))
				return
			}
		}
		httputil.ReturnSuccess(r, w, "successfully deleted")
	}

}
