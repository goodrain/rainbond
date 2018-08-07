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

package server

import (
	"fmt"
	"os"
	"syscall"

	"github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/node/api"
	"github.com/goodrain/rainbond/node/api/controller"
	"github.com/goodrain/rainbond/node/core/store"
	"github.com/goodrain/rainbond/node/kubecache"
	"github.com/goodrain/rainbond/node/masterserver"
	"github.com/goodrain/rainbond/node/nodem"

	"github.com/Sirupsen/logrus"

	eventLog "github.com/goodrain/rainbond/event"

	"os/signal"
)

//Run start run
func Run(c *option.Conf) error {
	errChan := make(chan error, 3)
	err := eventLog.NewManager(eventLog.EventConfig{
		EventLogServers: c.EventLogServer,
		DiscoverAddress: c.Etcd.Endpoints,
	})
	if err != nil {
		logrus.Errorf("error creating eventlog manager")
		return nil
	}
	defer eventLog.CloseManager()

	kubecli, err := kubecache.NewKubeClient(c)
	if err != nil {
		return err
	}
	defer kubecli.Stop()
	// init etcd client
	if err = store.NewClient(c); err != nil {
		return fmt.Errorf("Connect to ETCD %s failed: %s", c.Etcd.Endpoints, err)
	}
	nodemanager, err := nodem.NewNodeManager(c)
	if err != nil {
		return fmt.Errorf("create node manager failed: %s", err)
	}
	if err := nodemanager.Start(errChan); err != nil {
		return fmt.Errorf("start node manager failed: %s", err)
	}
	defer nodemanager.Stop()
	//master服务在node服务之后启动
	var ms *masterserver.MasterServer
	if c.RunMode == "master" {
		ms, err = masterserver.NewMasterServer(nodemanager.GetCurrentNode(), kubecli)
		if err != nil {
			logrus.Errorf(err.Error())
			return err
		}
		ms.Cluster.UpdateNode(nodemanager.GetCurrentNode())
		if err := ms.Start(errChan); err != nil {
			logrus.Errorf(err.Error())
			return err
		}
		defer ms.Stop(nil)
	}
	//create api manager
	apiManager := api.NewManager(*c, nodemanager.GetCurrentNode(), ms, kubecli)
	if err := apiManager.Start(errChan); err != nil {
		return err
	}
	if err := nodemanager.AddAPIManager(apiManager); err != nil {
		return err
	}
	defer apiManager.Stop()

	defer controller.Exist(nil)
	//step finally: listen Signal
	term := make(chan os.Signal)
	signal.Notify(term, os.Interrupt, syscall.SIGTERM)
	select {
	case <-term:
		logrus.Warn("Received SIGTERM, exiting gracefully...")
	case err := <-errChan:
		logrus.Errorf("Received a error %s, exiting gracefully...", err.Error())
	}
	logrus.Info("See you next time!")
	return nil
}
