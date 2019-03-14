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
}

// Equal tests for equality between two RbdEndpoint types
func (l1 *RbdEndpoint) Equal(l2 *RbdEndpoint) bool {
	return false
}
