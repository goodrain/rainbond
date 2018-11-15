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
	"fmt"
	"strings"
	"time"

	"github.com/goodrain/rainbond/event"
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
	//depend all service id
	Dependces []string
}

//AppService a service of rainbond app state in kubernetes
type AppService struct {
	AppServiceBase
	statefulset *v1.StatefulSet
	deployment  *v1.Deployment
	services    []*corev1.Service
	configMaps  []*corev1.ConfigMap
	ingresses   []*extensions.Ingress
	secrets     []*corev1.Secret
	pods        []*corev1.Pod
	status      AppServiceStatus
	Logger      event.Logger
}

//CacheKey app cache key
type CacheKey string

//ApproximatelyEqual cache key service id and version Equal
func (c CacheKey) ApproximatelyEqual(end CacheKey) bool {
	endinfo := strings.Split(string(end), "-")
	sourceinfo := strings.Split(string(c), "-")
	if len(endinfo) > 1 && len(sourceinfo) > 1 && endinfo[0] == sourceinfo[0] && endinfo[1] == sourceinfo[1] {
		return true
	}
	return false
}

//Equal cache key serviceid and version and createID Equal
func (c CacheKey) Equal(end CacheKey) bool {
	if string(c) == string(end) {
		return true
	}
	return false
}

//GetCacheKey get cache key
func GetCacheKey(serviceID, version, createrID string) CacheKey {
	if strings.Contains(serviceID, "-") {
		serviceID = strings.Replace(serviceID, "-", "", -1)
	}
	if strings.Contains(createrID, "-") {
		createrID = strings.Replace(createrID, "-", "", -1)
	}
	if strings.Contains(version, "-") {
		version = strings.Replace(version, "-", "", -1)
	}
	return CacheKey(fmt.Sprintf("%s-%s-%s", serviceID, version, createrID))
}

//GetNoCreaterCacheKey get cache key without createrID
func GetNoCreaterCacheKey(serviceID, version string) CacheKey {
	if strings.Contains(serviceID, "-") {
		serviceID = strings.Replace(serviceID, "-", "", -1)
	}
	if strings.Contains(version, "-") {
		version = strings.Replace(version, "-", "", -1)
	}
	return CacheKey(fmt.Sprintf("%s-%s", serviceID, version))
}

//GetDeployment get kubernetes deployment model
func (a *AppService) GetDeployment() *v1.Deployment {
	return a.deployment
}

//SetDeployment set kubernetes deployment model
func (a *AppService) SetDeployment(d *v1.Deployment) {
	a.deployment = d
}

//DeleteDeployment delete kubernetes deployment model
func (a *AppService) DeleteDeployment(d *v1.Deployment) {
	a.deployment = nil
}

//GetStatefulSet get kubernetes statefulset model
func (a *AppService) GetStatefulSet() *v1.StatefulSet {
	return a.statefulset
}

//SetStatefulSet set kubernetes statefulset model
func (a *AppService) SetStatefulSet(d *v1.StatefulSet) {
	a.statefulset = d
}

//DeleteStatefulSet set kubernetes statefulset model
func (a *AppService) DeleteStatefulSet(d *v1.StatefulSet) {
	a.statefulset = nil
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

//GetConfigMaps get configmaps
func (a *AppService) GetConfigMaps() []*corev1.ConfigMap {
	if len(a.configMaps) > 0 {
		return a.configMaps
	}
	return nil
}

//DeleteConfigMaps delete configmaps
func (a *AppService) DeleteConfigMaps(config *corev1.ConfigMap) {
	for i, c := range a.configMaps {
		if c.GetName() == config.GetName() {
			a.configMaps = append(a.configMaps[0:i], a.configMaps[i+1:]...)
			return
		}
	}
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

//GetServices get services
func (a *AppService) GetServices() []*corev1.Service {
	return a.services
}

//DeleteServices delete service
func (a *AppService) DeleteServices(service *corev1.Service) {
	for i, c := range a.services {
		if c.GetName() == service.GetName() {
			a.services = append(a.services[0:i], a.services[i+1:]...)
			return
		}
	}
}

//GetIngress get ingress
func (a *AppService) GetIngress() []*extensions.Ingress {
	return a.ingresses
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

//DeleteIngress delete kubernetes ingress model
func (a *AppService) DeleteIngress(d *extensions.Ingress) {
	for i, c := range a.ingresses {
		if c.GetName() == d.GetName() {
			a.ingresses = append(a.ingresses[0:i], a.ingresses[i+1:]...)
			return
		}
	}
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

//SetSecrets set srcrets
func (a *AppService) SetSecrets(d *corev1.Secret) {
	if len(a.secrets) > 0 {
		for i, secret := range a.secrets {
			if secret.GetName() == d.GetName() {
				a.secrets[i] = d
				return
			}
		}
	}
	a.secrets = append(a.secrets, d)
}

//DeleteSecrets set srcrets
func (a *AppService) DeleteSecrets(d *corev1.Secret) {
	for i, c := range a.secrets {
		if c.GetName() == d.GetName() {
			a.secrets = append(a.secrets[0:i], a.secrets[i+1:]...)
			return
		}
	}
}

//GetSecrets get secrets
func (a *AppService) GetSecrets() []*corev1.Secret {
	return a.secrets
}

//SetPods set pod
func (a *AppService) SetPods(d *corev1.Pod) {
	if len(a.pods) > 0 {
		for i, pod := range a.pods {
			if pod.GetName() == d.GetName() {
				a.pods[i] = d
				return
			}
		}
	}
	a.pods = append(a.pods, d)
}

//DeletePods delete pod
func (a *AppService) DeletePods(d *corev1.Pod) {
	for i, c := range a.pods {
		if c.GetName() == d.GetName() {
			a.pods = append(a.pods[0:i], a.pods[i+1:]...)
			return
		}
	}
}

//GetPods get pods
func (a *AppService) GetPods() []*corev1.Pod {
	return a.pods
}

//ErrWaitTimeOut wait time out
var ErrWaitTimeOut = fmt.Errorf("Wait time out")

//ErrWaitCancel wait cancel
var ErrWaitCancel = fmt.Errorf("Wait cancel")

//WaitReady wait ready
func (a *AppService) WaitReady(timeout time.Duration, logger event.Logger, cancel chan struct{}) error {
	if a.Ready() {
		return nil
	}
	timer := time.NewTimer(timeout)
	ticker := time.NewTicker(time.Second * 2)
	defer timer.Stop()
	select {
	case <-cancel:
		return ErrWaitCancel
	case <-timer.C:
		return ErrWaitTimeOut
	case <-ticker.C:
		if a.Ready() {
			return nil
		}
		a.printLogger(logger)
	}
	return nil
}

//WaitStop wait service stop complate
func (a *AppService) WaitStop(timeout time.Duration, logger event.Logger, cancel chan struct{}) error {
	if len(a.pods) == 0 && a.statefulset == nil && a.deployment == nil {
		return nil
	}
	timer := time.NewTimer(timeout)
	ticker := time.NewTicker(time.Second * 2)
	defer timer.Stop()
	select {
	case <-cancel:
		return ErrWaitCancel
	case <-timer.C:
		return ErrWaitTimeOut
	case <-ticker.C:
		if len(a.pods) == 0 && a.statefulset == nil && a.deployment == nil {
			return nil
		}
		a.printLogger(logger)
	}
	return nil
}
func (a *AppService) printLogger(logger event.Logger) {
	var ready int32
	if a.statefulset != nil {
		ready = a.statefulset.Status.ReadyReplicas
	}
	if a.deployment != nil {
		ready = a.deployment.Status.ReadyReplicas
	}
	logger.Info(fmt.Sprintf("current instance(count:%d ready:%d notready:%d)", len(a.pods), ready, int32(len(a.pods))-ready), map[string]string{"step": "appruntime", "status": "running"})
}

//Ready Whether ready
func (a *AppService) Ready() bool {
	if a.statefulset != nil {
		if a.statefulset.Status.ReadyReplicas >= int32(a.Replicas) {
			return true
		}
	}
	if a.deployment != nil {
		if a.deployment.Status.ReadyReplicas >= int32(a.Replicas) {
			return true
		}
	}
	return false
}
