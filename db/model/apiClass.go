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

//TableName 表名
func (t *RegionAPIClass) TableName() string {
	return "region_api_class"
}

//RegionAPIClass RegionAPIClass
type RegionAPIClass struct {
	Model
	ClassLevel string `gorm:"column:class_level;size:24;" json:"class_level"`
	Prefix     string `gorm:"column:prefix;size:128;" json:"prefix"`
	URI        string `gorm:"column:uri;size:256" json:"uri"`
	Alias      string `gorm:"column:alias;size:64" json:"alias"`
	Remark     string `gorm:"column:remark;size:64" json:"remark"`
}

//NODEMANAGER NODEMANAGER
var NODEMANAGER = "node_manager"

//SERVERSOURCE SERVERSOURCE
var SERVERSOURCE = "server_source"

//ALLPOWER ALLPOWER
var ALLPOWER = "all_power"
