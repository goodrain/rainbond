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

package pod

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/util"

	"github.com/pquerna/ffjson/ffjson"

	"github.com/Sirupsen/logrus"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/tools/cache"
)

//CacheManager pod cache manager
type CacheManager struct {
	caches       map[string]*v1.Pod
	lock         sync.Mutex
	kubeclient   *kubernetes.Clientset
	stop         chan struct{}
	cacheWatchs  []*cacheWatch
	oomInfos     map[string]*AbnormalInfo
	errorInfos   map[string]*AbnormalInfo
	rcController cache.Controller
}

//AbnormalInfo pod Abnormal info
//Record the container exception exit information in pod.
type AbnormalInfo struct {
	ServiceID     string    `json:"service_id"`
	ServiceAlias  string    `json:"service_alias"`
	PodName       string    `json:"pod_name"`
	ContainerName string    `json:"container_name"`
	Reason        string    `json:"reson"`
	Message       string    `json:"message"`
	CreateTime    time.Time `json:"create_time"`
	Count         int       `json:"count"`
}

//Hash get AbnormalInfo hash
func (a AbnormalInfo) Hash() string {
	hash := sha256.New()
	hash.Write([]byte(a.ServiceID + a.ServiceAlias + a.PodName + a.ContainerName))
	return fmt.Sprintf("%x", hash.Sum(nil))
}
func (a AbnormalInfo) String() string {
	return fmt.Sprintf("ServiceID: %s;ServiceAlias: %s;PodName: %s ; ContainerName: %s; Reason: %s; Message: %s",
		a.ServiceID, a.ServiceAlias, a.PodName, a.ContainerName, a.Reason, a.Message)
}

//NewCacheManager create pod cache manager and start it
func NewCacheManager(kubeclient *kubernetes.Clientset) *CacheManager {
	m := &CacheManager{
		kubeclient: kubeclient,
		stop:       make(chan struct{}),
		caches:     make(map[string]*v1.Pod),
		oomInfos:   make(map[string]*AbnormalInfo),
		errorInfos: make(map[string]*AbnormalInfo),
	}
	lw := NewListWatchPodFromClient(kubeclient.Core().RESTClient())
	_, rcController := cache.NewInformer(
		lw,
		&v1.Pod{},
		15*time.Minute,
		cache.ResourceEventHandlerFuncs{
			AddFunc:    m.addCachePod(),
			UpdateFunc: m.updateCachePod(),
			DeleteFunc: m.deleteCachePod(),
		},
	)
	m.rcController = rcController
	return m
}

//Start start watch pod event
func (c *CacheManager) Start() {
	logrus.Info("pod source watching started...")
	go c.rcController.Run(c.stop)
}

//Stop stop
func (c *CacheManager) Stop() {
	close(c.stop)
}

func (c *CacheManager) addCachePod() func(obj interface{}) {
	return func(obj interface{}) {
		pod, ok := obj.(*v1.Pod)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("cannot convert to *v1.Pod: %v", obj))
			return
		}
		c.lock.Lock()
		defer c.lock.Unlock()
		if err := c.savePod(pod); err != nil {
			logrus.Errorf("save or update pod error :%s", err.Error())
		}
		for _, w := range c.cacheWatchs {
			if w.satisfied(pod) {
				w.send(watch.Event{Type: watch.Added, Object: pod})
			}
		}
	}
}

// savePod save pod info to db
// From pod basic information.
func (c *CacheManager) savePod(pod *v1.Pod) error {
	creater, err := c.getPodCreator(pod)
	if err != nil {
		logrus.Error("add cache pod error:", err.Error())
		return err
	}
	var serviceID string
loop:
	for _, c := range pod.Spec.Containers {
		for _, env := range c.Env {
			if env.Name == "SERVICE_ID" {
				serviceID = env.Value
				break loop
			}
		}
	}
	if serviceID == "" {
		logrus.Warningf("Pod (%s) can not found SERVICE_ID", pod.Name)
		return nil
	}
	dbPod := &model.K8sPod{
		ServiceID:       serviceID,
		ReplicationID:   creater.Reference.Name,
		ReplicationType: strings.ToLower(creater.Reference.Kind),
		PodName:         pod.Name,
		PodIP:           pod.Status.PodIP,
	}
	dbPod.CreatedAt = time.Now()
	if pod.Status.StartTime != nil && !pod.Status.StartTime.IsZero() {
		dbPod.CreatedAt = pod.Status.StartTime.Time
	}
	if err := db.GetManager().K8sPodDao().AddModel(dbPod); err != nil {
		if !strings.HasSuffix(err.Error(), "is exist") {
			logrus.Error("save service pod relation error.", err.Error())
			return err
		}
	}
	//local scheduler pod host ip save
	if v, ok := pod.Labels["local-scheduler"]; ok && v == "true" {
		if pod.Status.HostIP != "" {
			if err := db.GetManager().LocalSchedulerDao().AddModel(&model.LocalScheduler{
				ServiceID: serviceID,
				PodName:   pod.Name,
				NodeIP:    pod.Status.HostIP,
			}); err != nil {
				if !strings.HasSuffix(err.Error(), "is exist") {
					logrus.Error("save local scheduler info error.", err.Error())
					return err
				}
			}
		}
	}
	return nil
}
func (c *CacheManager) getPodCreator(pod *v1.Pod) (*api.SerializedReference, error) {
	creatorRef, found := pod.ObjectMeta.Annotations[api.CreatedByAnnotation]
	if !found {
		return nil, fmt.Errorf("not found pod creator name")
	}
	sr := &api.SerializedReference{}
	err := ffjson.Unmarshal([]byte(creatorRef), sr)
	if err != nil {
		return nil, err
	}
	return sr, nil
}
func (c *CacheManager) updateCachePod() func(oldObj, newObj interface{}) {
	return func(_, obj interface{}) {
		pod, ok := obj.(*v1.Pod)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("cannot convert to *v1.Pod: %v", obj))
			return
		}
		//Analyze the cause of pod update.
		c.analyzePodStatus(pod)
		c.lock.Lock()
		defer c.lock.Unlock()
		if err := c.savePod(pod); err != nil {
			logrus.Errorf("save or update pod error :%s", err.Error())
		}
		for _, w := range c.cacheWatchs {
			if w.satisfied(pod) {
				w.send(watch.Event{Type: watch.Modified, Object: pod})
			}
		}
	}
}
func getServiceInfoFromPod(pod *v1.Pod) AbnormalInfo {
	var ai AbnormalInfo
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
	return ai
}
func (c *CacheManager) analyzePodStatus(pod *v1.Pod) {
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.LastTerminationState.Terminated != nil {
			ai := getServiceInfoFromPod(pod)
			ai.ContainerName = containerStatus.Name
			ai.Reason = containerStatus.LastTerminationState.Terminated.Reason
			ai.Message = containerStatus.LastTerminationState.Terminated.Message
			ai.CreateTime = time.Now()
			c.addAbnormalInfo(&ai)
		}
	}
}

func (c *CacheManager) addAbnormalInfo(ai *AbnormalInfo) {
	c.lock.Lock()
	defer c.lock.Unlock()
	switch ai.Reason {
	case "OOMKilled":
		if oldai, ok := c.oomInfos[ai.Hash()]; ok {
			oldai.Count++
		} else {
			ai.Count++
			c.oomInfos[ai.Hash()] = ai
		}
		db.GetManager().NotificationEventDao().AddModel(&model.NotificationEvent{
			Kind:        "service",
			KindID:      c.oomInfos[ai.Hash()].ServiceID,
			Hash:        ai.Hash(),
			Type:        "UnNormal",
			Message:     c.oomInfos[ai.Hash()].Message,
			Reason:      "OOMKilled",
			Count:       c.oomInfos[ai.Hash()].Count,
		})
	default:
		if oldai, ok := c.errorInfos[ai.Hash()]; ok && oldai != nil {
			oldai.Count++
		} else {
			ai.Count++
			c.errorInfos[ai.Hash()] = ai
		}
		db.GetManager().NotificationEventDao().AddModel(&model.NotificationEvent{
			Kind:        "service",
			KindID:      c.errorInfos[ai.Hash()].ServiceID,
			Hash:        ai.Hash(),
			Type:        "UnNormal",
			Message:     c.errorInfos[ai.Hash()].Message,
			Reason:      c.errorInfos[ai.Hash()].Reason,
			Count:       c.errorInfos[ai.Hash()].Count,
		})
	}

}
func (c *CacheManager) deleteCachePod() func(obj interface{}) {
	return func(obj interface{}) {
		pod, ok := obj.(*v1.Pod)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("cannot convert to *v1.Pod: %v", obj))
			return
		}
		if err := db.GetManager().K8sPodDao().DeleteK8sPodByName(pod.Name); err != nil {
			logrus.Error("delete service pod relation error.", err.Error())
		}
		c.lock.Lock()
		defer c.lock.Unlock()
		if _, ok := c.caches[pod.Name]; ok {
			delete(c.caches, pod.Name)
		}
		for _, w := range c.cacheWatchs {
			if w.satisfied(pod) {
				w.send(watch.Event{Type: watch.Deleted, Object: pod})
			}
		}
	}
}

//Watch pod cache watch
func (c *CacheManager) Watch(labelSelector string) watch.Interface {
	lbs := strings.Split(labelSelector, ",")
	sort.Strings(lbs)
	labelSelector = strings.Join(lbs, ",")
	w := &cacheWatch{
		id:            util.NewUUID(),
		ch:            make(chan watch.Event, 3),
		labelSelector: labelSelector,
		m:             c,
	}
	c.cacheWatchs = append(c.cacheWatchs, w)
	go c.sendCache(w)
	return w
}

//RemoveWatch remove watch
func (c *CacheManager) RemoveWatch(w *cacheWatch) {
	if c.cacheWatchs == nil {
		return
	}
	for i := range c.cacheWatchs {
		if c.cacheWatchs[i].id == w.id {
			c.cacheWatchs = append(c.cacheWatchs[:i], c.cacheWatchs[i+1:]...)
			return
		}
	}
}
func (c *CacheManager) sendCache(w *cacheWatch) {
	c.lock.Lock()
	defer c.lock.Unlock()
	for _, pod := range c.caches {
		if w.satisfied(pod) {
			w.send(watch.Event{Type: watch.Added, Object: pod})
		}
	}
}

type cacheWatch struct {
	id            string
	ch            chan watch.Event
	labelSelector string
	m             *CacheManager
}

func (c *cacheWatch) satisfied(pod *v1.Pod) bool {
	var lbs []string
	for k, v := range pod.Labels {
		lbs = append(lbs, fmt.Sprintf("%s=%s", k, v))
	}
	sort.Strings(lbs)
	lbStr := strings.Join(lbs, ",")
	return strings.Contains(lbStr, c.labelSelector)
}

func (c *cacheWatch) Stop() {
	close(c.ch)
	c.m.RemoveWatch(c)
}
func (c *cacheWatch) send(event watch.Event) {
	select {
	case c.ch <- event:
	default:
	}
}
func (c *cacheWatch) ResultChan() <-chan watch.Event {
	return c.ch
}

// NewListWatchPodFromClient creates a new ListWatch from the specified client, resource, namespace and field selector.
func NewListWatchPodFromClient(c cache.Getter) *cache.ListWatch {
	listFunc := func(options metav1.ListOptions) (runtime.Object, error) {
		return c.Get().
			Namespace(v1.NamespaceAll).
			Resource("pods").
			VersionedParams(&options, metav1.ParameterCodec).
			Do().
			Get()
	}
	watchFunc := func(options metav1.ListOptions) (watch.Interface, error) {
		options.Watch = true
		return c.Get().
			Namespace(v1.NamespaceAll).
			Resource("pods").
			VersionedParams(&options, metav1.ParameterCodec).
			Watch()
	}
	return &cache.ListWatch{ListFunc: listFunc, WatchFunc: watchFunc}
}
