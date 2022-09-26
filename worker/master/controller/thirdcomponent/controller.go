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
	rainbondlistersv1alpha1 "github.com/goodrain/rainbond/pkg/generated/listers/rainbond/v1alpha1"
	"github.com/goodrain/rainbond/util/apply"
	validation "github.com/goodrain/rainbond/util/endpoint"
	dis "github.com/goodrain/rainbond/worker/master/controller/thirdcomponent/discover"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	runtimecache "sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const reconcileTimeOut = 60 * time.Second

// Reconciler -
type Reconciler struct {
	Client               client.Client
	restConfig           *rest.Config
	Scheme               *runtime.Scheme
	concurrentReconciles int
	applyer              apply.Applicator
	discoverPool         *DiscoverPool
	discoverNum          prometheus.Gauge

	informer runtimecache.Informer
	lister   rainbondlistersv1alpha1.ThirdComponentLister

	recorder record.EventRecorder
}

// Reconcile is the main logic of appDeployment controller
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (res reconcile.Result, retErr error) {
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
	discover, err := dis.NewDiscover(component, r.restConfig, r.lister)
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
	r.discoverPool.AddDiscover(discover)

	endpoints, err := discover.DiscoverOne(ctx)
	if err != nil {
		component.Status.Phase = v1alpha1.ComponentFailed
		component.Status.Reason = err.Error()
		r.updateStatus(ctx, component)
		return ctrl.Result{}, nil
	}

	if len(endpoints) == 0 {
		component.Status.Phase = v1alpha1.ComponentPending
		component.Status.Reason = "endpoints not found"
		r.updateStatus(ctx, component)
		return ctrl.Result{}, nil
	}
	isUnhealthy := false
	for _, c := range component.Status.Endpoints {
		if c.Status == "Unhealthy" {
			isUnhealthy = true
			break
		}
	}
	if isUnhealthy {
		component.Status.Phase = v1alpha1.ComponentFailed
		component.Status.Reason = "endpoints has Unhealthy"
		r.updateStatus(ctx, component)
		return ctrl.Result{}, nil
	}
	// create endpoints for service
	if len(component.Spec.Ports) > 0 && len(component.Status.Endpoints) > 0 {
		var services corev1.ServiceList
		selector, _ := labels.Parse(labels.FormatLabels(map[string]string{
			"service_id": component.Labels["service_id"],
		}))
		if err = r.Client.List(ctx, &services, &client.ListOptions{LabelSelector: selector}); err != nil {
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
		if len(component.Spec.Ports) == 1 && len(component.Spec.EndpointSource.StaticEndpoints) > 1 {
			svc := services.Items[0]
			ep := createEndpointsOnlyOnePort(component, svc, component.Status.Endpoints)
			if ep != nil {
				controllerutil.SetControllerReference(component, ep, r.Scheme)
				r.applyEndpointService(ctx, log, &svc, ep)
			}
		} else {
			for _, service := range services.Items {
				service := service
				for _, port := range service.Spec.Ports {
					// if component port not exist in endpoint port list, ignore it.
					sourceEndpoint, ok := portMap[int(port.Port)]
					if !ok {
						continue
					}
					endpoint := createEndpoint(component, &service, sourceEndpoint)
					controllerutil.SetControllerReference(component, &endpoint, r.Scheme)
					r.applyEndpointService(ctx, log, &service, &endpoint)
				}
			}
		}
	}
	component.Status.Endpoints = endpoints
	component.Status.Phase = v1alpha1.ComponentRunning
	component.Status.Reason = ""
	if err := r.updateStatus(ctx, component); err != nil {
		log.Errorf("update status failure %s", err.Error())
		return commonResult, nil
	}
	return reconcile.Result{}, nil
}

func (r *Reconciler) applyEndpointService(ctx context.Context, log *logrus.Entry, svc *corev1.Service, ep *corev1.Endpoints) {
	var old corev1.Endpoints
	if err := r.Client.Get(ctx, types.NamespacedName{Namespace: ep.Namespace, Name: ep.Name}, &old); err == nil {
		// no change not apply
		if reflect.DeepEqual(old.Subsets, ep.Subsets) &&
			reflect.DeepEqual(old.Annotations, ep.Annotations) {
			return
		}
	}
	if err := r.applyer.Apply(ctx, ep); err != nil {
		log.Errorf("apply endpoint for service %s failure %s", svc.Name, err.Error())
	}

	svc.Annotations = ep.Annotations
	if err := r.applyer.Apply(ctx, svc); err != nil {
		log.Errorf("apply service(%s) for updating annotation: %v", svc.Name, err)
	}
	log.Infof("apply endpoint for service %s success", svc.Name)
}

func createEndpointsOnlyOnePort(thirdComponent *v1alpha1.ThirdComponent, service corev1.Service, sourceEndpoints []*v1alpha1.ThirdComponentEndpointStatus) *corev1.Endpoints {
	if len(thirdComponent.Spec.EndpointSource.StaticEndpoints) == 0 {
		// support static endpoints only for now
		return nil
	}
	if len(thirdComponent.Spec.Ports) != 1 {
		return nil
	}
	logrus.Debugf("create endpoints with one port")

	sourceEndpointPE := make(map[int][]*v1alpha1.ThirdComponentEndpointStatus)
	for _, ep := range sourceEndpoints {
		eps := sourceEndpointPE[ep.Address.GetPort()]
		sourceEndpointPE[ep.Address.GetPort()] = append(eps, ep)
	}

	endpoints := &corev1.Endpoints{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Endpoints",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      service.Name,
			Namespace: service.Namespace,
			Labels:    service.Labels,
		},
	}

	servicePort := service.Spec.Ports[0]
	var subsets []corev1.EndpointSubset
	var domain string
	for port, eps := range sourceEndpointPE {
		subset := corev1.EndpointSubset{
			Ports: []corev1.EndpointPort{
				{
					Name:        servicePort.Name,
					Port:        int32(port),
					Protocol:    servicePort.Protocol,
					AppProtocol: servicePort.AppProtocol,
				},
			},
		}
		for _, ep := range eps {
			if validation.IsDomainNotIP(ep.Address.GetIP()) {
				domain = string(ep.Address)
			}

			address := corev1.EndpointAddress{
				IP: ep.Address.GetIP(),
			}
			if ep.Status == v1alpha1.EndpointReady {
				subset.Addresses = append(subset.Addresses, address)
			} else {
				subset.NotReadyAddresses = append(subset.NotReadyAddresses, address)
			}
		}
		subsets = append(subsets, subset)
	}
	endpoints.Subsets = subsets

	if domain != "" {
		endpoints.Annotations = map[string]string{
			"domain": domain,
		}
	}

	return endpoints
}

func createEndpoint(component *v1alpha1.ThirdComponent, service *corev1.Service, sourceEndpoint []*v1alpha1.ThirdComponentEndpointStatus) corev1.Endpoints {
	spep := make(map[int]int, len(sourceEndpoint))
	for _, endpoint := range sourceEndpoint {
		if endpoint.ServicePort != 0 {
			spep[endpoint.ServicePort] = endpoint.Address.GetPort()
		}
	}

	var domain string
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
							if validation.IsDomainNotIP(se.Address.GetIP()) {
								domain = string(se.Address)
							}
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
							if se.Status == v1alpha1.EndpointNotReady || se.Status == v1alpha1.EndpointUnhealthy {
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

	if domain != "" {
		endpoints.Annotations = map[string]string{
			"domain": domain,
		}
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

// Collect -
func (r *Reconciler) Collect(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(r.discoverNum.Desc(), prometheus.GaugeValue, r.discoverPool.GetSize())
}

// Setup adds a controller that reconciles AppDeployment.
func Setup(ctx context.Context, mgr ctrl.Manager) (*Reconciler, error) {
	informer, err := mgr.GetCache().GetInformerForKind(ctx, v1alpha1.SchemeGroupVersion.WithKind("ThirdComponent"))
	if err != nil {
		return nil, errors.WithMessage(err, "get informer for thirdcomponent")
	}
	lister := rainbondlistersv1alpha1.NewThirdComponentLister(informer.(cache.SharedIndexInformer).GetIndexer())

	recorder := mgr.GetEventRecorderFor("thirdcomponent-controller")

	r := &Reconciler{
		Client:     mgr.GetClient(),
		restConfig: mgr.GetConfig(),
		Scheme:     mgr.GetScheme(),
		applyer:    apply.NewAPIApplicator(mgr.GetClient()),
		discoverNum: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "controller",
			Name:      "third_component_discover_number",
			Help:      "Number of running endpoint discover worker of third component.",
		}),
		informer: informer,
		lister:   lister,
		recorder: recorder,
	}
	dp := NewDiscoverPool(ctx, r, recorder)
	r.discoverPool = dp
	return r, r.SetupWithManager(mgr)
}
