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
	"sync"
	"time"

	"github.com/jinzhu/gorm"

	"github.com/goodrain/rainbond/db/model"

	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/goodrain/rainbond/cmd/worker/option"

	"github.com/Sirupsen/logrus"

	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/worker/appm/conversion"

	"github.com/goodrain/rainbond/worker/appm/types/v1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	extensions "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	listcorev1 "k8s.io/client-go/listers/core/v1"
)

//Storer app runtime store interface
type Storer interface {
	Start() error
	Ready() bool
	RegistAppService(*v1.AppService)
	GetAppService(serviceID string) *v1.AppService
	UpdateGetAppService(serviceID string) *v1.AppService
	GetAllAppServices() []*v1.AppService
	GetAppServiceStatus(serviceID string) string
	GetAppServicesStatus(serviceIDs []string) map[string]string
	GetNeedBillingStatus(serviceIDs []string) map[string]string
	OnDelete(obj interface{})
	GetPodLister() listcorev1.PodLister
}

//appRuntimeStore app runtime store
//cache all kubernetes object and appservice
type appRuntimeStore struct {
	ctx         context.Context
	cancel      context.CancelFunc
	informers   *Informer
	listers     *Lister
	appServices sync.Map
	appCount    int32
	dbmanager   db.Manager
	stopch      chan struct{}
	conf        option.Config
}

//NewStore new app runtime store
func NewStore(dbmanager db.Manager, conf option.Config) Storer {
	ctx, cancel := context.WithCancel(context.Background())
	store := &appRuntimeStore{
		ctx:         ctx,
		cancel:      cancel,
		informers:   &Informer{},
		listers:     &Lister{},
		appServices: sync.Map{},
		conf:        conf,
		dbmanager:   dbmanager,
	}
	// create informers factory, enable and assign required informers
	infFactory := informers.NewFilteredSharedInformerFactory(conf.KubeClient, time.Second, corev1.NamespaceAll,
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

	store.informers.ReplicaSet = infFactory.Apps().V1().ReplicaSets().Informer()

	store.informers.Deployment.AddEventHandlerWithResyncPeriod(store, time.Second*10)
	store.informers.StatefulSet.AddEventHandlerWithResyncPeriod(store, time.Second*10)
	store.informers.Pod.AddEventHandlerWithResyncPeriod(store, time.Second*10)
	store.informers.Secret.AddEventHandlerWithResyncPeriod(store, time.Second*10)
	store.informers.Service.AddEventHandlerWithResyncPeriod(store, time.Second*10)
	store.informers.Ingress.AddEventHandlerWithResyncPeriod(store, time.Second*10)
	store.informers.ConfigMap.AddEventHandlerWithResyncPeriod(store, time.Second*10)
	store.informers.ReplicaSet.AddEventHandlerWithResyncPeriod(store, time.Second*10)
	return store
}

func (a *appRuntimeStore) init() error {
	//init leader namespace
	leaderNamespace := a.conf.LeaderElectionNamespace
	if _, err := a.conf.KubeClient.CoreV1().Namespaces().Get(leaderNamespace, metav1.GetOptions{}); err != nil {
		if errors.IsNotFound(err) {
			_, err = a.conf.KubeClient.CoreV1().Namespaces().Create(&corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: leaderNamespace,
				},
			})
		}
		if err != nil {
			return err
		}
	}
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
				if err := a.conf.KubeClient.Apps().ReplicaSets(rs.Namespace).Delete(rs.Name, &metav1.DeleteOptions{}); err != nil && errors.IsNotFound(err) {
					logrus.Errorf("delete old version replicaset failure %s", err.Error())
				}
			}
		}
	}
}
func (a *appRuntimeStore) OnAdd(obj interface{}) {
	if deployment, ok := obj.(*appsv1.Deployment); ok {
		serviceID := deployment.Labels["service_id"]
		version := deployment.Labels["version"]
		createrID := deployment.Labels["creater_id"]
		if serviceID != "" && version != "" && createrID != "" {
			appservice, err := a.getAppService(serviceID, version, createrID, true)
			if err == conversion.ErrorNotFoundService {
				a.conf.KubeClient.AppsV1().Deployments(deployment.Namespace).Delete(deployment.Name, &metav1.DeleteOptions{})
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
			if err == conversion.ErrorNotFoundService {
				a.conf.KubeClient.AppsV1().StatefulSets(statefulset.Namespace).Delete(statefulset.Name, &metav1.DeleteOptions{})
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
			if err == conversion.ErrorNotFoundService {
				a.conf.KubeClient.AppsV1().Deployments(replicaset.Namespace).Delete(replicaset.Name, &metav1.DeleteOptions{})
			}
			if appservice != nil {
				appservice.SetReplicaSets(replicaset)
				a.checkReplicasetWhetherDelete(appservice, replicaset)
				return
			}
		}
	}
	if pod, ok := obj.(*corev1.Pod); ok {
		serviceID := pod.Labels["service_id"]
		version := pod.Labels["version"]
		createrID := pod.Labels["creater_id"]
		if serviceID != "" && version != "" && createrID != "" {
			appservice, err := a.getAppService(serviceID, version, createrID, true)
			if err == conversion.ErrorNotFoundService {
				a.conf.KubeClient.CoreV1().Pods(pod.Namespace).Delete(pod.Name, &metav1.DeleteOptions{})
			}
			if appservice != nil {
				appservice.SetPods(pod)
				a.analyzePodStatus(pod)
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
			if err == conversion.ErrorNotFoundService {
				a.conf.KubeClient.CoreV1().Secrets(secret.Namespace).Delete(secret.Name, &metav1.DeleteOptions{})
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
			if err == conversion.ErrorNotFoundService {
				a.conf.KubeClient.CoreV1().Services(service.Namespace).Delete(service.Name, &metav1.DeleteOptions{})
			}
			if appservice != nil {
				appservice.SetService(service)
				return
			}
		}
	}
	if ingress, ok := obj.(*extensions.Ingress); ok {
		serviceID := ingress.Labels["service_id"]
		version := ingress.Labels["version"]
		createrID := ingress.Labels["creater_id"]
		if serviceID != "" && createrID != "" {
			appservice, err := a.getAppService(serviceID, version, createrID, true)
			if err == conversion.ErrorNotFoundService {
				a.conf.KubeClient.Extensions().Ingresses(ingress.Namespace).Delete(ingress.Name, &metav1.DeleteOptions{})
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
			if err == conversion.ErrorNotFoundService {
				a.conf.KubeClient.CoreV1().ConfigMaps(configmap.Namespace).Delete(configmap.Name, &metav1.DeleteOptions{})
			}
			if appservice != nil {
				appservice.SetConfigMap(configmap)
				return
			}
		}
	}
}

//getAppService if  creater is true, will create new app service where not found in store
func (a *appRuntimeStore) getAppService(serviceID, version, createrID string, creater bool) (*v1.AppService, error) {
	var appservice *v1.AppService
	appservice = a.GetAppService(serviceID)
	if appservice == nil && creater {
		var err error
		appservice, err = conversion.InitCacheAppService(a.dbmanager, serviceID, version, createrID)
		if err != nil {
			logrus.Errorf("init cache app service failure:%s", err.Error())
			return nil, err
		}
		a.RegistAppService(appservice)
	}
	return appservice, nil
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
	if pod, ok := obj.(*corev1.Pod); ok {
		serviceID := pod.Labels["service_id"]
		version := pod.Labels["version"]
		createrID := pod.Labels["creater_id"]
		if serviceID != "" && version != "" && createrID != "" {
			appservice, _ := a.getAppService(serviceID, version, createrID, false)
			if appservice != nil {
				appservice.DeletePods(pod)
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
	if ingress, ok := obj.(*extensions.Ingress); ok {
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
}

//RegistAppService regist a app model to store.
func (a *appRuntimeStore) RegistAppService(app *v1.AppService) {
	a.appServices.Store(v1.GetCacheKeyOnlyServiceID(app.ServiceID), app)
	a.appCount++
	logrus.Debugf("current have %d app after add \n", a.appCount)
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
			if err != nil && errors.IsNotFound(err) {
				appService.DeleteStatefulSet(statefulset)
			}
			if stateful != nil {
				appService.SetStatefulSet(stateful)
			}
		}
		if deployment := appService.GetDeployment(); deployment != nil {
			deploy, err := a.listers.Deployment.Deployments(deployment.Namespace).Get(deployment.Name)
			if err != nil && errors.IsNotFound(err) {
				appService.DeleteDeployment(deployment)
			}
			if deploy != nil {
				appService.SetDeployment(deploy)
			}
		}
		if services := appService.GetServices(); services != nil {
			for _, service := range services {
				se, err := a.listers.Service.Services(service.Namespace).Get(service.Name)
				if err != nil && errors.IsNotFound(err) {
					appService.DeleteServices(service)
				}
				if se != nil {
					appService.SetService(se)
				}
			}
		}
		if ingresses := appService.GetIngress(); ingresses != nil {
			for _, ingress := range ingresses {
				in, err := a.listers.Ingress.Ingresses(ingress.Namespace).Get(ingress.Name)
				if err != nil && errors.IsNotFound(err) {
					appService.DeleteIngress(ingress)
				}
				if in != nil {
					appService.SetIngress(in)
				}
			}
		}
		if secrets := appService.GetSecrets(); secrets != nil {
			for _, secret := range secrets {
				se, err := a.listers.Secret.Secrets(secret.Namespace).Get(secret.Name)
				if err != nil && errors.IsNotFound(err) {
					appService.DeleteSecrets(secret)
				}
				if se != nil {
					appService.SetSecret(se)
				}
			}
		}
		if pods := appService.GetPods(); pods != nil {
			for _, pod := range pods {
				se, err := a.listers.Pod.Pods(pod.Namespace).Get(pod.Name)
				if err != nil && errors.IsNotFound(err) {
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

func (a *appRuntimeStore) GetAppServicesStatus(serviceIDs []string) map[string]string {
	statusMap := make(map[string]string, len(serviceIDs))
	if serviceIDs == nil || len(serviceIDs) == 0 {
		a.appServices.Range(func(k, v interface{}) bool {
			appService, _ := v.(*v1.AppService)
			statusMap[appService.ServiceID] = a.GetAppServiceStatus(appService.ServiceID)
			return true
		})
	}
	for _, serviceID := range serviceIDs {
		statusMap[serviceID] = a.GetAppServiceStatus(serviceID)
	}
	return statusMap
}

func (a *appRuntimeStore) GetNeedBillingStatus(serviceIDs []string) map[string]string {
	statusMap := make(map[string]string, len(serviceIDs))
	if serviceIDs == nil || len(serviceIDs) == 0 {
		a.appServices.Range(func(k, v interface{}) bool {
			appService, _ := v.(*v1.AppService)
			status := a.GetAppServiceStatus(appService.ServiceID)
			if isClosedStatus(status) {
				statusMap[appService.ServiceID] = status
			}
			return true
		})
	} else {
		for _, serviceID := range serviceIDs {
			status := a.GetAppServiceStatus(serviceID)
			if isClosedStatus(status) {
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
func (a *appRuntimeStore) GetPodLister() listcorev1.PodLister {
	return a.listers.Pod
}
