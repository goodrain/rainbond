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
package client_test

import (
	"testing"
	"time"

	"github.com/coreos/etcd/clientv3"
	conf "github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/node/core/store"
	"github.com/goodrain/rainbond/node/nodem/client"
)

func TestHostNodeMergeLabels(t *testing.T) {
	t.Parallel() // TODO: parallel
	hostNode := client.HostNode{
		Labels: map[string]string{
			"label 1": "value 1",
			"label 2": "value 2",
		},
		CustomLabels: map[string]string{
			"label a": "value a",
			"label b": "value b",
		},
	}
	sysLabelsLen := len(hostNode.Labels)
	exp := map[string]string{
		"label 1": "value 1",
		"label 2": "value 2",
		"label a": "value a",
		"label b": "value b",
	}
	labels := hostNode.MergeLabels()
	if len(exp) != len(labels) {
		t.Errorf("Expected %d for lables, but returned %d.", len(exp), len(labels))
	}
	equal := true
	for k, v := range exp {
		if labels[k] != v {
			equal = false
		}
	}
	if !equal {
		t.Errorf("Expected %+v for labels, but returned %+v", exp, labels)
	}
	if sysLabelsLen != len(hostNode.Labels) {
		t.Errorf("Expected %d for the length of system labels, but returned %+v", sysLabelsLen, len(hostNode.Labels))
	}
}

func TestHostNode_DelEndpoints(t *testing.T) {
	cfg := &conf.Conf{
		Etcd: clientv3.Config{
			Endpoints:   []string{"http://192.168.3.252:2379"},
			DialTimeout: 3 * time.Second,
		},
	}
	err := store.NewClient(cfg)
	if err != nil {
		t.Fatalf("error creating etcd client: %v", err)
	}
	n := &client.HostNode{
		InternalIP: "192.168.2.54",
	}
	n.DelEndpoints()
}
