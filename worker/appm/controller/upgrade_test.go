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

package controller

import (
	"testing"

	"github.com/eapache/channels"
	"github.com/goodrain/rainbond/worker/appm/store"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/goodrain/rainbond/cmd/worker/option"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/config"
)

func Test_upgradeController_upgradeOne(t *testing.T) {
	storer := getStoreForTest(t)
	ocfg := option.Config{
		DBType:                  "mysql",
		MysqlConnectionInfo:     "oc6Poh:noot6Mea@tcp(192.168.2.203:3306)/region",
		EtcdEndPoints:           []string{"http://192.168.2.203:2379"},
		EtcdTimeout:             5,
		KubeConfig:              "/Users/fanyangyang/Documents/company/goodrain/admin.kubeconfig",
		LeaderElectionNamespace: "rainbond",
	}
	c, err := clientcmd.BuildConfigFromFlags("", ocfg.KubeConfig)
	if err != nil {
		t.Fatalf("read kube config file error: %v", err)
	}
	clientset, err := kubernetes.NewForConfig(c)
	if err != nil {
		t.Fatalf("create kube api client error: %v", err)
	}
	manager := NewManager(storer, clientset)
	controller := upgradeController{
		stopChan:     make(chan struct{}),
		controllerID: "",
		appService:   nil,
		manager:      manager,
	}
	appservice := storer.GetAppService("e7d895fde0ab49e0eeb20e81a8967878")
	if appservice != nil {
		controller.upgradeOne(*appservice)
	} else {
		t.Log("app is nil")
	}
	db.CloseManager()

}

func getStoreForTest(t *testing.T) store.Storer {
	ocfg := option.Config{
		DBType:                  "mysql",
		MysqlConnectionInfo:     "oc6Poh:noot6Mea@tcp(192.168.2.203:3306)/region",
		EtcdEndPoints:           []string{"http://192.168.2.203:2379"},
		EtcdTimeout:             5,
		KubeConfig:              "/Users/fanyangyang/Documents/company/goodrain/admin.kubeconfig",
		LeaderElectionNamespace: "rainbond",
	}

	dbconfig := config.Config{
		DBType:              ocfg.DBType,
		MysqlConnectionInfo: ocfg.MysqlConnectionInfo,
		EtcdEndPoints:       ocfg.EtcdEndPoints,
		EtcdTimeout:         ocfg.EtcdTimeout,
	}
	//step 1:db manager init ,event log client init
	if err := db.CreateManager(dbconfig); err != nil {
		t.Fatalf("error creating db manager: %v", err)
	}

	c, err := clientcmd.BuildConfigFromFlags("", ocfg.KubeConfig)
	if err != nil {
		t.Fatalf("read kube config file error: %v", err)
	}
	clientset, err := kubernetes.NewForConfig(c)
	if err != nil {
		t.Fatalf("create kube api client error: %v", err)
	}
	startCh := channels.NewRingChannel(1024)
	probeCh := channels.NewRingChannel(1024)
	storer := store.NewStore(c, clientset, db.GetManager(), option.Config{LeaderElectionNamespace: ocfg.LeaderElectionNamespace, KubeClient: clientset}, startCh, probeCh)
	if err := storer.Start(); err != nil {
		t.Fatalf("error starting store: %v", err)
	}
	return storer
}
