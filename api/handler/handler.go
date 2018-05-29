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
	"time"

	"github.com/goodrain/rainbond/api/handler/group"

	"github.com/goodrain/rainbond/api/handler/share"

	"github.com/Sirupsen/logrus"
	"github.com/coreos/etcd/clientv3"
	api_db "github.com/goodrain/rainbond/api/db"
	"github.com/goodrain/rainbond/appruntimesync/client"
	"github.com/goodrain/rainbond/cmd/api/option"
)

//InitHandle 初始化handle
func InitHandle(conf option.Config, statusCli *client.AppRuntimeSyncClient) error {
	mq := api_db.MQManager{
		EtcdEndpoint:  conf.EtcdEndpoint,
		DefaultServer: conf.MQAPI,
	}
	mqClient, errMQ := mq.NewMQManager()
	if errMQ != nil {
		logrus.Errorf("new MQ manager failed, %v", errMQ)
		return errMQ
	}
	k8s := api_db.K8SManager{
		K8SConfig: conf.KubeConfig,
	}
	kubeClient, errK := k8s.NewKubeConnection()
	if errK != nil {
		logrus.Errorf("create kubeclient failed, %v", errK)
		return errK
	}
	etcdCli, err := clientv3.New(clientv3.Config{
		Endpoints:   conf.EtcdEndpoint,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		logrus.Errorf("create etcd client v3 error, %v", err)
		return err
	}
	defaultServieHandler = CreateManager(mqClient, kubeClient, etcdCli, statusCli)
	defaultPluginHandler = CreatePluginManager(mqClient)
	defaultAppHandler = CreateAppManager(mqClient)
	defaultTenantHandler = CreateTenManager(mqClient, kubeClient, statusCli)
	defaultNetRulesHandler = CreateNetRulesManager(etcdCli)
	defaultSourcesHandler = CreateSourcesManager(etcdCli)
	defaultCloudHandler = CreateCloudManager(conf)
	defaultAPPBackupHandler = group.CreateBackupHandle(mqClient, statusCli, etcdCli)
	//需要使用etcd v2 API
	defaultEventHandler = CreateLogManager(conf.EtcdEndpoint)
	shareHandler = &share.ServiceShareHandle{MQClient: mqClient, EtcdCli: etcdCli}
	pluginShareHandler = &share.PluginShareHandle{MQClient: mqClient, EtcdCli: etcdCli}
	if err := CreateTokenIdenHandler(conf); err != nil {
		logrus.Errorf("create token identification mannager error, %v", err)
		return err
	}
	return nil
}

var defaultServieHandler ServiceHandler
var shareHandler *share.ServiceShareHandle
var pluginShareHandler *share.PluginShareHandle

//GetShareHandle get share handle
func GetShareHandle() *share.ServiceShareHandle {
	return shareHandler
}

//GetPluginShareHandle get plugin share handle
func GetPluginShareHandle() *share.PluginShareHandle {
	return pluginShareHandler
}

//GetServiceManager get manager
func GetServiceManager() ServiceHandler {
	return defaultServieHandler
}

var defaultPluginHandler PluginHandler

//GetPluginManager get manager
func GetPluginManager() PluginHandler {
	return defaultPluginHandler
}

var defaultTenantHandler TenantHandler

//GetTenantManager get manager
func GetTenantManager() TenantHandler {
	return defaultTenantHandler
}

var defaultNetRulesHandler NetRulesHandler

//GetRulesManager get manager
func GetRulesManager() NetRulesHandler {
	return defaultNetRulesHandler
}

var defaultSourcesHandler SourcesHandler

//GetSourcesManager get manager
func GetSourcesManager() SourcesHandler {
	return defaultSourcesHandler
}

var defaultCloudHandler CloudHandler

//GetCloudManager get manager
func GetCloudManager() CloudHandler {
	return defaultCloudHandler
}

var defaultEventHandler EventHandler

//GetEventHandler get event handler
func GetEventHandler() EventHandler {
	return defaultEventHandler
}

var defaultAppHandler *AppAction

//GetAppHandler GetAppHandler
func GetAppHandler() *AppAction {
	return defaultAppHandler
}

var defaultAPPBackupHandler *group.BackupHandle

//GetAPPBackupHandler GetAPPBackupHandler
func GetAPPBackupHandler() *group.BackupHandle {
	return defaultAPPBackupHandler
}
