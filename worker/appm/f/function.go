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
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/worker/appm/types/v1"
	corev1 "k8s.io/api/core/v1"
	extensions "k8s.io/api/extensions/v1beta1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// ApplyOne applies one rule.
func ApplyOne(clientset *kubernetes.Clientset, app *v1.AppService) error {
	_, err := clientset.CoreV1().Namespaces().Get(app.TenantID, metav1.GetOptions{})
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			_, err = clientset.CoreV1().Namespaces().Create(app.GetTenant())
			if err != nil {
				return fmt.Errorf("error creating namespace: %v", err)
			}
		}
		if err != nil {
			return fmt.Errorf("error checking namespace: %v", err)
		}
	}
	// update service
	for _, service := range app.GetServices() {
		ensureService(service, clientset)
	}
	// update secret
	for _, secret := range app.GetSecrets() {
		ensureSecret(secret, clientset)
	}
	// update endpoints
	for _, ep := range app.GetEndpoints() {
		EnsureEndpoints(ep, clientset)
	}
	// update ingress
	for _, ing := range app.GetIngress() {
		ensureIngress(ing, clientset)
	}
	// delete delIngress
	for _, ing := range app.GetDelIngs() {
		err := clientset.ExtensionsV1beta1().Ingresses(ing.Namespace).Delete(ing.Name, &metav1.DeleteOptions{})
		if err != nil {
			// don't return error, hope it is ok next time
			logrus.Warningf("error deleting ingress(%v): %v", ing, err)
		}
	}
	// delete delSecrets
	for _, secret := range app.GetDelSecrets() {
		err := clientset.CoreV1().Secrets(secret.Namespace).Delete(secret.Name, &metav1.DeleteOptions{})
		if err != nil {
			// don't return error, hope it is ok next time
			logrus.Warningf("error deleting secret(%v): %v", secret, err)
		}
	}
	// delete delServices
	for _, svc := range app.GetDelServices() {
		err := clientset.CoreV1().Services(svc.Namespace).Delete(svc.Name, &metav1.DeleteOptions{})
		if err != nil {
			// don't return error, hope it is ok next time
			logrus.Warningf("error deleting service(%v): %v", svc, err)
			continue
		}
		logrus.Debugf("successfully deleted service(%v)", svc)
	}
	return nil
}

func ensureService(new *corev1.Service, clientSet kubernetes.Interface) {
	old, err := clientSet.CoreV1().Services(new.Namespace).Get(new.Name, metav1.GetOptions{})
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			_, err := clientSet.CoreV1().Services(new.Namespace).Create(new)
			if err != nil {
				logrus.Warningf("error creating service %+v: %v", new, err)
			}
			return
		}
		logrus.Errorf("error getting service(%s): %v", fmt.Sprintf("%s/%s", new.Namespace, new.Name), err)
		return
	}
	new.ResourceVersion = old.ResourceVersion
	new.Spec.ClusterIP = old.Spec.ClusterIP
	_, err = clientSet.CoreV1().Services(new.Namespace).Update(new)
	if err != nil {
		logrus.Warningf("error updating service %+v: %v", new, err)
		return
	}
}

func ensureIngress(ingress *extensions.Ingress, clientSet kubernetes.Interface) {
	_, err := clientSet.ExtensionsV1beta1().Ingresses(ingress.Namespace).Update(ingress)

	if err != nil {
		if k8sErrors.IsNotFound(err) {
			_, err := clientSet.ExtensionsV1beta1().Ingresses(ingress.Namespace).Create(ingress)
			if err != nil {
				logrus.Warningf("error creating ingress %+v: %v", ingress, err)
			}
			return
		}
		logrus.Warningf("error updating ingress %+v: %v", ingress, err)
	}
}

func ensureSecret(secret *corev1.Secret, clientSet kubernetes.Interface) {
	_, err := clientSet.CoreV1().Secrets(secret.Namespace).Update(secret)

	if err != nil {
		if k8sErrors.IsNotFound(err) {
			_, err := clientSet.CoreV1().Secrets(secret.Namespace).Create(secret)
			if err != nil {
				logrus.Warningf("error creating secret %+v: %v", secret, err)
			}
			return
		}
		logrus.Warningf("error updating secret %+v: %v", secret, err)
	}
}

// EnsureEndpoints creates or updates endpoints.
func EnsureEndpoints(ep *corev1.Endpoints, clientSet kubernetes.Interface) {
	_, err := clientSet.CoreV1().Endpoints(ep.Namespace).Update(ep)
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			_, err := clientSet.CoreV1().Endpoints(ep.Namespace).Create(ep)
			if err != nil {
				logrus.Warningf("error creating endpoints %+v: %v", ep, err)
			}
			return
		}
		logrus.Warningf("error updating endpoints %+v: %v", ep, err)
	}
}

// UpgradeIngress is used to update *extensions.Ingress.
func UpgradeIngress(clientset *kubernetes.Clientset,
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
			ing, err := clientset.ExtensionsV1beta1().Ingresses(n.Namespace).Update(n)
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
			ing, err := clientset.ExtensionsV1beta1().Ingresses(n.Namespace).Create(n)
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
			if err := clientset.ExtensionsV1beta1().Ingresses(ing.Namespace).Delete(ing.Name,
				&metav1.DeleteOptions{}); err != nil {
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
func UpgradeSecrets(clientset *kubernetes.Clientset,
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
			sec, err := clientset.CoreV1().Secrets(n.Namespace).Update(n)
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
			sec, err := clientset.CoreV1().Secrets(n.Namespace).Create(n)
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
			if err := clientset.CoreV1().Secrets(sec.Namespace).Delete(sec.Name, &metav1.DeleteOptions{}); err != nil {
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

// UpgradeEndpoints is used to update *corev1.Endpoints.
func UpgradeEndpoints(clientset *kubernetes.Clientset,
	as *v1.AppService, old, new []*corev1.Endpoints,
	handleErr func(msg string, err error) error) error {
	var oldMap = make(map[string]*corev1.Endpoints, len(old))
	for i, item := range old {
		oldMap[item.Name] = old[i]
	}
	for _, n := range new {
		if o, ok := oldMap[n.Name]; ok {
			ep, err := clientset.CoreV1().Endpoints(n.Namespace).Update(n)
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
			_, err := clientset.CoreV1().Endpoints(n.Namespace).Create(n)
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
			if err := clientset.CoreV1().Endpoints(sec.Namespace).Delete(sec.Name, &metav1.DeleteOptions{}); err != nil {
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

// UpdateEndpoints uses clientset to update the given Endpoints.
func UpdateEndpoints(ep *corev1.Endpoints, clientSet *kubernetes.Clientset) {
	_, err := clientSet.CoreV1().Endpoints(ep.Namespace).Update(ep)
	if err != nil {
		logrus.Warningf("error updating endpoints: %+v; err: %v", ep, err)
		return
	}
	logrus.Debugf("Key: %s/%s; Successfully update endpoints", ep.GetNamespace(), ep.GetName())
}
