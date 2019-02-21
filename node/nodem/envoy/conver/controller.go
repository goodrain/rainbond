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

package conver

import (
	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/node/kubecache"
	corev1 "k8s.io/api/core/v1"
)

//Controller conver controller
//conver k8s abstraction to envoy abstraction
type Controller struct {
	kubecli kubecache.KubeClient
}

//Start start
func (c *Controller) Start() {
	c.kubecli.AddEventWatch("all", c)
}

//OnAdd add k8s abstraction
func (c *Controller) OnAdd(obj interface{}) {
	if service, ok := obj.(*corev1.Service); ok {
		logrus.Info(service)
	}
	if configmap, ok := obj.(*corev1.ConfigMap); ok {
		logrus.Info(configmap)
	}
	if endpoint, ok := obj.(*corev1.Endpoints); ok {
		logrus.Info(endpoint)
	}
}

//OnUpdate update k8s abstraction
func (c *Controller) OnUpdate(oldObj, newObj interface{}) {
	// if service, ok := obj.(*corev1.Service); ok {

	// }
	// if configmap, ok := obj.(*corev1.ConfigMap); ok {

	// }
	// if endpoint, ok := obj.(*corev1.Endpoints); ok {

	// }
}

//OnDelete delete k8s abstraction
func (c *Controller) OnDelete(obj interface{}) {
	// if service, ok := obj.(*corev1.Service); ok {

	// }
	// if configmap, ok := obj.(*corev1.ConfigMap); ok {

	// }
	// if endpoint, ok := obj.(*corev1.Endpoints); ok {

	// }
}
