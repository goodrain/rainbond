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

package rs

import (
	"context"
	"testing"

	"github.com/goodrain/rainbond/gateway/controller"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestReplicaSetTimestamp(t *testing.T) {
	clientset, err := controller.NewClientSet("/opt/rainbond/etc/kubernetes/kubecfg/admin.kubeconfig")
	if err != nil {
		t.Errorf("can't create Kubernetes's client: %v", err)
	}

	ns := "c1a29fe4d7b0413993dc859430cf743d"
	rs, err := clientset.ExtensionsV1beta1().ReplicaSets(ns).Get(context.TODO(), "88d8c4c55657217522f3bb86cfbded7e-deployment-7545b75dbd", metav1.GetOptions{})
	if err != nil {
		t.Errorf("Unexpected error: %+v", err)
	}
	t.Logf("%+v", rs)
}
