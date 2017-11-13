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

package router

import (
	"time"

	"github.com/goodrain/rainbond/pkg/node/api/controller"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
)

//Routers 路由
func Routers(mode string) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.RequestID) //每个请求的上下文中注册一个id
	//Sets a http.Request's RemoteAddr to either X-Forwarded-For or X-Real-IP
	r.Use(middleware.RealIP)
	//Logs the start and end of each request with the elapsed processing time
	r.Use(middleware.Logger)
	//Gracefully absorb panics and prints the stack trace
	r.Use(middleware.Recoverer)
	//request time out
	r.Use(middleware.Timeout(time.Second * 5))
	r.Mount("/v1", DisconverRoutes())
	r.Route("/v2", func(r chi.Router) {
		r.Get("/ping", controller.Ping)
		r.Route("/apps", func(r chi.Router) {
			r.Get("/{app_name}/register", controller.APPRegister)
			r.Get("/{app_name}/discover", controller.APPDiscover)
			r.Get("/", controller.APPList)
		})
		//以下只有管理节点具有的API
		if mode == "master" {
			r.Route("/configs", func(r chi.Router) {
				r.Get("/datacenter", controller.GetDatacenterConfig)
				r.Put("/datacenter", controller.PutDatacenterConfig)
			})
			r.Route("/nodes", func(r chi.Router) {
				r.Get("/resources", controller.Resources)
				r.Get("/", controller.GetNodes)
				r.Get("/{rule}", controller.GetRuleNodes)
				r.Post("/", controller.NewNode) //增加一个节点
				r.Get("/{node}/details", controller.GetNodeDetails)
				r.Get("/{node}/basic", controller.GetNodeBasic)
				r.Post("/{node}/down", controller.DeleteNode)
				r.Delete("/{node}", controller.DeleteFromDB)
				r.Put("/{node}", controller.AddNode)
				r.Post("/{node}", controller.UpdateNode)
				//r.Put("/{node}",nil)
				r.Put("/{node}/unschedulable", controller.Cordon)
				r.Put("/{node}/reschedulable", controller.UnCordon)
				r.Post("/{node}/label", controller.AddLabel)

				r.Put("/login", controller.LoginCompute)
				//此处会安装
				r.Put("/{ip}/init", controller.NodeInit)
				r.Get("/{ip}/init/status", controller.CheckInitStatus)
				r.Get("/{ip}/install/status", controller.CheckJobGetStatus)
				r.Put("/{ip}/install", controller.StartBuildInJobs)
			})

			//TODO:
			//任务执行框架相关API
			//任务
			r.Route("/tasks", func(r chi.Router) {
				r.Post("/", controller.CreateTask)
				r.Get("/", controller.GetTasks)
				r.Get("/{task_id}", controller.GetTask)
				r.Delete("/{task_id}", controller.DeleteTask)
				r.Post("/{task_id}/exec", controller.ExecTask)
				r.Get("/{task_id}/status", controller.GetTaskStatus)
			})
			//任务模版
			r.Route("/tasktemps", func(r chi.Router) {
				r.Post("/", controller.CreateTaskTemp)
				r.Put("/{temp_id}", controller.UpdateTaskTemp)
				r.Delete("/{temp_id}", controller.DeleteTaskTemp)
			})
			//任务组
			r.Route("/taskgroups", func(r chi.Router) {
				r.Post("/", controller.CreateTaskGroup)
				r.Get("/", controller.GetTaskGroups)
				r.Get("/{group_id}", controller.GetTaskGroup)
				r.Delete("/{group_id}", controller.DeleteTaskGroup)
				r.Post("/{group_id}/exec", controller.ExecTaskGroup)
				r.Get("/{group_id}/status", controller.GetTaskGroupStatus)
			})
		}
	})

	return r
}
