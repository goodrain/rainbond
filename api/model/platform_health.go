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

package model

// PlatformHealthResponse 平台健康状态响应
type PlatformHealthResponse struct {
	Status      string          `json:"status"`       // 整体健康状态: healthy, warning, unhealthy
	TotalIssues int             `json:"total_issues"` // 问题总数
	Issues      []HealthIssue   `json:"issues"`       // 问题列表
}

// HealthIssue 健康问题详情
type HealthIssue struct {
	Priority string  `json:"priority"` // 优先级: P0, P1
	Category string  `json:"category"` // 类别: database, kubernetes, registry, storage, disk, compute, node, monitor
	Name     string  `json:"name"`     // 组件/资源名称
	Instance string  `json:"instance"` // 具体实例标识
	Status   string  `json:"status"`   // 状态: down, warning
	Message  string  `json:"message"`  // 问题描述
	Solution string  `json:"solution"` // 解决方案
	Metric   string  `json:"metric"`   // Prometheus 指标名称
	Value    float64 `json:"value"`    // 指标当前值
}
