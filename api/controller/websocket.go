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
	"net/http"
	"os"

	"github.com/goodrain/rainbond/api/discover"
	"github.com/goodrain/rainbond/api/proxy"

	"github.com/Sirupsen/logrus"

	"github.com/go-chi/chi"
)

//DockerConsole docker console
type DockerConsole struct {
	socketproxy proxy.Proxy
}

var defaultDockerConsoleEndpoints = []string{"127.0.0.1:7171"}
var defaultEventLogEndpoints = []string{"local=>127.0.0.1:6363"}
var defaultEtcdEndpoints = []string{"127.0.0.1:2379"}

var dockerConsole *DockerConsole

//GetDockerConsole get Docker console
func GetDockerConsole() *DockerConsole {
	if dockerConsole != nil {
		return dockerConsole
	}
	dockerConsole = &DockerConsole{
		socketproxy: proxy.CreateProxy("dockerconsole", "websocket", defaultDockerConsoleEndpoints),
	}
	discover.GetEndpointDiscover(defaultEtcdEndpoints).AddProject("acp_webcli", dockerConsole.socketproxy)
	return dockerConsole
}

//Get get
func (d DockerConsole) Get(w http.ResponseWriter, r *http.Request) {
	d.socketproxy.Proxy(w, r)
}

var dockerLog *DockerLog

//DockerLog docker log
type DockerLog struct {
	socketproxy proxy.Proxy
}

//GetDockerLog get docker log
func GetDockerLog() *DockerLog {
	if dockerLog == nil {
		dockerLog = &DockerLog{
			socketproxy: proxy.CreateProxy("dockerlog", "websocket", defaultEventLogEndpoints),
		}
		discover.GetEndpointDiscover(defaultEtcdEndpoints).AddProject("event_log_event_http", dockerLog.socketproxy)
	}
	return dockerLog
}

//Get get
func (d DockerLog) Get(w http.ResponseWriter, r *http.Request) {
	d.socketproxy.Proxy(w, r)
}

//MonitorMessage monitor message
type MonitorMessage struct {
	socketproxy proxy.Proxy
}

var monitorMessage *MonitorMessage

//GetMonitorMessage get MonitorMessage
func GetMonitorMessage() *MonitorMessage {
	if monitorMessage == nil {
		monitorMessage = &MonitorMessage{
			socketproxy: proxy.CreateProxy("monitormessage", "websocket", defaultEventLogEndpoints),
		}
		discover.GetEndpointDiscover(defaultEtcdEndpoints).AddProject("event_log_event_http", monitorMessage.socketproxy)
	}
	return monitorMessage
}

//Get get
func (d MonitorMessage) Get(w http.ResponseWriter, r *http.Request) {
	d.socketproxy.Proxy(w, r)
}

//EventLog event log
type EventLog struct {
	socketproxy proxy.Proxy
}

var eventLog *EventLog

//GetEventLog get event log
func GetEventLog() *EventLog {
	if eventLog == nil {
		eventLog = &EventLog{
			socketproxy: proxy.CreateProxy("eventlog", "websocket", defaultEventLogEndpoints),
		}
		discover.GetEndpointDiscover(defaultEtcdEndpoints).AddProject("event_log_event_http", eventLog.socketproxy)
	}
	return eventLog
}

//Get get
func (d EventLog) Get(w http.ResponseWriter, r *http.Request) {
	d.socketproxy.Proxy(w, r)
}

//LogFile log file down server
type LogFile struct {
	Root string
}

var logFile *LogFile

//GetLogFile get  log file
func GetLogFile() *LogFile {
	root := os.Getenv("SERVICE_LOG_ROOT")
	if root == "" {
		root = "/grdata/downloads/log/"
	}
	logrus.Infof("service logs file root path is :%s", root)
	if logFile == nil {
		logFile = &LogFile{
			Root: root,
		}
	}
	return logFile
}

//Get get
func (d LogFile) Get(w http.ResponseWriter, r *http.Request) {
	gid := chi.URLParam(r, "gid")
	filename := chi.URLParam(r, "filename")
	filePath := d.Root + gid + "/" + filename
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


//Get get
func (d LogFile) GetInstallLog(w http.ResponseWriter, r *http.Request) {
	filename := chi.URLParam(r, "filename")
	filePath := d.Root + filename
	if isExist(filePath) {
		http.ServeFile(w, r, filePath)
	} else {
		w.WriteHeader(404)
	}
}