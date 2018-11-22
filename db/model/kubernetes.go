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

//TypeStatefulSet typestateful
var TypeStatefulSet = "statefulset"

//TypeDeployment typedeployment
var TypeDeployment = "deployment"

//TypeReplicationController type rc
var TypeReplicationController = "replicationcontroller"

//LocalScheduler 本地调度暂存信息
type LocalScheduler struct {
	Model
	ServiceID string `gorm:"column:service_id;size:32"`
	NodeIP    string `gorm:"column:node_ip;size:32"`
	PodName   string `gorm:"column:pod_name;size:32"`
}

//TableName 表名
func (t *LocalScheduler) TableName() string {
	return "local_scheduler"
}

//ServiceSourceConfig service source config info
//such as deployment、statefulset、configmap
type ServiceSourceConfig struct {
	Model
	ServiceID  string `gorm:"column:service_id;size:32"`
	SourceType string `gorm:"column:source_type;size:32"`
	SourceBody string `gorm:"column:source_body;size:2000"`
}

//TableName 表名
func (t *ServiceSourceConfig) TableName() string {
	return "tenant_services_source"
}
