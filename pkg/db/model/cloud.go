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
func (t *RegionUserInfo) TableName() string {
	return "user_region_info"
}

//RegionUserInfo RegionUserInfo
type RegionUserInfo struct {
	Model
	EID            string `gorm:"column:eid;size:34" json:"eid"`
	APIRange       string `gorm:"column:api_range;size:24" json:"api_range"`
	RegionTag      string `gorm:"column:region_tag;size:24" json:"region_tag"`
	ValidityPeriod int    `gorm:"column:validity_period;size:10" json:"validity_period"`
	Token          string `gorm:"column:token;size:32" json:"token"`
	CA             string `gorm:"column:ca;size:4096" json:"ca"`
	Key            string `gorm:"column:key;size:4096" json:"key"`
}
