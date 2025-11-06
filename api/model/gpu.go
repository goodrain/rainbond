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

package model

// GPUCard GPU 卡信息
type GPUCard struct {
	ID              int     `json:"id"`                // GPU ID (0, 1, 2, ...)
	Model           string  `json:"model"`             // GPU 型号 (Tesla V100, A100, etc.)
	Memory          int64   `json:"memory"`            // 显存容量 (MB)
	UsedMemory      int64   `json:"used_memory"`       // 已使用显存 (MB)
	UtilizationRate float64 `json:"utilization_rate"`  // 使用率 (0-100)
	Temperature     int     `json:"temperature"`       // 温度 (摄氏度)
	UUID            string  `json:"uuid"`              // GPU UUID
}

// GPUResource GPU 资源信息
type GPUResource struct {
	GPUCards        []GPUCard `json:"gpu_cards"`         // GPU 卡列表
	TotalGPUCount   int       `json:"total_gpu_count"`   // GPU 总数
	UsedGPUCount    int       `json:"used_gpu_count"`    // 已使用 GPU 数
	TotalMemory     int64     `json:"total_memory"`      // 总显存 (MB)
	UsedMemory      int64     `json:"used_memory"`       // 已使用显存 (MB)
	UtilizationRate float64   `json:"utilization_rate"`  // 整体使用率 (0-100)
}

// NodeGPUInfo 节点 GPU 信息
type NodeGPUInfo struct {
	NodeName    string      `json:"node_name"`     // 节点名称
	Status      string      `json:"status"`        // 节点状态
	InternalIP  string      `json:"internal_ip"`   // 内部 IP
	GPUResource GPUResource `json:"gpu_resource"`  // GPU 资源
}

// ClusterGPUOverview 集群 GPU 总览
type ClusterGPUOverview struct {
	GPUNodeCount    int           `json:"gpu_node_count"`    // GPU 节点数
	TotalGPUCount   int           `json:"total_gpu_count"`   // GPU 卡总数
	UsedGPUCount    int           `json:"used_gpu_count"`    // 已使用 GPU 数
	TotalMemory     int64         `json:"total_memory"`      // 总显存 (MB)
	UsedMemory      int64         `json:"used_memory"`       // 已使用显存 (MB)
	UtilizationRate float64       `json:"utilization_rate"`  // 整体使用率
	Nodes           []NodeGPUInfo `json:"nodes"`             // 节点列表
}

// TenantGPUQuotaReq 设置团队 GPU 配额请求
type TenantGPUQuotaReq struct {
	GPULimit       int   `json:"gpu_limit" validate:"min=0"`
	GPUMemoryLimit int64 `json:"gpu_memory_limit" validate:"min=0"`
}

// TenantGPUQuotaResp 团队 GPU 配额响应
type TenantGPUQuotaResp struct {
	TenantID       string `json:"tenant_id"`
	GPULimit       int    `json:"gpu_limit"`
	GPUMemoryLimit int64  `json:"gpu_memory_limit"`
	CreateTime     string `json:"create_time,omitempty"`
	UpdateTime     string `json:"update_time,omitempty"`
}

// TenantGPUUsageResp 团队 GPU 使用情况响应
type TenantGPUUsageResp struct {
	TenantID       string  `json:"tenant_id"`
	UsedGPU        int     `json:"used_gpu"`
	UsedGPUMemory  int64   `json:"used_gpu_memory"`
	GPULimit       int     `json:"gpu_limit"`
	GPUMemoryLimit int64   `json:"gpu_memory_limit"`
	UsageRate      float64 `json:"usage_rate"`
}

// ServiceGPUConfigReq 设置组件 GPU 配置请求
type ServiceGPUConfigReq struct {
	EnableGPU          bool   `json:"enable_gpu"`
	GPUCount           int    `json:"gpu_count,omitempty" validate:"min=0"`
	GPUMemory          int64  `json:"gpu_memory,omitempty" validate:"min=0"`
	GPUCores           int    `json:"gpu_cores,omitempty" validate:"min=0,max=100"`
	GPUModelPreference string `json:"gpu_model_preference,omitempty"`
}

// ServiceGPUConfigResp 组件 GPU 配置响应
type ServiceGPUConfigResp struct {
	ServiceID          string                  `json:"service_id"`
	EnableGPU          bool                    `json:"enable_gpu"`
	GPUCount           int                     `json:"gpu_count"`
	GPUMemory          int64                   `json:"gpu_memory"`
	GPUCores           int                     `json:"gpu_cores"`
	GPUModelPreference string                  `json:"gpu_model_preference"`
	CreateTime         string                  `json:"create_time,omitempty"`
	UpdateTime         string                  `json:"update_time,omitempty"`
	TeamQuota          *TenantGPUUsageResp     `json:"team_quota,omitempty"`
}

// GPUModelsResp GPU 型号列表响应
type GPUModelsResp struct {
	Models []string `json:"models"`
}
