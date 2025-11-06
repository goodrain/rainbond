// Copyright (C) 2014-2024 Goodrain Co., Ltd.
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

	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/api/handler"
	httputil "github.com/goodrain/rainbond/util/http"
	"github.com/sirupsen/logrus"
)

// GPUController GPU 控制器
type GPUController struct {
	gpuHandler handler.GPUHandler
}

// NewGPUController 创建 GPU 控制器
func NewGPUController() *GPUController {
	return &GPUController{
		gpuHandler: handler.NewGPUHandler(),
	}
}

// GetClusterGPUOverview 获取集群 GPU 总览
// GET /v2/cluster/gpu-overview
func (g *GPUController) GetClusterGPUOverview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	overview, err := g.gpuHandler.GetClusterGPUOverview(ctx)
	if err != nil {
		logrus.Errorf("Failed to get cluster GPU overview: %v", err)
		httputil.ReturnError(r, w, 500, err.Error())
		return
	}

	httputil.ReturnSuccess(r, w, map[string]interface{}{
		"overview": overview,
	})
}

// GetNodeGPUDetail 获取节点 GPU 详情
// GET /v2/cluster/nodes/{node_name}/gpu
func (g *GPUController) GetNodeGPUDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	nodeName := chi.URLParam(r, "node_name")

	if nodeName == "" {
		httputil.ReturnError(r, w, 400, "node_name is required")
		return
	}

	nodeInfo, err := g.gpuHandler.GetNodeGPUDetail(ctx, nodeName)
	if err != nil {
		logrus.Errorf("Failed to get node GPU detail for %s: %v", nodeName, err)
		httputil.ReturnError(r, w, 500, err.Error())
		return
	}

	httputil.ReturnSuccess(r, w, nodeInfo)
}

// GetAvailableGPUModels 获取可用的 GPU 型号列表
// GET /v2/gpu-models
func (g *GPUController) GetAvailableGPUModels(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	models, err := g.gpuHandler.GetAvailableGPUModels(ctx)
	if err != nil {
		logrus.Errorf("Failed to get available GPU models: %v", err)
		httputil.ReturnError(r, w, 500, err.Error())
		return
	}

	httputil.ReturnSuccess(r, w, models)
}

// DetectHAMi 检测是否安装 HAMi
// GET /v2/cluster/hami-status
func (g *GPUController) DetectHAMi(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	installed, err := g.gpuHandler.DetectHAMi(ctx)
	if err != nil {
		logrus.Errorf("Failed to detect HAMi: %v", err)
		httputil.ReturnError(r, w, 500, err.Error())
		return
	}

	httputil.ReturnSuccess(r, w, map[string]interface{}{
		"installed": installed,
	})
}
