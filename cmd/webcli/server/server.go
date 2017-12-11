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
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	client "github.com/coreos/etcd/clientv3"
	"github.com/goodrain/rainbond/cmd/webcli/option"
	"github.com/goodrain/rainbond/pkg/webcli/app"

	"github.com/Sirupsen/logrus"
)

//Run start run
func Run(s *option.WebCliServer) error {
	errChan := make(chan error)
	option := app.DefaultOptions
	option.Address = s.Address
	option.Port = s.Port
	option.SessionKey = s.SessionKey
	ap, err := app.New(nil, &option)
	if err != nil {
		return err
	}
	err = ap.Run()
	if err != nil {
		return err
	}
	defer ap.Exit()
	go keepAlive(s.EtcdEndPoints, s.HostIP, s.Port)
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

func keepAlive(etcdEndpoint []string, address, port string) {
	duration := time.Duration(5) * time.Second
	ttl := int64(8)
	timer := time.NewTimer(duration)
	cli, err := client.New(client.Config{
		Endpoints: etcdEndpoint,
	})
	if err != nil {
		logrus.Error("create etcd client error,", err.Error())
	}
	var lid client.LeaseID
	for {
		select {
		case <-timer.C:
			if lid > 0 {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
				_, err := cli.KeepAliveOnce(ctx, lid)
				cancel()
				if err == nil {
					timer.Reset(duration)
					continue
				}
				logrus.Warnf("lid[%x] keepAlive err: %s, try to reset...", lid, err.Error())
				lid = 0
			} else {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
				resp, err := cli.Grant(ctx, ttl)
				if err != nil {
					logrus.Error("Grand from etcd error.", err.Error())
					timer.Reset(duration)
					cancel()
					continue
				}
				hostName, _ := os.Hostname()
				if _, err = cli.Put(ctx,
					fmt.Sprintf("/traefik/backends/acp_webcli/servers/%s/url", hostName),
					fmt.Sprintf("%s:%s", address, port), client.WithLease(resp.ID)); err != nil {
					logrus.Error("put web_cli endpoint to etcd error.", err.Error())
					timer.Reset(duration)
					cancel()
					continue
				}
				cancel()
				lid = resp.ID
			}
		}
	}
}
