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

package conversion

import (
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	extensions "k8s.io/api/extensions/v1beta1"
)

//AppServiceStatus the status of service, calculate in real time from kubernetes
type AppServiceStatus string

//AppServiceBase app service base info
type AppServiceBase struct {
	TenantID  string
	ServiceID string
}

//AppService a service of rainbond app state in kubernetes
type AppService struct {
	AppServiceBase
	statefulset *v1.StatefulSet
	deployment  *v1.Deployment
	services    []*corev1.Service
	configMaps  []*corev1.ConfigMap
	ingresses   []*extensions.Ingress
	endpoints   []*corev1.Endpoints
	status      AppServiceStatus
}

//GetDeployment get kubernetes deployment model
func (a *AppService) GetDeployment() *v1.Deployment {
	return a.deployment
}
