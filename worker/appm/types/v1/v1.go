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

//AppServiceType the deploy type of service.
type AppServiceType string

//TypeStatefulSet statefulset
var TypeStatefulSet AppServiceType = "statefulset"

//TypeDeployment deployment
var TypeDeployment AppServiceType = "deployment"

//TypeReplicationController rc
var TypeReplicationController AppServiceType = "replicationcontroller"

//AppServiceBase app service base info
type AppServiceBase struct {
	TenantID        string
	TenantName      string
	ServiceID       string
	ServiceAlias    string
	ServiceType     AppServiceType
	DeployVersion   string
	ContainerCPU    int
	ContainerMemory int
	Replicas        int
	NeedProxy       bool
	CreaterID       string
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

//SetDeployment set kubernetes deployment model
func (a *AppService) SetDeployment(d *v1.Deployment) {
	a.deployment = d
}

//GetStatefulSet get kubernetes statefulset model
func (a *AppService) GetStatefulSet() *v1.StatefulSet {
	return a.statefulset
}

//SetStatefulSet set kubernetes statefulset model
func (a *AppService) SetStatefulSet(d *v1.StatefulSet) {
	a.statefulset = d
}

//SetConfigMap set kubernetes configmap model
func (a *AppService) SetConfigMap(d *corev1.ConfigMap) {
	if len(a.configMaps) > 0 {
		for i, configMap := range a.configMaps {
			if configMap.GetName() == d.GetName() {
				a.configMaps[i] = d
				return
			}
		}
	}
	a.configMaps = append(a.configMaps, d)
}

//SetService set kubernetes service model
func (a *AppService) SetService(d *corev1.Service) {
	if len(a.services) > 0 {
		for i, service := range a.services {
			if service.GetName() == d.GetName() {
				a.services[i] = d
				return
			}
		}
	}
	a.services = append(a.services, d)
}

//SetIngress set kubernetes ingress model
func (a *AppService) SetIngress(d *extensions.Ingress) {
	if len(a.ingresses) > 0 {
		for i, ingress := range a.ingresses {
			if ingress.GetName() == d.GetName() {
				a.ingresses[i] = d
				return
			}
		}
	}
	a.ingresses = append(a.ingresses, d)
}

//SetEndpoint set kubernetes endpoint model
func (a *AppService) SetEndpoint(d *corev1.Endpoints) {
	if len(a.endpoints) > 0 {
		for i, endpoint := range a.endpoints {
			if endpoint.GetName() == d.GetName() {
				a.endpoints[i] = d
				return
			}
		}
	}
	a.endpoints = append(a.endpoints, d)
}

//SetPodTemplate set pod template spec
func (a *AppService) SetPodTemplate(d corev1.PodTemplateSpec) {
	if a.statefulset != nil {
		a.statefulset.Spec.Template = d
	}
	if a.deployment != nil {
		a.deployment.Spec.Template = d
	}
}

//GetPodTemplate get pod template
func (a *AppService) GetPodTemplate() *corev1.PodTemplateSpec {
	if a.statefulset != nil {
		return &a.statefulset.Spec.Template
	}
	if a.deployment != nil {
		return &a.deployment.Spec.Template
	}
	return nil
}
