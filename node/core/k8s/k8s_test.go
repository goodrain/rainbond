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

package k8s

import (
	"testing"
	"time"

	"github.com/goodrain/rainbond/cmd/node/option"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
)

func init() {
	NewK8sClient(&option.Conf{
		K8SConfPath: "/Users/qingguo/gopath/src/github.com/goodrain/rainbond/test/admin.kubeconfig",
	})
}
func TestGetPodsByNodeName(t *testing.T) {
	pods, err := GetPodsByNodeName("ae1c9751-a631-4b72-9310-da752b8e6dee")
	if err != nil {
		t.Fatal(err)
	}
	t.Log(pods)
}

func TestSharedInformerFactory(t *testing.T) {
	sharedInformers := informers.NewSharedInformerFactory(K8S, time.Hour*10)
	sharedInformers.Core().V1().Nodes().Informer()
	sharedInformers.Core().V1().Services().Informer()
	stop := make(chan struct{})
	sharedInformers.Start(stop)
	time.Sleep(time.Second * 30)
	selector, err := labels.Parse("")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		nodes, err := sharedInformers.Core().V1().Nodes().Lister().List(selector)
		if err != nil {
			t.Fatal(err)
		}
		t.Log(nodes)
	}
	for i := 0; i < 2; i++ {
		selector, _ := labels.Parse("name=gr87b487Service")
		nodes, err := sharedInformers.Core().V1().Services().Lister().Services("824b2e9dcc4d461a852ddea20369d377").List(selector)
		if err != nil {
			t.Fatal(err)
		}
		t.Log(nodes)
		time.Sleep(time.Second * 5)
	}

}
