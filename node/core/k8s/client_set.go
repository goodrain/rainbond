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
	"github.com/goodrain/rainbond/cmd/node/option"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	K8S *Client
)

//Client k8sclient
type Client struct {
	*kubernetes.Clientset
}

//NewK8sClient new k8s client
func NewK8sClient(cfg *option.Conf) error {
	config, err := clientcmd.BuildConfigFromFlags("", cfg.K8SConfPath)
	if err != nil {
		return err
	}
	config.QPS = 50
	config.Burst = 100
	cli, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}
	client := Client{
		Clientset: cli,
	}
	K8S = &client
	return nil
}
