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

/*
本文件提供了与 Kubernetes API 相关的转换功能，特别是处理端口协议的转换。
其中定义了 `conversionPortProtocol` 函数，用于将字符串类型的协议转换为 Kubernetes API 中 `v1.Protocol` 类型的协议。
该函数支持三种协议类型：UDP、SCTP 和 TCP，默认转换为 TCP 协议。

文件说明：
1. `conversionPortProtocol` 函数：根据传入的协议字符串，返回对应的 Kubernetes 协议类型。
   - 输入：协议字符串（"udp"、"sctp" 或其他）。
   - 输出：对应的 `v1.Protocol` 类型。
*/

package conversion

import (
	"k8s.io/api/core/v1"
)

func conversionPortProtocol(protocol string) v1.Protocol {
	if protocol == "udp" {
		return v1.ProtocolUDP
	} else if protocol == "sctp" {
		return v1.ProtocolSCTP
	} else {
		return v1.ProtocolTCP
	}
}
