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

	"github.com/Sirupsen/logrus"
	"github.com/eapache/channels"
	kubeaggregatorclientset "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/goodrain/rainbond/cmd/worker/option"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/config"
	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/worker/appm"
	"github.com/goodrain/rainbond/worker/appm/controller"
	"github.com/goodrain/rainbond/worker/appm/store"
	"github.com/goodrain/rainbond/worker/discover"
	"github.com/goodrain/rainbond/worker/gc"
	"github.com/goodrain/rainbond/worker/master"
	"github.com/goodrain/rainbond/worker/monitor"
	"github.com/goodrain/rainbond/worker/server"
)

//Run start run
func Run(s *option.Worker) error {
	errChan := make(chan error, 2)
	dbconfig := config.Config{
		DBType:              s.Config.DBType,
		MysqlConnectionInfo: s.Config.MysqlConnectionInfo,
		EtcdEndPoints:       s.Config.EtcdEndPoints,
		EtcdTimeout:         s.Config.EtcdTimeout,
	}
	//step 1:db manager init ,event log client init
	if err := db.CreateManager(dbconfig); err != nil {
		return err
	}
	defer db.CloseManager()

	if err := event.NewManager(event.EventConfig{
		EventLogServers: s.Config.EventLogServers,
		DiscoverAddress: s.Config.EtcdEndPoints,
	}); err != nil {
		return err
	}
	defer event.CloseManager()

	//step 2 : create kube client and etcd client
	c, err := clientcmd.BuildConfigFromFlags("", s.Config.KubeConfig)
	if err != nil {
		logrus.Error("read kube config file error.", err)
		return err
	}
	clientset, err := kubernetes.NewForConfig(c)
	if err != nil {
		logrus.Error("create kube api client error", err)
		return err
	}
	s.Config.KubeClient = clientset

	kubeaggregatorclientset, err := kubeaggregatorclientset.NewForConfig(c)
	if err != nil {
		logrus.Error("kube aggregator; read kube config file error.", err)
		return err
	}

	//step 3: create resource store
	startCh := channels.NewRingChannel(1024)
	updateCh := channels.NewRingChannel(1024)
	probeCh := channels.NewRingChannel(1024)
	cachestore := store.NewStore(clientset, db.GetManager(), s.Config, startCh, probeCh)
	appmController := appm.NewAPPMController(clientset, cachestore, startCh, updateCh, probeCh)
	if err := appmController.Start(); err != nil {
		logrus.Errorf("error starting appm controller: %v", err)
	}
	defer appmController.Stop()
	if err := cachestore.Start(); err != nil {
		logrus.Error("start kube cache store error", err)
		return err
	}

	//step 4: create controller manager
	controllerManager := controller.NewManager(cachestore, clientset)
	defer controllerManager.Stop()

	//step 5 : start runtime master
	masterCon, err := master.NewMasterController(s.Config, cachestore, kubeaggregatorclientset)
	if err != nil {
		return err
	}
	if err := masterCon.Start(); err != nil {
		return err
	}
	defer masterCon.Stop()

	//step 6 : create discover module
	garbageCollector := gc.NewGarbageCollector(clientset)
	taskManager := discover.NewTaskManager(s.Config, cachestore, controllerManager, garbageCollector, startCh)
	if err := taskManager.Start(); err != nil {
		return err
	}
	defer taskManager.Stop()
	//step 7: start app runtimer server
	runtimeServer := server.CreaterRuntimeServer(s.Config, cachestore, clientset, updateCh)
	runtimeServer.Start(errChan)
	//step 8: create application use resource exporter.
	exporterManager := monitor.NewManager(s.Config, masterCon)
	if err := exporterManager.Start(); err != nil {
		return err
	}
	defer exporterManager.Stop()

	logrus.Info("worker begin running...")

	//step finally: listen Signal
	term := make(chan os.Signal)
	signal.Notify(term, os.Interrupt, syscall.SIGTERM)
	select {
	case <-term:
		logrus.Warn("Received SIGTERM, exiting gracefully...")
	case err := <-errChan:
		if err != nil {
			logrus.Errorf("Received a error %s, exiting gracefully...", err.Error())
		}
	}
	logrus.Info("See you next time!")
	return nil
}
