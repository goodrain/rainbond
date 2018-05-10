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

package webhook

import (
	"github.com/goodrain/rainbond/eventlog/conf"
	"net/http"

	"sync"

	"bytes"

	"fmt"

	"strings"

	"net/url"

	"strconv"

	"github.com/Sirupsen/logrus"
)

type Manager struct {
	hooks map[string]*WebHook
	conf  conf.WebHookConf
	log   *logrus.Entry
	lock  sync.Mutex
}

type WebHook struct {
	EndPoint         string
	RequestParameter map[string]interface{}
	RequestBody      []byte
	RequestHeader    map[string]string
	Name             string
	Method           string
}

const UpDateEventStatus = "console_update_event_status"
const UpdateEventCodeVersion = "console_update_event_code_version"

var manager *Manager

func GetManager() *Manager {
	return manager
}

func InitManager(conf conf.WebHookConf, log *logrus.Entry) error {
	ma := &Manager{
		conf:  conf,
		log:   log,
		hooks: make(map[string]*WebHook, 0),
	}
	eventStatus := &WebHook{
		Name:          UpDateEventStatus,
		Method:        "PUT",
		EndPoint:      conf.ConsoleURL + "/api/event/update",
		RequestHeader: make(map[string]string, 0),
	}
	eventStatus.RequestHeader["Content-Type"] = "application/json"
	if conf.ConsoleToken != "" {
		eventStatus.RequestHeader["Authorization"] = "Token " + conf.ConsoleToken
	}
	codeVsersion := &WebHook{
		Name:          UpdateEventCodeVersion,
		Method:        "PUT",
		EndPoint:      conf.ConsoleURL + "/api/event/update-code",
		RequestHeader: make(map[string]string, 0),
	}
	codeVsersion.RequestHeader["Content-Type"] = "application/json"
	if conf.ConsoleToken != "" {
		codeVsersion.RequestHeader["Authorization"] = "Token " + conf.ConsoleToken
	}
	ma.Regist(eventStatus)
	ma.Regist(codeVsersion)
	manager = ma
	return nil
}

func (m *Manager) GetWebhook(name string) *WebHook {
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.hooks[name]
}

func (m *Manager) RunWebhook(name string, body []byte) {
	w := m.GetWebhook(name)
	w.RequestBody = body
	if err := w.Run(); err != nil {
		m.log.Errorf("Webhook %s run error. %s", w.Name, err.Error())
	}
	m.log.Debugf("Run web hook %s success", w.Name)
}

func (m *Manager) RunWebhookWithParameter(name string, body []byte, parameter map[string]interface{}) {
	w := m.GetWebhook(name)
	w.RequestBody = body
	w.RequestParameter = parameter
	if err := w.Run(); err != nil {
		m.log.Errorf("Webhook %s run error. %s", w.Name, err.Error())
	}
}

//Regist 注册
func (m *Manager) Regist(w *WebHook) {
	if w == nil || w.Name == "" {
		return
	}
	m.lock.Lock()
	defer m.lock.Unlock()
	m.hooks[w.Name] = w
}

//Run 执行
func (w *WebHook) Run() error {
	var err error
	if w.RequestParameter != nil && len(w.RequestParameter) > 0 {
		if strings.ToUpper(w.Method) == "GET" {
			values := make(url.Values)
			for k, v := range w.RequestParameter {
				switch v.(type) {
				case int:
					values.Set(k, strconv.Itoa(v.(int)))
				case string:
					values.Set(k, v.(string))
				}
			}
			w.EndPoint += "?" + values.Encode()
		} else {
			var jsonStr = "{"
			for k, v := range w.RequestParameter {
				switch v.(type) {
				case int:
					jsonStr += `"` + k + `":` + strconv.Itoa(v.(int)) + `,`
				case string:
					jsonStr += `"` + k + `":"` + v.(string) + `",`
				}
			}
			jsonStr = jsonStr[0:len(jsonStr)-1] + "}"
			w.RequestBody = []byte(jsonStr)
		}
	}
	var request *http.Request
	if w.RequestBody != nil {
		request, err = http.NewRequest(strings.ToUpper(w.Method), w.EndPoint, bytes.NewReader(w.RequestBody))
		if err != nil {
			return err
		}
	} else {
		request, err = http.NewRequest(strings.ToUpper(w.Method), w.EndPoint, nil)
		if err != nil {
			return err
		}
	}
	request.Header.Add("Content-Type", "application/json")
	for k, v := range w.RequestHeader {
		request.Header.Add(k, v)
	}
	res, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	if res.Body != nil {
		defer res.Body.Close()
	}
	if res.StatusCode/100 == 2 {
		return nil
	}
	return fmt.Errorf("Endpoint return status code is %d", res.StatusCode)
}
