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

//K8sService service source in k8s
type K8sService struct {
	Model
	TenantID  string `gorm:"column:tenant_id;size:32"`
	ServiceID string `gorm:"column:service_id;size:32"`
	//部署资源的ID ,例如rc ,deploment, statefulset
	ReplicationID   string `gorm:"column:rc_id;size:32"`
	ReplicationType string `gorm:"column:rc_type;"`
	K8sServiceID    string `gorm:"column:inner_service_id;size:60;unique_index"`
	ContainerPort   int    `gorm:"column:container_port;default:0"`
	//是否是对外服务
	IsOut bool `gorm:"column:is_out"`
}

//TableName 表名
func (t *K8sService) TableName() string {
	return "inner_service_port"
}

//K8sDeployReplication 应用与k8s资源对应情况
type K8sDeployReplication struct {
	Model
	TenantID  string `gorm:"column:tenant_id;size:32"`
	ServiceID string `gorm:"column:service_id;size:32"`
	//部署资源的ID ,例如rc ,deploment, statefulset
	ReplicationID   string `gorm:"column:rc_id;size:32"`
	ReplicationType string `gorm:"column:rc_type;"`
	DeployVersion   string `gorm:"column:deploy_version"`
	IsDelete        bool   `gorm:"column:is_delete"`
}

var TypeStatefulSet = "statefulset"
var TypeDeployment = "deployment"
var TypeReplicationController = "replicationcontroller"

//TableName 表名
func (t *K8sDeployReplication) TableName() string {
	return "service_deploy_record"
}

//K8sPod 应用Pod信息
type K8sPod struct {
	Model
	ServiceID string `gorm:"column:service_id;size:32"`
	//部署资源的ID ,例如rc ,deploment, statefulset
	ReplicationID   string `gorm:"column:rc_id;size:32"`
	ReplicationType string `gorm:"column:rc_type;"`
	PodName         string `gorm:"column:pod_name;size:60"`
	PodIP           string `gorm:"column:pod_ip;size:32"`
}

//TableName 表名
func (t *K8sPod) TableName() string {
	return "tenant_service_pod"
}

//ServiceProbe 应用探针信息
type ServiceProbe struct {
	Model
	ServiceID string `gorm:"column:service_id;size:32" json:"service_id" validate:"service_id|between:30,33"`
	ProbeID   string `gorm:"column:probe_id;size:32" json:"probe_id" validate:"probe_id|between:30,33"`
	Mode      string `gorm:"column:mode;default:'liveness'" json:"mode" validate:"mode"`
	Scheme    string `gorm:"column:scheme;default:'scheme'" json:"scheme" validate:"scheme"`
	Path      string `gorm:"column:path" json:"path" validate:"path"`
	Port      int    `gorm:"column:port;size:5;default:80" json:"port" validate:"port|required|numeric_between:1,65535"`
	Cmd       string `gorm:"column:cmd;size:150" json:"cmd" validate:"cmd"`
	//http请求头，key=value,key2=value2
	HTTPHeader string `gorm:"column:http_header;size:300" json:"http_header" validate:"http_header"`
	//初始化等候时间
	InitialDelaySecond int `gorm:"column:initial_delay_second;size:2;default:1" json:"initial_delay_second" validate:"initial_delay_second"`
	//检测间隔时间
	PeriodSecond int `gorm:"column:period_second;size:2;default:3" json:"period_second" validate:"period_second"`
	//检测超时时间
	TimeoutSecond int `gorm:"column:timeout_second;size:3;default:30" json:"timeout_second" validate:"timeout_second"`
	//是否启用
	IsUsed int `gorm:"column:is_used;size:1;default:1" json:"is_used" validate:"is_used"`
	//标志为失败的检测次数
	FailureThreshold int `gorm:"column:failure_threshold;size:2;default:3" json:"failure_threshold" validate:"failure_threshold"`
	//标志为成功的检测次数
	SuccessThreshold int `gorm:"column:success_threshold;size:2;default:1" json:"success_threshold" validate:"success_threshold"`
}

//TableName 表名
func (t *ServiceProbe) TableName() string {
	return "service_probe"
}

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
