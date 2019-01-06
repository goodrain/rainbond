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
	"bytes"
	"fmt"

	"github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/api/util"
	dbmodel "github.com/goodrain/rainbond/db/model"
	coreutil "github.com/goodrain/rainbond/util"
	utilhttp "github.com/goodrain/rainbond/util/http"
)

type services struct {
	tenant
	prefix string
	model  model.ServiceStruct
}

//ServiceInterface ServiceInterface
type ServiceInterface interface {
	Get() (*serviceInfo, *util.APIHandleError)
	GetDeployInfo() (*ServiceDeployInfo, *util.APIHandleError)
	Pods() ([]*podInfo, *util.APIHandleError)
	List() ([]*dbmodel.TenantServices, *util.APIHandleError)
	Stop(eventID string) (string, *util.APIHandleError)
	Start(eventID string) (string, *util.APIHandleError)
	EventLog(eventID, level string) ([]*model.MessageData, *util.APIHandleError)
}

func (s *services) Pods() ([]*podInfo, *util.APIHandleError) {
	var gc []*podInfo
	var decode utilhttp.ResponseBody
	decode.List = &gc
	code, err := s.DoRequest(s.prefix+"/pods", "GET", nil, &decode)
	if err != nil {
		return nil, util.CreateAPIHandleError(code, err)
	}
	if code != 200 {
		return nil, util.CreateAPIHandleError(code, fmt.Errorf("Get pods code %d", code))
	}
	return gc, nil
}
func (s *services) Get() (*serviceInfo, *util.APIHandleError) {
	var service serviceInfo
	var decode utilhttp.ResponseBody
	decode.Bean = &service
	code, err := s.DoRequest(s.prefix, "GET", nil, &decode)
	if err != nil {
		return nil, util.CreateAPIHandleError(code, err)
	}
	if code != 200 {
		return nil, util.CreateAPIHandleError(code, fmt.Errorf("Get err with code %d", code))
	}
	return &service, nil
}
func (s *services) EventLog(eventID, level string) ([]*model.MessageData, *util.APIHandleError) {
	data := []byte(`{"event_id":"` + eventID + `","level":"` + level + `"}`)
	var message []*model.MessageData
	var decode utilhttp.ResponseBody
	decode.List = &message
	code, err := s.DoRequest(s.prefix+"/event-log", "POST", bytes.NewBuffer(data), &decode)
	if err != nil {
		return nil, util.CreateAPIHandleError(code, err)
	}
	if code != 200 {
		return nil, util.CreateAPIHandleError(code, fmt.Errorf("Get event log code %d", code))
	}
	return message, nil
}

func (s *services) List() ([]*dbmodel.TenantServices, *util.APIHandleError) {
	var gc []*dbmodel.TenantServices
	var decode utilhttp.ResponseBody
	decode.List = &gc
	code, err := s.DoRequest(s.prefix, "GET", nil, &decode)
	if err != nil {
		return nil, util.CreateAPIHandleError(code, err)
	}
	if code != 200 {
		return nil, util.CreateAPIHandleError(code, fmt.Errorf("Get with code %d", code))
	}
	return gc, nil
}
func (s *services) Stop(eventID string) (string, *util.APIHandleError) {
	if eventID == "" {
		eventID = coreutil.NewUUID()
	}
	data := []byte(`{"event_id":"` + eventID + `"}`)
	var res utilhttp.ResponseBody
	code, err := s.DoRequest(s.prefix+"/stop", "POST", bytes.NewBuffer(data), nil)
	if err != nil {
		return "", handleErrAndCode(err, code)
	}
	return eventID, handleAPIResult(code, res)
}
func (s *services) Start(eventID string) (string, *util.APIHandleError) {
	if eventID == "" {
		eventID = coreutil.NewUUID()
	}
	var res utilhttp.ResponseBody
	data := []byte(`{"event_id":"` + eventID + `"}`)
	code, err := s.DoRequest(s.prefix+"/start", "POST", bytes.NewBuffer(data), nil)
	if err != nil {
		return "", handleErrAndCode(err, code)
	}
	return eventID, handleAPIResult(code, res)
}

//GetDeployInfo get service deploy info
func (s *services) GetDeployInfo() (*ServiceDeployInfo, *util.APIHandleError) {
	var deployInfo ServiceDeployInfo
	var decode utilhttp.ResponseBody
	decode.Bean = &deployInfo
	code, err := s.DoRequest(s.prefix+"/deploy-info", "GET", nil, &decode)
	return &deployInfo, handleErrAndCode(err, code)
}
