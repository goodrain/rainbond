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
)

//ClusterInterface cluster api
type ClusterInterface interface {
	GetClusterInfo() (*model.ClusterResource, *util.APIHandleError)
	GetClusterHealth() (*utilhttp.ResponseBody, *util.APIHandleError)
}

func (r *regionImpl) Cluster() ClusterInterface {
	return &cluster{prefix: "/v2/cluster", regionImpl: *r}
}

type cluster struct {
	regionImpl
	prefix string
}

func (c *cluster) GetClusterInfo() (*model.ClusterResource, *util.APIHandleError) {
	var cr model.ClusterResource
	var decode utilhttp.ResponseBody
	decode.Bean = &cr
	code, err := c.DoRequest(c.prefix, "GET", nil, &decode)
	if err != nil {
		return nil, handleErrAndCode(err, code)
	}
	return &cr, nil
}

func (c *cluster) GetClusterHealth() (*utilhttp.ResponseBody, *util.APIHandleError) {

	var decode utilhttp.ResponseBody
	code, err := c.DoRequest(c.prefix, "GET", nil, &decode)
	if err != nil {
		return nil, handleErrAndCode(err, code)
	}
	return &decode, nil
}
