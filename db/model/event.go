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

import "time"

// ASYNEVENTTYPE asyn event type
const ASYNEVENTTYPE = 0

// SYNEVENTTYPE syn event type
const SYNEVENTTYPE = 1

// TargetTypeService service target
const TargetTypeService = "service"

// TargetTypePod -
const TargetTypePod = "pod"

// TargetTypeTenant tenant target
const TargetTypeTenant = "tenant"

// UsernameSystem -
const UsernameSystem = "system"

// EventFinalStatus -
type EventFinalStatus string

// String -
func (e EventFinalStatus) String() string {
	return string(e)
}

// EventFinalStatusComplete -
var EventFinalStatusComplete EventFinalStatus = "complete"

// EventFinalStatusFailure -
var EventFinalStatusFailure EventFinalStatus = "failure"

// EventFinalStatusRunning -
var EventFinalStatusRunning EventFinalStatus = "running"

// EventFinalStatusEmpty -
var EventFinalStatusEmpty EventFinalStatus = "empty"

// EventFinalStatusEmptyComplete -
var EventFinalStatusEmptyComplete EventFinalStatus = "emptycomplete"

// EventStatus -
type EventStatus string

// String -
func (e EventStatus) String() string {
	return string(e)
}

// EventStatusSuccess -
var EventStatusSuccess EventStatus = "success"

// EventStatusFailure -
var EventStatusFailure EventStatus = "failure"

//ServiceEvent event struct
type ServiceEvent struct {
	Model
	EventID      string `gorm:"column:event_id;size:40"`
	TenantID     string `gorm:"column:tenant_id;size:40;index:tenant_id"`
	ServiceID    string `gorm:"column:service_id;size:40;index:service_id"`
	Target       string `gorm:"column:target;size:40"`
	TargetID     string `gorm:"column:target_id;size:255;index:target_id"`
	RequestBody  string `gorm:"column:request_body;size:1024"`
	UserName     string `gorm:"column:user_name;size:40"`
	StartTime    string `gorm:"column:start_time;size:40"`
	EndTime      string `gorm:"column:end_time;size:40"`
	OptType      string `gorm:"column:opt_type;size:40"`
	SynType      int    `gorm:"column:syn_type;size:1"`
	Status       string `gorm:"column:status;size:40"`
	FinalStatus  string `gorm:"column:final_status;size:40"`
	Message      string `gorm:"column:message"`
	Reason       string `gorm:"column:reason"`
	BuildVersion string `gorm:"column:build_version;size:40" json:"build_version"`
}

//TableName 表名
func (t *ServiceEvent) TableName() string {
	return "tenant_services_event"
}

//NotificationEvent NotificationEvent
type NotificationEvent struct {
	Model
	//Kind could be service, tenant, cluster, node
	Kind string `gorm:"column:kind;size:40"`
	//KindID could be service_id,tenant_id,cluster_id,node_id
	KindID string `gorm:"column:kind_id;size:40"`
	Hash   string `gorm:"column:hash;size:100"`
	//Type could be Normal UnNormal Notification
	Type          string    `gorm:"column:type;size:40"`
	Message       string    `gorm:"column:message;size:200"`
	Reason        string    `gorm:"column:reson;size:200"`
	Count         int       `gorm:"column:count;"`
	LastTime      time.Time `gorm:"column:last_time;"`
	FirstTime     time.Time `gorm:"column:first_time;"`
	IsHandle      bool      `gorm:"column:is_handle;"`
	HandleMessage string    `gorm:"column:handle_message;"`
	ServiceName   string    `gorm:"column:service_name;size:40"`
	TenantName    string    `gorm:"column:tenant_name;size:40"`
}

//TableName table name
func (n *NotificationEvent) TableName() string {
	return "region_notification_event"
}
