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

package config

import (
	"fmt"
	"time"

	"github.com/Sirupsen/logrus"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
)

// NewSourceAPI creates config source that watches for changes to the services and pods.
func NewSourceAPI(c cache.Getter, period time.Duration, servicesChan chan<- ServiceUpdate, podsChan chan<- PodUpdate, stopCh <-chan struct{}) {
	selecter, err := labels.Parse("service_type=outer")
	if err != nil {
		logrus.Error("create label selector error.", err.Error())
		utilruntime.HandleError(err)
		return
	}
	optionsModifier := func(options *metav1.ListOptions) {
		options.FieldSelector = fields.Everything().String()
		options.LabelSelector = selecter.String()
	}
	servicesLW := cache.NewFilteredListWatchFromClient(c, "services", v1.NamespaceAll, optionsModifier)
	podsLW := cache.NewFilteredListWatchFromClient(c, "pods", v1.NamespaceAll, optionsModifier)
	logrus.Debug("Start new source api for pod and service")
	newSourceAPI(servicesLW, podsLW, period, servicesChan, podsChan, stopCh)
}

func newSourceAPI(
	servicesLW cache.ListerWatcher,
	podsLW cache.ListerWatcher,
	period time.Duration,
	servicesChan chan<- ServiceUpdate,
	podsChan chan<- PodUpdate,
	stopCh <-chan struct{}) {
	serviceController := NewServiceController(servicesLW, period, servicesChan)
	go serviceController.Run(stopCh)

	podsController := NewPodsController(podsLW, period, podsChan)
	go podsController.Run(stopCh)

	if !cache.WaitForCacheSync(stopCh, serviceController.HasSynced, podsController.HasSynced) {
		utilruntime.HandleError(fmt.Errorf("source controllers not synced"))
		return
	}
	servicesChan <- ServiceUpdate{Op: SYNCED}
	podsChan <- PodUpdate{Op: SYNCED}
}

// NewServiceController creates a controller that is watching services and sending
// updates into ServiceUpdate channel.
func NewServiceController(lw cache.ListerWatcher, period time.Duration, ch chan<- ServiceUpdate) cache.Controller {
	_, serviceController := cache.NewInformer(
		lw,
		&v1.Service{},
		period,
		cache.ResourceEventHandlerFuncs{
			AddFunc:    sendAddService(ch),
			UpdateFunc: sendUpdateService(ch),
			DeleteFunc: sendDeleteService(ch),
		},
	)
	return serviceController
}

// NewPodsController creates a controller that is watching pods and sending
// updates into pods channel.
func NewPodsController(lw cache.ListerWatcher, period time.Duration, ch chan<- PodUpdate) cache.Controller {
	_, endpointsController := cache.NewInformer(
		lw,
		&v1.Pod{},
		period,
		cache.ResourceEventHandlerFuncs{
			AddFunc:    sendAddPod(ch),
			UpdateFunc: sendUpdatePod(ch),
			DeleteFunc: sendDeletePod(ch),
		},
	)
	return endpointsController
}

func sendAddService(servicesChan chan<- ServiceUpdate) func(obj interface{}) {
	return func(obj interface{}) {
		service, ok := obj.(*v1.Service)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("cannot convert to *v1.Service: %v", obj))
			return
		}
		servicesChan <- ServiceUpdate{Op: ADD, Service: service}
	}
}

func sendUpdateService(servicesChan chan<- ServiceUpdate) func(oldObj, newObj interface{}) {
	return func(_, newObj interface{}) {
		service, ok := newObj.(*v1.Service)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("cannot convert to *v1.Service: %v", newObj))
			return
		}
		servicesChan <- ServiceUpdate{Op: UPDATE, Service: service}
	}
}

func sendDeleteService(servicesChan chan<- ServiceUpdate) func(obj interface{}) {
	return func(obj interface{}) {
		var service *v1.Service
		switch t := obj.(type) {
		case *v1.Service:
			service = t
		case cache.DeletedFinalStateUnknown:
			var ok bool
			service, ok = t.Obj.(*v1.Service)
			if !ok {
				utilruntime.HandleError(fmt.Errorf("cannot convert to *v1.Service: %v", t.Obj))
				return
			}
		default:
			utilruntime.HandleError(fmt.Errorf("cannot convert to *v1.Service: %v", t))
			return
		}
		servicesChan <- ServiceUpdate{Op: REMOVE, Service: service}
	}
}

func sendAddPod(podsChan chan<- PodUpdate) func(obj interface{}) {
	return func(obj interface{}) {
		pod, ok := obj.(*v1.Pod)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("cannot convert to *v1.Service: %v", obj))
			return
		}
		podsChan <- PodUpdate{Op: ADD, Pod: pod}
	}
}

func sendUpdatePod(podsChan chan<- PodUpdate) func(oldObj, newObj interface{}) {
	return func(_, newObj interface{}) {
		pod, ok := newObj.(*v1.Pod)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("cannot convert to *v1.Service: %v", newObj))
			return
		}
		podsChan <- PodUpdate{Op: UPDATE, Pod: pod}
	}
}

func sendDeletePod(podsChan chan<- PodUpdate) func(obj interface{}) {
	return func(obj interface{}) {
		var pod *v1.Pod
		switch t := obj.(type) {
		case *v1.Pod:
			pod = t
		case cache.DeletedFinalStateUnknown:
			var ok bool
			pod, ok = t.Obj.(*v1.Pod)
			if !ok {
				utilruntime.HandleError(fmt.Errorf("cannot convert to *v1.Service: %v", t.Obj))
				return
			}
		default:
			utilruntime.HandleError(fmt.Errorf("cannot convert to *v1.Service: %v", t))
			return
		}
		podsChan <- PodUpdate{Op: REMOVE, Pod: pod}
	}
}
