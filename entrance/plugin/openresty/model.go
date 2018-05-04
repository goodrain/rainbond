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

package openresty

type NginxInstance struct {
	Addr                 string // 10.10.10.11:8081
	State                string // health/unhealth/dead
	HeartbeatLast        int64  // time.Now().Unix()
	GetHeartbeatInterval int64  // 3s
	HeartbeatTimeOut     int64  // 30s
	TimeOutExprie        int64  // 90s
}

type NginxNode struct {
	State string `json:"state"` // Active/Draining/Disabled

	// 每个server是IP加端口的形，后面可以加nginx兼容的选项
	// 10.10.10.11:8080 weight=5 service=http resolve
	Addr   string `json:"addr"`
	Weight int    `json:"weight"`
}

type NginxUpstream struct {
	Name     string      `json:"name"`
	Servers  []NginxNode `json:"servers"`
	Protocol string      `json:"protocol"`
}

func (u *NginxUpstream) AddNode(node NginxNode) {
	u.Servers = append(u.Servers, node)
}

type NginxServer struct {
	Name     string            `json:"name"`
	Domain   string            `json:"domain"`
	Port     int32             `json:"port"`
	Path     string            `json:"path"`
	Protocol string            `json:"protocol"` // http and https only
	Cert     string            `json:"cert"`
	Key      string            `json:"key"`
	Options  map[string]string `json:"options"`
	Upstream string            `json:"upstream"`
	ToHTTPS  bool              `json:"toHTTPS"`
}

type Options struct {
	Protocol string `json:"protocol"` // http/https/tcp/udp
}
