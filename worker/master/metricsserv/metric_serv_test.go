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

package metricsserv

import (
	"github.com/coreos/etcd/clientv3"
	"k8s.io/client-go/kubernetes"
	"testing"
	"time"

	"k8s.io/client-go/tools/clientcmd"
	kubeaggregatorclientset "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset"
)

func TestNewMetricsServerAPIServer(t *testing.T) {
	c, err := clientcmd.BuildConfigFromFlags("", "/opt/rainbond/etc/kubernetes/kubecfg/admin.kubeconfig")
	if err != nil {
		t.Fatal(err)
	}

	kubeaggregatorclientset, err := kubeaggregatorclientset.NewForConfig(c)
	if err != nil {
		t.Fatal(err)
	}

	metricsServiceManager := New(nil, kubeaggregatorclientset, nil)
	if err := metricsServiceManager.newMetricsServerAPIService(); err != nil {
		t.Fatal(err)
	}
}

func TestStart(t *testing.T) {
	clientv3, err := clientv3.New(clientv3.Config{
		Endpoints:        []string{"http://192.168.2.78:2379"},
		AutoSyncInterval: time.Second * 30,
		DialTimeout:      time.Second * 10,
	})
	if err != nil {
		t.Fatal(err)
	}

	c, err := clientcmd.BuildConfigFromFlags("", "/opt/rainbond/etc/kubernetes/kubecfg/admin.kubeconfig")
	if err != nil {
		t.Fatal(err)
	}

	clientset, err := kubernetes.NewForConfig(c)
	if err != nil {
		t.Fatal(err)
	}

	kubeaggregatorclientset, err := kubeaggregatorclientset.NewForConfig(c)
	if err != nil {
		t.Fatal(err)
	}

	metricsServiceManager := New(clientset, kubeaggregatorclientset, clientv3)
	if err := metricsServiceManager.Start(); err != nil {
		t.Fatal(err)
	}

	select {}
}