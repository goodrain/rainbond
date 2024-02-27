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
	"github.com/goodrain/rainbond/cmd/api/option"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/pkg/component/grpc"
	"github.com/goodrain/rainbond/pkg/component/hubregistry"
	"github.com/goodrain/rainbond/pkg/component/k8s"
	"github.com/goodrain/rainbond/pkg/component/mq"
	"github.com/goodrain/rainbond/pkg/component/prom"
	"github.com/sirupsen/logrus"
)

// InitHandle 初始化handle
func InitHandle(conf option.Config) error {

	// 注意：以下 client 将不要再次通过参数形式传递 ！！！直接在你想调用的地方调用即可
	// 注意：以下 client 将不要再次通过参数形式传递 ！！！直接在你想调用的地方调用即可
	// 注意：以下 client 将不要再次通过参数形式传递 ！！！直接在你想调用的地方调用即可

	statusCli := grpc.Default().StatusClient
	clientset := k8s.Default().Clientset
	rainbondClient := k8s.Default().RainbondClient
	k8sClient := k8s.Default().K8sClient
	restconfig := k8s.Default().RestConfig
	dynamicClient := k8s.Default().DynamicClient
	gatewayClient := k8s.Default().GatewayClient
	kubevirtCli := k8s.Default().KubevirtCli
	mapper := k8s.Default().Mapper
	registryCli := hubregistry.Default().RegistryCli
	mqClient := mq.Default().MqClient
	prometheusCli := prom.Default().PrometheusCli

	dbmanager := db.GetManager()
	defaultServieHandler = CreateManager(conf, mqClient, statusCli, prometheusCli, rainbondClient, clientset, kubevirtCli, dbmanager, registryCli)
	defaultPluginHandler = CreatePluginManager(mqClient)
	defaultAppHandler = CreateAppManager(mqClient)
	defaultTenantHandler = CreateTenManager(mqClient, statusCli, &conf, clientset, prometheusCli, k8sClient)
	defaultHelmHandler = CreateHelmManager(clientset, rainbondClient, restconfig, mapper)
	defaultCloudHandler = CreateCloudManager(conf)
	defaultAPPBackupHandler = group.CreateBackupHandle(mqClient, statusCli)
	defaultEventHandler = CreateLogManager(conf)
	shareHandler = &share.ServiceShareHandle{MQClient: mqClient}
	pluginShareHandler = &share.PluginShareHandle{MQClient: mqClient}
	if err := CreateTokenIdenHandler(conf); err != nil {
		logrus.Errorf("create token identification mannager error, %v", err)
		return err
	}

	defaultGatewayHandler = CreateGatewayManager(dbmanager, mqClient, gatewayClient, clientset, clientset, nil, nil)
	def3rdPartySvcHandler = Create3rdPartySvcHandler(dbmanager, statusCli)
	operationHandler = CreateOperationHandler(mqClient)
	batchOperationHandler = CreateBatchOperationHandler(mqClient, statusCli, operationHandler)
	defaultAppRestoreHandler = NewAppRestoreHandler()
	defPodHandler = NewPodHandler(statusCli)
	defClusterHandler = NewClusterHandler(clientset, conf.RbdNamespace, conf.GrctlImage, restconfig, mapper, prometheusCli, rainbondClient, statusCli, dynamicClient, gatewayClient, mqClient)
	defaultVolumeTypeHandler = CreateVolumeTypeManger(statusCli)
	defaultCleanDateBaseHandler = NewCleanDateBaseHandler()
	defaultmonitorHandler = NewMonitorHandler(prometheusCli)
	defServiceEventHandler = NewServiceEventHandler()
	defApplicationHandler = NewApplicationHandler(statusCli, prometheusCli, rainbondClient, clientset, dynamicClient)
	defRegistryAuthSecretHandler = CreateRegistryAuthSecretManager(dbmanager, mqClient)
	defNodesHandler = NewNodesHandler(clientset, conf.RbdNamespace, restconfig, mapper, prometheusCli)
	return nil
}

var defaultServieHandler ServiceHandler
var shareHandler *share.ServiceShareHandle
var pluginShareHandler *share.PluginShareHandle
var defaultmonitorHandler MonitorHandler

// GetMonitorHandle get monitor handler
func GetMonitorHandle() MonitorHandler {
	return defaultmonitorHandler
}

// GetShareHandle get share handle
func GetShareHandle() *share.ServiceShareHandle {
	return shareHandler
}

// GetPluginShareHandle get plugin share handle
func GetPluginShareHandle() *share.PluginShareHandle {
	return pluginShareHandler
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
