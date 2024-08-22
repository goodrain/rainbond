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

// 该文件定义了一个用于获取通用标签的方法 `GetCommonLabels`，
// 该方法是 Rainbond 平台中 `AppService` 结构体的一部分。
// 通过合并传入的多个标签（键值对）并添加一些额外的默认标签，
// 此方法返回一个完整的标签集合，用于标识和管理应用服务。

// 文件的主要内容包括：
// 1. `GetCommonLabels` 方法：该方法接收多个标签的 `map` 作为可变参数，并将它们合并到一个结果标签集中。
//    如果当前服务不处于干运行模式 (`DryRun` 为 `false`)，还会添加创建者ID（`creater_id`）到标签中。
//    最后，还会自动添加一些默认标签，例如 `creator`（固定为 "Rainbond"）、`service_id`、`service_alias`、
//    `tenant_name`、`tenant_id`、`app_id` 和 `rainbond_app`。
//    这些标签通常用于在 Kubernetes 环境中标识和管理 Rainbond 应用服务。

// 总的来说，该文件的功能在于为 Rainbond 应用服务生成一组通用的标签集合，
// 这些标签可以在 Kubernetes 中用于标识和追踪相关的资源和操作。

package v1

// GetCommonLabels get common labels
func (a *AppService) GetCommonLabels(labels ...map[string]string) map[string]string {
	var resultLabel = make(map[string]string)
	for _, l := range labels {
		for k, v := range l {
			resultLabel[k] = v
		}
	}
	if !a.DryRun {
		resultLabel["creater_id"] = a.CreaterID
	}
	resultLabel["creator"] = "Rainbond"
	resultLabel["service_id"] = a.ServiceID
	resultLabel["service_alias"] = a.ServiceAlias
	resultLabel["tenant_name"] = a.TenantName
	resultLabel["tenant_id"] = a.TenantID
	resultLabel["app_id"] = a.AppID
	resultLabel["rainbond_app"] = a.K8sApp
	return resultLabel
}
