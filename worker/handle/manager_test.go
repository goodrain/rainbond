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

package handle

import (
	"context"
	"github.com/goodrain/rainbond/cmd/worker/option"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/config"
	"github.com/goodrain/rainbond/worker/appm/controller"
	"github.com/goodrain/rainbond/worker/appm/store"
	"github.com/goodrain/rainbond/worker/discover/model"
	"github.com/spf13/pflag"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"testing"
	"time"
)

func TestManager_AnalystToExec(t *testing.T) {
	s := option.NewWorker()
	s.AddFlags(pflag.CommandLine)
	s.Config.MysqlConnectionInfo = "username:password@tcp(127.0.0.1:3306)/region"
	s.Config.KubeConfig = "/Users/abe/go/src/github.com/goodrain/rainbond/test/admin.kubeconfig"

	dbconfig := config.Config{
		DBType:              s.Config.DBType,
		MysqlConnectionInfo: s.Config.MysqlConnectionInfo,
		EtcdEndPoints:       s.Config.EtcdEndPoints,
		EtcdTimeout:         s.Config.EtcdTimeout,
	}
	//step 1:db manager init ,event log client init
	if err := db.CreateManager(dbconfig); err != nil {
		t.Fatalf("Can't create db manager: %v", err)
	}
	defer db.CloseManager()

	c, err := clientcmd.BuildConfigFromFlags("", s.Config.KubeConfig)
	if err != nil {
		t.Fatalf("read kube config file error: %v", err)
	}
	clientset, err := kubernetes.NewForConfig(c)
	if err != nil {
		t.Fatalf("create kube api client error: %v", err)
	}
	s.Config.KubeClient = clientset

	cachestore := store.NewStore(db.GetManager(), s.Config)
	if err := cachestore.Start(); err != nil {
		t.Fatalf("start kube cache store error: %v", err)
	}
	controllerManager := controller.NewManager(cachestore, clientset)
	defer controllerManager.Stop()

	task := &model.Task{
		Type:       "start",
		CreateTime: time.Now(),
		User:       "huangrh",
		Body: model.StartTaskBody{
			TenantID:      "e8539a9c33fd418db11cce26d2bca431",
			ServiceID:     "43eaae441859eda35b02075d37d83589",
			DeployVersion: "1.0.0",
			EventID:       "dummy-event-id",
		},
	}

	ctx, _ := context.WithCancel(context.Background())
	handleManager := NewManager(ctx, s.Config, cachestore, controllerManager)
	if err := handleManager.AnalystToExec(task); err != nil {
		t.Errorf("analyst exec failed: %v", err)
	}
}
