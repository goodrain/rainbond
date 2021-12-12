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

package store

import (
	"context"
	"fmt"
	betav1 "k8s.io/api/networking/v1beta1"
	"os"
	"sync"
	"time"

	"github.com/goodrain/rainbond/api/util/bcode"
	"github.com/goodrain/rainbond/cmd/worker/option"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/pkg/apis/rainbond/v1alpha1"
	rainbondversioned "github.com/goodrain/rainbond/pkg/generated/clientset/versioned"
	"github.com/goodrain/rainbond/pkg/generated/informers/externalversions"
	"github.com/goodrain/rainbond/util/constants"
	k8sutil "github.com/goodrain/rainbond/util/k8s"
	"github.com/goodrain/rainbond/worker/appm/componentdefinition"
	"github.com/goodrain/rainbond/worker/appm/conversion"
	v1 "github.com/goodrain/rainbond/worker/appm/types/v1"
	"github.com/goodrain/rainbond/worker/server/pb"
	workerutil "github.com/goodrain/rainbond/worker/util"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"
	monitorv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2beta2"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	storagev1 "k8s.io/api/storage/v1"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	internalclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	internalinformers "k8s.io/apiextensions-apiserver/pkg/client/informers/externalversions"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

var rc2RecordType = map[string]string{
	"Deployment":              "mumaul",
	"Statefulset":             "mumaul",
	"HorizontalPodAutoscaler": "hpa",
}

//Storer app runtime store interface
type Storer interface {
	Start() error
	Ready() bool
	GetPod(namespace, name string) (*corev1.Pod, error)
	RegistAppService(*v1.AppService)
	GetAppService(serviceID string) *v1.AppService
	UpdateGetAppService(serviceID string) *v1.AppService
	GetAllAppServices() []*v1.AppService
	GetAppServiceStatus(serviceID string) string
	GetAppServiceStatuses(serviceIDs []string) map[string]string
	GetAppServicesStatus(serviceIDs []string) map[string]string
	GetTenantResource(tenantID string) TenantResource
	GetTenantResourceList() []TenantResource
	GetTenantRunningApp(tenantID string) []*v1.AppService
	GetNeedBillingStatus(serviceIDs []string) map[string]string
	OnDeletes(obj ...interface{})
	RegistPodUpdateListener(string, chan<- *corev1.Pod)
	UnRegistPodUpdateListener(string)
	RegisterVolumeTypeListener(string, chan<- *model.TenantServiceVolumeType)
	UnRegisterVolumeTypeListener(string)
	GetCrds() ([]*apiextensions.CustomResourceDefinition, error)
	GetCrd(name string) (*apiextensions.CustomResourceDefinition, error)
	GetServiceMonitorClient() (*versioned.Clientset, error)
	GetAppStatus(appID string) (pb.AppStatus_Status, error)
	GetAppResources(appID string) (int64, int64, error)
	GetHelmApp(namespace, name string) (*v1alpha1.HelmApp, error)
	ListPods(namespace string, selector labels.Selector) ([]*corev1.Pod, error)
	ListReplicaSets(namespace string, selector labels.Selector) ([]*appsv1.ReplicaSet, error)
	ListServices(namespace string, selector labels.Selector) ([]*corev1.Service, error)

	Informer() *Informer
	Lister() *Lister
}

// EventType type of event associated with an informer
type EventType string

const (
	// CreateEvent event associated with new objects in an informer
	CreateEvent EventType = "CREATE"
	// UpdateEvent event associated with an object update in an informer
	UpdateEvent EventType = "UPDATE"
	// DeleteEvent event associated when an object is removed from an informer
	DeleteEvent EventType = "DELETE"
)

// Event holds the context of an event.
type Event struct {
	Type EventType
	Obj  interface{}
}

//appRuntimeStore app runtime store
//cache all kubernetes object and appservice
type appRuntimeStore struct {
	kubeconfig             *rest.Config
	clientset              kubernetes.Interface
	crdClient              *internalclientset.Clientset
	rainbondClient         rainbondversioned.Interface
	crClients              map[string]interface{}
	ctx                    context.Context
	cancel                 context.CancelFunc
	informers              *Informer
	listers                *Lister
	replicaSets            *Lister
	appServices            sync.Map
	appCount               int32
	dbmanager              db.Manager
	conf                   option.Config
	stopch                 chan struct{}
	podUpdateListeners     map[string]chan<- *corev1.Pod
	podUpdateListenerLock  sync.Mutex
	volumeTypeListeners    map[string]chan<- *model.TenantServiceVolumeType
	volumeTypeListenerLock sync.Mutex
	resourceCache          *ResourceCache
}

//NewStore new app runtime store
func NewStore(
	kubeconfig *rest.Config,
	clientset kubernetes.Interface,
	rainbondClient rainbondversioned.Interface,
	dbmanager db.Manager,
	conf option.Config) Storer {
	ctx, cancel := context.WithCancel(context.Background())
	store := &appRuntimeStore{
		kubeconfig:          kubeconfig,
		clientset:           clientset,
		rainbondClient:      rainbondClient,
		ctx:                 ctx,
		cancel:              cancel,
		informers:           &Informer{CRS: make(map[string]cache.SharedIndexInformer)},
		listers:             &Lister{},
		appServices:         sync.Map{},
		conf:                conf,
		dbmanager:           dbmanager,
		crClients:           make(map[string]interface{}),
		resourceCache:       NewResourceCache(),
		podUpdateListeners:  make(map[string]chan<- *corev1.Pod, 1),
		volumeTypeListeners: make(map[string]chan<- *model.TenantServiceVolumeType, 1),
	}
	crdClient, err := internalclientset.NewForConfig(kubeconfig)
	if err != nil {
		logrus.Errorf("create crd client failure %s", err.Error())
	}
	if crdClient != nil {
		store.crdClient = crdClient
		crdFactory := internalinformers.NewSharedInformerFactory(crdClient, 5*time.Minute)
		store.informers.CRD = crdFactory.Apiextensions().V1().CustomResourceDefinitions().Informer()
		store.listers.CRD = crdFactory.Apiextensions().V1().CustomResourceDefinitions().Lister()
	}

	// create informers factory, enable and assign required informers
	infFactory := informers.NewSharedInformerFactoryWithOptions(conf.KubeClient, 10*time.Second,
		informers.WithNamespace(corev1.NamespaceAll))

	store.informers.Namespace = infFactory.Core().V1().Namespaces().Informer()

	store.informers.Deployment = infFactory.Apps().V1().Deployments().Informer()
	store.listers.Deployment = infFactory.Apps().V1().Deployments().Lister()

	store.informers.StatefulSet = infFactory.Apps().V1().StatefulSets().Informer()
	store.listers.StatefulSet = infFactory.Apps().V1().StatefulSets().Lister()

	store.informers.Service = infFactory.Core().V1().Services().Informer()
	store.listers.Service = infFactory.Core().V1().Services().Lister()

	store.informers.Pod = infFactory.Core().V1().Pods().Informer()
	store.listers.Pod = infFactory.Core().V1().Pods().Lister()

	store.informers.Secret = infFactory.Core().V1().Secrets().Informer()
	store.listers.Secret = infFactory.Core().V1().Secrets().Lister()

	store.informers.ConfigMap = infFactory.Core().V1().ConfigMaps().Informer()
	store.listers.ConfigMap = infFactory.Core().V1().ConfigMaps().Lister()

	if k8sutil.IsHighVersion() {
		store.informers.Ingress = infFactory.Networking().V1().Ingresses().Informer()
		store.listers.Ingress = infFactory.Networking().V1().Ingresses().Lister()
	} else {
		store.informers.Ingress = infFactory.Networking().V1beta1().Ingresses().Informer()
		store.listers.BetaIngress = infFactory.Networking().V1beta1().Ingresses().Lister()
	}

	store.informers.ReplicaSet = infFactory.Apps().V1().ReplicaSets().Informer()
	store.listers.ReplicaSets = infFactory.Apps().V1().ReplicaSets().Lister()

	store.informers.Endpoints = infFactory.Core().V1().Endpoints().Informer()
	store.listers.Endpoints = infFactory.Core().V1().Endpoints().Lister()

	store.informers.Nodes = infFactory.Core().V1().Nodes().Informer()
	store.listers.Nodes = infFactory.Core().V1().Nodes().Lister()

	store.informers.StorageClass = infFactory.Storage().V1().StorageClasses().Informer()
	store.listers.StorageClass = infFactory.Storage().V1().StorageClasses().Lister()

	store.informers.Claims = infFactory.Core().V1().PersistentVolumeClaims().Informer()
	store.listers.Claims = infFactory.Core().V1().PersistentVolumeClaims().Lister()

	store.informers.Events = infFactory.Core().V1().Events().Informer()

	store.informers.HorizontalPodAutoscaler = infFactory.Autoscaling().V2beta2().HorizontalPodAutoscalers().Informer()
	store.listers.HorizontalPodAutoscaler = infFactory.Autoscaling().V2beta2().HorizontalPodAutoscalers().Lister()

	// rainbond custom resource
	rainbondInformer := externalversions.NewSharedInformerFactoryWithOptions(rainbondClient, 10*time.Second,
		externalversions.WithNamespace(corev1.NamespaceAll))
	store.listers.HelmApp = rainbondInformer.Rainbond().V1alpha1().HelmApps().Lister()
	store.informers.HelmApp = rainbondInformer.Rainbond().V1alpha1().HelmApps().Informer()
	store.listers.ThirdComponent = rainbondInformer.Rainbond().V1alpha1().ThirdComponents().Lister()
	store.informers.ThirdComponent = rainbondInformer.Rainbond().V1alpha1().ThirdComponents().Informer()
	store.listers.ComponentDefinition = rainbondInformer.Rainbond().V1alpha1().ComponentDefinitions().Lister()
	store.informers.ComponentDefinition = rainbondInformer.Rainbond().V1alpha1().ComponentDefinitions().Informer()
	store.informers.ComponentDefinition.AddEventHandlerWithResyncPeriod(componentdefinition.GetComponentDefinitionBuilder(), time.Second*300)

	// Endpoint Event Handler
	epEventHandler := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			ep := obj.(*corev1.Endpoints)

			serviceID := ep.Labels["service_id"]
			version := ep.Labels["version"]
			createrID := ep.Labels["creater_id"]
			if serviceID != "" && createrID != "" {
				appservice, err := store.getAppService(serviceID, version, createrID, true)
				if err == conversion.ErrServiceNotFound {
					logrus.Debugf("ServiceID: %s; Action: AddFunc; service not found", serviceID)
				}
				if appservice != nil {
					appservice.AddEndpoints(ep)
				}
			}
		},
		DeleteFunc: func(obj interface{}) {
			ep := obj.(*corev1.Endpoints)
			serviceID := ep.Labels["service_id"]
			version := ep.Labels["version"]
			createrID := ep.Labels["creater_id"]
			if serviceID != "" && createrID != "" {
				appservice, _ := store.getAppService(serviceID, version, createrID, false)
				if appservice != nil {
					appservice.DelEndpoints(ep)
					if appservice.IsClosed() {
						logrus.Debugf("ServiceID: %s; Action: DeleteFunc;service is closed", serviceID)
						store.DeleteAppService(appservice)
					}
				}
			}
		},
		UpdateFunc: func(old, cur interface{}) {
			cep := cur.(*corev1.Endpoints)

			serviceID := cep.Labels["service_id"]
			version := cep.Labels["version"]
			createrID := cep.Labels["creater_id"]
			if serviceID != "" && createrID != "" {
				appservice, err := store.getAppService(serviceID, version, createrID, true)
				if err == conversion.ErrServiceNotFound {
					logrus.Debugf("ServiceID: %s; Action: UpdateFunc; service not found", serviceID)
				}
				if appservice != nil {
					appservice.AddEndpoints(cep)
				}
			}
		},
	}

	store.informers.Namespace.AddEventHandler(store.nsEventHandler())
	store.informers.Deployment.AddEventHandlerWithResyncPeriod(store, time.Second*10)
	store.informers.StatefulSet.AddEventHandlerWithResyncPeriod(store, time.Second*10)
	store.informers.Pod.AddEventHandlerWithResyncPeriod(store.podEventHandler(), time.Second*10)
	store.informers.Secret.AddEventHandlerWithResyncPeriod(store, time.Second*10)
	store.informers.Service.AddEventHandlerWithResyncPeriod(store, time.Second*10)
	store.informers.Ingress.AddEventHandlerWithResyncPeriod(store, time.Second*10)
	store.informers.ConfigMap.AddEventHandlerWithResyncPeriod(store, time.Second*10)
	store.informers.ReplicaSet.AddEventHandlerWithResyncPeriod(store, time.Second*10)
	store.informers.Endpoints.AddEventHandlerWithResyncPeriod(epEventHandler, time.Second*10)
	store.informers.Nodes.AddEventHandlerWithResyncPeriod(store, time.Second*10)
	store.informers.StorageClass.AddEventHandlerWithResyncPeriod(store, time.Second*300)
	store.informers.Claims.AddEventHandlerWithResyncPeriod(store, time.Second*10)
	store.informers.Events.AddEventHandlerWithResyncPeriod(store.evtEventHandler(), time.Second*10)
	store.informers.HorizontalPodAutoscaler.AddEventHandlerWithResyncPeriod(store, time.Second*10)
	store.informers.ThirdComponent.AddEventHandlerWithResyncPeriod(store, time.Second*10)

	return store
}

func (a *appRuntimeStore) Informer() *Informer {
	return a.informers
}

func (a *appRuntimeStore) Lister() *Lister {
	return a.listers
}

func (a *appRuntimeStore) init() error {
	//init leader namespace
	leaderNamespace := a.conf.LeaderElectionNamespace
	if _, err := a.conf.KubeClient.CoreV1().Namespaces().Get(context.Background(), leaderNamespace, metav1.GetOptions{}); err != nil {
		if k8sErrors.IsNotFound(err) {
			_, err = a.conf.KubeClient.CoreV1().Namespaces().Create(context.Background(), &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: leaderNamespace,
				},
			}, metav1.CreateOptions{})
		}
		if err != nil {
			return err
		}
	}
	// init third-party service
	return a.initStorageclass()
}

func (a *appRuntimeStore) Start() error {
	if err := a.init(); err != nil {
		return err
	}
	stopch := make(chan struct{})
	a.informers.Start(stopch)
	a.stopch = stopch
	go a.clean()
	for !a.Ready() {
	}
	// init core componentdefinition
	componentdefinition.GetComponentDefinitionBuilder().InitCoreComponentDefinition(a.rainbondClient)
	go func() {
		a.initCustomResourceInformer(stopch)
	}()
	return nil
}

//Ready if all kube informers is syncd, store is ready
func (a *appRuntimeStore) Ready() bool {
	return a.informers.Ready()
}

//checkReplicasetWhetherDelete if rs is old version,if it is old version and it always delete all pod.
// will delete it
func (a *appRuntimeStore) checkReplicasetWhetherDelete(app *v1.AppService, rs *appsv1.ReplicaSet) {
	current := app.GetCurrentReplicaSet()
	if current != nil && current.Name != rs.Name {
		//delete old version
		if v1.GetReplicaSetVersion(current) > v1.GetReplicaSetVersion(rs) {
			if rs.Status.Replicas == 0 && rs.Status.ReadyReplicas == 0 && rs.Status.AvailableReplicas == 0 {
				if err := a.conf.KubeClient.AppsV1().ReplicaSets(rs.Namespace).Delete(context.Background(), rs.Name, metav1.DeleteOptions{}); err != nil && k8sErrors.IsNotFound(err) {
					logrus.Errorf("delete old version replicaset failure %s", err.Error())
				}
			}
		}
	}
}

func (a *appRuntimeStore) OnAdd(obj interface{}) {
	if thirdComponent, ok := obj.(*v1alpha1.ThirdComponent); ok {
		serviceID := thirdComponent.Labels["service_id"]
		createrID := thirdComponent.Labels["creater_id"]
		if serviceID != "" && createrID != "" {
			appservice, _ := a.getAppService(serviceID, "", createrID, true)
			if appservice != nil {
				appservice.SetWorkload(thirdComponent)
				return
			}
		}
	}
	if deployment, ok := obj.(*appsv1.Deployment); ok {
		serviceID := deployment.Labels["service_id"]
		version := deployment.Labels["version"]
		createrID := deployment.Labels["creater_id"]
		if serviceID != "" && version != "" && createrID != "" {
			appservice, err := a.getAppService(serviceID, version, createrID, true)
			if err == conversion.ErrServiceNotFound {
				a.conf.KubeClient.AppsV1().Deployments(deployment.Namespace).Delete(context.Background(), deployment.Name, metav1.DeleteOptions{})
			}
			if appservice != nil {
				appservice.SetDeployment(deployment)
				return
			}
		}
	}
	if statefulset, ok := obj.(*appsv1.StatefulSet); ok {
		serviceID := statefulset.Labels["service_id"]
		version := statefulset.Labels["version"]
		createrID := statefulset.Labels["creater_id"]
		if serviceID != "" && version != "" && createrID != "" {
			appservice, err := a.getAppService(serviceID, version, createrID, true)
			if err == conversion.ErrServiceNotFound {
				a.conf.KubeClient.AppsV1().StatefulSets(statefulset.Namespace).Delete(context.Background(), statefulset.Name, metav1.DeleteOptions{})
			}
			if appservice != nil {
				appservice.SetStatefulSet(statefulset)
				return
			}
		}
	}
	if replicaset, ok := obj.(*appsv1.ReplicaSet); ok {
		serviceID := replicaset.Labels["service_id"]
		version := replicaset.Labels["version"]
		createrID := replicaset.Labels["creater_id"]
		if serviceID != "" && version != "" && createrID != "" {
			appservice, err := a.getAppService(serviceID, version, createrID, true)
			if err == conversion.ErrServiceNotFound {
				a.conf.KubeClient.AppsV1().Deployments(replicaset.Namespace).Delete(context.Background(), replicaset.Name, metav1.DeleteOptions{})
			}
			if appservice != nil {
				appservice.SetReplicaSets(replicaset)
				a.checkReplicasetWhetherDelete(appservice, replicaset)
				return
			}
		}
	}
	if secret, ok := obj.(*corev1.Secret); ok {
		serviceID := secret.Labels["service_id"]
		version := secret.Labels["version"]
		createrID := secret.Labels["creater_id"]
		if serviceID != "" && createrID != "" {
			appservice, err := a.getAppService(serviceID, version, createrID, true)
			if err == conversion.ErrServiceNotFound {
				a.conf.KubeClient.CoreV1().Secrets(secret.Namespace).Delete(context.Background(), secret.Name, metav1.DeleteOptions{})
			}
			if appservice != nil {
				appservice.SetSecret(secret)
				return
			}
		}
	}
	if service, ok := obj.(*corev1.Service); ok {
		serviceID := service.Labels["service_id"]
		version := service.Labels["version"]
		createrID := service.Labels["creater_id"]
		if serviceID != "" && createrID != "" {
			appservice, err := a.getAppService(serviceID, version, createrID, true)
			if err == conversion.ErrServiceNotFound {
				a.conf.KubeClient.CoreV1().Services(service.Namespace).Delete(context.Background(), service.Name, metav1.DeleteOptions{})
			}
			if appservice != nil {
				appservice.SetService(service)
				return
			}
		}
	}
	if ingress, ok := obj.(*networkingv1.Ingress); ok {
		serviceID := ingress.Labels["service_id"]
		version := ingress.Labels["version"]
		createrID := ingress.Labels["creater_id"]
		if serviceID != "" && createrID != "" {
			appservice, err := a.getAppService(serviceID, version, createrID, true)
			if err == conversion.ErrServiceNotFound {
				a.conf.KubeClient.NetworkingV1().Ingresses(ingress.Namespace).Delete(context.Background(), ingress.Name, metav1.DeleteOptions{})
			}
			if appservice != nil {
				appservice.SetIngress(ingress)
				return
			}
		}
	}
	if ingress, ok := obj.(*betav1.Ingress); ok {
		serviceID := ingress.Labels["service_id"]
		version := ingress.Labels["version"]
		createrID := ingress.Labels["creater_id"]
		if serviceID != "" && createrID != "" {
			appservice, err := a.getAppService(serviceID, version, createrID, true)
			if err == conversion.ErrServiceNotFound {
				a.conf.KubeClient.NetworkingV1beta1().Ingresses(ingress.Namespace).Delete(context.Background(), ingress.Name, metav1.DeleteOptions{})
			}
			if appservice != nil {
				appservice.SetIngress(ingress)
				return
			}
		}
	}
	if configmap, ok := obj.(*corev1.ConfigMap); ok {
		serviceID := configmap.Labels["service_id"]
		version := configmap.Labels["version"]
		createrID := configmap.Labels["creater_id"]
		if serviceID != "" && createrID != "" {
			appservice, err := a.getAppService(serviceID, version, createrID, true)
			if err == conversion.ErrServiceNotFound {
				a.conf.KubeClient.CoreV1().ConfigMaps(configmap.Namespace).Delete(context.Background(), configmap.Name, metav1.DeleteOptions{})
			}
			if appservice != nil {
				appservice.SetConfigMap(configmap)
				return
			}
		}
	}
	if hpa, ok := obj.(*autoscalingv2.HorizontalPodAutoscaler); ok {
		serviceID := hpa.Labels["service_id"]
		version := hpa.Labels["version"]
		createrID := hpa.Labels["creater_id"]
		if serviceID != "" && version != "" && createrID != "" {
			appservice, err := a.getAppService(serviceID, version, createrID, true)
			if err == conversion.ErrServiceNotFound {
				a.conf.KubeClient.AutoscalingV2beta2().HorizontalPodAutoscalers(hpa.GetNamespace()).Delete(context.Background(), hpa.GetName(), metav1.DeleteOptions{})
			}
			if appservice != nil {
				appservice.SetHPA(hpa)
			}

			return
		}
	}
	if sc, ok := obj.(*storagev1.StorageClass); ok {
		vt := workerutil.TransStorageClass2RBDVolumeType(sc)
		for _, ch := range a.volumeTypeListeners {
			select {
			case ch <- vt:
			default:
			}
		}
	}
	if claim, ok := obj.(*corev1.PersistentVolumeClaim); ok {
		serviceID := claim.Labels["service_id"]
		version := claim.Labels["version"]
		createrID := claim.Labels["creater_id"]
		if serviceID != "" && createrID != "" {
			appservice, err := a.getAppService(serviceID, version, createrID, true)
			if err == conversion.ErrServiceNotFound {
				a.conf.KubeClient.CoreV1().PersistentVolumeClaims(claim.Namespace).Delete(context.Background(), claim.Name, metav1.DeleteOptions{})
			}
			if appservice != nil {
				appservice.SetClaim(claim)
				return
			}
		}
	}
	if sm, ok := obj.(*monitorv1.ServiceMonitor); ok {
		serviceID := sm.Labels["service_id"]
		version := sm.Labels["version"]
		createrID := sm.Labels["creater_id"]
		if serviceID != "" && createrID != "" {
			appservice, err := a.getAppService(serviceID, version, createrID, true)
			if err == conversion.ErrServiceNotFound {
				smClient, err := a.GetServiceMonitorClient()
				if err != nil {
					logrus.Errorf("create service monitor client failure %s", err.Error())
				}
				if smClient != nil {
					err := smClient.MonitoringV1().ServiceMonitors(sm.GetNamespace()).Delete(context.Background(), sm.GetName(), metav1.DeleteOptions{})
					if err != nil && !k8sErrors.IsNotFound(err) {
						logrus.Errorf("delete service monitor failure: %s", err.Error())
					}
				}
			}
			if appservice != nil {
				appservice.SetServiceMonitor(sm)
				return
			}
		}
	}
}

//getAppService if  creator is true, will create new app service where not found in store
func (a *appRuntimeStore) getAppService(serviceID, version, createrID string, creator bool) (*v1.AppService, error) {
	var appservice *v1.AppService
	appservice = a.GetAppService(serviceID)
	if appservice == nil && creator {
		var err error
		appservice, err = conversion.InitCacheAppService(a.dbmanager, serviceID, createrID)
		if err != nil {
			logrus.Debugf("init cache app service %s failure:%s ", serviceID, err.Error())
			return nil, err
		}
		a.RegistAppService(appservice)
	}
	return appservice, nil
}
func (a *appRuntimeStore) OnUpdate(oldObj, newObj interface{}) {
	// ingress update maybe change owner component
	if ingress, ok := newObj.(*networkingv1.Ingress); ok {
		oldIngress := oldObj.(*networkingv1.Ingress)
		if oldIngress.Labels["service_id"] != ingress.Labels["service_id"] {
			logrus.Infof("ingress %s change owner component", oldIngress.Name)
			serviceID := oldIngress.Labels["service_id"]
			version := oldIngress.Labels["version"]
			createrID := oldIngress.Labels["creater_id"]
			oldComponent, _ := a.getAppService(serviceID, version, createrID, true)
			if oldComponent != nil {
				oldComponent.DeleteIngress(oldIngress)
			}
		}
	}
	if ingress, ok := newObj.(*betav1.Ingress); ok {
		oldIngress := oldObj.(*betav1.Ingress)
		if oldIngress.Labels["service_id"] != ingress.Labels["service_id"] {
			logrus.Infof("ingress %s change owner component", oldIngress.Name)
			serviceID := oldIngress.Labels["service_id"]
			version := oldIngress.Labels["version"]
			createrID := oldIngress.Labels["creater_id"]
			oldComponent, _ := a.getAppService(serviceID, version, createrID, true)
			if oldComponent != nil {
				oldComponent.DeleteBetaIngress(oldIngress)
			}
		}
	}
	a.OnAdd(newObj)
}
func (a *appRuntimeStore) OnDelete(objs interface{}) {
	a.OnDeletes(objs)
}
func (a *appRuntimeStore) OnDeletes(objs ...interface{}) {
	for i := range objs {
		obj := objs[i]
		if thirdComponent, ok := obj.(*v1alpha1.ThirdComponent); ok {
			serviceID := thirdComponent.Labels["service_id"]
			createrID := thirdComponent.Labels["creater_id"]
			if serviceID != "" && createrID != "" {
				appservice, _ := a.getAppService(serviceID, "", createrID, true)
				if appservice != nil {
					appservice.DeleteWorkload(thirdComponent)
					if appservice.IsClosed() {
						a.DeleteAppService(appservice)
					}
					return
				}
			}
		}
		if deployment, ok := obj.(*appsv1.Deployment); ok {
			serviceID := deployment.Labels["service_id"]
			version := deployment.Labels["version"]
			createrID := deployment.Labels["creater_id"]
			if serviceID != "" && version != "" && createrID != "" {
				appservice, _ := a.getAppService(serviceID, version, createrID, false)
				if appservice != nil {
					appservice.DeleteDeployment(deployment)
					if appservice.IsClosed() {
						a.DeleteAppService(appservice)
					}
					return
				}
			}
		}
		if statefulset, ok := obj.(*appsv1.StatefulSet); ok {
			serviceID := statefulset.Labels["service_id"]
			version := statefulset.Labels["version"]
			createrID := statefulset.Labels["creater_id"]
			if serviceID != "" && version != "" && createrID != "" {
				appservice, _ := a.getAppService(serviceID, version, createrID, false)
				if appservice != nil {
					appservice.DeleteStatefulSet(statefulset)
					if appservice.IsClosed() {
						a.DeleteAppService(appservice)
					}
					return
				}
			}
		}
		if replicaset, ok := obj.(*appsv1.ReplicaSet); ok {
			serviceID := replicaset.Labels["service_id"]
			version := replicaset.Labels["version"]
			createrID := replicaset.Labels["creater_id"]
			if serviceID != "" && version != "" && createrID != "" {
				appservice, _ := a.getAppService(serviceID, version, createrID, false)
				if appservice != nil {
					appservice.DeleteReplicaSet(replicaset)
					if appservice.IsClosed() {
						a.DeleteAppService(appservice)
					}
					return
				}
			}
		}
		if secret, ok := obj.(*corev1.Secret); ok {
			serviceID := secret.Labels["service_id"]
			version := secret.Labels["version"]
			createrID := secret.Labels["creater_id"]
			if serviceID != "" && createrID != "" {
				appservice, _ := a.getAppService(serviceID, version, createrID, false)
				if appservice != nil {
					appservice.DeleteSecrets(secret)
					if appservice.IsClosed() {
						a.DeleteAppService(appservice)
					}
					return
				}
			}
		}
		if service, ok := obj.(*corev1.Service); ok {
			serviceID := service.Labels["service_id"]
			version := service.Labels["version"]
			createrID := service.Labels["creater_id"]
			if serviceID != "" && createrID != "" {
				appservice, _ := a.getAppService(serviceID, version, createrID, false)
				if appservice != nil {
					appservice.DeleteServices(service)
					if appservice.IsClosed() {
						a.DeleteAppService(appservice)
					}
					return
				}
			}
		}
		if ingress, ok := obj.(*networkingv1.Ingress); ok {
			serviceID := ingress.Labels["service_id"]
			version := ingress.Labels["version"]
			createrID := ingress.Labels["creater_id"]
			if serviceID != "" && createrID != "" {
				appservice, _ := a.getAppService(serviceID, version, createrID, false)
				if appservice != nil {
					appservice.DeleteIngress(ingress)
					if appservice.IsClosed() {
						a.DeleteAppService(appservice)
					}
					return
				}
			}
		}
		if configmap, ok := obj.(*corev1.ConfigMap); ok {
			serviceID := configmap.Labels["service_id"]
			version := configmap.Labels["version"]
			createrID := configmap.Labels["creater_id"]
			if serviceID != "" && createrID != "" {
				appservice, _ := a.getAppService(serviceID, version, createrID, false)
				if appservice != nil {
					appservice.DeleteConfigMaps(configmap)
					if appservice.IsClosed() {
						a.DeleteAppService(appservice)
					}
					return
				}
			}
		}
		if hpa, ok := obj.(*autoscalingv2.HorizontalPodAutoscaler); ok {
			serviceID := hpa.Labels["service_id"]
			version := hpa.Labels["version"]
			createrID := hpa.Labels["creater_id"]
			if serviceID != "" && version != "" && createrID != "" {
				appservice, _ := a.getAppService(serviceID, version, createrID, false)
				if appservice != nil {
					appservice.DelHPA(hpa)
					if appservice.IsClosed() {
						a.DeleteAppService(appservice)
					}
					return
				}
			}
		}
		if sc, ok := obj.(*storagev1.StorageClass); ok {
			if err := a.dbmanager.VolumeTypeDao().DeleteModelByVolumeTypes(sc.GetName()); err != nil {
				logrus.Errorf("delete volumeType from db error: %s", err.Error())
				return
			}
		}
		if claim, ok := obj.(*corev1.PersistentVolumeClaim); ok {
			serviceID := claim.Labels["service_id"]
			version := claim.Labels["version"]
			createrID := claim.Labels["creater_id"]
			if serviceID != "" && createrID != "" {
				appservice, _ := a.getAppService(serviceID, version, createrID, false)
				if appservice != nil {
					appservice.DeleteClaim(claim)
					if appservice.IsClosed() {
						a.DeleteAppService(appservice)
					}
					return
				}
			}
		}
		if sm, ok := obj.(*monitorv1.ServiceMonitor); ok {
			serviceID := sm.Labels["service_id"]
			version := sm.Labels["version"]
			createrID := sm.Labels["creater_id"]
			if serviceID != "" && createrID != "" {
				appservice, _ := a.getAppService(serviceID, version, createrID, true)
				if appservice != nil {
					appservice.DeleteServiceMonitor(sm)
					return
				}
			}
		}
	}
}

//RegistAppService regist a app model to store.
func (a *appRuntimeStore) RegistAppService(app *v1.AppService) {
	a.appServices.Store(v1.GetCacheKeyOnlyServiceID(app.ServiceID), app)
	a.appCount++
	logrus.Debugf("current have %d app after add \n", a.appCount)
}

func (a *appRuntimeStore) GetPod(namespace, name string) (*corev1.Pod, error) {
	return a.listers.Pod.Pods(namespace).Get(name)
}

//DeleteAppService delete cache app service
func (a *appRuntimeStore) DeleteAppService(app *v1.AppService) {
	//a.appServices.Delete(v1.GetCacheKeyOnlyServiceID(app.ServiceID))
	//a.appCount--
	//logrus.Debugf("current have %d app after delete \n", a.appCount)
}

//DeleteAppServiceByKey delete cache app service
func (a *appRuntimeStore) DeleteAppServiceByKey(key v1.CacheKey) {
	a.appServices.Delete(key)
	a.appCount--
	logrus.Debugf("current have %d app after delete \n", a.appCount)
}

func (a *appRuntimeStore) GetAppService(serviceID string) *v1.AppService {
	key := v1.GetCacheKeyOnlyServiceID(serviceID)
	app, ok := a.appServices.Load(key)
	if ok {
		appService := app.(*v1.AppService)
		return appService
	}
	return nil
}

func (a *appRuntimeStore) UpdateGetAppService(serviceID string) *v1.AppService {
	key := v1.GetCacheKeyOnlyServiceID(serviceID)
	app, ok := a.appServices.Load(key)
	if ok {
		appService := app.(*v1.AppService)
		if statefulset := appService.GetStatefulSet(); statefulset != nil {
			stateful, err := a.listers.StatefulSet.StatefulSets(statefulset.Namespace).Get(statefulset.Name)
			if err != nil && k8sErrors.IsNotFound(err) {
				appService.DeleteStatefulSet(statefulset)
			}
			if stateful != nil {
				appService.SetStatefulSet(stateful)
			}
		}
		if deployment := appService.GetDeployment(); deployment != nil {
			deploy, err := a.listers.Deployment.Deployments(deployment.Namespace).Get(deployment.Name)
			if err != nil && k8sErrors.IsNotFound(err) {
				appService.DeleteDeployment(deployment)
			}
			if deploy != nil {
				appService.SetDeployment(deploy)
			}
		}
		if services := appService.GetServices(true); services != nil {
			for _, service := range services {
				se, err := a.listers.Service.Services(service.Namespace).Get(service.Name)
				if err != nil && k8sErrors.IsNotFound(err) {
					appService.DeleteServices(service)
				}
				if se != nil {
					appService.SetService(se)
				}
			}
		}
		ingresses, betaIngresses := appService.GetIngress(true)
		for _, ingress := range ingresses {
			in, err := a.listers.Ingress.Ingresses(ingress.Namespace).Get(ingress.Name)
			if err != nil && k8sErrors.IsNotFound(err) {
				appService.DeleteIngress(ingress)
			}
			if in != nil {
				appService.SetIngress(in)
			}
		}
		for _, ingress := range betaIngresses {
			in, err := a.listers.BetaIngress.Ingresses(ingress.Namespace).Get(ingress.Name)
			if err != nil && k8sErrors.IsNotFound(err) {
				appService.DeleteBetaIngress(ingress)
			}
			if in != nil {
				appService.SetIngress(in)
			}
		}

		if secrets := appService.GetSecrets(true); secrets != nil {
			for _, secret := range secrets {
				se, err := a.listers.Secret.Secrets(secret.Namespace).Get(secret.Name)
				if err != nil && k8sErrors.IsNotFound(err) {
					appService.DeleteSecrets(secret)
				}
				if se != nil {
					appService.SetSecret(se)
				}
			}
		}
		if pods := appService.GetPods(true); pods != nil {
			for _, pod := range pods {
				se, err := a.listers.Pod.Pods(pod.Namespace).Get(pod.Name)
				if err != nil && k8sErrors.IsNotFound(err) {
					appService.DeletePods(pod)
				}
				if se != nil {
					appService.SetPods(se)
				}
			}
		}

		return appService
	}
	return nil
}

func (a *appRuntimeStore) GetAllAppServices() (apps []*v1.AppService) {
	a.appServices.Range(func(k, value interface{}) bool {
		appService, _ := value.(*v1.AppService)
		if appService != nil {
			apps = append(apps, appService)
		}
		return true
	})
	return
}

func (a *appRuntimeStore) GetAppServiceStatus(serviceID string) string {
	app := a.GetAppService(serviceID)
	if app == nil {
		component, _ := a.dbmanager.TenantServiceDao().GetServiceByID(serviceID)
		if component == nil {
			return v1.UNKNOW
		}
		if component.Kind == model.ServiceKindThirdParty.String() {
			return v1.CLOSED
		}
		versions, err := a.dbmanager.VersionInfoDao().GetVersionByServiceID(serviceID)
		if (err != nil && err == gorm.ErrRecordNotFound) || len(versions) == 0 {
			return v1.UNDEPLOY
		}
		return v1.CLOSED
	}
	status := app.GetServiceStatus()
	if status == v1.UNKNOW {
		app := a.UpdateGetAppService(serviceID)
		if app == nil {
			versions, err := a.dbmanager.VersionInfoDao().GetVersionByServiceID(serviceID)
			if (err != nil && err == gorm.ErrRecordNotFound) || len(versions) == 0 {
				return v1.UNDEPLOY
			}
			return v1.CLOSED
		}
		return app.GetServiceStatus()
	}
	return status
}

// GetAppServiceStatuses -
func (a *appRuntimeStore) GetAppServiceStatuses(serviceIDs []string) map[string]string {
	statusMap := make(map[string]string, len(serviceIDs))
	var queryComponentIDs []string
	for _, serviceID := range serviceIDs {
		app := a.GetAppService(serviceID)
		if app == nil {
			queryComponentIDs = append(queryComponentIDs, serviceID)
			continue
		}
		status := app.GetServiceStatus()
		if status == v1.UNKNOW {
			app := a.UpdateGetAppService(serviceID)
			if app == nil {
				queryComponentIDs = append(queryComponentIDs, serviceID)
				continue
			}
			statusMap[serviceID] = app.GetServiceStatus()
			continue
		}
		statusMap[serviceID] = status
	}
	components, err := a.dbmanager.TenantServiceDao().GetServiceByIDs(queryComponentIDs)
	if err != nil {
		logrus.Errorf("get components by serviceIDs failed: %s", err.Error())
	}
	versions, err := a.dbmanager.VersionInfoDao().ListVersionsByComponentIDs(queryComponentIDs)
	if err != nil {
		logrus.Errorf("get component versions by serviceIDs failed: %s", err.Error())
	}
	existComponents := make(map[string]*model.TenantServices)
	for _, component := range components {
		existComponents[component.ServiceID] = component
	}
	existVersions := make(map[string]*model.VersionInfo)
	for _, version := range versions {
		existVersions[version.ServiceID] = version
	}
	for _, componentID := range queryComponentIDs {
		if _, ok := existComponents[componentID]; !ok {
			statusMap[componentID] = v1.UNKNOW
			continue
		}
		if existComponents[componentID].Kind == model.ServiceKindThirdParty.String() {
			statusMap[componentID] = v1.CLOSED
			continue
		}
		if _, ok := existVersions[componentID]; !ok {
			statusMap[componentID] = v1.UNDEPLOY
			continue
		}
		statusMap[componentID] = v1.CLOSED
	}
	return statusMap
}

func (a *appRuntimeStore) GetAppServicesStatus(serviceIDs []string) map[string]string {
	statusMap := make(map[string]string, len(serviceIDs))
	if len(serviceIDs) == 0 {
		// When serviceIDs is empty, return the status of all services
		a.appServices.Range(func(k, v interface{}) bool {
			appService, _ := v.(*v1.AppService)
			statusMap[appService.ServiceID] = a.GetAppServiceStatus(appService.ServiceID)
			return true
		})
		return statusMap
	}
	statusMap = a.GetAppServiceStatuses(serviceIDs)
	return statusMap
}

func (a *appRuntimeStore) GetAppStatus(appID string) (pb.AppStatus_Status, error) {
	services, err := db.GetManager().TenantServiceDao().ListByAppID(appID)
	if err != nil || len(services) == 0 {
		return pb.AppStatus_NIL, err
	}
	var serviceIDs []string
	for _, s := range services {
		serviceIDs = append(serviceIDs, s.ServiceID)
	}
	componentStatuses := a.GetAppServicesStatus(serviceIDs)

	return getAppStatus(componentStatuses), nil
}

func getAppStatus(componentStatuses map[string]string) pb.AppStatus_Status {
	var statuses []string
	for _, status := range componentStatuses {
		if status != v1.UNDEPLOY {
			statuses = append(statuses, status)
		}
	}

	appStatus := pb.AppStatus_RUNNING
	switch {
	case len(statuses) == 0 || appNil(statuses):
		appStatus = pb.AppStatus_NIL
	case appClosed(statuses):
		appStatus = pb.AppStatus_CLOSED
	case appAbnormal(statuses):
		appStatus = pb.AppStatus_ABNORMAL
	case appStarting(statuses):
		appStatus = pb.AppStatus_STARTING
	case appStopping(statuses):
		appStatus = pb.AppStatus_STOPPING
	}

	return appStatus
}

func appNil(statuses []string) bool {
	for _, status := range statuses {
		if status != v1.UNDEPLOY {
			return false
		}
	}
	return true
}

func appClosed(statuses []string) bool {
	for _, status := range statuses {
		if status != v1.CLOSED {
			return false
		}
	}
	return true
}

func appAbnormal(statuses []string) bool {
	for _, status := range statuses {
		if status == v1.ABNORMAL || status == v1.SOMEABNORMAL {
			return true
		}
	}
	return false
}

func appStarting(statuses []string) bool {
	for _, status := range statuses {
		if status == v1.STARTING {
			return true
		}
	}
	return false
}

func appStopping(statuses []string) bool {
	stopping := false
	for _, status := range statuses {
		if status == v1.STOPPING {
			stopping = true
			continue
		}
		if status == v1.CLOSED {
			continue
		}
		return false
	}
	return stopping
}

func (a *appRuntimeStore) GetNeedBillingStatus(serviceIDs []string) map[string]string {
	statusMap := make(map[string]string, len(serviceIDs))
	if len(serviceIDs) == 0 {
		a.appServices.Range(func(k, v interface{}) bool {
			appService, _ := v.(*v1.AppService)
			status := a.GetAppServiceStatus(appService.ServiceID)
			if !isClosedStatus(status) {
				statusMap[appService.ServiceID] = status
			}
			return true
		})
	} else {
		for _, serviceID := range serviceIDs {
			status := a.GetAppServiceStatus(serviceID)
			if !isClosedStatus(status) {
				statusMap[serviceID] = status
			}
		}
	}
	return statusMap
}
func isClosedStatus(curStatus string) bool {
	return curStatus == v1.BUILDEFAILURE || curStatus == v1.CLOSED || curStatus == v1.UNDEPLOY || curStatus == v1.BUILDING || curStatus == v1.UNKNOW
}

func getServiceInfoFromPod(pod *corev1.Pod) v1.AbnormalInfo {
	var ai v1.AbnormalInfo
	if len(pod.Spec.Containers) > 0 {
		var i = 0
		container := pod.Spec.Containers[0]
		for _, env := range container.Env {
			if env.Name == "SERVICE_ID" {
				ai.ServiceID = env.Value
				i++
			}
			if env.Name == "SERVICE_NAME" {
				ai.ServiceAlias = env.Value
				i++
			}
			if i == 2 {
				break
			}
		}
	}
	ai.PodName = pod.Name
	ai.TenantID = pod.Namespace
	return ai
}

func (a *appRuntimeStore) analyzePodStatus(pod *corev1.Pod) {
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.LastTerminationState.Terminated != nil {
			ai := getServiceInfoFromPod(pod)
			ai.ContainerName = containerStatus.Name
			ai.Reason = containerStatus.LastTerminationState.Terminated.Reason
			ai.Message = containerStatus.LastTerminationState.Terminated.Message
			ai.CreateTime = time.Now()
			a.addAbnormalInfo(&ai)
		}
	}
}

func (a *appRuntimeStore) addAbnormalInfo(ai *v1.AbnormalInfo) {
	switch ai.Reason {
	case "OOMKilled":
		a.dbmanager.NotificationEventDao().AddModel(&model.NotificationEvent{
			Kind:        "service",
			KindID:      ai.ServiceID,
			Hash:        ai.Hash(),
			Type:        "UnNormal",
			Message:     fmt.Sprintf("Container %s OOMKilled %s", ai.ContainerName, ai.Message),
			Reason:      "OOMKilled",
			Count:       ai.Count,
			ServiceName: ai.ServiceAlias,
			TenantName:  ai.TenantID,
		})
	default:
		db.GetManager().NotificationEventDao().AddModel(&model.NotificationEvent{
			Kind:        "service",
			KindID:      ai.ServiceID,
			Hash:        ai.Hash(),
			Type:        "UnNormal",
			Message:     fmt.Sprintf("Container %s restart %s", ai.ContainerName, ai.Message),
			Reason:      ai.Reason,
			Count:       ai.Count,
			ServiceName: ai.ServiceAlias,
			TenantName:  ai.TenantID,
		})
	}
}

//GetTenantResource get tenant resource
func (a *appRuntimeStore) GetTenantResource(tenantID string) TenantResource {
	return a.resourceCache.GetTenantResource(tenantID)
}

//GetTenantResource get tenant resource
func (a *appRuntimeStore) GetTenantResourceList() []TenantResource {
	return a.resourceCache.GetAllTenantResource()
}

//GetTenantRunningApp get running app by tenant
func (a *appRuntimeStore) GetTenantRunningApp(tenantID string) (list []*v1.AppService) {
	a.appServices.Range(func(k, v interface{}) bool {
		appService, _ := v.(*v1.AppService)
		if appService != nil && (appService.TenantID == tenantID || tenantID == corev1.NamespaceAll || appService.GetNamespace() == tenantID) && !appService.IsClosed() {
			list = append(list, appService)
		}
		return true
	})
	return
}

func (a *appRuntimeStore) podEventHandler() cache.ResourceEventHandlerFuncs {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			pod := obj.(*corev1.Pod)
			a.resourceCache.SetPodResource(pod)
			_, serviceID, version, createrID := k8sutil.ExtractLabels(pod.GetLabels())
			if serviceID != "" && version != "" && createrID != "" {
				appservice, err := a.getAppService(serviceID, version, createrID, true)
				if err == conversion.ErrServiceNotFound {
					a.conf.KubeClient.CoreV1().Pods(pod.Namespace).Delete(context.Background(), pod.Name, metav1.DeleteOptions{})
				}
				if appservice != nil {
					appservice.SetPods(pod)
				}
			}
		},
		DeleteFunc: func(obj interface{}) {
			pod := obj.(*corev1.Pod)
			a.resourceCache.RemovePod(pod)
			_, serviceID, version, createrID := k8sutil.ExtractLabels(pod.GetLabels())
			if serviceID != "" && version != "" && createrID != "" {
				appservice, _ := a.getAppService(serviceID, version, createrID, false)
				if appservice != nil {
					appservice.DeletePods(pod)
					if appservice.IsClosed() {
						a.DeleteAppService(appservice)
					}
				}
			}
		},
		UpdateFunc: func(old, cur interface{}) {
			pod := cur.(*corev1.Pod)
			a.resourceCache.SetPodResource(pod)
			_, serviceID, version, createrID := k8sutil.ExtractLabels(pod.GetLabels())
			if serviceID != "" && version != "" && createrID != "" {
				appservice, err := a.getAppService(serviceID, version, createrID, true)
				if err == conversion.ErrServiceNotFound {
					a.conf.KubeClient.CoreV1().Pods(pod.Namespace).Delete(context.Background(), pod.Name, metav1.DeleteOptions{})
				}
				if appservice != nil {
					appservice.SetPods(pod)
				}
			}
			for _, pech := range a.podUpdateListeners {
				select {
				case pech <- pod:
				default:
				}
			}
		},
	}
}

func (a *appRuntimeStore) evtEventHandler() cache.ResourceEventHandlerFuncs {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			evt := obj.(*corev1.Event)
			recordType, ok := rc2RecordType[evt.InvolvedObject.Kind]
			if !ok {
				return
			}

			serviceID, ruleID := a.scalingRecordServiceAndRuleID(evt)
			if serviceID == "" || ruleID == "" {
				return
			}
			record := &model.TenantServiceScalingRecords{
				ServiceID:   serviceID,
				RuleID:      ruleID,
				EventName:   evt.GetName(),
				RecordType:  recordType,
				Count:       evt.Count,
				Reason:      evt.Reason,
				Description: evt.Message,
				Operator:    "system",
				LastTime:    evt.LastTimestamp.Time,
			}
			logrus.Debugf("received add record: %#v", record)

			if err := db.GetManager().TenantServiceScalingRecordsDao().UpdateOrCreate(record); err != nil {
				logrus.Warningf("update or create scaling record: %v", err)
			}
		},
		UpdateFunc: func(old, cur interface{}) {
			oevt := old.(*corev1.Event)
			cevt := cur.(*corev1.Event)

			recordType, ok := rc2RecordType[cevt.InvolvedObject.Kind]
			if !ok {
				return
			}
			if oevt.ResourceVersion == cevt.ResourceVersion {
				return
			}

			serviceID, ruleID := a.scalingRecordServiceAndRuleID(cevt)
			if serviceID == "" || ruleID == "" {
				return
			}
			record := &model.TenantServiceScalingRecords{
				ServiceID:   serviceID,
				RuleID:      ruleID,
				EventName:   cevt.GetName(),
				RecordType:  recordType,
				Count:       cevt.Count,
				Reason:      cevt.Reason,
				LastTime:    cevt.LastTimestamp.Time,
				Description: cevt.Message,
			}
			logrus.Debugf("received update record: %#v", record)

			if err := db.GetManager().TenantServiceScalingRecordsDao().UpdateOrCreate(record); err != nil {
				logrus.Warningf("update or create scaling record: %v", err)
			}
		},
	}
}

func (a *appRuntimeStore) nsEventHandler() cache.ResourceEventHandlerFuncs {
	return cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(old, cur interface{}) {
			ns := cur.(*corev1.Namespace)

			// check if the namespace is created by Rainbond
			if !filterOutNotRainbondNamespace(ns) {
				return
			}

			if ns.Status.Phase == corev1.NamespaceTerminating {
				return
			}

			if err := a.createOrUpdateImagePullSecret(ns.Name); err != nil {
				logrus.Errorf("create or update imagepullsecret: %v", err)
			}
		},
	}
}

func (a *appRuntimeStore) scalingRecordServiceAndRuleID(evt *corev1.Event) (string, string) {
	var ruleID, serviceID string
	switch evt.InvolvedObject.Kind {
	case "Deployment":
		deploy, err := a.listers.Deployment.Deployments(evt.InvolvedObject.Namespace).Get(evt.InvolvedObject.Name)
		if err != nil {
			logrus.Warningf("retrieve deployment: %v", err)
			return "", ""
		}
		serviceID = deploy.GetLabels()["service_id"]
		ruleID = deploy.GetLabels()["rule_id"]
	case "Statefulset":
		statefulset, err := a.listers.StatefulSet.StatefulSets(evt.InvolvedObject.Namespace).Get(evt.InvolvedObject.Name)
		if err != nil {
			logrus.Warningf("retrieve statefulset: %v", err)
			return "", ""
		}
		serviceID = statefulset.GetLabels()["service_id"]
		ruleID = statefulset.GetLabels()["rule_id"]
	case "HorizontalPodAutoscaler":
		hpa, err := a.listers.HorizontalPodAutoscaler.HorizontalPodAutoscalers(evt.InvolvedObject.Namespace).Get(evt.InvolvedObject.Name)
		if err != nil {
			logrus.Warningf("retrieve statefulset: %v", err)
			return "", ""
		}
		serviceID = hpa.GetLabels()["service_id"]
		ruleID = hpa.GetLabels()["rule_id"]
	default:
		logrus.Warningf("unsupported object kind: %s", evt.InvolvedObject.Kind)
	}

	return serviceID, ruleID
}

func (a *appRuntimeStore) RegistPodUpdateListener(name string, ch chan<- *corev1.Pod) {
	a.podUpdateListenerLock.Lock()
	defer a.podUpdateListenerLock.Unlock()
	a.podUpdateListeners[name] = ch
}

func (a *appRuntimeStore) UnRegistPodUpdateListener(name string) {
	logger := logrus.WithField("WHO", "appRuntimeStore")
	logger.Infof("unregist pod update lisener: %s", name)

	a.podUpdateListenerLock.Lock()
	defer a.podUpdateListenerLock.Unlock()
	delete(a.podUpdateListeners, name)
}

// RegisterVolumeTypeListener -
func (a *appRuntimeStore) RegisterVolumeTypeListener(name string, ch chan<- *model.TenantServiceVolumeType) {
	a.volumeTypeListenerLock.Lock()
	defer a.volumeTypeListenerLock.Unlock()
	a.volumeTypeListeners[name] = ch
}

// UnRegisterVolumeTypeListener -
func (a *appRuntimeStore) UnRegisterVolumeTypeListener(name string) {
	a.volumeTypeListenerLock.Lock()
	defer a.volumeTypeListenerLock.Unlock()
	delete(a.volumeTypeListeners, name)
}

func (a *appRuntimeStore) GetAppResources(appID string) (int64, int64, error) {
	pods, err := a.listPodsByAppIDLegacy(appID)
	if err != nil {
		return 0, 0, err
	}

	var cpu, memory int64
	for _, pod := range pods {
		for _, c := range pod.Spec.Containers {
			cpu += c.Resources.Requests.Cpu().MilliValue()
			memory += c.Resources.Requests.Memory().Value() / 1024 / 1024
		}
	}

	return cpu, memory, nil
}

// Compatible with the old version.
// Versions prior to 5.3.0 have no app_id in the label, only service_id.
func (a *appRuntimeStore) listPodsByAppIDLegacy(appID string) ([]*corev1.Pod, error) {
	services, err := a.dbmanager.TenantServiceDao().ListByAppID(appID)
	if err != nil {
		return nil, err
	}
	var serviceIDs []string
	for _, svc := range services {
		serviceIDs = append(serviceIDs, svc.ServiceID)
	}

	if len(serviceIDs) == 0 {
		return nil, nil
	}

	// list pod based on the given appID
	requirement, err := labels.NewRequirement("service_id", selection.In, serviceIDs)
	if err != nil {
		return nil, err
	}
	selector := labels.NewSelector()
	selector = selector.Add(*requirement)
	return a.listers.Pod.List(selector)
}

func (a *appRuntimeStore) createOrUpdateImagePullSecret(ns string) error {
	imagePullSecretName := os.Getenv(constants.ImagePullSecretKey)
	if imagePullSecretName == "" {
		return nil
	}

	// get secret in namespace rbd-system
	rawSecret, err := a.secretByKey(types.NamespacedName{Namespace: a.conf.RBDNamespace, Name: imagePullSecretName})
	if err != nil {
		return fmt.Errorf("get secret %s: %v",
			types.NamespacedName{Namespace: a.conf.RBDNamespace, Name: imagePullSecretName}.String(), err)
	}
	// get secret in current namespace
	curSecret, err := a.secretByKey(types.NamespacedName{Namespace: ns, Name: imagePullSecretName})
	if err != nil {
		// current secret not exists. create a new one.
		if k8sErrors.IsNotFound(err) {
			curSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      rawSecret.Name,
					Namespace: ns,
				},
				Data: rawSecret.Data,
				Type: rawSecret.Type,
			}
			_, err := a.clientset.CoreV1().Secrets(ns).Create(context.Background(), curSecret, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("create secret for pulling images: %v", err)
			}
			logrus.Infof("successfully create secret: %s", types.NamespacedName{Namespace: ns, Name: imagePullSecretName}.String())
			return nil
		}
		return fmt.Errorf("get secret %s: %v", types.NamespacedName{Namespace: ns, Name: imagePullSecretName}.String(), err)
	}

	// check if the raw secret is different from the current one
	if isImagePullSecretEqual(rawSecret, curSecret) {
		return nil
	}

	// if the raw secret is different from the current one, then update the current one.
	curSecret.Data = rawSecret.Data
	if _, err := a.clientset.CoreV1().Secrets(ns).Update(context.Background(), curSecret, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("update secret for pulling images: %v", err)
	}
	logrus.Infof("successfully update secret: %s", types.NamespacedName{Namespace: ns, Name: imagePullSecretName}.String())
	return nil
}

func (a *appRuntimeStore) secretByKey(key types.NamespacedName) (*corev1.Secret, error) {
	return a.listers.Secret.Secrets(key.Namespace).Get(key.Name)
}

func (a *appRuntimeStore) GetHelmApp(namespace, name string) (*v1alpha1.HelmApp, error) {
	helmApp, err := a.listers.HelmApp.HelmApps(namespace).Get(name)
	if err != nil && !k8sErrors.IsNotFound(err) {
		err = errors.Wrap(bcode.ErrApplicationNotFound, "get helm app")
	}
	return helmApp, err
}

func (a *appRuntimeStore) ListPods(namespace string, selector labels.Selector) ([]*corev1.Pod, error) {
	return a.listers.Pod.Pods(namespace).List(selector)
}

func (a *appRuntimeStore) ListReplicaSets(namespace string, selector labels.Selector) ([]*appsv1.ReplicaSet, error) {
	return a.listers.ReplicaSets.ReplicaSets(namespace).List(selector)
}

func (a *appRuntimeStore) ListServices(namespace string, selector labels.Selector) ([]*corev1.Service, error) {
	return a.listers.Service.Services(namespace).List(selector)
}

func isImagePullSecretEqual(a, b *corev1.Secret) bool {
	if len(a.Data) != len(b.Data) {
		return false
	}
	for key, av := range a.Data {
		bv, ok := b.Data[key]
		if !ok {
			return false
		}
		if string(av) != string(bv) {
			return false
		}
	}
	return true
}

func filterOutNotRainbondNamespace(ns *corev1.Namespace) bool {
	// compatible with pre-5.2 versions
	oldVersion := len(ns.Name) == 32
	curVersion := func() bool {
		if ns.Labels == nil {
			return false
		}
		return ns.Labels["creator"] == "Rainbond" || ns.Labels[constants.ResourceManagedByLabel] == constants.Rainbond
	}()
	return curVersion || oldVersion
}
