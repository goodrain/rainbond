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

package appm

import (
	"github.com/goodrain/rainbond/pkg/db"
	"github.com/goodrain/rainbond/pkg/db/model"
	"github.com/goodrain/rainbond/pkg/util"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

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

//PodCacheManager pod缓存
type PodCacheManager struct {
	caches      map[string]*v1.Pod
	lock        sync.Mutex
	kubeclient  *kubernetes.Clientset
	stop        chan struct{}
	cacheWatchs []*cacheWatch
}

//NewPodCacheManager 创建pod缓存器
func NewPodCacheManager(kubeclient *kubernetes.Clientset) *PodCacheManager {
	m := &PodCacheManager{
		kubeclient: kubeclient,
		stop:       make(chan struct{}),
		caches:     make(map[string]*v1.Pod),
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
	go rcController.Run(m.stop)
	return m
}

func (c *PodCacheManager) addCachePod() func(obj interface{}) {
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

//此处不通过获取部署信息，由于pod创建时可能部署信息未存储
func (c *PodCacheManager) savePod(pod *v1.Pod) error {
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
	if err := db.GetManager().K8sPodDao().AddModel(&model.K8sPod{
		ServiceID:       serviceID,
		ReplicationID:   creater.Reference.Name,
		ReplicationType: strings.ToLower(creater.Reference.Kind),
		PodName:         pod.Name,
	}); err != nil {
		if !strings.HasSuffix(err.Error(), "is exist") {
			logrus.Error("save service pod relation error.", err.Error())
			return err
		}
	}
	//本地调度POD,存储调度信息，下次调度时直接使用
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
func (c *PodCacheManager) getPodCreator(pod *v1.Pod) (*api.SerializedReference, error) {
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
func (c *PodCacheManager) updateCachePod() func(oldObj, newObj interface{}) {
	return func(_, obj interface{}) {
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
				w.send(watch.Event{Type: watch.Modified, Object: pod})
			}
		}
	}
}
func (c *PodCacheManager) deleteCachePod() func(obj interface{}) {
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
func (c *PodCacheManager) Watch(labelSelector string) watch.Interface {
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

//RemoveWatch 移除watch
func (c *PodCacheManager) RemoveWatch(w *cacheWatch) {
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
func (c *PodCacheManager) sendCache(w *cacheWatch) {
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
	m             *PodCacheManager
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
