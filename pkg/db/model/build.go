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

//EventLogMessage event log message struct
type BuildInfo struct {
	Model
	Version         string `gorm:"column:tenant_id;size:40"`
	DeliveredType        string `gorm:"column:service_id;size:40"`
	DeliveredPath        string `gorm:"column:service_id;size:40"`
	ServiceID         string `gorm:"column:user_name;size:40"`
	TenantID        string `gorm:"column:start_time;size:40"`
	GitURL          string `gorm:"column:end_time;size:40"`
	Status           string `gorm:"column:status;size:40"`
	FinalStatus      string `gorm:"column:final_status;size:40"`
	OldDeployVersion string `gorm:"column:old_deploy_version;size:40"`
	CodeVersion      string `gorm:"column:code_version;size:40"`
	CodeMD5      string `gorm:"column:code_version;size:40"`
	OldCodeVersion   string `gorm:"column:old_code_version;size:40"`
	Message          string `gorm:"column:message"`
}

//TableName 表名
func (t *BuildInfo) TableName() string {
	return "service_event"
}
