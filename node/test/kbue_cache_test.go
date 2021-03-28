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

package test

import (
	"testing"
	"time"

	"github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/node/kubecache"
)

func TestGetCluster(t *testing.T) {
	c := &option.Conf{
		K8SConfPath:     "/opt/rainbond/etc/kubernetes/kubecfg/admin.kubeconfig",
		MinResyncPeriod: 10 * time.Second,
	}
	kubecli, err := kubecache.NewKubeClient(c, nil)
	if err != nil {
		t.Fatalf("error creating kube client: %v", err)
	}
	defer kubecli.Stop()

	nodes, err := kubecli.GetNodes()
	if err != nil {
		t.Errorf("error getting nodes: %v", err)
	}
	t.Log(nodes)
	t.Error("")
}
