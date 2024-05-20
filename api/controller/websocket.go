// Copyright (C) 2014-2018 Goodrain Co., Ltd.
// RAINBOND, Application Management Platform

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version. For any non-GPL usage of Rainbond,
// one or multiple Commercial Licenses authorized by Goodrain Co., Ltd.
// must be obtained first.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package controller

import (
	"archive/tar"
	"context"
	"fmt"
	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/api/handler"
	"github.com/goodrain/rainbond/api/proxy"
	"github.com/goodrain/rainbond/api/util"
	ctxutil "github.com/goodrain/rainbond/api/util/ctx"
	"github.com/goodrain/rainbond/config/configs"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/pkg/component/k8s"
	"github.com/goodrain/rainbond/util/constants"
	httputil "github.com/goodrain/rainbond/util/http"
	"github.com/sirupsen/logrus"
	"io"
	corev1 "k8s.io/api/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// DockerConsole docker console
type DockerConsole struct {
	socketproxy proxy.Proxy
}

var dockerConsole *DockerConsole

// GetDockerConsole get Docker console
func GetDockerConsole() *DockerConsole {
	if dockerConsole != nil {
		return dockerConsole
	}
	dockerConsole = &DockerConsole{
		socketproxy: proxy.CreateProxy("dockerconsole", "websocket", configs.Default().APIConfig.DockerConsoleServers),
	}
	return dockerConsole
}

// Get get
func (d DockerConsole) Get(w http.ResponseWriter, r *http.Request) {
	d.socketproxy.Proxy(w, r)
}

var dockerLog *DockerLog

// DockerLog docker log
type DockerLog struct {
	socketproxy proxy.Proxy
}

// GetDockerLog get docker log
func GetDockerLog() *DockerLog {
	if dockerLog == nil {
		dockerLog = &DockerLog{
			socketproxy: proxy.CreateProxy("dockerlog", "websocket", configs.Default().APIConfig.EventLogEndpoints),
		}
	}
	return dockerLog
}

// Get get
func (d DockerLog) Get(w http.ResponseWriter, r *http.Request) {
	d.socketproxy.Proxy(w, r)
}

// MonitorMessage monitor message
type MonitorMessage struct {
	socketproxy proxy.Proxy
}

var monitorMessage *MonitorMessage

// GetMonitorMessage get MonitorMessage
func GetMonitorMessage() *MonitorMessage {
	if monitorMessage == nil {
		monitorMessage = &MonitorMessage{
			socketproxy: proxy.CreateProxy("monitormessage", "websocket", configs.Default().APIConfig.EventLogEndpoints),
		}
	}
	return monitorMessage
}

// Get get
func (d MonitorMessage) Get(w http.ResponseWriter, r *http.Request) {
	d.socketproxy.Proxy(w, r)
}

// EventLog event log
type EventLog struct {
	socketproxy proxy.Proxy
}

var eventLog *EventLog

// GetEventLog get event log
func GetEventLog() *EventLog {
	if eventLog == nil {
		eventLog = &EventLog{
			socketproxy: proxy.CreateProxy("eventlog", "websocket", configs.Default().APIConfig.EventLogEndpoints),
		}
	}
	return eventLog
}

// Get get
func (d EventLog) Get(w http.ResponseWriter, r *http.Request) {
	d.socketproxy.Proxy(w, r)
}

// LogFile log file down server
type LogFile struct {
	Root string
}

var logFile *LogFile

// GetLogFile get  log file
func GetLogFile() *LogFile {
	root := os.Getenv("SERVICE_LOG_ROOT")
	if root == "" {
		root = constants.GrdataLogPath
	}
	logrus.Infof("service logs file root path is :%s", root)
	if logFile == nil {
		logFile = &LogFile{
			Root: root,
		}
	}
	return logFile
}

// Get get
func (d LogFile) Get(w http.ResponseWriter, r *http.Request) {
	gid := chi.URLParam(r, "gid")
	filename := chi.URLParam(r, "filename")
	filePath := path.Join(d.Root, gid, filename)
	if isExist(filePath) {
		http.ServeFile(w, r, filePath)
	} else {
		w.WriteHeader(404)
	}
}
func isExist(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil || os.IsExist(err)
}

// GetInstallLog get
func (d LogFile) GetInstallLog(w http.ResponseWriter, r *http.Request) {
	filename := chi.URLParam(r, "filename")
	filePath := d.Root + filename
	if isExist(filePath) {
		http.ServeFile(w, r, filePath)
	} else {
		w.WriteHeader(404)
	}
}

var pubSubControll *PubSubControll

// PubSubControll service pub sub
type PubSubControll struct {
	socketproxy proxy.Proxy
}

// GetPubSubControll get service pub sub controller
func GetPubSubControll() *PubSubControll {
	if pubSubControll == nil {
		pubSubControll = &PubSubControll{
			socketproxy: proxy.CreateProxy("dockerlog", "websocket", configs.Default().APIConfig.EventLogEndpoints),
		}
	}
	return pubSubControll
}

// Get pubsub controller
func (d PubSubControll) Get(w http.ResponseWriter, r *http.Request) {
	serviceID := chi.URLParam(r, "serviceID")
	name, _ := handler.GetEventHandler().GetLogInstance(serviceID)
	if name != "" {
		r.URL.Query().Add("host_id", name)
		r = r.WithContext(context.WithValue(r.Context(), proxy.ContextKey("host_id"), name))
	}
	d.socketproxy.Proxy(w, r)
}

// GetHistoryLog get service docker logs
func (d PubSubControll) GetHistoryLog(w http.ResponseWriter, r *http.Request) {
	serviceID := r.Context().Value(ctxutil.ContextKey("service_id")).(string)
	name, _ := handler.GetEventHandler().GetLogInstance(serviceID)
	if name != "" {
		r.URL.Query().Add("host_id", name)
		r = r.WithContext(context.WithValue(r.Context(), proxy.ContextKey("host_id"), name))
	}
	d.socketproxy.Proxy(w, r)
}

var fileManage *FileManage

// FileManage docker log
type FileManage struct {
	socketproxy proxy.Proxy
	clientset   *clientset.Clientset
	config      *rest.Config
}

// GetFileManage get docker log
func GetFileManage() *FileManage {
	if fileManage == nil {
		fileManage = &FileManage{
			socketproxy: proxy.CreateProxy("acp_node", "http", []string{"rbd-node:6100"}),
			clientset:   k8s.Default().Clientset,
			config:      k8s.Default().RestConfig,
		}
		//discover.GetEndpointDiscover().AddProject("acp_node", fileManage.socketproxy)
	}
	return fileManage
}

// Get get
func (f FileManage) Get(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	w.Header().Add("Access-Control-Allow-Origin", origin)
	w.Header().Add("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
	w.Header().Add("Access-Control-Allow-Credentials", "true")
	w.Header().Add("Access-Control-Allow-Headers", "x-requested-with,Content-Type,X-Custom-Header")
	switch r.Method {
	case "GET":
		w.Header().Set("Content-Type", "application/octet-stream")
		f.DownloadFile(w, r)
	case "POST":
		f.UploadFile(w, r)
		f.UploadEvent(w, r)
	case "OPTIONS":
		httputil.ReturnSuccess(r, w, nil)
	}
}

// UploadEvent volume file upload event
func (f FileManage) UploadEvent(w http.ResponseWriter, r *http.Request) {
	volumeName := w.Header().Get("volume_name")
	userName := w.Header().Get("user_name")
	tenantID := w.Header().Get("tenant_id")
	serviceID := w.Header().Get("service_id")
	fileName := w.Header().Get("file_name")
	status := w.Header().Get("status")
	msg := fmt.Sprintf("%v to upload file %v in storage %v", status, fileName, volumeName)
	_, err := util.CreateEvent(dbmodel.TargetTypeService, "volume-file-upload", serviceID, tenantID, "", userName, status, msg, 1)
	if err != nil {
		logrus.Error("create event error: ", err)
		httputil.ReturnError(r, w, 500, "操作失败")
	}
	httputil.ReturnSuccess(r, w, nil)
}

func (f FileManage) UploadFile(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("volume_name", r.FormValue("volume_name"))
	w.Header().Add("user_name", r.FormValue("user_name"))
	w.Header().Add("tenant_id", r.FormValue("tenant_id"))
	w.Header().Add("service_id", r.FormValue("service_id"))
	w.Header().Add("status", "failed")
	destPath := r.FormValue("path")
	podName := r.FormValue("pod_name")
	namespace := r.FormValue("namespace")
	containerName := r.FormValue("container_name")
	if destPath == "" {
		httputil.ReturnError(r, w, 400, "Path cannot be empty")
		return
	}
	reader, header, err := r.FormFile("file")
	if err != nil {
		logrus.Errorf("Failed to parse upload file: %s", err.Error())
		httputil.ReturnError(r, w, 501, "Failed to parse upload file.")
		return
	}
	defer reader.Close()
	w.Header().Add("file_name", header.Filename)
	srcPath := path.Join("./", header.Filename)
	file, err := os.OpenFile(srcPath, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		logrus.Errorf("upload file open %v failure: %v", header.Filename, err.Error())
		httputil.ReturnError(r, w, 502, "Failed to open file: "+err.Error())
	}
	defer os.Remove(srcPath)
	defer file.Close()
	if _, err := io.Copy(file, reader); err != nil {
		logrus.Errorf("upload file write %v failure: %v", srcPath, err.Error())
		httputil.ReturnError(r, w, 503, "Failed to write file: "+err.Error())
		return
	}

	err = f.AppFileUpload(containerName, podName, srcPath, destPath, namespace)
	if err != nil {
		logrus.Errorf("upload file %v to %v %v failure: %v", header.Filename, podName, destPath, err.Error())
		httputil.ReturnError(r, w, 503, "Failed to write file: "+err.Error())
		return
	}
	w.Header().Set("status", "success")
}

func (f FileManage) DownloadFile(w http.ResponseWriter, r *http.Request) {
	podName := r.FormValue("pod_name")
	p := r.FormValue("path")
	namespace := r.FormValue("namespace")
	fileName := strings.TrimSpace(chi.URLParam(r, "fileName"))

	filePath := path.Join(p, fileName)
	containerName := r.FormValue("container_name")

	err := f.AppFileDownload(containerName, podName, filePath, namespace)
	if err != nil {
		logrus.Errorf("downloading file from Pod failure: %v", err)
		http.Error(w, "Error downloading file from Pod", http.StatusInternalServerError)
		return
	}
	defer os.Remove(path.Join("./", fileName))
	w.Header().Set("Content-Disposition", "attachment;filename="+fileName)
	http.ServeFile(w, r, path.Join("./", fileName))
}

func (f FileManage) AppFileUpload(containerName, podName, srcPath, destPath, namespace string) error {
	reader, writer := io.Pipe()
	if destPath != "/" && strings.HasSuffix(string(destPath[len(destPath)-1]), "/") {
		destPath = destPath[:len(destPath)-1]
	}
	destPath = destPath + "/" + path.Base(srcPath)
	go func() {
		defer writer.Close()
		cmdutil.CheckErr(cpMakeTar(srcPath, destPath, writer))
	}()
	var cmdArr []string
	cmdArr = []string{"tar", "-xmf", "-"}
	destDir := path.Dir(destPath)
	if len(destDir) > 0 {
		cmdArr = append(cmdArr, "-C", destDir)
	}
	req := f.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: containerName,
			Command:   cmdArr,
			Stdin:     true,
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(f.config, "POST", req.URL())
	if err != nil {
		return err
	}
	return exec.Stream(remotecommand.StreamOptions{
		Stdin:  reader,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Tty:    false,
	})
}

func (f FileManage) AppFileDownload(containerName, podName, filePath, namespace string) error {
	reader, outStream := io.Pipe()
	req := f.clientset.CoreV1().RESTClient().Get().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: containerName,
			Command:   []string{"tar", "cf", "-", filePath},
			Stdin:     true,
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(f.config, "POST", req.URL())
	if err != nil {
		return err
	}
	go func() {
		defer outStream.Close()
		err = exec.Stream(remotecommand.StreamOptions{
			Stdin:  os.Stdin,
			Stdout: outStream,
			Stderr: os.Stderr,
			Tty:    false,
		})
		cmdutil.CheckErr(err)
	}()
	prefix := getPrefix(filePath)
	prefix = path.Clean(prefix)
	destPath := path.Join("./", path.Base(prefix))
	err = unTarAll(reader, destPath, prefix)
	if err != nil {
		return err
	}
	return nil
}

func cpMakeTar(srcPath string, destPath string, out io.Writer) error {
	tw := tar.NewWriter(out)

	defer func() {
		if err := tw.Close(); err != nil {
			fmt.Printf("Error closing tar writer: %v\n", err)
		}
	}()

	// 获取源路径的信息
	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		return fmt.Errorf("failed to get info for %s: %v", srcPath, err)
	}

	basePath := srcPath
	if srcInfo.IsDir() {
		basePath = filepath.Dir(srcPath)
	}

	return filepath.Walk(srcPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 获取相对于源路径的相对路径
		relPath, err := filepath.Rel(basePath, path)
		if err != nil {
			return err
		}

		// 创建 tar 归档文件的文件头信息
		hdr, err := tar.FileInfoHeader(info, relPath)
		if err != nil {
			return fmt.Errorf("failed to create header for %s: %v", path, err)
		}

		// 写入文件头信息到 tar 归档
		if err := tw.WriteHeader(hdr); err != nil {
			return fmt.Errorf("failed to write header for %s: %v", path, err)
		}

		if info.Mode().IsRegular() {
			// 如果是普通文件，则将文件内容写入到 tar 归档
			file, err := os.Open(path)
			if err != nil {
				return fmt.Errorf("failed to open %s: %v", path, err)
			}
			defer file.Close()

			_, err = io.Copy(tw, file)
			if err != nil {
				return fmt.Errorf("failed to write file %s to tar: %v", path, err)
			}
		}

		return nil
	})
}

func unTarAll(reader io.Reader, destDir, prefix string) error {
	tarReader := tar.NewReader(reader)
	for {
		header, err := tarReader.Next()
		if err != nil {
			if err != io.EOF {
				return err
			}
			break
		}

		if !strings.HasPrefix(header.Name, prefix) {
			return fmt.Errorf("tar contents corrupted")
		}

		mode := header.FileInfo().Mode()
		destFileName := filepath.Join(destDir, header.Name[len(prefix):])

		baseName := filepath.Dir(destFileName)
		if err := os.MkdirAll(baseName, 0755); err != nil {
			return err
		}
		if header.FileInfo().IsDir() {
			if err := os.MkdirAll(destFileName, 0755); err != nil {
				return err
			}
			continue
		}

		evaledPath, err := filepath.EvalSymlinks(baseName)
		if err != nil {
			return err
		}

		if mode&os.ModeSymlink != 0 {
			linkname := header.Linkname

			if !filepath.IsAbs(linkname) {
				_ = filepath.Join(evaledPath, linkname)
			}

			if err := os.Symlink(linkname, destFileName); err != nil {
				return err
			}
		} else {
			outFile, err := os.Create(destFileName)
			if err != nil {
				return err
			}
			defer outFile.Close()
			if _, err := io.Copy(outFile, tarReader); err != nil {
				return err
			}
			if err := outFile.Close(); err != nil {
				return err
			}
		}
	}

	return nil
}

func getPrefix(file string) string {
	return strings.TrimLeft(file, "/")
}
