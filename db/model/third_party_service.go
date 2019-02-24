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

// Endpoint is a persistent object for table 3rd_party_svc_endpoints.
type Endpoint struct {
	Model
	UUID      string `gorm:"column:uuid;size:32"`
	ServiceID string `gorm:"column:service_id;size:32;not null"`
	IP        string `gorm:"column:ip;not null"`
	Port      int    `gorm:"column:port;size:65535"`
	IsOnline  bool   `gorm:"column:is_online;default:true"`
}

// TableName returns table name of Endpoint.
func (Endpoint) TableName() string {
	return "3rd_party_svc_endpoints"
}

// ThirdPartyServiceProbe is a persistent object for table 3rd_party_svc_probe.
// One third-party service can only have one probe.
type ThirdPartyServiceProbe struct {
	Model
	ServiceID    string `gorm:"column:service_id;size:32"`
	Scheme       string `gorm:"column:scheme;"`
	Port         int    `gorm:"column:port"`
	Path         string `gorm:"column:path"`
	TimeInterval int    `gorm:"column:time_interval"`
	MaxErrorNum  int    `gorm:"column:max_error_num"`
	Action       string `gorm:"column:action"`
}

// TableName returns table name of ThirdPartyServiceProbe
func (ThirdPartyServiceProbe) TableName() string {
	return "3rd_party_svc_probe"
}

// ThirdPartyServiceDiscoveryCfg s a persistent object for table
// 3rd_party_svc_discovery_cfg. 3rd_party_svc_discovery_cfg contains
// service discovery center configuration for third party service.
type ThirdPartyServiceDiscoveryCfg struct {
	Model
	ServiceID string `gorm:"column:service_id;size:32"`
	Type      string `gorm:"column:type"`
	Servers   string `gorm:"column:servers"`
	Key       string `gorm:"key"`
	Username  string `gorm:"username"`
	Password  string `gorm:"password"`
}

// TableName returns table name of ThirdPartyServiceDiscoveryCfg.
func (ThirdPartyServiceDiscoveryCfg) TableName() string {
	return "3rd_party_svc_discovery_cfg"
}