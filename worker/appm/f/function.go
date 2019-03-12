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
		ensureEndpoints(ep, clientset)
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
	// delete delEndpoints
	for _, ep := range app.GetDelEndpoints() {
		err := clientset.CoreV1().Endpoints(ep.Namespace).Delete(ep.Name, &metav1.DeleteOptions{})
		if err != nil {
			// don't return error, hope it is ok next time
			logrus.Warningf("error deleting endpoints(%v): %v", ep, err)
			continue
		}
		logrus.Debugf("successfully deleted endpoints(%v)", ep)
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

func ensureEndpoints(ep *corev1.Endpoints, clientSet kubernetes.Interface) {
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

// ConvRbdEndpoint converts RbdEndpoint to RbdEndpoints.
func ConvRbdEndpoint(eps []*v1.RbdEndpoint) ([]*v1.RbdEndpoints, error) {
	var res []*v1.RbdEndpoints
	m := make(map[int]*v1.RbdEndpoints)
	for _, ep := range eps {
		if !ep.IsOnline {
			continue
		}
		v1ep, ok := m[ep.Port] // the value of port may be 0
		if ok {
			if ep.Status == "unhealty" {
				v1ep.NotReadyIPs = append(v1ep.NotReadyIPs, ep.IP)
			} else {
				v1ep.IPs = append(v1ep.IPs, ep.IP)
			}
			continue
		}
		v1ep = &v1.RbdEndpoints{
			Port: ep.Port,
		}
		if ep.Status == "unhealty" {
			v1ep.NotReadyIPs = append(v1ep.NotReadyIPs, ep.IP)
		} else {
			v1ep.IPs = append(v1ep.IPs, ep.IP)
		}
		m[ep.Port] = v1ep
		res = append(res, v1ep)
	}
	if !checkRbdEndpoints(res) {
		return nil, fmt.Errorf("Invalid endpoints: if the port has three different values, one of them cannot be 0")
	}

	return res, nil
}

// If the port has three different values, one of them cannot be 0
func checkRbdEndpoints(rbdEndpoints []*v1.RbdEndpoints) bool {
	if len(rbdEndpoints) < 2 {
		return true
	}
	for _, item := range rbdEndpoints {
		if item.Port == 0 {
			return false
		}
	}
	return true
}
