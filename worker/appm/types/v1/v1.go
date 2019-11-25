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

package v1

import (
	"fmt"
	"os"
	"strconv"

	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/builder"
	"github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/event"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	extensions "k8s.io/api/extensions/v1beta1"
)

// EventType type of event
type EventType string

const (
	// StartEvent event about to start third-party service
	StartEvent EventType = "START"
	// StopEvent event about to stop third-party service
	StopEvent EventType = "STOP"
)

// Event holds the context of a start event.
type Event struct {
	Type    EventType
	Sid     string // service id
	Port    int
	IsInner bool
}

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

//TypeUpgradeMethod upgrade service method type
type TypeUpgradeMethod string

//Rolling Start the new version before stoping the old version the rolling upgrade
var Rolling TypeUpgradeMethod = "Rolling"

//OnDelete Stop the old version before starting the new version the upgrade
var OnDelete TypeUpgradeMethod = "OnDelete"

//AppServiceBase app service base info
type AppServiceBase struct {
	TenantID         string
	TenantName       string
	ServiceID        string
	ServiceAlias     string
	ServiceType      AppServiceType
	ServiceKind      model.ServiceKind
	DeployVersion    string
	ContainerCPU     int
	ContainerMemory  int
	UpgradeMethod    TypeUpgradeMethod
	Replicas         int
	NeedProxy        bool
	IsWindowsService bool
	CreaterID        string
	//depend all service id
	Dependces    []string
	ExtensionSet map[string]string
}

//AppService a service of rainbond app state in kubernetes
type AppService struct {
	AppServiceBase
	tenant       *corev1.Namespace
	statefulset  *v1.StatefulSet
	deployment   *v1.Deployment
	replicasets  []*v1.ReplicaSet
	services     []*corev1.Service
	delServices  []*corev1.Service
	endpoints    []*corev1.Endpoints
	configMaps   []*corev1.ConfigMap
	ingresses    []*extensions.Ingress
	delIngs      []*extensions.Ingress // ingresses which need to be deleted
	secrets      []*corev1.Secret
	delSecrets   []*corev1.Secret // secrets which need to be deleted
	pods         []*corev1.Pod
	status       AppServiceStatus
	Logger       event.Logger
	UpgradePatch map[string][]byte
}

//CacheKey app cache key
type CacheKey string

//Equal cache key serviceid and version and createID Equal
func (c CacheKey) Equal(end CacheKey) bool {
	if string(c) == string(end) {
		return true
	}
	return false
}

//GetCacheKeyOnlyServiceID get cache key only service id
func GetCacheKeyOnlyServiceID(serviceID string) CacheKey {
	return CacheKey(serviceID)
}

//GetDeployment get kubernetes deployment model
func (a AppService) GetDeployment() *v1.Deployment {
	return a.deployment
}

//SetDeployment set kubernetes deployment model
func (a *AppService) SetDeployment(d *v1.Deployment) {
	a.deployment = d
	if v, ok := d.Spec.Template.Labels["version"]; ok && v != "" {
		a.DeployVersion = v
	}
	a.Replicas = int(*d.Spec.Replicas)
}

//DeleteDeployment delete kubernetes deployment model
func (a *AppService) DeleteDeployment(d *v1.Deployment) {
	a.deployment = nil
}

//GetStatefulSet get kubernetes statefulset model
func (a AppService) GetStatefulSet() *v1.StatefulSet {
	return a.statefulset
}

//SetStatefulSet set kubernetes statefulset model
func (a *AppService) SetStatefulSet(d *v1.StatefulSet) {
	a.statefulset = d
	if v, ok := d.Spec.Template.Labels["version"]; ok && v != "" {
		a.DeployVersion = v
	}
	a.Replicas = int(*d.Spec.Replicas)
}

//SetReplicaSets set kubernetes replicaset
func (a *AppService) SetReplicaSets(d *v1.ReplicaSet) {
	if len(a.replicasets) > 0 {
		for i, replicaset := range a.replicasets {
			if replicaset.GetName() == d.GetName() {
				a.replicasets[i] = d
				return
			}
		}
	}
	a.replicasets = append(a.replicasets, d)
}

//DeleteReplicaSet delete replicaset
func (a *AppService) DeleteReplicaSet(d *v1.ReplicaSet) {
	for i, c := range a.replicasets {
		if c.GetName() == d.GetName() {
			a.replicasets = append(a.replicasets[0:i], a.replicasets[i+1:]...)
			return
		}
	}
}

//GetReplicaSets get replicaset
func (a *AppService) GetReplicaSets() []*v1.ReplicaSet {
	return a.replicasets
}

// GetNewestReplicaSet returns the newest replica set.
func (a *AppService) GetNewestReplicaSet() (newest *v1.ReplicaSet) {
	if len(a.replicasets) == 0 {
		return
	}
	newest = a.replicasets[0]
	for _, rs := range a.replicasets {
		if newest.ObjectMeta.CreationTimestamp.Before(&rs.ObjectMeta.CreationTimestamp) {
			newest = rs
		}
	}
	return
}

//GetReplicaSetVersion get rs version
func GetReplicaSetVersion(rs *v1.ReplicaSet) int {
	if version, ok := rs.Annotations["deployment.kubernetes.io/revision"]; ok {
		v, _ := strconv.Atoi(version)
		return v
	}
	return 0
}

//GetCurrentReplicaSet get current replicaset
func (a *AppService) GetCurrentReplicaSet() *v1.ReplicaSet {
	if a.deployment != nil {
		revision, ok := a.deployment.Annotations["deployment.kubernetes.io/revision"]
		if ok {
			for _, rs := range a.replicasets {
				if rs.Annotations["deployment.kubernetes.io/revision"] == revision {
					return rs
				}
			}
		}
	}
	return nil
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

// SetServices set set k8s service model list
func (a *AppService) SetServices(svcs []*corev1.Service) {
	a.services = svcs
}

//GetServices get services
func (a *AppService) GetServices() []*corev1.Service {
	return a.services
}

//GetDelServices returns services that need to be deleted.
func (a *AppService) GetDelServices() []*corev1.Service {
	return a.delServices
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

// AddEndpoints adds k8s endpoints to receiver *AppService.
func (a *AppService) AddEndpoints(ep *corev1.Endpoints) {
	if len(a.endpoints) > 0 {
		for i, e := range a.endpoints {
			if e.GetName() == ep.GetName() {
				a.endpoints[i] = ep
				return
			}
		}
	}
	a.endpoints = append(a.endpoints, ep)
}

// GetEndpoints returns endpoints in AppService
func (a *AppService) GetEndpoints() []*corev1.Endpoints {
	return a.endpoints
}

// GetEndpointsByName returns endpoints in AppService
func (a *AppService) GetEndpointsByName(name string) *corev1.Endpoints {
	for _, ep := range a.endpoints {
		if ep.GetName() == name {
			return ep
		}
	}
	return nil
}

//DelEndpoints deletes *corev1.Endpoints
func (a *AppService) DelEndpoints(ep *corev1.Endpoints) {
	for i, c := range a.endpoints {
		if c.GetName() == ep.GetName() {
			a.endpoints = append(a.endpoints[0:i], a.endpoints[i+1:]...)
			return
		}
	}
}

//GetIngress get ingress
func (a *AppService) GetIngress() []*extensions.Ingress {
	return a.ingresses
}

//GetDelIngs gets delIngs which need to be deleted
func (a *AppService) GetDelIngs() []*extensions.Ingress {
	return a.delIngs
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

// SetIngresses sets k8s ingress list
func (a *AppService) SetIngresses(i []*extensions.Ingress) {
	a.ingresses = i
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

//SetSecret set srcrets
func (a *AppService) SetSecret(d *corev1.Secret) {
	if d == nil {
		return
	}
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

// SetSecrets sets k8s secret list
func (a *AppService) SetSecrets(s []*corev1.Secret) {
	a.secrets = s
}

//SetAllSecrets sets secrets
func (a *AppService) SetAllSecrets(secrets []*corev1.Secret) {
	a.secrets = secrets
}

//DeleteSecrets set secrets
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

//GetDelSecrets get delSecrets which need to be deleted
func (a *AppService) GetDelSecrets() []*corev1.Secret {
	return a.delSecrets
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

// GetPodsByName returns the pod based on podname.
func (a *AppService) GetPodsByName(podname string) *corev1.Pod {
	for _, pod := range a.pods {
		if pod.ObjectMeta.Name == podname {
			return pod
		}
	}
	return nil
}

//SetTenant set tenant
func (a *AppService) SetTenant(d *corev1.Namespace) {
	a.tenant = d
}

//GetTenant get tenant namespace
func (a *AppService) GetTenant() *corev1.Namespace {
	return a.tenant
}

// SetDeletedResources sets the resources that need to be deleted
func (a *AppService) SetDeletedResources(old *AppService) {
	if old == nil {
		logrus.Debugf("empty old app service.")
		return
	}
	for _, o := range old.GetIngress() {
		del := true
		for _, n := range a.GetIngress() {
			if o.Name == n.Name {
				del = false
				break
			}
		}
		if del {
			a.delIngs = append(a.delIngs, o)
		}
	}
	for _, o := range old.GetSecrets() {
		del := true
		for _, n := range a.GetSecrets() {
			if o.Name == n.Name {
				del = false
				break
			}
		}
		if del {
			a.delSecrets = append(a.delSecrets, o)
		}
	}
	for _, o := range old.GetServices() {
		del := true
		for _, n := range a.GetServices() {
			if o.Name == n.Name {
				del = false
				break
			}
		}
		if del {
			a.delServices = append(a.delServices, o)
		}
	}
}

// DistinguishPod uses replica set to distinguish between old and new pods
// true: new pod; false: old pod.
func (a *AppService) DistinguishPod(pod *corev1.Pod) bool {
	rss := a.GetNewestReplicaSet()
	if rss == nil {
		return true
	}
	return !pod.ObjectMeta.CreationTimestamp.Before(&rss.ObjectMeta.CreationTimestamp)
}

func (a *AppService) String() string {
	return fmt.Sprintf(`
	-----------------------------------------------------
	App:%s
	DeployVersion:%s
	Statefulset %+v
	Deployment %+v
	Pod %d
	ingresses %s
	service %s
	endpoints %+v
	-----------------------------------------------------
	`,
		a.ServiceAlias,
		a.DeployVersion,
		a.statefulset,
		a.deployment,
		len(a.pods),
		func(ing []*extensions.Ingress) string {
			result := ""
			for _, i := range ing {
				result += i.Name + ","
			}
			return result
		}(a.ingresses),
		func(ing []*corev1.Service) string {
			result := ""
			for _, i := range ing {
				result += i.Name + ","
			}
			return result
		}(a.services),
		a.endpoints,
	)
}

//TenantResource tenant resource statistical models
type TenantResource struct {
	TenantID         string `json:"tenant_id,omitempty"`
	CPURequest       int64  `json:"cpu_request,omitempty"`
	CPULimit         int64  `json:"cpu_limit,omitempty"`
	MemoryRequest    int64  `json:"memory_request,omitempty"`
	MemoryLimit      int64  `json:"memory_limit,omitempty"`
	UnscdCPUReq      int64  `json:"unscd_cpu_req,omitempty"`
	UnscdCPULimit    int64  `json:"unscd_cpu_limit,omitempty"`
	UnscdMemoryReq   int64  `json:"unscd_memory_req,omitempty"`
	UnscdMemoryLimit int64  `json:"unscd_memory_limit,omitempty"`
}

// K8sResources holds kubernetes resources(svc, sercert, ep, ing).
type K8sResources struct {
	Services  []*corev1.Service
	Secrets   []*corev1.Secret
	Ingresses []*extensions.Ingress
}

//GetTCPMeshImageName get tcp mesh image name
func GetTCPMeshImageName() string {
	if d := os.Getenv("TCPMESH_DEFAULT_IMAGE_NAME"); d != "" {
		return d
	}
	return builder.REGISTRYDOMAIN + "/rbd-mesh-data-panel"
}

//GetProbeMeshImageName get probe init mesh image name
func GetProbeMeshImageName() string {
	if d := os.Getenv("PROBE_MESH_IMAGE_NAME"); d != "" {
		return d
	}
	return builder.REGISTRYDOMAIN + "/rbd-init-probe"
}

// StorageClass storage class
type StorageClass struct {
	Name                 string
	Parameters           map[string]string
	Provisioner          string
	ReclaimPolicy        string
	VolumeBindingMode    string
	AllowVolumeExpansion *bool
	AllowedTopologies    []corev1.TopologySelectorTerm
	Claims               []*corev1.PersistentVolumeClaim
	PervistentVolume     []*corev1.PersistentVolume
}
