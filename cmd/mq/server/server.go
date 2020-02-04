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

	"github.com/goodrain/rainbond/cmd/mq/option"
	discover "github.com/goodrain/rainbond/discover.v2"
	"github.com/goodrain/rainbond/mq/api"

	"github.com/Sirupsen/logrus"
	etcdutil "github.com/goodrain/rainbond/util/etcd"
)

//Run start run
func Run(s *option.MQServer) error {
	errChan := make(chan error)

	//step 1:start mq api manager
	apiManager, err := api.NewManager(s.Config)
	if err != nil {
		return err
	}
	apiManager.Start(errChan)
	defer apiManager.Stop()

	etcdClientArgs := &etcdutil.ClientArgs{
		Endpoints: s.Config.EtcdEndPoints,
		CaFile:    s.Config.EtcdCaFile,
		CertFile:  s.Config.EtcdCertFile,
		KeyFile:   s.Config.EtcdKeyFile,
	}

	//step 2:regist mq endpoint
	keepalive, err := discover.CreateKeepAlive(etcdClientArgs, "rainbond_mq", s.Config.HostName, s.Config.HostIP, s.Config.APIPort)
	if err != nil {
		return err
	}
	if err := keepalive.Start(); err != nil {
		return err
	}
	defer keepalive.Stop()

	//step 3:regist prometheus export endpoint
	exportKeepalive, err := discover.CreateKeepAlive(etcdClientArgs, "mq", s.Config.HostName, s.Config.HostIP, 6301)
	if err != nil {
		return err
	}
	if err := exportKeepalive.Start(); err != nil {
		return err
	}
	defer exportKeepalive.Stop()

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
