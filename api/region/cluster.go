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
	utilhttp "github.com/goodrain/rainbond/util/http"
)

//ClusterResource cluster resource model
type ClusterResource struct {
	AllNode        int     `json:"all_node"`
	NotReadyNode   int     `json:"notready_node"`
	ComputeNode    int     `json:"compute_node"`
	Tenant         int     `json:"tenant"`
	CapCPU         int     `json:"cap_cpu"`          //可分配CPU总额
	CapMem         int     `json:"cap_mem"`          //可分配Mem总额
	HealthCapCPU   int     `json:"health_cap_cpu"`   //健康可分配CPU
	HealthCapMem   int     `json:"health_cap_mem"`   //健康可分配Mem
	UnhealthCapCPU int     `json:"unhealth_cap_cpu"` //不健康可分配CPU
	UnhealthCapMem int     `json:"unhealth_cap_mem"` //不健康可分配Mem
	ReqCPU         float32 `json:"req_cpu"`          //已使用CPU总额
	ReqMem         int     `json:"req_mem"`          //已使用Mem总额
	HealthReqCPU   float32 `json:"health_req_cpu"`   //健康已使用CPU
	HealthReqMem   int     `json:"health_req_mem"`   //健康已使用Mem
	UnhealthReqCPU float32 `json:"unhealth_req_cpu"` //不健康已使用CPU
	UnhealthReqMem int     `json:"unhealth_req_mem"` //不健康已使用Mem
	CapDisk        uint64  `json:"cap_disk"`
	ReqDisk        uint64  `json:"req_disk"`
}

//ClusterInterface cluster api
type ClusterInterface interface {
	GetClusterInfo() (*ClusterResource, *util.APIHandleError)
	GetClusterHealth() (*utilhttp.ResponseBody, *util.APIHandleError)
}

func (r *regionImpl) Cluster() ClusterInterface {
	return &cluster{prefix: "/v2/cluster", regionImpl: *r}
}

type cluster struct {
	regionImpl
	prefix string
}

func (c *cluster) GetClusterInfo() (*ClusterResource, *util.APIHandleError) {
	var cr ClusterResource
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
