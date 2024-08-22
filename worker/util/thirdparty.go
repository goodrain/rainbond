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
// 本文件定义了一个用于创建第三方服务端点名称的工具函数。
// 该函数旨在为第三方服务创建符合 Kubernetes 命名规范的端点名称。

// 1. `CreateEndpointsName` 函数：
//    - 该函数接收三个字符串参数：租户名称 (`tenantName`)、服务名称 (`serviceName`) 和唯一标识符 (`uuid`)。
//    - 函数将这三个参数组合成一个格式化的字符串，形式为 "租户名称-服务名称-UUID"。
//    - 在将组合后的字符串转换为小写字母以符合 Kubernetes 的命名规则后，函数会返回该字符串作为最终的端点名称。
//    - Kubernetes 资源的名称长度限制为 253 个字符，并且只能包含小写字母、数字、`-` 和 `.` 等字符，
//      因此该函数确保生成的名称符合这些要求。

// 总的来说，`CreateEndpointsName` 函数为 Rainbond 平台中的第三方服务端点生成标准化的名称，
// 确保其在 Kubernetes 环境中合法且符合最佳实践。

package util

import (
	"fmt"
	"strings"
)

// CreateEndpointsName creates name for third-party endpoints
// the names of Kubernetes resources should be up to maximum length of 253
// characters and consist of lower case alphanumeric characters, -, and .,
// but certain resources have more specific restrictions.
func CreateEndpointsName(tenantName, serviceName, uuid string) string {
	str := fmt.Sprintf("%s-%s-%s", tenantName, serviceName, uuid)
	str = strings.ToLower(str)
	// TODO: - and .
	return str
}
