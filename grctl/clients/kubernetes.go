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

package clients

import (
	"path"

	"github.com/goodrain/rainbond/builder/sources"

	"github.com/Sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

//K8SClient K8SClient
var K8SClient *kubernetes.Clientset

//InitClient init k8s client
func InitClient(kubeconfig string) error {
	if kubeconfig == "" {
		homePath, _ := sources.Home()
		kubeconfig = path.Join(homePath, ".kube/config")
	}
	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return err
	}
	config.QPS = 50
	config.Burst = 100
	K8SClient, err = kubernetes.NewForConfig(config)
	if err != nil {
		logrus.Error("Create kubernetes client error.", err.Error())
		return err
	}
	return nil
}
