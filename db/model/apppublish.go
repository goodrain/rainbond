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

//AppPublish AppPublish
type AppPublish struct {
	Model
	ServiceKey string `gorm:"column:service_key;size:70"`
	AppVersion string `gorm:"column:app_version;size:70"`
	Status     string `gorm:"column:status;size:10"`
	Image      string `gorm:"column:image;size:200"`
	Slug       string `gorm:"column:slug;size:200"`
	DestYS     bool   `gorm:"column:dest_ys"`
	DestYB     bool   `gorm:"column:dest_yb"`
	ShareID    string `gorm:"column:share_id;size:70"`
}

//TableName 表名
func (t *AppPublish) TableName() string {
	return "app_publish"
}
