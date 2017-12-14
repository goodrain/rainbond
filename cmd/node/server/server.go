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

package server

import (
	"fmt"

	"github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/pkg/node/api/controller"
	"github.com/goodrain/rainbond/pkg/node/core/job"
	"github.com/goodrain/rainbond/pkg/node/core/k8s"
	"github.com/goodrain/rainbond/pkg/node/core/store"
	"github.com/goodrain/rainbond/pkg/node/masterserver"
	"github.com/goodrain/rainbond/pkg/node/nodeserver"
	"github.com/goodrain/rainbond/pkg/node/statsd"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/Sirupsen/logrus"

	eventLog "github.com/goodrain/rainbond/pkg/event"

	"github.com/goodrain/rainbond/pkg/node/api"
	"github.com/goodrain/rainbond/pkg/node/event"
)

//Run start run
func Run(c *option.Conf) error {
	errChan := make(chan error, 1)
	err := eventLog.NewManager(eventLog.EventConfig{
		EventLogServers: c.EventLogServer,
		DiscoverAddress: c.Etcd.Endpoints,
	})
	if err != nil {
		logrus.Errorf("error creating eventlog manager")
	}
	defer eventLog.CloseManager()

	// init etcd client
	if err = store.NewClient(c); err != nil {
		return fmt.Errorf("Connect to ETCD %s failed: %s",
			c.Etcd.Endpoints, err)
	}
	if c.K8SConfPath != "" {
		if err := k8s.NewK8sClient(c); err != nil {
			return fmt.Errorf("Connect to K8S %s failed: %s",
				c.K8SConfPath, err)
		}
	} else {
		return fmt.Errorf("Connect to K8S %s failed: kubeconfig file not found",
			c.K8SConfPath)
	}

	s, err := nodeserver.NewNodeServer(c) //todo 配置文件 done
	if err != nil {
		return err
	}
	if err := s.Run(); err != nil {
		logrus.Errorf(err.Error())
		return err
	}
	//master服务在node服务之后启动
	var ms *masterserver.MasterServer
	if c.RunMode == "master" {
		ms, err = masterserver.NewMasterServer(s.HostNode, k8s.K8S.Clientset)
		if err != nil {
			logrus.Errorf(err.Error())
			return err
		}
		if err := ms.Start(); err != nil {
			logrus.Errorf(err.Error())
			return err
		}
		event.On(event.EXIT, ms.Stop)
	}
	//statsd exporter
	registry := prometheus.NewRegistry()
	exporter := statsd.CreateExporter(c.StatsdConfig, registry)
	if err := exporter.Start(); err != nil {
		logrus.Errorf("start statsd exporter server error,%s", err.Error())
		return err
	}

	//启动API服务
	apiManager := api.NewManager(*s.Conf, s.HostNode, ms, exporter)
	apiManager.Start(errChan)
	defer apiManager.Stop()

	// 注册退出事件
	//todo conf.Exit cronsun.exit 重写
	event.On(event.EXIT, s.Stop, option.Exit, job.Exit, controller.Exist)
	// 监听退出信号
	event.Wait()
	// 处理退出事件
	event.Emit(event.EXIT, nil)
	logrus.Infof("exit success")
	logrus.Info("See you next time!")
	return nil
}
