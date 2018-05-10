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
	"os"
	"os/signal"
	"syscall"

	"github.com/goodrain/rainbond/cmd/entrance/option"
	"github.com/goodrain/rainbond/discover"
	"github.com/goodrain/rainbond/entrance/api"
	"github.com/goodrain/rainbond/entrance/cluster"
	"github.com/goodrain/rainbond/entrance/core"
	"github.com/goodrain/rainbond/entrance/core/sync"
	"github.com/goodrain/rainbond/entrance/source"
	"github.com/goodrain/rainbond/entrance/store"

	"github.com/Sirupsen/logrus"

	"github.com/goodrain/rainbond/entrance/plugin"
)

//Run start run
func Run(s *option.ACPLBServer) error {
	errChan := make(chan error)
	//step 1: new cluster manager
	cluster, err := cluster.NewManager(s.Config)
	if err != nil {
		return err
	}
	//step 2: new store manager
	storeManager, err := store.NewManager(s.Config, cluster)
	if err != nil {
		return err
	}
	//step 3: new plugin manager
	pluginManager, err := plugin.NewPluginManager(s.Config)
	if err != nil {
		return err
	}
	defer pluginManager.Stop()

	//step 4: new core manager and start
	coreManager := core.NewManager(s, pluginManager, storeManager, cluster)
	if err := coreManager.Start(); err != nil {
		return err
	}
	defer coreManager.Stop()

	//step 5:new api manager and start
	apiManager := api.NewManager(s.Config, coreManager, storeManager)
	apiManager.Start(errChan)
	defer apiManager.Stop()

	//step 6:registor acp_entrance host_ip into etcd
	keepalive, err := discover.CreateKeepAlive(s.Config.EtcdEndPoints, "acp_entrance",
		s.Config.HostName, s.Config.HostIP, s.Config.BindPort)
	if err != nil {
		return err
	}
	if err := keepalive.Start(); err != nil {
		return err
	}
	defer keepalive.Stop()

	//step 7:new source manager and start
	sourceManager := source.NewSourceManager(s.Config, coreManager, errChan)

	if s.RunMode == "sync" {
		sync := sync.NewManager(sourceManager, storeManager, coreManager)
		if err := sync.Start(); err != nil {
			return err
		}
	}

	if err := sourceManager.Start(); err != nil {
		return err
	}
	defer sourceManager.Stop()

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
