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

package source

import (
	"fmt"
	"time"

	"github.com/Sirupsen/logrus"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/apis/apps/v1beta1"
	"k8s.io/client-go/tools/cache"
)

// Operation is a type of operation of services or endpoints.
type Operation int

// These are the available operation types.
const (
	ADD Operation = iota
	UPDATE
	REMOVE
	SYNCED
)

// DeploymentUpdate describes an operation of deployment, sent on the channel.
// You can add, update or remove single endpoints by setting Op == ADD|UPDATE|REMOVE.
type DeploymentUpdate struct {
	Deployment *v1beta1.Deployment
	Op         Operation
}

// RCUpdate describes an operation of endpoints, sent on the channel.
// You can add, update or remove single endpoints by setting Op == ADD|UPDATE|REMOVE.
type RCUpdate struct {
	RC *v1.ReplicationController
	Op Operation
}

// StatefulSetUpdate describes an operation of endpoints, sent on the channel.
// You can add, update or remove single endpoints by setting Op == ADD|UPDATE|REMOVE.
type StatefulSetUpdate struct {
	StatefulSet *v1beta1.StatefulSet
	Op          Operation
}

// NewSourceAPI creates config source that watches for changes to the services and pods.
func NewSourceAPI(v1get cache.Getter, batev1 cache.Getter, period time.Duration, rcsChan chan<- RCUpdate, deploymentsChan chan<- DeploymentUpdate, statefulChan chan<- StatefulSetUpdate, stopCh <-chan struct{}) {
	rcsLW := NewListWatchFromClient(v1get, "replicationcontrollers", v1.NamespaceAll, fields.Everything())
	statefulsLW := NewListWatchFromClient(batev1, "statefulsets", v1.NamespaceAll, fields.Everything())
	deploymentsLW := NewListWatchFromClient(batev1, "deployments", v1.NamespaceAll, fields.Everything())
	logrus.Debug("Start new source api for replicationcontrollers and statefulsets and deployments")
	newSourceAPI(rcsLW, statefulsLW, deploymentsLW, period, rcsChan, deploymentsChan, statefulChan, stopCh)
}

// NewListWatchFromClient creates a new ListWatch from the specified client, resource, namespace and field selector.
func NewListWatchFromClient(c cache.Getter, resource string, namespace string, fieldSelector fields.Selector) *cache.ListWatch {
	listFunc := func(options metav1.ListOptions) (runtime.Object, error) {
		return c.Get().
			Namespace(namespace).
			Resource(resource).
			VersionedParams(&options, metav1.ParameterCodec).
			FieldsSelectorParam(fieldSelector).
			Do().
			Get()
	}
	watchFunc := func(options metav1.ListOptions) (watch.Interface, error) {
		options.Watch = true
		return c.Get().
			Namespace(namespace).
			Resource(resource).
			VersionedParams(&options, metav1.ParameterCodec).
			FieldsSelectorParam(fieldSelector).
			Watch()
	}
	return &cache.ListWatch{ListFunc: listFunc, WatchFunc: watchFunc}
}

func newSourceAPI(
	rcsLW cache.ListerWatcher,
	statefulsLW cache.ListerWatcher,
	deploymentsLW cache.ListerWatcher,
	period time.Duration,
	rcsChan chan<- RCUpdate,
	deploymentsChan chan<- DeploymentUpdate,
	statefulChan chan<- StatefulSetUpdate,
	stopCh <-chan struct{}) {
	rcController := NewRCController(rcsLW, period, rcsChan)
	go rcController.Run(stopCh)

	deploymentController := NewDeploymentsController(deploymentsLW, period, deploymentsChan)
	go deploymentController.Run(stopCh)

	statefulController := NewStatefulSetsController(statefulsLW, period, statefulChan)
	go statefulController.Run(stopCh)

	if !cache.WaitForCacheSync(stopCh, rcController.HasSynced, deploymentController.HasSynced, statefulController.HasSynced) {
		utilruntime.HandleError(fmt.Errorf("source controllers not synced"))
		return
	}
}

// NewRCController creates a controller that is watching rc and sending
// updates into ServiceUpdate channel.
func NewRCController(lw cache.ListerWatcher, period time.Duration, ch chan<- RCUpdate) cache.Controller {
	_, rcController := cache.NewInformer(
		lw,
		&v1.ReplicationController{},
		period,
		cache.ResourceEventHandlerFuncs{
			AddFunc:    sendAddRc(ch),
			UpdateFunc: sendUpdateRc(ch),
			DeleteFunc: sendDeleteRc(ch),
		},
	)
	return rcController
}

// NewDeploymentsController creates a controller that is watching deployments and sending
// updates into deployments channel.
func NewDeploymentsController(lw cache.ListerWatcher, period time.Duration, ch chan<- DeploymentUpdate) cache.Controller {
	_, deploymentsController := cache.NewInformer(
		lw,
		&v1beta1.Deployment{},
		period,
		cache.ResourceEventHandlerFuncs{
			AddFunc:    sendAddDeployment(ch),
			UpdateFunc: sendUpdateDeployment(ch),
			DeleteFunc: sendDeleteDeployment(ch),
		},
	)
	return deploymentsController
}

// NewStatefulSetsController creates a controller that is watching statefulset and sending
// updates into deployments channel.
func NewStatefulSetsController(lw cache.ListerWatcher, period time.Duration, ch chan<- StatefulSetUpdate) cache.Controller {
	_, statefulssetsController := cache.NewInformer(
		lw,
		&v1beta1.StatefulSet{},
		period,
		cache.ResourceEventHandlerFuncs{
			AddFunc:    sendAddStatefulSet(ch),
			UpdateFunc: sendUpdateStatefulSet(ch),
			DeleteFunc: sendDeleteStatefulSet(ch),
		},
	)
	return statefulssetsController
}
func sendAddRc(rcsChan chan<- RCUpdate) func(obj interface{}) {
	return func(obj interface{}) {
		rc, ok := obj.(*v1.ReplicationController)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("cannot convert to *v1.ReplicationController: %v", obj))
			return
		}
		rcsChan <- RCUpdate{Op: ADD, RC: rc}
	}
}
func sendUpdateRc(rcsChan chan<- RCUpdate) func(oldObj, newObj interface{}) {
	return func(_, obj interface{}) {
		rc, ok := obj.(*v1.ReplicationController)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("cannot convert to *v1.ReplicationController: %v", obj))
			return
		}
		rcsChan <- RCUpdate{Op: UPDATE, RC: rc}
	}
}
func sendDeleteRc(rcsChan chan<- RCUpdate) func(obj interface{}) {
	return func(obj interface{}) {
		var rc *v1.ReplicationController
		switch t := obj.(type) {
		case *v1.ReplicationController:
			rc = t
		case cache.DeletedFinalStateUnknown:
			var ok bool
			rc, ok = t.Obj.(*v1.ReplicationController)
			if !ok {
				utilruntime.HandleError(fmt.Errorf("cannot convert to *v1.ReplicationController: %v", t.Obj))
				return
			}
		default:
			utilruntime.HandleError(fmt.Errorf("cannot convert to *v1.ReplicationController: %v", t))
			return
		}
		rcsChan <- RCUpdate{Op: REMOVE, RC: rc}
	}
}

func sendAddDeployment(deploymentsChan chan<- DeploymentUpdate) func(obj interface{}) {
	return func(obj interface{}) {
		pod, ok := obj.(*v1beta1.Deployment)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("cannot convert to *v1beta1.Deployment: %v", obj))
			return
		}
		deploymentsChan <- DeploymentUpdate{Op: ADD, Deployment: pod}
	}
}

func sendUpdateDeployment(deploymentsChan chan<- DeploymentUpdate) func(oldObj, newObj interface{}) {
	return func(_, newObj interface{}) {
		d, ok := newObj.(*v1beta1.Deployment)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("cannot convert to *v1beta1.Deployment: %v", newObj))
			return
		}
		deploymentsChan <- DeploymentUpdate{Op: UPDATE, Deployment: d}
	}
}

func sendDeleteDeployment(deploymentsChan chan<- DeploymentUpdate) func(obj interface{}) {
	return func(obj interface{}) {
		var d *v1beta1.Deployment
		switch t := obj.(type) {
		case *v1beta1.Deployment:
			d = t
		case cache.DeletedFinalStateUnknown:
			var ok bool
			d, ok = t.Obj.(*v1beta1.Deployment)
			if !ok {
				utilruntime.HandleError(fmt.Errorf("cannot convert to *v1beta1.Deployment: %v", t.Obj))
				return
			}
		default:
			utilruntime.HandleError(fmt.Errorf("cannot convert to *v1beta1.Deployment: %v", t))
			return
		}
		deploymentsChan <- DeploymentUpdate{Op: REMOVE, Deployment: d}
	}
}

func sendAddStatefulSet(statefulsChan chan<- StatefulSetUpdate) func(obj interface{}) {
	return func(obj interface{}) {
		stateful, ok := obj.(*v1beta1.StatefulSet)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("cannot convert to *v1beta1.StatefulSet: %v", obj))
			return
		}
		statefulsChan <- StatefulSetUpdate{Op: ADD, StatefulSet: stateful}
	}
}

func sendUpdateStatefulSet(statefulsChan chan<- StatefulSetUpdate) func(oldObj, newObj interface{}) {
	return func(_, newObj interface{}) {
		d, ok := newObj.(*v1beta1.StatefulSet)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("cannot convert to *v1beta1.StatefulSet: %v", newObj))
			return
		}
		statefulsChan <- StatefulSetUpdate{Op: UPDATE, StatefulSet: d}
	}
}

func sendDeleteStatefulSet(statefulsChan chan<- StatefulSetUpdate) func(obj interface{}) {
	return func(obj interface{}) {
		var d *v1beta1.StatefulSet
		switch t := obj.(type) {
		case *v1beta1.StatefulSet:
			d = t
		case cache.DeletedFinalStateUnknown:
			var ok bool
			d, ok = t.Obj.(*v1beta1.StatefulSet)
			if !ok {
				utilruntime.HandleError(fmt.Errorf("cannot convert to *v1beta1.StatefulSet: %v", t.Obj))
				return
			}
		default:
			utilruntime.HandleError(fmt.Errorf("cannot convert to *v1beta1.StatefulSet: %v", t))
			return
		}
		statefulsChan <- StatefulSetUpdate{Op: REMOVE, StatefulSet: d}
	}
}
