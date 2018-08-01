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
)

//ClusterInterface cluster api
type MonitorInterface interface {
	GetRule(name string) (*model.AlertingNameConfig, *util.APIHandleError)
}

func (r *regionImpl) Monitor() MonitorInterface {
	return &monitor{prefix: "/v2/rules", regionImpl: *r}
}

type monitor struct {
	regionImpl
	prefix string
}

func (m *monitor) GetRule(name string) (*model.AlertingNameConfig, *util.APIHandleError) {
	println("======>1.1")
	var ac model.AlertingNameConfig
	var decode utilhttp.ResponseBody
	decode.Bean = &ac
	println("======>1.2")
	code, err := m.DoRequest(m.prefix+"/"+name, "GET", nil, &decode)
	println("======err1>",code,err)
	if err != nil {
		println("======err2>",code,err)
		return nil, handleErrAndCode(err, code)
	}
	if code != 200 {
		return nil, util.CreateAPIHandleError(code, fmt.Errorf("get alerting rules error code %d", code))
	}
	return &ac, nil
}

