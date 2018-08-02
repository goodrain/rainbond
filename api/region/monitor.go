// RAINBOND, Application Management Platform
// Copyright (C) 2014-2017 Goodrain Co., Ltd.

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

package region

import (
	"github.com/goodrain/rainbond/api/util"
	"github.com/goodrain/rainbond/node/api/model"
	utilhttp "github.com/goodrain/rainbond/util/http"
	"fmt"
	"encoding/json"
	"bytes"
	"os"
	"errors"
	"io/ioutil"
	"github.com/Sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

//ClusterInterface cluster api
type MonitorInterface interface {
	GetRule(name string) (*model.AlertingNameConfig, *util.APIHandleError)
	GetAllRule() (*model.AlertingRulesConfig, *util.APIHandleError)
	DelRule(name string) (*utilhttp.ResponseBody, *util.APIHandleError)
	AddRule(path string) (*utilhttp.ResponseBody, *util.APIHandleError)
	RegRule(ruleName string, path string) (*utilhttp.ResponseBody, *util.APIHandleError)
}

func (r *regionImpl) Monitor() MonitorInterface {
	return &monitor{prefix: "/v2/rules", regionImpl: *r}
}

type monitor struct {
	regionImpl
	prefix string
}

func (m *monitor) GetRule(name string) (*model.AlertingNameConfig, *util.APIHandleError) {
	var ac model.AlertingNameConfig
	var decode utilhttp.ResponseBody
	decode.Bean = &ac
	code, err := m.DoRequest(m.prefix+"/"+name, "GET", nil, &decode)
	if err != nil {
		return nil, handleErrAndCode(err, code)
	}
	if code != 200 {
		logrus.Error("Return failure message ", decode.Bean)
		return nil, util.CreateAPIHandleError(code, fmt.Errorf("get alerting rules error code %d", code))
	}
	return &ac, nil
}

func (m *monitor) GetAllRule() (*model.AlertingRulesConfig, *util.APIHandleError) {
	var ac model.AlertingRulesConfig
	var decode utilhttp.ResponseBody
	decode.Bean = &ac
	code, err := m.DoRequest(m.prefix+"/all", "GET", nil, &decode)
	if err != nil {
		return nil, handleErrAndCode(err, code)
	}
	if code != 200 {
		logrus.Error("Return failure message ", decode.Bean)
		return nil, util.CreateAPIHandleError(code, fmt.Errorf("get alerting rules error code %d", code))
	}
	return &ac, nil
}

func (m *monitor) DelRule(name string) (*utilhttp.ResponseBody, *util.APIHandleError) {
	var decode utilhttp.ResponseBody
	code, err := m.DoRequest(m.prefix+"/"+name, "DELETE", nil, &decode)
	if err != nil {
		return nil, handleErrAndCode(err, code)
	}
	if code != 200 {
		logrus.Error("Return failure message ", decode.Bean)
		return nil, util.CreateAPIHandleError(code, fmt.Errorf("del alerting rules error code %d", code))
	}
	return &decode, nil
}

func (m *monitor) AddRule(path string) (*utilhttp.ResponseBody, *util.APIHandleError) {
	_, err := os.Stat(path)
	if err != nil {
		if !os.IsExist(err) {
			return nil, util.CreateAPIHandleError(400, errors.New("file does not exist"))
		}
	}

	content, err := ioutil.ReadFile(path)
	if err != nil {
		logrus.Error("Failed to read AlertingRules config file: ", err.Error())
		return nil, util.CreateAPIHandleError(400, err)
	}
	var rulesConfig model.AlertingNameConfig
	if err := yaml.Unmarshal(content, &rulesConfig); err != nil {
		logrus.Error("Unmarshal AlertingRulesConfig config string to object error.", err.Error())
		return nil, util.CreateAPIHandleError(400, err)

	}
	var decode utilhttp.ResponseBody
	body, err := json.Marshal(rulesConfig)
	if err != nil {
		return nil, util.CreateAPIHandleError(400, err)
	}
	code, err := m.DoRequest(m.prefix, "POST", bytes.NewBuffer(body), &decode)
	if err != nil {
		return nil, handleErrAndCode(err, code)
	}
	if code != 200 {
		logrus.Error("Return failure message ", decode.Bean)
		return nil, util.CreateAPIHandleError(code, fmt.Errorf("add alerting rules error code %d", code))
	}
	return &decode, nil
}

func (m *monitor) RegRule(ruleName string, path string) (*utilhttp.ResponseBody, *util.APIHandleError) {
	_, err := os.Stat(path)
	if err != nil {
		if !os.IsExist(err) {
			return nil, util.CreateAPIHandleError(400, errors.New("file does not exist"))
		}
	}

	content, err := ioutil.ReadFile(path)
	if err != nil {
		logrus.Error("Failed to read AlertingRules config file: ", err.Error())
		return nil, util.CreateAPIHandleError(400, err)
	}
	var rulesConfig model.AlertingNameConfig
	if err := yaml.Unmarshal(content, &rulesConfig); err != nil {
		logrus.Error("Unmarshal AlertingRulesConfig config string to object error.", err.Error())
		return nil, util.CreateAPIHandleError(400, err)

	}
	var decode utilhttp.ResponseBody
	body, err := json.Marshal(rulesConfig)
	if err != nil {
		return nil, util.CreateAPIHandleError(400, err)
	}
	code, err := m.DoRequest(m.prefix+"/"+ruleName, "PUT", bytes.NewBuffer(body), &decode)
	if err != nil {
		return nil, handleErrAndCode(err, code)
	}
	if code != 200 {
		logrus.Error("Return failure message ", decode.Bean)
		return nil, util.CreateAPIHandleError(code, fmt.Errorf("add alerting rules error code %d", code))
	}
	return &decode, nil
}
