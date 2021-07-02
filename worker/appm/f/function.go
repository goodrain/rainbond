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

package f

import (
	"context"
	"fmt"
	"time"

	"github.com/goodrain/rainbond/gateway/annotations/parser"
	v1 "github.com/goodrain/rainbond/worker/appm/types/v1"
	"github.com/oam-dev/kubevela/pkg/utils/apply"
	monitorv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	"github.com/sirupsen/logrus"
	autoscalingv2 "k8s.io/api/autoscaling/v2beta2"
	corev1 "k8s.io/api/core/v1"
	extensions "k8s.io/api/extensions/v1beta1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	clientRetryCount    = 5
	clientRetryInterval = 5 * time.Second
)

// ApplyOne applies one rule.
func ApplyOne(ctx context.Context, apply apply.Applicator, clientset kubernetes.Interface, app *v1.AppService) error {
	_, err := clientset.CoreV1().Namespaces().Get(context.Background(), app.TenantID, metav1.GetOptions{})
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			_, err = clientset.CoreV1().Namespaces().Create(context.Background(), app.GetTenant(), metav1.CreateOptions{})
			if err != nil && !k8sErrors.IsAlreadyExists(err) {
				return fmt.Errorf("error creating namespace: %v", err)
			}
		}
		if err != nil {
			return fmt.Errorf("error checking namespace: %v", err)
		}
	}
	// for custom component
	if len(app.GetManifests()) > 0 && apply != nil {
		for _, manifest := range app.GetManifests() {
			if err := apply.Apply(ctx, manifest); err != nil {
				return fmt.Errorf("apply custom component manifest %s/%s failure %s", manifest.GetKind(), manifest.GetName(), err.Error())
			}
		}
	}
	if app.CustomParams != nil {
		if domain, exist := app.CustomParams["domain"]; exist {
			// update ingress
			for _, ing := range app.GetIngress(true) {
				if len(ing.Spec.Rules) > 0 && ing.Spec.Rules[0].Host == domain {
					if len(ing.Spec.TLS) > 0 {
						for _, secret := range app.GetSecrets(true) {
							if ing.Spec.TLS[0].SecretName == secret.Name {
								ensureSecret(secret, clientset)
							}
						}
					}
					ensureIngress(ing, clientset)
				}
			}
		}
		if domain, exist := app.CustomParams["tcp-address"]; exist {
			// update ingress
			for _, ing := range app.GetIngress(true) {
				if host, exist := ing.Annotations[parser.GetAnnotationWithPrefix("l4-host")]; exist {
					address := fmt.Sprintf("%s:%s", host, ing.Annotations[parser.GetAnnotationWithPrefix("l4-port")])
					if address == domain {
						ensureIngress(ing, clientset)
					}
				}
			}
		}
	} else {
		// update service
		for _, service := range app.GetServices(true) {
			ensureService(service, clientset)
		}
		// update secret
		for _, secret := range app.GetSecrets(true) {
			ensureSecret(secret, clientset)
		}
		// update endpoints
		for _, ep := range app.GetEndpoints(true) {
			if err := EnsureEndpoints(ep, clientset); err != nil {
				logrus.Errorf("create or update endpoint %s failure %s", ep.Name, err.Error())
			}
		}
		// update ingress
		for _, ing := range app.GetIngress(true) {
			ensureIngress(ing, clientset)
		}
	}
	// delete delIngress
	for _, ing := range app.GetDelIngs() {
		err := clientset.ExtensionsV1beta1().Ingresses(ing.Namespace).Delete(context.Background(), ing.Name, metav1.DeleteOptions{})
		if err != nil && !k8sErrors.IsNotFound(err) {
			// don't return error, hope it is ok next time
			logrus.Warningf("error deleting ingress(%v): %v", ing, err)
		}
	}
	// delete delSecrets
	for _, secret := range app.GetDelSecrets() {
		err := clientset.CoreV1().Secrets(secret.Namespace).Delete(context.Background(), secret.Name, metav1.DeleteOptions{})
		if err != nil && !k8sErrors.IsNotFound(err) {
			// don't return error, hope it is ok next time
			logrus.Warningf("error deleting secret(%v): %v", secret, err)
		}
	}
	// delete delServices
	for _, svc := range app.GetDelServices() {
		err := clientset.CoreV1().Services(svc.Namespace).Delete(context.Background(), svc.Name, metav1.DeleteOptions{})
		if err != nil && !k8sErrors.IsNotFound(err) {
			// don't return error, hope it is ok next time
			logrus.Warningf("error deleting service(%v): %v", svc, err)
			continue
		}
		logrus.Debugf("successfully deleted service(%v)", svc)
	}
	return nil
}

func ensureService(new *corev1.Service, clientSet kubernetes.Interface) error {
	old, err := clientSet.CoreV1().Services(new.Namespace).Get(context.Background(), new.Name, metav1.GetOptions{})
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			_, err = clientSet.CoreV1().Services(new.Namespace).Create(context.Background(), new, metav1.CreateOptions{})
			if err != nil && !k8sErrors.IsAlreadyExists(err) {
				logrus.Warningf("error creating service %+v: %v", new, err)
			}
			return nil
		}
		logrus.Errorf("error getting service(%s): %v", fmt.Sprintf("%s/%s", new.Namespace, new.Name), err)
		return err
	}
	updateService := old.DeepCopy()
	updateService.Spec = new.Spec
	updateService.Labels = new.Labels
	updateService.Annotations = new.Annotations
	return persistUpdate(updateService, clientSet)
}

func persistUpdate(service *corev1.Service, clientSet kubernetes.Interface) error {
	var err error
	for i := 0; i < clientRetryCount; i++ {
		_, err = clientSet.CoreV1().Services(service.Namespace).UpdateStatus(context.Background(), service, metav1.UpdateOptions{})
		if err == nil {
			return nil
		}
		// If the object no longer exists, we don't want to recreate it. Just bail
		// out so that we can process the delete, which we should soon be receiving
		// if we haven't already.
		if errors.IsNotFound(err) {
			logrus.Infof("Not persisting update to service '%s/%s' that no longer exists: %v",
				service.Namespace, service.Name, err)
			return nil
		}
		// TODO: Try to resolve the conflict if the change was unrelated to load
		// balancer status. For now, just pass it up the stack.
		if errors.IsConflict(err) {
			return fmt.Errorf("not persisting update to service '%s/%s' that has been changed since we received it: %v",
				service.Namespace, service.Name, err)
		}
		logrus.Warningf("Failed to update service '%s/%s' %s", service.Namespace, service.Name, err)
		time.Sleep(clientRetryInterval)
	}
	return err
}

func ensureIngress(ingress *extensions.Ingress, clientSet kubernetes.Interface) {
	_, err := clientSet.ExtensionsV1beta1().Ingresses(ingress.Namespace).Update(context.Background(), ingress, metav1.UpdateOptions{})
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			_, err := clientSet.ExtensionsV1beta1().Ingresses(ingress.Namespace).Create(context.Background(), ingress, metav1.CreateOptions{})
			if err != nil && !k8sErrors.IsAlreadyExists(err) {
				logrus.Errorf("error creating ingress %+v: %v", ingress, err)
			}
			return
		}
		logrus.Warningf("error updating ingress %+v: %v", ingress, err)
	}
}

func ensureSecret(secret *corev1.Secret, clientSet kubernetes.Interface) {
	_, err := clientSet.CoreV1().Secrets(secret.Namespace).Update(context.Background(), secret, metav1.UpdateOptions{})

	if err != nil {
		if k8sErrors.IsNotFound(err) {
			_, err := clientSet.CoreV1().Secrets(secret.Namespace).Create(context.Background(), secret, metav1.CreateOptions{})
			if err != nil && !k8sErrors.IsAlreadyExists(err) {
				logrus.Warningf("error creating secret %+v: %v", secret, err)
			}
			return
		}
		logrus.Warningf("error updating secret %+v: %v", secret, err)
	}
}

// EnsureEndpoints creates or updates endpoints.
func EnsureEndpoints(ep *corev1.Endpoints, clientSet kubernetes.Interface) error {
	// See if there's actually an update here.
	currentEndpoints, err := clientSet.CoreV1().Endpoints(ep.Namespace).Get(context.Background(), ep.Name, metav1.GetOptions{})
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			currentEndpoints = &corev1.Endpoints{
				ObjectMeta: metav1.ObjectMeta{
					Name:   ep.Name,
					Labels: ep.Labels,
				},
			}
		} else {
			return err
		}
	}

	createEndpoints := len(currentEndpoints.ResourceVersion) == 0

	if !createEndpoints &&
		apiequality.Semantic.DeepEqual(currentEndpoints.Subsets, ep.Subsets) &&
		apiequality.Semantic.DeepEqual(currentEndpoints.Labels, ep.Labels) {
		logrus.Debugf("endpoints are equal for %s/%s, skipping update", ep.Namespace, ep.Name)
		return nil
	}
	newEndpoints := currentEndpoints.DeepCopy()
	newEndpoints.Subsets = ep.Subsets
	newEndpoints.Labels = ep.Labels
	if newEndpoints.Annotations == nil {
		newEndpoints.Annotations = make(map[string]string)
	}
	if createEndpoints {
		// No previous endpoints, create them
		_, err = clientSet.CoreV1().Endpoints(ep.Namespace).Create(context.Background(), newEndpoints, metav1.CreateOptions{})
		logrus.Infof("Create endpoints for %v/%v", ep.Namespace, ep.Name)
	} else {
		// Pre-existing
		_, err = clientSet.CoreV1().Endpoints(ep.Namespace).Update(context.Background(), newEndpoints, metav1.UpdateOptions{})
		logrus.Infof("Update endpoints for %v/%v", ep.Namespace, ep.Name)
	}
	if err != nil {
		if createEndpoints && errors.IsForbidden(err) {
			// A request is forbidden primarily for two reasons:
			// 1. namespace is terminating, endpoint creation is not allowed by default.
			// 2. policy is misconfigured, in which case no service would function anywhere.
			// Given the frequency of 1, we log at a lower level.
			logrus.Infof("Forbidden from creating endpoints: %v", err)
		}
		return err
	}
	return nil
}

// EnsureService ensure service:update or create service
func EnsureService(new *corev1.Service, clientSet kubernetes.Interface) error {
	return ensureService(new, clientSet)
}

// EnsureHPA -
func EnsureHPA(new *autoscalingv2.HorizontalPodAutoscaler, clientSet kubernetes.Interface) {
	_, err := clientSet.AutoscalingV2beta2().HorizontalPodAutoscalers(new.Namespace).Get(context.Background(), new.Name, metav1.GetOptions{})
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			_, err = clientSet.AutoscalingV2beta2().HorizontalPodAutoscalers(new.Namespace).Create(context.Background(), new, metav1.CreateOptions{})
			if err != nil {
				logrus.Warningf("error creating hpa %+v: %v", new, err)
			}
			return
		}
		logrus.Errorf("error getting hpa(%s): %v", fmt.Sprintf("%s/%s", new.Namespace, new.Name), err)
		return
	}
	_, err = clientSet.AutoscalingV2beta2().HorizontalPodAutoscalers(new.Namespace).Update(context.Background(), new, metav1.UpdateOptions{})
	if err != nil {
		logrus.Warningf("error updating hpa %+v: %v", new, err)
		return
	}
}

// UpgradeIngress is used to update *extensions.Ingress.
func UpgradeIngress(clientset kubernetes.Interface,
	as *v1.AppService,
	old, new []*extensions.Ingress,
	handleErr func(msg string, err error) error) error {
	var oldMap = make(map[string]*extensions.Ingress, len(old))
	for i, item := range old {
		oldMap[item.Name] = old[i]
	}
	for _, n := range new {
		if o, ok := oldMap[n.Name]; ok {
			n.UID = o.UID
			n.ResourceVersion = o.ResourceVersion
			ing, err := clientset.ExtensionsV1beta1().Ingresses(n.Namespace).Update(context.Background(), n, metav1.UpdateOptions{})
			if err != nil {
				if err := handleErr(fmt.Sprintf("error updating ingress: %+v: err: %v",
					ing, err), err); err != nil {
					return err
				}
				continue
			}
			as.SetIngress(ing)
			delete(oldMap, o.Name)
			logrus.Debugf("ServiceID: %s; successfully update ingress: %s", as.ServiceID, ing.Name)
		} else {
			logrus.Debugf("ingress: %+v", n)
			ing, err := clientset.ExtensionsV1beta1().Ingresses(n.Namespace).Create(context.Background(), n, metav1.CreateOptions{})
			if err != nil {
				if err := handleErr(fmt.Sprintf("error creating ingress: %+v: err: %v",
					ing, err), err); err != nil {
					return err
				}
				continue
			}
			as.SetIngress(ing)
			logrus.Debugf("ServiceID: %s; successfully create ingress: %s", as.ServiceID, ing.Name)
		}
	}
	for _, ing := range oldMap {
		if ing != nil {
			if err := clientset.ExtensionsV1beta1().Ingresses(ing.Namespace).Delete(context.Background(), ing.Name,
				metav1.DeleteOptions{}); err != nil {
				if err := handleErr(fmt.Sprintf("error deleting ingress: %+v: err: %v",
					ing, err), err); err != nil {
					return err
				}
				continue
			}
			logrus.Debugf("ServiceID: %s; successfully delete ingress: %s", as.ServiceID, ing.Name)
		}
	}
	return nil
}

// UpgradeSecrets is used to update *corev1.Secret.
func UpgradeSecrets(clientset kubernetes.Interface,
	as *v1.AppService, old, new []*corev1.Secret,
	handleErr func(msg string, err error) error) error {
	var oldMap = make(map[string]*corev1.Secret, len(old))
	for i, item := range old {
		oldMap[item.Name] = old[i]
	}
	for _, n := range new {
		if o, ok := oldMap[n.Name]; ok {
			n.UID = o.UID
			n.ResourceVersion = o.ResourceVersion
			sec, err := clientset.CoreV1().Secrets(n.Namespace).Update(context.Background(), n, metav1.UpdateOptions{})
			if err != nil {
				if err := handleErr(fmt.Sprintf("error updating secret: %+v: err: %v",
					sec, err), err); err != nil {
					return err
				}
				continue
			}
			as.SetSecret(sec)
			delete(oldMap, o.Name)
			logrus.Debugf("ServiceID: %s; successfully update secret: %s", as.ServiceID, sec.Name)
		} else {
			sec, err := clientset.CoreV1().Secrets(n.Namespace).Create(context.Background(), n, metav1.CreateOptions{})
			if err != nil {
				if err := handleErr(fmt.Sprintf("error creating secret: %+v: err: %v",
					sec, err), err); err != nil {
					return err
				}
				continue
			}
			as.SetSecret(sec)
			logrus.Debugf("ServiceID: %s; successfully create secret: %s", as.ServiceID, sec.Name)
		}
	}
	for _, sec := range oldMap {
		if sec != nil {
			if err := clientset.CoreV1().Secrets(sec.Namespace).Delete(context.Background(), sec.Name, metav1.DeleteOptions{}); err != nil {
				if err := handleErr(fmt.Sprintf("error deleting secret: %+v: err: %v",
					sec, err), err); err != nil {
					return err
				}
				continue
			}
			logrus.Debugf("ServiceID: %s; successfully delete secret: %s", as.ServiceID, sec.Name)
		}
	}
	return nil
}

// UpgradeClaims is used to update *corev1.PVC.
func UpgradeClaims(clientset *kubernetes.Clientset, as *v1.AppService, old, new []*corev1.PersistentVolumeClaim, handleErr func(msg string, err error) error) error {
	var oldMap = make(map[string]*corev1.PersistentVolumeClaim, len(old))
	for i, item := range old {
		oldMap[item.Name] = old[i]
	}
	for _, n := range new {
		if o, ok := oldMap[n.Name]; ok {
			n.UID = o.UID
			n.ResourceVersion = o.ResourceVersion
			claim, err := clientset.CoreV1().PersistentVolumeClaims(n.Namespace).Update(context.Background(), n, metav1.UpdateOptions{})
			if err != nil {
				if err := handleErr(fmt.Sprintf("error updating claim: %+v: err: %v", claim, err), err); err != nil {
					return err
				}
				continue
			}
			as.SetClaim(claim)
			delete(oldMap, o.Name)
			logrus.Debugf("ServiceID: %s; successfully update claim: %s", as.ServiceID, claim.Name)
		} else {
			claim, err := clientset.CoreV1().PersistentVolumeClaims(n.Namespace).Get(context.Background(), n.Name, metav1.GetOptions{})
			if err != nil {
				if k8sErrors.IsNotFound(err) {
					_, err := clientset.CoreV1().PersistentVolumeClaims(n.Namespace).Create(context.Background(), n, metav1.CreateOptions{})
					if err != nil {
						if err := handleErr(fmt.Sprintf("error creating claim: %+v: err: %v",
							n, err), err); err != nil {
							return err
						}
						continue
					}
				} else {
					if e := handleErr(fmt.Sprintf("err get claim[%s:%s], err: %+v", n.Namespace, n.Name, err), err); err != nil {
						return e
					}
				}
			}
			if claim != nil {
				logrus.Infof("claim is exists, do not create again, and can't update it: %s", claim.Name)
			} else {
				claim, err = clientset.CoreV1().PersistentVolumeClaims(n.Namespace).Update(context.Background(), n, metav1.UpdateOptions{})
				if err != nil {
					if err := handleErr(fmt.Sprintf("error update claim: %+v: err: %v", claim, err), err); err != nil {
						return err
					}
					continue
				}
				logrus.Debugf("ServiceID: %s; successfully create claim: %s", as.ServiceID, claim.Name)
			}
			as.SetClaim(claim)
		}
	}
	for _, claim := range oldMap {
		if claim != nil {
			if err := clientset.CoreV1().PersistentVolumeClaims(claim.Namespace).Delete(context.Background(), claim.Name, metav1.DeleteOptions{}); err != nil {
				if err := handleErr(fmt.Sprintf("error deleting claim: %+v: err: %v", claim, err), err); err != nil {
					return err
				}
				continue
			}
			logrus.Debugf("ServiceID: %s; successfully delete claim: %s", as.ServiceID, claim.Name)
		}
	}
	return nil
}

// UpgradeEndpoints is used to update *corev1.Endpoints.
func UpgradeEndpoints(clientset kubernetes.Interface,
	as *v1.AppService, old, new []*corev1.Endpoints,
	handleErr func(msg string, err error) error) error {
	var oldMap = make(map[string]*corev1.Endpoints, len(old))
	for i, item := range old {
		oldMap[item.Name] = old[i]
	}
	for _, n := range new {
		if o, ok := oldMap[n.Name]; ok {
			oldEndpoint, err := clientset.CoreV1().Endpoints(n.Namespace).Get(context.Background(), n.Name, metav1.GetOptions{})
			if err != nil {
				if k8sErrors.IsNotFound(err) {
					_, err := clientset.CoreV1().Endpoints(n.Namespace).Create(context.Background(), n, metav1.CreateOptions{})
					if err != nil {
						if err := handleErr(fmt.Sprintf("error creating endpoints: %+v: err: %v",
							n, err), err); err != nil {
							return err
						}
						continue
					}
				}
				if e := handleErr(fmt.Sprintf("err get endpoint[%s:%s], err: %+v", n.Namespace, n.Name, err), err); err != nil {
					return e
				}
			}
			n.ResourceVersion = oldEndpoint.ResourceVersion
			ep, err := clientset.CoreV1().Endpoints(n.Namespace).Update(context.Background(), n, metav1.UpdateOptions{})
			if err != nil {
				if e := handleErr(fmt.Sprintf("error updating endpoints: %+v: err: %v",
					ep, err), err); e != nil {
					return e
				}
				continue
			}
			as.AddEndpoints(ep)
			delete(oldMap, o.Name)
			logrus.Debugf("ServiceID: %s; successfully update endpoints: %s", as.ServiceID, ep.Name)
		} else {
			_, err := clientset.CoreV1().Endpoints(n.Namespace).Create(context.Background(), n, metav1.CreateOptions{})
			if err != nil {
				if err := handleErr(fmt.Sprintf("error creating endpoints: %+v: err: %v",
					n, err), err); err != nil {
					return err
				}
				continue
			}
			as.AddEndpoints(n)
			logrus.Debugf("ServiceID: %s; successfully create endpoints: %s", as.ServiceID, n.Name)
		}
	}
	for _, sec := range oldMap {
		if sec != nil {
			if err := clientset.CoreV1().Endpoints(sec.Namespace).Delete(context.Background(), sec.Name, metav1.DeleteOptions{}); err != nil {
				if err := handleErr(fmt.Sprintf("error deleting endpoints: %+v: err: %v",
					sec, err), err); err != nil {
					return err
				}
				continue
			}
			logrus.Debugf("ServiceID: %s; successfully delete endpoints: %s", as.ServiceID, sec.Name)
		}
	}
	return nil
}

// UpgradeServiceMonitor -
func UpgradeServiceMonitor(
	clientset *versioned.Clientset,
	as *v1.AppService,
	old, new []*monitorv1.ServiceMonitor,
	handleErr func(msg string, err error) error) error {

	var oldMap = make(map[string]*monitorv1.ServiceMonitor, len(old))
	for i := range old {
		oldMap[old[i].Name] = old[i]
	}
	for _, n := range new {
		if o, ok := oldMap[n.Name]; ok {
			n.UID = o.UID
			n.ResourceVersion = o.ResourceVersion
			ing, err := clientset.MonitoringV1().ServiceMonitors(n.Namespace).Update(context.Background(), n, metav1.UpdateOptions{})
			if err != nil {
				if err := handleErr(fmt.Sprintf("error updating service monitor: %+v: err: %v",
					ing, err), err); err != nil {
					return err
				}
				continue
			}
			as.SetServiceMonitor(n)
			delete(oldMap, o.Name)
			logrus.Debugf("ServiceID: %s; successfully update service monitor: %s", as.ServiceID, ing.Name)
		} else {
			ing, err := clientset.MonitoringV1().ServiceMonitors(n.Namespace).Create(context.Background(), n, metav1.CreateOptions{})
			if err != nil {
				if err := handleErr(fmt.Sprintf("error creating service monitor: %+v: err: %v",
					ing, err), err); err != nil {
					return err
				}
				continue
			}
			as.SetServiceMonitor(ing)
			logrus.Debugf("ServiceID: %s; successfully create service monitor: %s", as.ServiceID, ing.Name)
		}
	}
	for _, ing := range oldMap {
		if ing != nil {
			if err := clientset.MonitoringV1().ServiceMonitors(ing.Namespace).Delete(context.Background(), ing.Name,
				metav1.DeleteOptions{}); err != nil {
				if err := handleErr(fmt.Sprintf("error deleting service monitor: %+v: err: %v",
					ing, err), err); err != nil {
					return err
				}
				continue
			}
			logrus.Debugf("ServiceID: %s; successfully delete service monitor: %s", as.ServiceID, ing.Name)
		}
	}
	return nil
}

// CreateOrUpdateSecret creates or updates secret.
func CreateOrUpdateSecret(clientset kubernetes.Interface, secret *corev1.Secret) error {
	old, err := clientset.CoreV1().Secrets(secret.Namespace).Get(context.Background(), secret.Name, metav1.GetOptions{})
	if err != nil {
		if !k8sErrors.IsNotFound(err) {
			return err
		}
		// create secret
		_, err := clientset.CoreV1().Secrets(secret.Namespace).Create(context.Background(), secret, metav1.CreateOptions{})
		return err
	}

	// update secret
	secret.ResourceVersion = old.ResourceVersion
	_, err = clientset.CoreV1().Secrets(secret.Namespace).Update(context.Background(), secret, metav1.UpdateOptions{})
	return err
}
