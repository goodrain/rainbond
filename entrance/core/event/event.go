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

package event

import (
	"github.com/goodrain/rainbond/cmd/entrance/option"
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
)

//LogMessage 事件日志实体
type LogMessage struct {
	EventID string `json:"event_id"`
	Step    string `json:"step"`
	Status  string `json:"status"`
	Message string `json:"message"`
	Level   string `json:"level"`
	Time    string `json:"time"`
}

//Manager 管理器
type Manager struct {
	EventServerAddress []string
	Path               string
	client             *http.Client
}

//NewManager new manager
func NewManager(c option.Config) *Manager {
	cli := &http.Client{
		Timeout: time.Second * 2,
	}
	return &Manager{
		client:             cli,
		Path:               "/event_push",
		EventServerAddress: c.EventServerAddress,
	}
}
func (m *Manager) sendMessage(eventID, step, status, message, level string) {
	me := LogMessage{
		EventID: eventID,
		Status:  status,
		Step:    step,
		Message: message,
		Level:   level,
		Time:    time.Now().Format(time.RFC3339),
	}
	meByte, _ := json.Marshal(me)
	for _, add := range m.EventServerAddress {
		url := add + m.Path
		if !strings.HasPrefix(url, "http") {
			url = "http://" + url
		}
		req, err := http.NewRequest("POST", url, bytes.NewBuffer(meByte))
		if err != nil {
			logrus.Error("new send event message request error.", err.Error())
			continue
		}
		res, err := m.client.Do(req)
		if err != nil {
			logrus.Error("Send event message to server error.", err.Error())
			continue
		}
		if res != nil && res.StatusCode != 200 {
			rb, _ := ioutil.ReadAll(res.Body)
			logrus.Error("Post EventMessage Error:" + string(rb))
		}
		if res != nil && res.Body != nil {
			res.Body.Close()
		}
		if res != nil && res.StatusCode == 200 {
			break
		}
		continue
	}
}

//Info info
func (m *Manager) Info(eventID, status, message string) {
	if eventID == "" {
		eventID = "system"
	}
	m.sendMessage(eventID, "entrance", status, message, "info")
}

//Error error
func (m *Manager) Error(eventID, status, message string) {
	if eventID == "" {
		eventID = "system"
	}
	m.sendMessage(eventID, "entrance", status, message, "error")
}

//Debug debug
func (m *Manager) Debug(eventID, status, message string) {
	if eventID == "" {
		eventID = "system"
	}
	m.sendMessage(eventID, "entrance", status, message, "debug")
}
