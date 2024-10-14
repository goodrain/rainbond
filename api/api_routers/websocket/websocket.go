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

package websocket

import (
	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/api/controller"
	"github.com/goodrain/rainbond/pkg/component/eventlog"
)

// Routes routes
func Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/docker_console", controller.GetWebCli().HandleWS)
	r.Get("/docker_log", eventlog.Default().SocketServer.PushDockerLog)
	r.Get("/monitor_message", controller.GetMonitorMessage().Get)
	r.Get("/new_monitor_message", controller.GetMonitorMessage().Get)
	r.Get("/event_log", eventlog.Default().SocketServer.PushEventMessage)
	r.Get("/services/{serviceID}/pubsub", eventlog.Default().SocketServer.Pubsub)
	return r
}

// LogRoutes 日志下载路由
func LogRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/{gid}/{filename}", controller.GetLogFile().Get)
	r.Get("/install_log/{filename}", controller.GetLogFile().GetInstallLog)
	return r
}

// AppRoutes 应用导出包下载路由
func AppRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/download/{format}/{fileName}", controller.GetManager().Download)
	r.Post("/upload/{eventID}", controller.GetManager().Upload)
	r.Options("/upload/{eventID}", controller.GetManager().Upload)
	return r
}

// PackageBuildRoutes 本地文件上传路由
func PackageBuildRoutes() chi.Router {
	r := chi.NewRouter()
	r.Post("/component/events/{eventID}", controller.GetManager().UploadPackage)
	r.Options("/component/events/{eventID}", controller.GetManager().UploadPackage)
	return r
}

// FileOperateRoutes 共享存储的文件操作路由
func FileOperateRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/download/{fileName}", controller.GetFileManage().Get)
	r.Options("/download/{fileName}", controller.GetFileManage().Get)
	r.Post("/upload", controller.GetFileManage().Get)
	r.Options("/upload", controller.GetFileManage().Get)
	return r
}

// LongVersionRoutes 语言包处理
func LongVersionRoutes() chi.Router {
	r := chi.NewRouter()
	r.Options("/upload", controller.GetManager().OptionLongVersion)
	r.Post("/upload", controller.GetManager().UploadLongVersion)
	r.Get("/download/{language}/{version}", controller.GetManager().DownloadLongVersion)
	r.Head("/download/{language}/{version}", controller.GetManager().DownloadLongVersion)
	return r
}
