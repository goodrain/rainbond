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

package provider

import (
	"context"
	"testing"

	"k8s.io/client-go/tools/clientcmd"

	"k8s.io/client-go/kubernetes"
)

func TestSelectNode(t *testing.T) {
	c, err := clientcmd.BuildConfigFromFlags("", "../../../../test/admin.kubeconfig")
	if err != nil {
		t.Fatal(err)
	}
	client, _ := kubernetes.NewForConfig(c)
	pr := &rainbondsslcProvisioner{
		name:    "rainbond.io/provisioner-sslc",
		kubecli: client,
	}
	node, err := pr.selectNode(context.TODO(), "linux", "")
	if err != nil {
		t.Fatal(err)
	}
	t.Log(node)
}

func TestGetVolumeIDByPVCName(t *testing.T) {
	t.Log(getVolumeIDByPVCName("manual17-gra02c40-0"))
	t.Log(getVolumeIDByPVCName("manual17"))
}

func TestGetPodNameByPVCName(t *testing.T) {
	t.Log(getPodNameByPVCName("manual17-gra02c40-0"))
}
