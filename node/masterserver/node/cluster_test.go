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

package node

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func TestCluster_handleNodeStatus(t *testing.T) {
	config, err := clientcmd.BuildConfigFromFlags("", "/Users/fanyangyang/Documents/company/goodrain/remote/192.168.2.200/admin.kubeconfig")
	if err != nil {
		return
	}
	cli, err := kubernetes.NewForConfig(config)
	if err != nil {
		t.Fatal(err)
	}

	node, err := cli.CoreV1().Nodes().Get(context.Background(), "192.168.2.200", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("node is :%+v", node)
	t.Logf("cpu:%v", node.Status.Allocatable.Cpu().Value())
	t.Logf("mem: %v", node.Status.Allocatable.Memory().Value())
}
