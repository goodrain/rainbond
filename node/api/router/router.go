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

package router

import (
	"time"

	"github.com/goodrain/rainbond/node/api/controller"

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
			r.Route("/cluster", func(r chi.Router) {
				r.Get("/", controller.ClusterInfo)
				r.Get("/service-health", controller.GetServicesHealthy)
			})
			r.Route("/nodes", func(r chi.Router) {
				r.Get("/fullres", controller.ClusterInfo)
				r.Get("/{node_id}/node_resource", controller.GetNodeResource)
				r.Get("/resources", controller.Resources)
				r.Get("/capres", controller.CapRes)
				r.Get("/", controller.GetNodes)
				r.Get("/rule/{rule}", controller.GetRuleNodes)
				r.Post("/", controller.NewNode)                       //增加一个节点
				r.Post("/multiple", controller.NewMultipleNode)       //增加多个节点
				r.Delete("/{node_id}", controller.DeleteRainbondNode) //删除一个节点
				r.Get("/{node_id}", controller.GetNode)
				r.Put("/{node_id}/unschedulable", controller.Cordon)
				r.Put("/{node_id}/reschedulable", controller.UnCordon)
				r.Put("/{node_id}/labels", controller.PutLabel)
				r.Post("/{node_id}/down", controller.DownNode)     //节点下线
				r.Post("/{node_id}/up", controller.UpNode)         //节点上线
				r.Get("/{node_id}/instance", controller.Instances) //节点实例列表
				r.Get("/{node_id}/check", controller.CheckNode)
				r.Get("/{node_id}/resource", controller.Resource)

				r.Get("/{node_ip}/init", controller.InitStatus)
				r.Post("/{node_id}/install", controller.Install)

				r.Get("/{node_id}/prometheus/cpu", controller.GetCpu)
				r.Get("/{node_id}/prometheus/mem", controller.GetMem)
				r.Get("/{node_id}/prometheus/disk", controller.GetDisk)
				r.Get("/{node_id}/prometheus/load1", controller.GetLoad1)
				r.Get("/{node_id}/prometheus/start/{start}/end/{end}/step/{step}/cpu", controller.GetCpuRange)
				r.Get("/{node_id}/prometheus/start/{start}/end/{end}/step/{step}/mem", controller.GetMemRange)
				r.Get("/{node_id}/prometheus/start/{start}/end/{end}/step/{step}/disk", controller.GetDiskRange)
				r.Get("/{node_id}/prometheus/start/{start}/end/{end}/step/{step}/load1", controller.GetLoad1Range)
				r.Put("/prometheus/expr", controller.GetExpr)
				r.Put("/prometheus/start/{start}/end/{end}/step/{step}/expr", controller.GetExpr)

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
			r.Put("/tasks/taskreload", controller.ReloadStaticTasks)
		}
	})
	return r
}
