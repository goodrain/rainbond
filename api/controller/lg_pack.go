package controller

import (
	"encoding/json"
	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/db"
	httputil "github.com/goodrain/rainbond/util/http"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path"
	"reflect"
	"strings"
)

// LongVersionStruct -
type LongVersionStruct struct{}

// BaseUploadPath lang version base dir
const BaseUploadPath = "/grdata/lang"

// UploadLongVersion -
func (t *LongVersionStruct) UploadLongVersion(w http.ResponseWriter, r *http.Request) {
	//从表单中读取文件
	file, fileHeader, err := r.FormFile("file")
	if err != nil {
		sendResponse(w, http.StatusBadRequest, "failure", "get file failure.")
		return
	}
	//defer 结束时关闭文件
	defer file.Close()
	eventID, err := generateRandomString(32)
	langPath := path.Join(BaseUploadPath, eventID)
	_, err = os.Stat(langPath)
	if os.IsNotExist(err) {
		// 创建文件夹
		err := os.MkdirAll(langPath, os.ModePerm)
		if err != nil {
			sendResponse(w, http.StatusInternalServerError, "failure", "dir is not exist,mkdir failure")
			return
		}
	}
	//创建文件
	newFile, err := os.Create(path.Join(langPath, fileHeader.Filename))
	if err != nil {
		sendResponse(w, http.StatusInternalServerError, "failure", "Create file error"+err.Error())
		return
	}
	//defer 结束时关闭文件
	defer newFile.Close()

	//将文件写到本地
	_, err = io.Copy(newFile, file)
	if err != nil {
		sendResponse(w, http.StatusInternalServerError, "failure", "Failed to write file: "+err.Error())
		return
	}
	long := struct {
		EventID  string `json:"event_id"`
		FileName string `json:"file_name"`
	}{
		eventID,
		fileHeader.Filename,
	}
	origin := r.Header.Get("Origin")
	w.Header().Add("Access-Control-Allow-Origin", origin)
	w.Header().Add("Access-Control-Allow-Methods", "POST, GET, DELETE, PUT, OPTIONS")
	w.Header().Add("Access-Control-Allow-Credentials", "true")
	w.Header().Add("Access-Control-Allow-Headers", "authorization,x_region_name,x_team_name,x-requested-with")
	sendResponse(w, http.StatusOK, "successful", long)
}

// DownloadLongVersion -
func (t *LongVersionStruct) DownloadLongVersion(w http.ResponseWriter, r *http.Request) {
	language := strings.TrimSpace(chi.URLParam(r, "language"))
	version := strings.TrimSpace(chi.URLParam(r, "version"))
	ver, err := db.GetManager().LongVersionDao().GetVersionByLanguageAndVersion(language, version)
	if err != nil {
		sendResponse(w, http.StatusBadRequest, "failure", "get version failure"+err.Error())
		return
	}
	http.ServeFile(w, r, path.Join(BaseUploadPath, ver.EventID, ver.FileName))
}

// OptionLongVersion -
func (t *LongVersionStruct) OptionLongVersion(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	w.Header().Add("Access-Control-Allow-Origin", origin)
	w.Header().Add("Access-Control-Allow-Methods", "POST, GET, DELETE, PUT, OPTIONS")
	w.Header().Add("Access-Control-Allow-Credentials", "true")
	w.Header().Add("Access-Control-Allow-Headers", "authorization,x_region_name,x_team_name,content-type,x-requested-with")
	httputil.ReturnSuccess(r, w, nil)
	return
}

func generateRandomString(n int) (string, error) {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	for i, b := range bytes {
		bytes[i] = letters[b%byte(len(letters))]
	}
	return string(bytes), nil
}

// APIResponse =
type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Bean    interface{} `json:"bean,omitempty"`
	List    interface{} `json:"list,omitempty"`
}

func sendResponse(w http.ResponseWriter, code int, message string, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	response := APIResponse{
		Code:    code,
		Message: message,
	}

	switch reflect.TypeOf(data).Kind() {
	case reflect.Slice, reflect.Array:
		response.List = data
	default:
		response.Bean = data
	}

	json.NewEncoder(w).Encode(response)
}
