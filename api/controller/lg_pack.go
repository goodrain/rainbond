package controller

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/db"
	httputil "github.com/goodrain/rainbond/util/http"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"
)

// LongVersionStruct -
type LongVersionStruct struct{}

// BaseUploadPath lang version base dir
const BaseUploadPath = "/grdata/lang"
const LSUploadPath = "/run/lang"

// 定义允许上传的文件扩展名白名单
var allowedExtensions = map[string]bool{
	".jar":    true,
	".tar.gz": true,
}

func (t *LongVersionStruct) UploadLongVersion(w http.ResponseWriter, r *http.Request) {
	// 从表单中读取文件
	file, fileHeader, err := r.FormFile("file")
	if err != nil {
		sendResponse(w, http.StatusBadRequest, "failure", "get file failure.")
		return
	}
	// defer 结束时关闭文件
	defer file.Close()

	// 获取文件扩展名并转换为小写
	fileName := fileHeader.Filename
	fileExtension := strings.ToLower(filepath.Ext(fileName))

	// 验证文件扩展名是否在白名单中
	if !allowedExtensions[fileExtension] && !(strings.HasSuffix(fileName, ".tar.gz")) {
		sendResponse(w, http.StatusBadRequest, "failure", "file type not allowed. Only .jar and .tar.gz files are permitted.")
		return
	}

	// 验证文件内容格式
	if fileExtension == ".tar.gz" || strings.HasSuffix(fileName, ".tar.gz") {
		if err := validateTarGz(file); err != nil {
			sendResponse(w, http.StatusBadRequest, "failure", "invalid tar.gz file: "+err.Error())
			return
		}
	} else if fileExtension == ".jar" {
		if err := validateJar(file); err != nil {
			sendResponse(w, http.StatusBadRequest, "failure", "invalid jar file: "+err.Error())
			return
		}
	}

	// 生成事件ID
	eventID, err := generateRandomString(32)
	if err != nil {
		sendResponse(w, http.StatusInternalServerError, "failure", "failed to generate event ID: "+err.Error())
		return
	}

	langPath := path.Join(LSUploadPath, eventID)
	if _, err = os.Stat(langPath); os.IsNotExist(err) {
		// 创建文件夹
		err := os.MkdirAll(langPath, os.ModePerm)
		if err != nil {
			sendResponse(w, http.StatusInternalServerError, "failure", "dir does not exist, mkdir failure")
			return
		}
	}

	// 创建文件
	newFile, err := os.Create(path.Join(langPath, fileHeader.Filename))
	if err != nil {
		sendResponse(w, http.StatusInternalServerError, "failure", "Create file error: "+err.Error())
		return
	}
	// defer 结束时关闭文件
	defer newFile.Close()

	// 将文件写到本地
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		sendResponse(w, http.StatusInternalServerError, "failure", "failed to reset file pointer: "+err.Error())
		return
	}
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

// validateTarGz 验证上传的文件是否为有效的 tar.gz 格式
func validateTarGz(file io.Reader) error {
	gzr, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tarReader := tar.NewReader(gzr)
	_, err = tarReader.Next() // 尝试读取第一个文件头，以验证内容
	if err != nil {
		return err
	}
	return nil
}

// validateJar 验证上传的文件是否为有效的 jar 格式
func validateJar(file io.Reader) error {
	// JAR 文件实际上是 ZIP 格式，因此可以通过尝试读取 ZIP 头来进行简单验证
	buffer := make([]byte, 4)
	if _, err := file.Read(buffer); err != nil {
		return err
	}
	if string(buffer) != "PK\x03\x04" { // ZIP 文件的魔术字节
		return errors.New("invalid jar file header")
	}
	return nil
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
