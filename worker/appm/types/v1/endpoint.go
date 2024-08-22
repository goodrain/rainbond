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

// 该文件定义了Rainbond平台中的RbdEndpoint和RbdEndpoints数据结构，这些结构用于描述服务端点的信息。
// 在Rainbond平台中，服务的端点可能包括多个IP地址和端口，它们用于与其他服务或客户端进行通信。

// 文件的主要内容包括：
// 1. `RbdEndpoints` 结构体：用于存储多个RbdEndpoint信息的集合。该结构体包含了一个端口和多个IP地址，
//    其中包括已准备好的IP地址和未准备好的IP地址。
// 2. `RbdEndpoint` 结构体：用于描述单个服务端点的信息。它包含了端点的唯一标识（UUID）、服务ID（Sid）、
//    IP地址、端口、状态、在线状态、动作类型以及是否为域名的标志。
// 3. `Equal` 方法：用于比较两个 `RbdEndpoint` 实例是否相等。当前的实现始终返回 `false`，
//    可能需要根据具体需求对该方法进行实现。

// 总的来说，该文件定义了Rainbond平台中关于服务端点的基本数据结构，
// 这些结构为管理和操作服务的端点信息提供了基础。

package v1

// RbdEndpoints is a collection of RbdEndpoint.
type RbdEndpoints struct {
	Port        int      `json:"port"`
	IPs         []string `json:"ips"`
	NotReadyIPs []string `json:"not_ready_ips"`
}

// RbdEndpoint hold information to create k8s endpoints.
type RbdEndpoint struct {
	UUID     string `json:"uuid"`
	Sid      string `json:"sid"`
	IP       string `json:"ip"`
	Port     int    `json:"port"`
	Status   string `json:"status"`
	IsOnline bool   `json:"is_online"`
	Action   string `json:"action"`
	IsDomain bool   `json:"is_domain"`
}

// Equal tests for equality between two RbdEndpoint types
func (l1 *RbdEndpoint) Equal(l2 *RbdEndpoint) bool {
	return false
}
