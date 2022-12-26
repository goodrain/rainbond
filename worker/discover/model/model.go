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

import (
	"time"

	"github.com/goodrain/rainbond/mq/api/grpc/pb"
	"github.com/pquerna/ffjson/ffjson"
	"github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
)

//TaskType 任务类型
type TaskType string

//Task 任务
type Task struct {
	Type       TaskType  `json:"type"`
	Body       TaskBody  `json:"body"`
	CreateTime time.Time `json:"time,omitempty"`
	User       string    `json:"user"`
}

//NewTask 从json bytes data create task
func NewTask(data []byte) (*Task, error) {
	taskType := gjson.GetBytes(data, "type").String()
	body := CreateTaskBody(taskType)
	task := Task{
		Body: body,
	}
	err := ffjson.Unmarshal(data, &task)
	if err != nil {
		return nil, err
	}
	return &task, err
}

//TransTask transtask
func TransTask(task *pb.TaskMessage) (*Task, error) {
	timeT, _ := time.Parse(time.RFC3339, task.CreateTime)
	return &Task{
		Type:       TaskType(task.TaskType),
		Body:       NewTaskBody(task.TaskType, task.TaskBody),
		CreateTime: timeT,
		User:       task.User,
	}, nil
}

//NewTaskBody new task body
func NewTaskBody(taskType string, body []byte) TaskBody {
	switch taskType {
	case "start":
		b := StartTaskBody{}
		err := ffjson.Unmarshal(body, &b)
		if err != nil {
			return nil
		}
		return b
	case "stop":
		b := StopTaskBody{}
		err := ffjson.Unmarshal(body, &b)
		if err != nil {
			return nil
		}
		return b
	case "restart":
		b := RestartTaskBody{}
		err := ffjson.Unmarshal(body, &b)
		if err != nil {
			return nil
		}
		return b
	case "rolling_upgrade":
		b := RollingUpgradeTaskBody{}
		err := ffjson.Unmarshal(body, &b)
		if err != nil {
			return nil
		}
		return b
	case "rollback":
		b := RollBackTaskBody{}
		err := ffjson.Unmarshal(body, &b)
		if err != nil {
			return nil
		}
		return b
	case "group_start":
		b := GroupStartTaskBody{}
		err := ffjson.Unmarshal(body, &b)
		if err != nil {
			return nil
		}
		return b
	case "group_stop":
		b := GroupStopTaskBody{}
		err := ffjson.Unmarshal(body, &b)
		if err != nil {
			return nil
		}
		return b
	case "horizontal_scaling":
		b := HorizontalScalingTaskBody{}
		err := ffjson.Unmarshal(body, &b)
		if err != nil {
			return nil
		}
		return b
	case "vertical_scaling":
		b := VerticalScalingTaskBody{}
		err := ffjson.Unmarshal(body, &b)
		if err != nil {
			return nil
		}
		return b
	case "apply_rule":
		b := ApplyRuleTaskBody{}
		err := ffjson.Unmarshal(body, &b)
		if err != nil {
			logrus.Debugf("error unmarshal data: %v", err)
			return nil
		}
		return &b
	case "apply_plugin_config":
		b := &ApplyPluginConfigTaskBody{}
		err := ffjson.Unmarshal(body, &b)
		if err != nil {
			return nil
		}
		return b
	case "service_gc":
		b := ServiceGCTaskBody{}
		err := ffjson.Unmarshal(body, &b)
		if err != nil {
			return nil
		}
		return b
	case "delete_tenant":
		b := &DeleteTenantTaskBody{}
		err := ffjson.Unmarshal(body, &b)
		if err != nil {
			return nil
		}
		return b
	case "refreshhpa":
		b := &RefreshHPATaskBody{}
		err := ffjson.Unmarshal(body, &b)
		if err != nil {
			return nil
		}
		return b
	case "apply_registry_auth_secret":
		b := ApplyRegistryAuthSecretTaskBody{}
		err := ffjson.Unmarshal(body, &b)
		if err != nil {
			logrus.Debugf("error unmarshal data: %v", err)
			return nil
		}
		return &b
	default:
		return DefaultTaskBody{}
	}
}

//CreateTaskBody 通过类型串创建实体
func CreateTaskBody(taskType string) TaskBody {
	switch taskType {
	case "start":
		return StartTaskBody{}
	case "stop":
		return StopTaskBody{}
	case "restart":
		return RestartTaskBody{}
	case "rolling_upgrade":
		return RollingUpgradeTaskBody{}
	case "rollback":
		return RollBackTaskBody{}
	case "group_start":
		return GroupStartTaskBody{}
	case "group_stop":
		return GroupStopTaskBody{}
	case "horizontal_scaling":
		return HorizontalScalingTaskBody{}
	case "vertical_scaling":
		return VerticalScalingTaskBody{}
	case "apply_plugin_config":
		return ApplyPluginConfigTaskBody{}
	case "delete_tenant":
		return DeleteTenantTaskBody{}
	case "refreshhpa":
		return RefreshHPATaskBody{}
	default:
		return DefaultTaskBody{}
	}
}

//TaskBody task body
type TaskBody interface{}

//StartTaskBody 启动操作任务主体
type StartTaskBody struct {
	TenantID      string            `json:"tenant_id"`
	ServiceID     string            `json:"service_id"`
	DeployVersion string            `json:"deploy_version"`
	EventID       string            `json:"event_id"`
	Configs       map[string]string `json:"configs"`
	// When determining the startup sequence of services, you need to know the services they depend on
	DepServiceIDInBootSeq []string `json:"dep_service_ids_in_boot_seq"`
}

//StopTaskBody 停止操作任务主体
type StopTaskBody struct {
	TenantID      string            `json:"tenant_id"`
	ServiceID     string            `json:"service_id"`
	DeployVersion string            `json:"deploy_version"`
	EventID       string            `json:"event_id"`
	Configs       map[string]string `json:"configs"`
}

//HorizontalScalingTaskBody 水平伸缩操作任务主体
type HorizontalScalingTaskBody struct {
	TenantID  string `json:"tenant_id"`
	ServiceID string `json:"service_id"`
	Replicas  int32  `json:"replicas"`
	EventID   string `json:"event_id"`
	Username  string `json:"username"`
}

//VerticalScalingTaskBody 垂直伸缩操作任务主体
type VerticalScalingTaskBody struct {
	TenantID        string `json:"tenant_id"`
	ServiceID       string `json:"service_id"`
	ContainerCPU    *int   `json:"container_cpu"`
	ContainerMemory *int   `json:"container_memory"`
	ContainerGPU    *int   `json:"container_gpu"`
	EventID         string `json:"event_id"`
}

//RestartTaskBody 重启操作任务主体
type RestartTaskBody struct {
	TenantID      string `json:"tenant_id"`
	ServiceID     string `json:"service_id"`
	DeployVersion string `json:"deploy_version"`
	EventID       string `json:"event_id"`
	//重启策略，此策略不保证生效
	//例如应用如果为有状态服务，此策略如配置为先启动后关闭，此策略不生效
	//无状态服务默认使用先启动后关闭，保证服务不受影响
	Strategy []string          `json:"strategy"`
	Configs  map[string]string `json:"configs"`
}

//StrategyIsValid 验证策略是否有效
//策略包括以下值：
// prestart 先启动后关闭
// prestop 先关闭后启动
// rollingupdate 滚动形式
// grayupdate 灰度形式
// bluegreenupdate 蓝绿形式
//
func StrategyIsValid(strategy []string, serviceDeployType string) bool {
	return false
}

//RollingUpgradeTaskBody 升级操作任务主体
type RollingUpgradeTaskBody struct {
	TenantID         string            `json:"tenant_id"`
	ServiceID        string            `json:"service_id"`
	NewDeployVersion string            `json:"deploy_version"`
	EventID          string            `json:"event_id"`
	Strategy         []string          `json:"strategy"`
	Configs          map[string]string `json:"configs"`
	DryRun           bool              `json:"dry_run"`
	AppName          string            `json:"app_name"`
	AppVersion       string            `json:"app_version"`
	EventIDs         []string          `json:"event_ids"`
	End              bool              `json:"end"`
}

//RollBackTaskBody 回滚操作任务主体
type RollBackTaskBody struct {
	TenantID  string `json:"tenant_id"`
	ServiceID string `json:"service_id"`
	//当前版本
	CurrentDeployVersion string `json:"current_deploy_version"`
	//回滚目标版本
	OldDeployVersion string `json:"old_deploy_version"`
	EventID          string `json:"event_id"`
	//重启策略，此策略不保证生效
	//例如应用如果为有状态服务，此策略如配置为先启动后关闭，此策略不生效
	//无状态服务默认使用先启动后关闭，保证服务不受影响
	//如需使用滚动升级等策略，使用多策略方式
	Strategy []string `json:"strategy"`
}

//GroupStartTaskBody 组应用启动操作任务主体
type GroupStartTaskBody struct {
	Services    []StartTaskBody `json:"services"`
	Dependences []Dependence    `json:"dependences"`
	//组启动策略
	//顺序启动，无序并发启动
	Strategy []string `json:"strategy"`
}

// ApplyRuleTaskBody contains information for ApplyRuleTask
type ApplyRuleTaskBody struct {
	ServiceID   string            `json:"service_id"`
	EventID     string            `json:"event_id"`
	ServiceKind string            `json:"service_kind"`
	Action      string            `json:"action"`
	Port        int               `json:"port"`
	IsInner     bool              `json:"is_inner"`
	Limit       map[string]string `json:"limit"`
}

// ApplyPluginConfigTaskBody apply plugin dynamic discover config
type ApplyPluginConfigTaskBody struct {
	ServiceID string `json:"service_id"`
	PluginID  string `json:"plugin_id"`
	EventID   string `json:"event_id"`
	//Action put delete
	Action string `json:"action"`
}

//Dependence 依赖关系
type Dependence struct {
	CurrentServiceID string `json:"current_service_id"`
	DependServiceID  string `json:"depend_service_id"`
}

//GroupStopTaskBody 组应用停止操作任务主体
type GroupStopTaskBody struct {
	Services    []StartTaskBody `json:"services"`
	Dependences []Dependence    `json:"dependences"`
	//组关闭策略
	//顺序关系，无序并发关闭
	Strategy []string `json:"strategy"`
}

// ServiceGCTaskBody holds the request body to execute service gc task.
type ServiceGCTaskBody struct {
	TenantID  string   `json:"tenant_id"`
	ServiceID string   `json:"service_id"`
	EventIDs  []string `json:"event_ids"`
}

// DeleteTenantTaskBody -
type DeleteTenantTaskBody struct {
	TenantID string `json:"tenant_id"`
}

// RefreshHPATaskBody -
type RefreshHPATaskBody struct {
	ServiceID string `json:"service_id"`
	RuleID    string `json:"rule_id"`
	EventID   string `json:"eventID"`
}

// ApplyRegistryAuthSecretTaskBody contains information for ApplyRegistryAuthSecretTask
type ApplyRegistryAuthSecretTaskBody struct {
	Action   string `json:"action"`
	TenantID string `json:"tenant_id"`
	SecretID string `json:"secret_id"`
	Domain   string `json:"domain"`
	Username string `json:"username"`
	Password string `json:"password"`
}

//DefaultTaskBody 默认操作任务主体
type DefaultTaskBody map[string]interface{}
