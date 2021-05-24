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

package service

import (
	"github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/node/api/model"
	"github.com/goodrain/rainbond/node/utils"
)

//PrometheusService prometheus service
type PrometheusService struct {
	prometheusAPI *model.PrometheusAPI
	conf          *option.Conf
}

var prometheusService *PrometheusService

//CreatePrometheusService create prometheus service
func CreatePrometheusService(c *option.Conf) *PrometheusService {
	if prometheusService == nil {
		prometheusService = &PrometheusService{
			prometheusAPI: &model.PrometheusAPI{API: c.PrometheusAPI},
			conf:          c,
		}
	}
	return prometheusService
}

//Exec exec prometheus query
func (ts *PrometheusService) Exec(expr string) (*model.Prome, *utils.APIHandleError) {
	resp, err := ts.prometheusAPI.Query(expr)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

//ExecRange exec prometheus query range
func (ts *PrometheusService) ExecRange(expr, start, end, step string) (*model.Prome, *utils.APIHandleError) {
	resp, err := ts.prometheusAPI.QueryRange(expr, start, end, step)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
