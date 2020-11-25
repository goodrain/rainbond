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
	"strconv"
	"syscall"

	"github.com/goodrain/rainbond/cmd/webcli/option"
	"github.com/goodrain/rainbond/discover"
	"github.com/goodrain/rainbond/webcli/app"

	etcdutil "github.com/goodrain/rainbond/util/etcd"
	"github.com/sirupsen/logrus"
)

//Run start run
func Run(s *option.WebCliServer) error {
	errChan := make(chan error)
	option := app.DefaultOptions
	option.Address = s.Address
	option.Port = strconv.Itoa(s.Port)
	option.SessionKey = s.SessionKey
	option.K8SConfPath = s.K8SConfPath
	ap, err := app.New(&option)
	if err != nil {
		return err
	}
	err = ap.Run()
	if err != nil {
		return err
	}
	defer ap.Exit()
	etcdClientArgs := &etcdutil.ClientArgs{
		Endpoints: s.EtcdEndPoints,
		CaFile:    s.EtcdCaFile,
		CertFile:  s.EtcdCertFile,
		KeyFile:   s.EtcdKeyFile,
	}
	keepalive, err := discover.CreateKeepAlive(etcdClientArgs, "acp_webcli", s.HostName, s.HostIP, s.Port)
	if err != nil {
		return err
	}
	if err := keepalive.Start(); err != nil {
		return err
	}
	defer keepalive.Stop()
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
