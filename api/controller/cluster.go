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

package controller

import (
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/api/handler"
	apimodel "github.com/goodrain/rainbond/api/model"
	httputil "github.com/goodrain/rainbond/util/http"
	"net/http"
)

type ClusterStruct struct{}

func (c *ClusterStruct) GetClusterResources(w http.ResponseWriter, r *http.Request) {
	logrus.Debugf("request uri: %s", r.RequestURI)

	cpu, memory, err := handler.GetClusterHandler().GetAllocatableResources(handler.GetNodeProxy())
	if err != nil {
		httputil.ReturnError(r, w, 500, fmt.Sprintf("error getting allocatable resources: %v", err))
		return
	}
	resp := apimodel.ClusterResourceRespVO{
		AllocatableCPU:    cpu,
		AllocatableMemory: memory,
	}
	httputil.ReturnSuccess(r, w, resp)
}
