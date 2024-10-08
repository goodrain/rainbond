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
	"context"
	webcli "github.com/goodrain/rainbond/api/webcli/app"
	"github.com/goodrain/rainbond/config/configs"
	"github.com/goodrain/rainbond/pkg/component/storage"
	"github.com/gorilla/websocket"
	"net/http"
	"os"
	"path"

	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/api/handler"
	"github.com/goodrain/rainbond/api/proxy"
	ctxutil "github.com/goodrain/rainbond/api/util/ctx"
	"github.com/goodrain/rainbond/util/constants"
	"github.com/sirupsen/logrus"
)

var app *webcli.App

func GetWebCli() *webcli.App {
	if app == nil {
		app = &webcli.App{}
		app.SetUpgrader(&websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			Subprotocols:    []string{"webtty"},
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		})
		//create kube client and config
		if err := app.CreateKubeClient(); err != nil {
			logrus.Errorf("create kube client error: %v", err)
		}
	}
	return app
}

// DockerConsole docker console
type DockerConsole struct {
	socketproxy proxy.Proxy
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
		storage.Default().StorageCli.ServeFile(w, r, filePath)
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
		storage.Default().StorageCli.ServeFile(w, r, filePath)
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
