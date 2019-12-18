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

package store

import (
	"testing"
	"time"

	"github.com/eapache/channels"
	v2beta1 "k8s.io/api/autoscaling/v2beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/goodrain/rainbond/cmd/worker/option"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/config"
)

func TestAppRuntimeStore_GetTenantResource(t *testing.T) {
	ocfg := option.Config{
		DBType:                  "mysql",
		MysqlConnectionInfo:     "ree8Ai:Een3meeY@tcp(192.168.1.152:3306)/region",
		EtcdEndPoints:           []string{"http://192.168.1.152:2379"},
		EtcdTimeout:             5,
		KubeConfig:              "/opt/rainbond/etc/kubernetes/kubecfg/admin.kubeconfig",
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
	defer db.CloseManager()

	c, err := clientcmd.BuildConfigFromFlags("", ocfg.KubeConfig)
	if err != nil {
		t.Fatalf("read kube config file error: %v", err)
	}
	clientset, err := kubernetes.NewForConfig(c)
	if err != nil {
		t.Fatalf("create kube api client error: %v", err)
	}
	ocfg.KubeClient = clientset
	startCh := channels.NewRingChannel(1024)
	probeCh := channels.NewRingChannel(1024)
	store := NewStore(clientset, db.GetManager(), ocfg, startCh, probeCh)
	if err := store.Start(); err != nil {
		t.Fatalf("error starting store: %v", err)
	}

	tenantID := "d22797956503441abce65e40705aac29"
	resource := store.GetTenantResource(tenantID)
	t.Logf("%+v", resource)
}

func TestGetStorageClass(t *testing.T) {
	getStoreForTest(t)

}

func TestGetAppVolumeStatus(t *testing.T) {
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
	defer db.CloseManager()

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
	storer := NewStore(clientset, db.GetManager(), option.Config{LeaderElectionNamespace: ocfg.LeaderElectionNamespace, KubeClient: clientset}, startCh, probeCh)
	if err := storer.Start(); err != nil {
		t.Fatalf("error starting store: %v", err)
	}
	time.Sleep(10 * time.Second)
	// serviceID := "69123df08744e36800c29c91574370d5"
	apps := storer.GetAllAppServices()
	for _, app := range apps {
		if app != nil {
			t.Logf("%+v", app.GetClaims())
		} else {
			t.Log("app is nil")
		}
	}
	// app := storer.GetAppService(serviceID)

	time.Sleep(20 * time.Second)
}
func TestListHPAEvents(t *testing.T) {

	c, err := clientcmd.BuildConfigFromFlags("", "/opt/rainbond/etc/kubernetes/kubecfg/admin.kubeconfig")
	if err != nil {
		t.Fatalf("read kube config file error: %v", err)
	}
	clientset, err := kubernetes.NewForConfig(c)
	if err != nil {
		t.Fatalf("create kube api client error: %v", err)
	}
	a := appRuntimeStore{
		clientset: clientset,
	}

	hpa := &v2beta1.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "xxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
			Namespace: "bab18e6b1c8640979b91f8dfdd211226",
		},
	}
	if err := a.listHPAEvents(hpa); err != nil {
		t.Fatal(err)
	}
}

func getStoreForTest(t *testing.T) Storer {
	ocfg := option.Config{
		DBType:                  "mysql",
		MysqlConnectionInfo:     "ieZoo9:Maigoed0@tcp(192.168.2.108:3306)/region",
		EtcdEndPoints:           []string{"http://192.168.2.108:2379"},
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
	defer db.CloseManager()

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
	storer := NewStore(clientset, db.GetManager(), option.Config{LeaderElectionNamespace: ocfg.LeaderElectionNamespace, KubeClient: clientset}, startCh, probeCh)
	if err := storer.Start(); err != nil {
		t.Fatalf("error starting store: %v", err)
	}
	return storer
}
