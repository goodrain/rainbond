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
	"sync"
	"time"

	"github.com/Sirupsen/logrus"

	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/worker/appm/conversion"

	v1 "github.com/goodrain/rainbond/worker/appm/types/v1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	extensions "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
)

//Storer app runtime store interface
type Storer interface {
	RegistAppService(*v1.AppService)
	GetAppService(serviceID, version, createrID string) *v1.AppService
	GetAppServiceWithoutCreaterID(serviceID, version string) *v1.AppService
}

//appRuntimeStore app runtime store
//cache all kubernetes object and appservice
type appRuntimeStore struct {
	informers   *Informer
	listers     *Lister
	appServices sync.Map
	dbmanager   db.Manager
}

//NewStore new app runtime store
func NewStore(client kubernetes.Interface, dbmanager db.Manager) Storer {
	store := &appRuntimeStore{
		informers:   &Informer{},
		appServices: sync.Map{},
	}
	// create informers factory, enable and assign required informers
	infFactory := informers.NewFilteredSharedInformerFactory(client, time.Second, corev1.NamespaceAll,
		func(options *metav1.ListOptions) {
			options.LabelSelector = "creater=Rainbond"
		})
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

	store.informers.Ingress = infFactory.Extensions().V1beta1().Ingresses().Informer()
	store.listers.Ingress = infFactory.Extensions().V1beta1().Ingresses().Lister()

	store.informers.Deployment.AddEventHandler(store)
	store.informers.StatefulSet.AddEventHandler(store)
	store.informers.Pod.AddEventHandler(store)
	store.informers.Secret.AddEventHandler(store)
	store.informers.Service.AddEventHandler(store)
	store.informers.Ingress.AddEventHandler(store)
	store.informers.ConfigMap.AddEventHandler(store)
	return store
}

func (a *appRuntimeStore) OnAdd(obj interface{}) {
	if deployment, ok := obj.(*appsv1.Deployment); ok {
		serviceID := deployment.Labels["service_id"]
		version := deployment.Labels["version"]
		createrID := deployment.Labels["creater_id"]
		if serviceID != "" && version != "" && createrID != "" {
			appservice := a.getAppService(serviceID, version, createrID)
			if appservice != nil {
				appservice.SetDeployment(deployment)
			}
		}
	}
	if statefulset, ok := obj.(*appsv1.StatefulSet); ok {
		serviceID := statefulset.Labels["service_id"]
		version := statefulset.Labels["version"]
		createrID := statefulset.Labels["creater_id"]
		if serviceID != "" && version != "" && createrID != "" {
			appservice := a.getAppService(serviceID, version, createrID)
			if appservice != nil {
				appservice.SetStatefulSet(statefulset)
			}
		}
	}
	if pod, ok := obj.(*corev1.Pod); ok {
		serviceID := pod.Labels["service_id"]
		version := pod.Labels["version"]
		createrID := pod.Labels["creater_id"]
		if serviceID != "" && version != "" && createrID != "" {
			appservice := a.getAppService(serviceID, version, createrID)
			if appservice != nil {
				appservice.SetPods(pod)
			}
		}
	}
	if secret, ok := obj.(*corev1.Secret); ok {
		serviceID := secret.Labels["service_id"]
		version := secret.Labels["version"]
		createrID := secret.Labels["creater_id"]
		if serviceID != "" && version != "" && createrID != "" {
			appservice := a.getAppService(serviceID, version, createrID)
			if appservice != nil {
				appservice.SetSecrets(secret)
			}
		}
	}
	if service, ok := obj.(*corev1.Service); ok {
		serviceID := service.Labels["service_id"]
		version := service.Labels["version"]
		createrID := service.Labels["creater_id"]
		if serviceID != "" && version != "" && createrID != "" {
			appservice := a.getAppService(serviceID, version, createrID)
			if appservice != nil {
				appservice.SetService(service)
			}
		}
	}
	if ingress, ok := obj.(*extensions.Ingress); ok {
		serviceID := ingress.Labels["service_id"]
		version := ingress.Labels["version"]
		createrID := ingress.Labels["creater_id"]
		if serviceID != "" && version != "" && createrID != "" {
			appservice := a.getAppService(serviceID, version, createrID)
			if appservice != nil {
				appservice.SetIngress(ingress)
			}
		}
	}
	if configmap, ok := obj.(*corev1.ConfigMap); ok {
		serviceID, oks := configmap.Labels["service_id"]
		version, okv := configmap.Labels["version"]
		createrID := configmap.Labels["creater_id"]
		if oks && okv && serviceID != "" && version != "" && createrID != "" {
			appservice := a.getAppService(serviceID, version, createrID)
			if appservice != nil {
				appservice.SetConfigMap(configmap)
			}
		}
	}
}
func (a *appRuntimeStore) getAppService(serviceID, version, createrID string) *v1.AppService {
	var appservice *v1.AppService
	appservice = a.GetAppService(serviceID, version, createrID)
	if appservice == nil {
		var err error
		appservice, err = conversion.InitCacheAppService(a.dbmanager, serviceID)
		if err != nil {
			logrus.Errorf("init cache app service failure:%s", err.Error())
			return nil
		}
		a.RegistAppService(appservice)
	}
	return appservice
}
func (a *appRuntimeStore) OnUpdate(oldObj, newObj interface{}) {
	a.OnAdd(newObj)
}
func (a *appRuntimeStore) OnDelete(obj interface{}) {
	if deployment, ok := obj.(*appsv1.Deployment); ok {
		serviceID := deployment.Labels["service_id"]
		version := deployment.Labels["version"]
		createrID := deployment.Labels["creater_id"]
		if serviceID != "" && version != "" && createrID != "" {
			appservice := a.getAppService(serviceID, version, createrID)
			if appservice != nil {
				appservice.DeleteDeployment(deployment)
			}
		}
	}
	if statefulset, ok := obj.(*appsv1.StatefulSet); ok {
		serviceID := statefulset.Labels["service_id"]
		version := statefulset.Labels["version"]
		createrID := statefulset.Labels["creater_id"]
		if serviceID != "" && version != "" && createrID != "" {
			appservice := a.getAppService(serviceID, version, createrID)
			if appservice != nil {
				appservice.DeleteStatefulSet(statefulset)
			}
		}
	}
	if pod, ok := obj.(*corev1.Pod); ok {
		serviceID := pod.Labels["service_id"]
		version := pod.Labels["version"]
		createrID := pod.Labels["creater_id"]
		if serviceID != "" && version != "" && createrID != "" {
			appservice := a.getAppService(serviceID, version, createrID)
			if appservice != nil {
				appservice.DeletePods(pod)
			}
		}
	}
	if secret, ok := obj.(*corev1.Secret); ok {
		serviceID := secret.Labels["service_id"]
		version := secret.Labels["version"]
		createrID := secret.Labels["creater_id"]
		if serviceID != "" && version != "" && createrID != "" {
			appservice := a.getAppService(serviceID, version, createrID)
			if appservice != nil {
				appservice.DeleteSecrets(secret)
			}
		}
	}
	if service, ok := obj.(*corev1.Service); ok {
		serviceID := service.Labels["service_id"]
		version := service.Labels["version"]
		createrID := service.Labels["creater_id"]
		if serviceID != "" && version != "" && createrID != "" {
			appservice := a.getAppService(serviceID, version, createrID)
			if appservice != nil {
				appservice.DeleteServices(service)
			}
		}
	}
	if ingress, ok := obj.(*extensions.Ingress); ok {
		serviceID := ingress.Labels["service_id"]
		version := ingress.Labels["version"]
		createrID := ingress.Labels["creater_id"]
		if serviceID != "" && version != "" && createrID != "" {
			appservice := a.getAppService(serviceID, version, createrID)
			if appservice != nil {
				appservice.DeleteIngress(ingress)
			}
		}
	}
	if configmap, ok := obj.(*corev1.ConfigMap); ok {
		serviceID := configmap.Labels["service_id"]
		version := configmap.Labels["version"]
		createrID := configmap.Labels["creater_id"]
		if serviceID != "" && version != "" && createrID != "" {
			appservice := a.getAppService(serviceID, version, createrID)
			if appservice != nil {
				appservice.DeleteConfigMaps(configmap)
			}
		}
	}
}

//RegistAppService regist a app model to store.
func (a *appRuntimeStore) RegistAppService(app *v1.AppService) {
	a.appServices.Store(v1.GetCacheKey(app.ServiceID, app.DeployVersion, app.CreaterID), app)
}
func (a *appRuntimeStore) GetAppService(serviceID, version, createrID string) *v1.AppService {
	key := v1.GetCacheKey(serviceID, version, createrID)
	app, _ := a.appServices.Load(key)
	appService := app.(*v1.AppService)
	return appService
}

func (a *appRuntimeStore) GetAppServiceWithoutCreaterID(serviceID, version string) *v1.AppService {
	key := v1.GetNoCreaterCacheKey(serviceID, version)
	app, _ := a.appServices.Load(key)
	appService := app.(*v1.AppService)
	return appService
}
