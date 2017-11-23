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

//VersionInfo version info struct
type VersionInfo struct {
	Model
	BuildVersion  string `gorm:"column:build_version;size:40"` //唯一
	EventID       string `gorm:"column:event_id;size:40"`
	ServiceID     string `gorm:"column:service_id;size:40"`
	BuildFrom     string `gorm:"column:build_from;size:40"` //kind
	DeliveredType string `gorm:"column:delivered_type;size:40"` //kind
	DeliveredPath string `gorm:"column:delivered_path;size:40"` //交付物path
	RepoURL        string `gorm:"column:repo_url;size:100"`
	CodeVersion   string `gorm:"column:code_version;size:40"`
	CommitMsg   string `gorm:"column:code_commit_msg;size:40"`
	Author   string `gorm:"column:code_commit_author;size:40"`
	FinalStatus string `gorm:"column:final_status;size:40"`

}

//TableName 表名
func (t *VersionInfo) TableName() string {
	return "version_info"
}
