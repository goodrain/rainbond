// RAINBOND, Application Management Platform
// Copyright (C) 2021-2021 Goodrain Co., Ltd.

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

package thirdcomponent

import (
	"context"
	"reflect"
	"time"

	"github.com/goodrain/rainbond/pkg/apis/rainbond/v1alpha1"
	"github.com/oam-dev/kubevela/pkg/utils/apply"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const reconcileTimeOut = 60 * time.Second

type Reconciler struct {
	Client               client.Client
	restConfig           *rest.Config
	Scheme               *runtime.Scheme
	concurrentReconciles int
	applyer              apply.Applicator
	discoverPool         *DiscoverPool
	discoverNum          prometheus.Gauge
}

// Reconcile is the main logic of appDeployment controller
func (r *Reconciler) Reconcile(req ctrl.Request) (res reconcile.Result, retErr error) {
	log := logrus.WithField("thirdcomponent", req)
	commonResult := ctrl.Result{RequeueAfter: time.Second * 5}
	component := &v1alpha1.ThirdComponent{}
	ctx, cancel := context.WithTimeout(context.TODO(), reconcileTimeOut)
	defer cancel()
	defer func() {
		if retErr == nil {
			log.Debugf("finished reconciling")
		} else {
			log.Errorf("Failed to reconcile %v", retErr)
		}
	}()

	if err := r.Client.Get(ctx, req.NamespacedName, component); err != nil {
		if apierrors.IsNotFound(err) {
			log.Warningf("thirdcomponent %s does not exist", req)
			r.discoverPool.RemoveDiscoverByName(req.NamespacedName)
		}
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if component.DeletionTimestamp != nil {
		log.Infof("component %s will be deleted", req)
		r.discoverPool.RemoveDiscover(component)
		return ctrl.Result{}, nil
	}
	logrus.Debugf("start to reconcile component %s/%s", component.Namespace, component.Name)
	discover, err := NewDiscover(component, r.restConfig)
	if err != nil {
		component.Status.Phase = v1alpha1.ComponentFailed
		component.Status.Reason = err.Error()
		r.updateStatus(ctx, component)
		return ctrl.Result{}, nil
	}
	if discover == nil {
		component.Status.Phase = v1alpha1.ComponentFailed
		component.Status.Reason = "third component source not support"
		r.updateStatus(ctx, component)
		return ctrl.Result{}, nil
	}
	endpoints, err := discover.DiscoverOne(ctx)
	if err != nil {
		component.Status.Phase = v1alpha1.ComponentFailed
		component.Status.Reason = err.Error()
		r.updateStatus(ctx, component)
		return ctrl.Result{}, nil
	}
	r.discoverPool.AddDiscover(discover)

	if len(endpoints) == 0 {
		component.Status.Phase = v1alpha1.ComponentPending
		component.Status.Reason = "endpoints not found"
		r.updateStatus(ctx, component)
		return ctrl.Result{}, nil
	}

	// create endpoints for service
	if len(component.Spec.Ports) > 0 && len(component.Status.Endpoints) > 0 {
		var services corev1.ServiceList
		selector, err := labels.Parse(labels.FormatLabels(map[string]string{
			"service_id": component.Labels["service_id"],
		}))
		if err != nil {
			logrus.Errorf("create selector failure %s", err.Error())
			return ctrl.Result{}, err
		}
		err = r.Client.List(ctx, &services, &client.ListOptions{LabelSelector: selector})
		if err != nil {
			return commonResult, nil
		}
		log.Infof("list component service success, size:%d", len(services.Items))
		if len(services.Items) == 0 {
			log.Warning("component service is empty")
			return commonResult, nil
		}
		// init component port
		var portMap = make(map[int][]*v1alpha1.ThirdComponentEndpointStatus)
		for _, end := range component.Status.Endpoints {
			port := end.Address.GetPort()
			if end.ServicePort != 0 {
				port = end.ServicePort
			}
			portMap[port] = append(portMap[end.Address.GetPort()], end)
		}
		// create endpoint for component service
		for _, service := range services.Items {
			for _, port := range service.Spec.Ports {
				// if component port not exist in endpoint port list, ignore it.
				if sourceEndpoint, ok := portMap[int(port.Port)]; ok {
					endpoint := createEndpoint(component, &service, sourceEndpoint, int(port.Port))
					controllerutil.SetControllerReference(component, &endpoint, r.Scheme)
					var old corev1.Endpoints
					var apply = true
					if err := r.Client.Get(ctx, types.NamespacedName{Namespace: endpoint.Namespace, Name: endpoint.Name}, &old); err == nil {
						// no change not apply
						if reflect.DeepEqual(old.Subsets, endpoint.Subsets) {
							apply = false
						}
					}
					if apply {
						if err := r.applyer.Apply(ctx, &endpoint); err != nil {
							log.Errorf("apply endpoint for service %s failure %s", service.Name, err.Error())
						}
						log.Infof("apply endpoint for service %s success", service.Name)
					}
				}
			}
		}
	}
	component.Status.Endpoints = endpoints
	component.Status.Phase = v1alpha1.ComponentRunning
	if err := r.updateStatus(ctx, component); err != nil {
		log.Errorf("update status failure %s", err.Error())
		return commonResult, nil
	}
	return reconcile.Result{}, nil
}

func createEndpoint(component *v1alpha1.ThirdComponent, service *corev1.Service, sourceEndpoint []*v1alpha1.ThirdComponentEndpointStatus, port int) corev1.Endpoints {
	spep := make(map[int]int, len(sourceEndpoint))
	for _, endpoint := range sourceEndpoint {
		if endpoint.ServicePort != 0 {
			spep[endpoint.ServicePort] = endpoint.Address.GetPort()
		}
	}
	endpoints := corev1.Endpoints{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Endpoints",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      service.Name,
			Namespace: service.Namespace,
			Labels:    service.Labels,
		},
		Subsets: func() []corev1.EndpointSubset {
			return []corev1.EndpointSubset{
				{
					Ports: func() (re []corev1.EndpointPort) {
						for _, servicePort := range service.Spec.Ports {
							ep := corev1.EndpointPort{
								Name:        servicePort.Name,
								Port:        servicePort.Port,
								Protocol:    servicePort.Protocol,
								AppProtocol: servicePort.AppProtocol,
							}
							endPort, exist := spep[int(servicePort.Port)]
							if exist {
								ep.Port = int32(endPort)
							}
							re = append(re, ep)
						}
						return
					}(),
					Addresses: func() (re []corev1.EndpointAddress) {
						for _, se := range sourceEndpoint {
							if se.Status == v1alpha1.EndpointReady {
								re = append(re, corev1.EndpointAddress{
									IP: se.Address.GetIP(),
									TargetRef: &corev1.ObjectReference{
										Namespace:       component.Namespace,
										Name:            component.Name,
										Kind:            component.Kind,
										APIVersion:      component.APIVersion,
										UID:             component.UID,
										ResourceVersion: component.ResourceVersion,
									},
								})
							}
						}
						return
					}(),
					NotReadyAddresses: func() (re []corev1.EndpointAddress) {
						for _, se := range sourceEndpoint {
							if se.Status == v1alpha1.EndpointNotReady {
								re = append(re, corev1.EndpointAddress{
									IP: se.Address.GetIP(),
									TargetRef: &corev1.ObjectReference{
										Namespace:       component.Namespace,
										Name:            component.Name,
										Kind:            component.Kind,
										APIVersion:      component.APIVersion,
										UID:             component.UID,
										ResourceVersion: component.ResourceVersion,
									},
								})
							}
						}
						return
					}(),
				},
			}
		}(),
	}
	return endpoints
}

// UpdateStatus updates ThirdComponent's Status with retry.RetryOnConflict
func (r *Reconciler) updateStatus(ctx context.Context, appd *v1alpha1.ThirdComponent, opts ...client.UpdateOption) error {
	status := appd.DeepCopy().Status
	return retry.RetryOnConflict(retry.DefaultBackoff, func() (err error) {
		if err = r.Client.Get(ctx, client.ObjectKey{Namespace: appd.Namespace, Name: appd.Name}, appd); err != nil {
			return
		}
		if status.Endpoints == nil {
			status.Endpoints = []*v1alpha1.ThirdComponentEndpointStatus{}
		}
		appd.Status = status
		return r.Client.Status().Update(ctx, appd, opts...)
	})
}

// SetupWithManager setup the controller with manager
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: r.concurrentReconciles,
		}).
		For(&v1alpha1.ThirdComponent{}).
		Complete(r)
}

func (r *Reconciler) Collect(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(r.discoverNum.Desc(), prometheus.GaugeValue, r.discoverPool.GetSize())
}

// Setup adds a controller that reconciles AppDeployment.
func Setup(ctx context.Context, mgr ctrl.Manager) (*Reconciler, error) {
	applyer := apply.NewAPIApplicator(mgr.GetClient())
	r := &Reconciler{
		Client:     mgr.GetClient(),
		restConfig: mgr.GetConfig(),
		Scheme:     mgr.GetScheme(),
		applyer:    applyer,
		discoverNum: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "controller",
			Name:      "third_component_discover_number",
			Help:      "Number of running endpoint discover worker of third component.",
		}),
	}
	dp := NewDiscoverPool(ctx, r)
	r.discoverPool = dp
	return r, r.SetupWithManager(mgr)
}
