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
func (t *CodeCheckResult) TableName() string {
	return "tenant_services_codecheck"
}

//CodeCheckResult codecheck result struct
type CodeCheckResult struct {
	Model
	ServiceID       string `gorm:"column:service_id;size:70"`
	Condition       string `gorm:"column:condition"`
	Language        string `gorm:"column:language"`
	CheckType       string `gorm:"column:check_type"`
	GitURL          string `gorm:"column:git_url"`
	CodeVersion     string `gorm:"column:code_version"`
	GitProjectId    string `gorm:"column:git_project_id"`
	CodeFrom        string `gorm:"column:code_from"`
	URLRepos        string `gorm:"column:url_repos"`
	DockerFileReady bool   `gorm:"column:docker_file_ready"`
	InnerPort       string `gorm:"column:inner_port"`
	VolumeMountPath string `gorm:"column:volume_mount_path"`
	BuildImageName  string `gorm:"column:image"`
	PortList        string `gorm:"column:port_list"`
	VolumeList      string `gorm:"column:volume_list"`
}
