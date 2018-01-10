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

package model

import "time"

//TableName 表名
func (t *RegionProcotols) TableName() string {
	return "region_protocols"
}

//RegionProcotols RegionProcotol
type RegionProcotols struct {
	ID            uint      `gorm:"column:ID"`
	CreatedAt     time.Time `gorm:"column:create_time"`
	ProtocolGroup string    `gorm:"column:protocol_group;size:32;primary_key" json:"protocol_group"`
	ProtocolChild string    `gorm:"column:protocol_child;size:32;primary_key" json:"protocol_child"`
	APIVersion    string    `gorm:"column:api_version;size:8" json:"api_version"`
	IsSupport     bool      `gorm:"column:is_support;default:false" json:"is_support"`
}

//STREAMGROUP STREAMGROUP
var STREAMGROUP = "stream"

//HTTPGROUP HTTPGROUP
var HTTPGROUP = "http"

//MYSQLPROTOCOL MYSQLPROTOCOL
var MYSQLPROTOCOL = "mysql"

//UDPPROTOCOL UDPPROTOCOL
var UDPPROTOCOL = "udp"

//TCPPROTOCOL TCPPROTOCOL
var TCPPROTOCOL = "tcp"

//V2VERSION region version
var V2VERSION = "v2"
