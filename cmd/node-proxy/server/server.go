// Copyright (C) 2014-2021 Goodrain Co., Ltd.
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
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/goodrain/rainbond/cmd/node-proxy/option"
	"github.com/goodrain/rainbond/discover.v2"
	"github.com/goodrain/rainbond/node/api"
	"github.com/goodrain/rainbond/node/initiate"
	"github.com/goodrain/rainbond/node/nodem"
	"github.com/goodrain/rainbond/node/nodem/docker"
	"github.com/goodrain/rainbond/node/nodem/envoy"
	"github.com/goodrain/rainbond/util/constants"
	k8sutil "github.com/goodrain/rainbond/util/k8s"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
)

//Run start run
func Run(cfg *option.Conf) error {
	var stoped = make(chan struct{})
	stopfunc := func() error {
		close(stoped)
		return nil
	}
	startfunc := func() error {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		if err := cfg.ParseClient(ctx); err != nil {
			return fmt.Errorf("config parse error:%s", err.Error())
		}

		config, err := k8sutil.NewRestConfig(cfg.KubeConfigPath)
		if err != nil {
			return err
		}
		clientset, err := kubernetes.NewForConfig(config)
		if err != nil {
			return err
		}

		nodemanager, err := nodem.NewNodeManager(ctx, cfg)
		if err != nil {
			return fmt.Errorf("create node manager failed: %s", err)
		}
		if cfg.ImageRepositoryHost == constants.DefImageRepository {
			k8sDiscover := discover.NewK8sDiscover(ctx, clientset, cfg)
			defer k8sDiscover.Stop()
			hostManager, err := initiate.NewHostManager(cfg, k8sDiscover)
			if err != nil {
				return fmt.Errorf("create new host manager: %v", err)
			}
			hostManager.Start()
		}

		logrus.Debugf("rbd-namespace=%s; rbd-docker-secret=%s", os.Getenv("RBD_NAMESPACE"), os.Getenv("RBD_DOCKER_SECRET"))
		// sync docker inscure registries cert info into all rainbond node
		if err = docker.SyncDockerCertFromSecret(clientset, os.Getenv("RBD_NAMESPACE"), os.Getenv("RBD_DOCKER_SECRET")); err != nil { // TODO fanyangyang namespace secretname
			return fmt.Errorf("sync docker cert from secret error: %s", err.Error())
		}

		errChan := make(chan error, 3)
		if err := nodemanager.Start(errChan); err != nil {
			return fmt.Errorf("start node manager failed: %s", err)
		}
		defer nodemanager.Stop()
		logrus.Debug("create and start node manager moudle success")

		//create api manager
		apiManager := api.NewManager(*cfg, clientset)
		if err := apiManager.Start(errChan); err != nil {
			return err
		}
		if err := nodemanager.AddAPIManager(apiManager); err != nil {
			return err
		}
		defer apiManager.Stop()

		//create service mesh controller
		grpcserver, err := envoy.CreateDiscoverServerManager(clientset, *cfg)
		if err != nil {
			return err
		}
		if err := grpcserver.Start(errChan); err != nil {
			return err
		}
		defer grpcserver.Stop()

		logrus.Debug("create and start api server moudle success")
		//step finally: listen Signal
		term := make(chan os.Signal, 1)
		signal.Notify(term, os.Interrupt, syscall.SIGTERM)
		select {
		case <-stoped:
			logrus.Infof("windows service stoped..")
		case <-term:
			logrus.Warn("Received SIGTERM, exiting gracefully...")
		case err := <-errChan:
			logrus.Errorf("Received a error %s, exiting gracefully...", err.Error())
		}
		logrus.Info("See you next time!")
		return nil
	}
	err := initService(cfg, startfunc, stopfunc)
	if err != nil {
		return err
	}
	return nil
}
