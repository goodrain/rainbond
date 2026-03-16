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

package handler

import (
	"github.com/goodrain/rainbond/api/handler/group"
	"github.com/goodrain/rainbond/api/handler/share"
	"github.com/goodrain/rainbond/pkg/component/mq"
	"github.com/sirupsen/logrus"
)

// InitAPIHandle 初始化handle
func InitAPIHandle() error {
	defaultServieHandler = CreateManager()
	defaultPluginHandler = CreatePluginManager()
	defaultAppHandler = CreateAppManager()
	defaultTenantHandler = CreateTenManager()
	defaultHelmHandler = CreateHelmManager()
	defaultCloudHandler = CreateCloudManager()
	defaultAPPBackupHandler = group.CreateBackupHandle()
	defaultEventHandler = CreateLogManager()
	if err := CreateTokenIdenHandler(); err != nil {
		logrus.Errorf("create token identification mannager error, %v", err)
		return err
	}
	defaultGatewayHandler = CreateGatewayManager()
	def3rdPartySvcHandler = Create3rdPartySvcHandler()
	operationHandler = CreateOperationHandler()
	batchOperationHandler = CreateBatchOperationHandler(operationHandler)
	defaultAppRestoreHandler = NewAppRestoreHandler()
	defPodHandler = NewPodHandler()
	defClusterHandler = NewClusterHandler()
	defaultVolumeTypeHandler = CreateVolumeTypeManger()
	defaultCleanDateBaseHandler = NewCleanDateBaseHandler()
	defaultmonitorHandler = NewMonitorHandler()
	defServiceEventHandler = NewServiceEventHandler()
	defApplicationHandler = NewApplicationHandler()
	defRegistryAuthSecretHandler = CreateRegistryAuthSecretManager()
	defNodesHandler = NewNodesHandler()

	CreateLicenseV2Handler()

	// 初始化 TarImageHandle
	// 镜像的加载和推送在 builder 服务中异步完成
	CreateTarImageHandle(mq.Default().MqClient)
	logrus.Info("tar image handler initialized successfully")

	return nil
}

var defaultServieHandler ServiceHandler
var defaultmonitorHandler MonitorHandler

// GetMonitorHandle get monitor handler
func GetMonitorHandle() MonitorHandler {
	return defaultmonitorHandler
}

// GetShareHandle get share handle
func GetShareHandle() *share.ServiceShareHandle {
	return &share.ServiceShareHandle{MQClient: mq.Default().MqClient}
}

// GetPluginShareHandle get plugin share handle
func GetPluginShareHandle() *share.PluginShareHandle {
	return &share.PluginShareHandle{MQClient: mq.Default().MqClient}
}

// GetServiceManager get manager
func GetServiceManager() ServiceHandler {
	return defaultServieHandler
}

var defaultPluginHandler PluginHandler

// GetPluginManager get manager
func GetPluginManager() PluginHandler {
	return defaultPluginHandler
}

var defaultTenantHandler TenantHandler

// GetTenantManager get manager
func GetTenantManager() TenantHandler {
	return defaultTenantHandler
}

var defaultHelmHandler HelmHandler

// GetHelmManager get manager
func GetHelmManager() HelmHandler {
	return defaultHelmHandler
}

var defaultCloudHandler CloudHandler

// GetCloudManager get manager
func GetCloudManager() CloudHandler {
	return defaultCloudHandler
}

var defaultEventHandler EventHandler

// GetEventHandler get event handler
func GetEventHandler() EventHandler {
	return defaultEventHandler
}

var defaultAppHandler *AppAction

// GetAppHandler GetAppHandler
func GetAppHandler() *AppAction {
	return defaultAppHandler
}

var defaultAPPBackupHandler *group.BackupHandle

// GetAPPBackupHandler GetAPPBackupHandler
func GetAPPBackupHandler() *group.BackupHandle {
	return defaultAPPBackupHandler
}

var defaultAPIGatewayHandler APIGatewayHandler

// GetAPIGatewayHandler -
func GetAPIGatewayHandler() APIGatewayHandler {
	return defaultAPIGatewayHandler
}

var defaultGatewayHandler GatewayHandler

// GetGatewayHandler returns a default GatewayHandler
func GetGatewayHandler() GatewayHandler {
	return defaultGatewayHandler
}

var def3rdPartySvcHandler *ThirdPartyServiceHanlder

// Get3rdPartySvcHandler returns the defalut ThirdParthServiceHanlder
func Get3rdPartySvcHandler() *ThirdPartyServiceHanlder {
	return def3rdPartySvcHandler
}

var batchOperationHandler *BatchOperationHandler

// GetBatchOperationHandler get handler
func GetBatchOperationHandler() *BatchOperationHandler {
	return batchOperationHandler
}

var operationHandler *OperationHandler

// GetOperationHandler get handler
func GetOperationHandler() *OperationHandler {
	return operationHandler
}

var defaultAppRestoreHandler AppRestoreHandler

// GetAppRestoreHandler returns a default AppRestoreHandler
func GetAppRestoreHandler() AppRestoreHandler {
	return defaultAppRestoreHandler
}

var defPodHandler PodHandler

// GetPodHandler returns the defalut PodHandler
func GetPodHandler() PodHandler {
	return defPodHandler
}

var defaultCleanDateBaseHandler *CleanDateBaseHandler

// GetCleanDateBaseHandler returns the default db clean handler.
func GetCleanDateBaseHandler() *CleanDateBaseHandler {
	return defaultCleanDateBaseHandler
}

var defClusterHandler ClusterHandler

// GetClusterHandler returns the default cluster handler.
func GetClusterHandler() ClusterHandler {
	return defClusterHandler
}

var defNodesHandler NodesHandler

// GetNodesHandler returns the default cluster handler.
func GetNodesHandler() NodesHandler {
	return defNodesHandler
}

var defApplicationHandler ApplicationHandler

// GetApplicationHandler  returns the default tenant application handler.
func GetApplicationHandler() ApplicationHandler {
	return defApplicationHandler
}

var defServiceEventHandler *ServiceEventHandler

// GetServiceEventHandler -
func GetServiceEventHandler() *ServiceEventHandler {
	return defServiceEventHandler
}

var defRegistryAuthSecretHandler RegistryAuthSecretHandler

// GetRegistryAuthSecretHandler -
func GetRegistryAuthSecretHandler() RegistryAuthSecretHandler {
	return defRegistryAuthSecretHandler
}

