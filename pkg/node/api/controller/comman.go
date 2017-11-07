
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

package controller

import (
	"github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/pkg/node/core/config"
	"github.com/goodrain/rainbond/pkg/node/core/service"
)

var datacenterConfig *config.DataCenterConfig
var taskService *service.TaskService
var taskTempService *service.TaskTempService
var taskGroupService *service.TaskGroupService
var appService *service.AppService

//Init 初始化
func Init(c *option.Conf) {
	datacenterConfig = config.CreateDataCenterConfig(c)
	taskService = service.CreateTaskService(c)
	taskTempService = service.CreateTaskTempService(c)
	taskGroupService = service.CreateTaskGroupService(c)
	appService = service.CreateAppService(c)
}
