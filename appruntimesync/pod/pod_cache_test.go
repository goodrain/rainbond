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

package pod

import (
	"testing"

	"github.com/goodrain/rainbond/db/config"

	"github.com/goodrain/rainbond/db"

	"github.com/Sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/tools/clientcmd"
)

func TestSatisfied(t *testing.T) {
	c := &cacheWatch{
		labelSelector: "a1c=b,ac1=d",
	}
	pod := &v1.Pod{}
	pod.Labels = map[string]string{"a1c": "b", "ke": "va", "ac1": "d"}
	if ok := c.satisfied(pod); ok {
		t.Log("OK")
		return
	}
	t.Log("False")
}
func init() {
	if err := db.CreateManager(config.Config{
		MysqlConnectionInfo: "root:admin@tcp(127.0.0.1:3306)/region",
		DBType:              "mysql",
	}); err != nil {
		logrus.Error(err)
	}
}
func TestNewPodCacheManager(t *testing.T) {
	c, err := clientcmd.BuildConfigFromFlags("", "../../../test/admin.kubeconfig")
	if err != nil {
		logrus.Error("read kube config file error.", err)
		return
	}
	clientset, err := kubernetes.NewForConfig(c)
	if err != nil {
		logrus.Error("create kube api client error", err)
		return
	}
	podCache := NewCacheManager(clientset, make(chan struct{}))
	w := podCache.Watch("")
	defer w.Stop()
	for {
		select {
		case e := <-w.ResultChan():
			logrus.Info(e)
		}
	}
}

func TestRemoveWatch(t *testing.T) {
	c, err := clientcmd.BuildConfigFromFlags("", "../../../test/admin.kubeconfig")
	if err != nil {
		logrus.Error("read kube config file error.", err)
		return
	}
	clientset, err := kubernetes.NewForConfig(c)
	if err != nil {
		logrus.Error("create kube api client error", err)
		return
	}
	podCache := NewCacheManager(clientset, make(chan struct{}))
	b := podCache.Watch("a=b")
	b.Stop()
	podCache.Watch("c=d")
	a := podCache.Watch("c=d")
	podCache.Watch("c=d")
	podCache.Watch("c=d")
	a.Stop()
}
